package foobar

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"

	// "github.com/Microsoft/hcsshim/cmd/containerd-shim-runhcs-v1/options"

	taskAPI "github.com/containerd/containerd/api/runtime/task/v2"
	apitypes "github.com/containerd/containerd/api/types"
	tasktypes "github.com/containerd/containerd/api/types/task"
	"github.com/containerd/containerd/protobuf"
	"github.com/containerd/containerd/v2/core/snapshots"
	ptypes "github.com/containerd/containerd/v2/pkg/protobuf/types"
	"github.com/containerd/containerd/v2/pkg/shim"
	"github.com/containerd/containerd/v2/pkg/shutdown"
	"github.com/containerd/containerd/v2/plugins"
	"github.com/containerd/errdefs"
	"github.com/containerd/fifo"
	"github.com/containerd/log"
	"google.golang.org/protobuf/types/known/anypb"

	// _ "shim/snapshotter" // Register the snapshotter plugin

	"github.com/containerd/plugin"
	"github.com/containerd/plugin/registry"
	"github.com/containerd/ttrpc"
)

// https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_21_18
const exitCodeSignal = 128
const initPidFile = "myexample.pid"


func init() {
	registry.Register(&plugin.Registration{
		Type: plugins.TTRPCPlugin,
		ID:   "task",
		Requires: []plugin.Type{
			// plugins.EventPlugin,
			// plugins.SnapshotPlugin,
			plugins.InternalPlugin,
		},
		InitFn: func(ic *plugin.InitContext) (interface{}, error) {
			ss, err := ic.GetByID(plugins.InternalPlugin, "shutdown")
			if err != nil {
				return nil, err
			}
			// sh, err := ic.GetByID(plugins.SnapshotPlugin, "native")
			// if err != nil {
			// 	return nil, err
			// }
			return newTaskService(ic.Context, ss.(shutdown.Service))
		},
	})
}

type ErrNotImplementedMsg struct {
	Msg string
}

func NewErrNotImplementedMsg(msg string) *ErrNotImplementedMsg {
	return &ErrNotImplementedMsg{
		Msg: msg,
	}
}

func (e *ErrNotImplementedMsg) Error() string {
	return "not implemented: " + e.Msg
}

func NewManager(name string) shim.Manager {
	return manager{name: name}
}

type manager struct {
	name string
}

func (m manager) Name() string {
	return m.name
}

// // BootstrapParams is a JSON payload returned in stdout from shim.Start call.
// type BootstrapParams struct {
// 	// Version is the version of shim parameters (expected 2 for shim v2)
// 	Version int `json:"version"`
// 	// Address is a address containerd should use to connect to shim.
// 	Address string `json:"address"`
// 	// Protocol is either TTRPC or GRPC.
// 	Protocol string `json:"protocol"`
// }

func (m manager) Start(ctx context.Context, id string, opts shim.StartOpts) (retShim shim.BootstrapParams, retErr error) {
	// log.G(ctx).Infof("Starting shim for container %s", id)

	self, err := os.Executable()
	if err != nil {
		return retShim, fmt.Errorf("getting executable of current process: %w", err)
	}
	
	cwd, err := os.Getwd()
	if err != nil {
		return retShim, fmt.Errorf("getting current working directory: %w", err)
	}

	var args []string
	if opts.Debug {
		args = append(args, "-debug")
	}

	cmdCfg := &shim.CommandConfig{
		Runtime:      self,
		Address:      opts.Address,
		TTRPCAddress: opts.TTRPCAddress,
		Path:         cwd,
		Args:         args,
	}

	cmd, err := shim.Command(ctx, cmdCfg)
	if err != nil {
		return retShim, fmt.Errorf("creating shim command: %w", err)
	}
	// fmt.Println("cmd:", cmd)

	sockAddr, err := shim.SocketAddress(ctx, opts.Address, id, opts.Debug)
	if err != nil {
		return retShim, fmt.Errorf("getting a socket address: %w", err)
	}

	socket, err := shim.NewSocket(sockAddr)
	if err != nil {
		return retShim, fmt.Errorf("creating socket: %w", err)
	}
	
	sockF, err := socket.File()
	if err != nil {
		return retShim, fmt.Errorf("getting shim socket file descriptor: %w", err)
	}

	cmd.ExtraFiles = append(cmd.ExtraFiles, sockF)

	runtime.LockOSThread()

	// if cmdCfg.SchedCore {
	// 	if err := schedcore.Create(schedcore.ProcessGroup); err != nil {
	// 		return "", fmt.Errorf("enabling sched core support: %w", err)
	// 	}
	// }

	if err := cmd.Start(); err != nil {
		sockF.Close()
		return retShim, fmt.Errorf("starting shim command: %w", err)
	}

	runtime.UnlockOSThread()

	go func() {
		if err := cmd.Wait(); err != nil {
			if _, ok := err.(*exec.ExitError); !ok {
				log.G(ctx).WithError(err).Errorf("failed to wait for shim process %d", cmd.Process.Pid)
			}
		}
	}()

	if err := shim.AdjustOOMScore(cmd.Process.Pid); err != nil {
		return retShim, fmt.Errorf("adjusting shim process OOM score: %w", err)
	}

	retShim = shim.BootstrapParams{
		Version: 2,
		Address:  sockAddr,
		Protocol: "ttrpc",
	}

	return retShim, nil
}


func (m manager) Stop(ctx context.Context, id string) (shim.StopStatus, error) {
	log.G(ctx).Infof("Stopping shim for container %s", id)

	cwd, err := os.Getwd()
	if err != nil {
		return shim.StopStatus{}, fmt.Errorf("getting current working directory: %w", err)
	}

	pidPath := filepath.Join(filepath.Join(filepath.Dir(cwd), id), initPidFile)
	pid, err := readPidFile(pidPath)
	if err != nil {
		log.G(ctx).WithError(err).Warn("failed to read init pid file")
	}

	if pid > 0 {
		p, _ := os.FindProcess(pid)
		// The POSIX standard specifies that a null-signal can be sent to check
		// whether a PID is valid.
		if err := p.Signal(syscall.Signal(0)); err == nil {
			if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
				log.G(ctx).WithError(err).Warnf("failed to send kill syscall to init process %d", pid)
			}
		}
	}

	return shim.StopStatus{
		Pid: pid,
		ExitedAt: time.Now(),
		ExitStatus: int(exitCodeSignal + syscall.SIGKILL),
	}, nil
}

func readPidFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return -1, err
	}
	return strconv.Atoi(string(data))
}

func (m manager) Info(ctx context.Context, optionsR io.Reader) (*apitypes.RuntimeInfo, error) {
	info := &apitypes.RuntimeInfo{
		Name: m.name,
		Version: &apitypes.RuntimeVersion{
			Version: "v1.1.0",
		},
	}
	return info, nil
}

func newTaskService(ctx context.Context, sd shutdown.Service) (taskAPI.TaskService, error) {
	// The shim.Publisher and shutdown.Service are usually useful for your task service,
	// but we don't need them in the exampleTaskService.
	return &exampleTaskService{
		procs: make(initProcByTaskID, 1),
		ss: sd,
	}, nil
}

var (
	_ = shim.TTRPCService(&exampleTaskService{})
)

// initProcByTaskID maps init (parent) processes to their associated task by ID.
type initProcByTaskID map[string]*initProcess

// initProcess encapsulates information about an init (parent) process.
type initProcess struct {
	pid int

	doneCtx    context.Context
	exitTime   time.Time
	exitStatus int

	stdout string
}

func (pid *initProcess) String() string {
	return fmt.Sprintf("pid:%d, exitTime:%s, exitStatus:%d", pid.pid, pid.exitTime.Format(time.RFC3339), pid.exitStatus)
}


type exampleTaskService struct {
	m sync.RWMutex
	procs initProcByTaskID

	ss shutdown.Service
	sh snapshots.Snapshotter
}

// RegisterTTRPC allows TTRPC services to be registered with the underlying server
func (s *exampleTaskService) RegisterTTRPC(server *ttrpc.Server) error {
	taskAPI.RegisterTaskService(server, s)
	return nil
}

const configFilename = "config.json"

type root struct {
	// Path is the path to the rootfs
	Path string `json:"path"`
}

type process struct {
	// Args is the command to run
	Args []string `json:"args"`
	// Env is the environment variables to set
	// Env []string `json:"env"`
	// Cwd is the working directory
	// Cwd string `json:"cwd"`
}

type Config struct {
	// RootPath is the path to the rootfs
	Root root `json:"root"`
	Process process `json:"process"`
}

// ReadOptions reads the option information from the path.
// When the file does not exist, ReadOptions returns nil without an error.
func ReadConfig(path string) (*Config, error) {
	filePath := filepath.Join(path, configFilename)
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

const brainfuck_bin = "/bf/brainfuck"

// Create a new container
func (s *exampleTaskService) Create(ctx context.Context, r *taskAPI.CreateTaskRequest) (_ *taskAPI.CreateTaskResponse, retErr error) {
	s.m.Lock()
	defer s.m.Unlock()

	if _, ok := s.procs[r.ID]; ok {
		return nil, errdefs.ErrAlreadyExists
	}

	log.G(ctx).Infof("r: %v", r)
	log.G(ctx).Infof("Checkpoint: %s", r.Checkpoint)
	log.G(ctx).Infof("ParentCheckpoint: %s", r.ParentCheckpoint)

	// Read '.root.path' from the config file
	config, err := ReadConfig(r.Bundle)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}
	log.G(ctx).Infof("config: %+v", config)
	

	///////

	start_stopped_script := `
#!/bin/sh
kill -STOP $$
exec $@
	`

	start_stopped_script_path := filepath.Join(r.Bundle, "start-stopped.sh")
	if err := os.WriteFile(start_stopped_script_path, []byte(start_stopped_script), 0755); err != nil {
		return nil, fmt.Errorf("writing start-stopped.sh: %w", err)
	}

	///////

	script := config.Root.Path + config.Process.Args[0]



	///////

	// cmd := exec.CommandContext(ctx, "sh", "-c",
	// 	"while date --rfc-3339=seconds; do "+
	// 		"sleep 5; "+
	// 		"done",
	// )

	cmd := exec.CommandContext(ctx, "/bf/start-stopped.sh", brainfuck_bin, "-file", script)
	// cmd := exec.CommandContext(ctx, "/bf/start-stopped.sh", "/bf/brainfuck", "-file", "/bf/hello.bf")

	// pio, err := io.NewPipeIO(r.Stdout)
	// if err != nil {
	// 	return nil, fmt.Errorf("creating pipe io for stdout %s: %w", r.Stdout, err)
	// }

	// go func() {
	// 	if err := pio.Copy(ctx); err != nil {
	// 		log.G(ctx).WithError(err).Warn("failed to copy from stdout pipe")
	// 	}
	// }()
	
	ok, err := fifo.IsFifo(r.Stdout)
	if err != nil {
		return nil, fmt.Errorf("checking whether file %s is a fifo: %w", r.Stdout, err)
	}
	if !ok {
		return nil, fmt.Errorf("file %s is not a fifo", r.Stdout)
	}

	var fw io.WriteCloser
	// var fr io.Closer

	if fw, err = fifo.OpenFifo(ctx, r.Stdout, syscall.O_WRONLY, 0); err != nil {
		return nil, fmt.Errorf("opening write only fifo %s: %w", r.Stdout, err)
	}
	// defer fw.Close()

	cmd.Stdout = fw
	cmd.WaitDelay = 100 * time.Millisecond

	// Start the process (in a suspended state)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("running init command: %w", err)
	}

	pid := cmd.Process.Pid

	doneCtx, markDone := context.WithCancel(context.Background())

	ready_ch := make(chan struct{})
	go func(ready_ch chan struct{}) {
		ready_ch <- struct{}{}
		log.G(ctx).Infof("init process started: %d", pid)
		if err := cmd.Wait(); err != nil {
			if _, ok := err.(*exec.ExitError); !ok {
				log.G(ctx).WithError(err).Errorf("failed to wait for init process %d", pid)
			}
		}
		log.G(ctx).Infof("init process exited: %d", pid)

		// if err := pio.Close(); err != nil {
		// 	log.G(ctx).WithError(err).Error("failed to close stdout pipe io")
		// }

		exitStatus := 255

		if cmd.ProcessState != nil {
			switch unixWaitStatus := cmd.ProcessState.Sys().(syscall.WaitStatus); {
			case cmd.ProcessState.Exited():
				exitStatus = cmd.ProcessState.ExitCode()
			case unixWaitStatus.Signaled():
				exitStatus = exitCodeSignal + int(unixWaitStatus.Signal())
			}
		} else {
			log.G(ctx).Warn("init process wait returned without setting process state")
		}

		log.G(ctx).Infof("grabbing the lock")
		s.m.Lock()
		defer s.m.Unlock()
		log.G(ctx).Infof("grabbing the lock done")

		proc, ok := s.procs[r.ID]
		if !ok {
			log.G(ctx).WithError(err).Errorf("failed to write final status of done init process: task was removed")
		}

		proc.exitStatus = exitStatus
		proc.exitTime = time.Now()
		markDone()

		log.G(ctx).Infof("init process exited: %s", proc)

		// // Check if all the tasks have exited
		log.G(ctx).Infof("checking if all tasks have exited")
		log.G(ctx).Infof("all tasks: %+v", s.procs)
		all_exited := func() bool {
			for _, proc := range s.procs {
				if !(proc.doneCtx.Err() != nil) {
					return false
				}
			}
			return true
		}()

		if all_exited {
			log.G(ctx).Infof("all tasks exited. shutting down the shim")
			s.ss.Shutdown()
		}
		log.G(ctx).Infof("end of finalizer")
	}(ready_ch)
	
	<-ready_ch

	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting current working directory: %w", err)
	}

	// If containerd needs to resort to calling the shim's "delete" command to
	// clean things up, having the process' pid readable from a file is the
	// only way for it to know what init process is associated with the task.
	pidPath := filepath.Join(filepath.Join(filepath.Dir(cwd), r.ID), initPidFile)
	if err := shim.WritePidFile(pidPath, pid); err != nil {
		return nil, fmt.Errorf("writing pid file of init process: %w", err)
	}

	s.procs[r.ID] = &initProcess{
		pid:     pid,
		doneCtx: doneCtx,
		stdout:  r.Stdout,
	}

	return &taskAPI.CreateTaskResponse{
		Pid:     uint32(pid),
	},  nil
}

// Start the primary user process inside the container
func (s *exampleTaskService) Start(ctx context.Context, r *taskAPI.StartRequest) (*taskAPI.StartResponse, error) {
	log.G(ctx).Infof("start id:%s execid:%s", r.ID, r.ExecID)

	s.m.RLock()
	defer s.m.RUnlock()
	proc, ok := s.procs[r.ID]
	if !ok {
		return nil, fmt.Errorf("task not created: %w", errdefs.ErrNotFound)
	}
	log.G(ctx).Infof("proc: %+v", proc)

	cmd := exec.CommandContext(ctx, "kill", "-CONT", strconv.Itoa(proc.pid))
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting init command: %w", err)
	}

	return &taskAPI.StartResponse{
		Pid: uint32(proc.pid),
	}, nil
}

// Delete a process or container
func (s *exampleTaskService) Delete(ctx context.Context, r *taskAPI.DeleteRequest) (*taskAPI.DeleteResponse, error) {
	log.G(ctx).Infof("delete id:%s execid:%s", r.ID, r.ExecID)

	// return nil, NewErrNotImplementedMsg("Delete (task)")

	s.m.Lock()
	defer s.m.Unlock()

	proc, ok := s.procs[r.ID]
	if !ok {
		return nil, fmt.Errorf("task not created: %w", errdefs.ErrNotFound)
	}

	if proc.doneCtx.Err() != nil {
		log.G(ctx).Infof("task already exited: %s", r.ID)
		delete(s.procs, r.ID)
	}
	return &taskAPI.DeleteResponse{}, nil

	// // if proc.pid > 0 {
	// // 	p, _ := os.FindProcess(proc.pid)
	// // 	if err := p.Signal(syscall.Signal(0)); err == nil {
	// // 		log.G(ctx).Infof("delete id:%s execid:%s pid:%d", r.ID, r.ExecID, proc.pid)
	// // 		if err := p.Signal(syscall.SIGKILL); err != nil {
	// // 			return nil, fmt.Errorf("sending SIGKILL to init process: %w", err)
	// // 		}
	// // 	}
	// // }

	// // // Wait for the process to exit
	// // doneCtx, err := func() (context.Context, error) {
	// // 	s.m.RLock()
	// // 	defer s.m.RUnlock()
	// // 	proc, ok := s.procs[r.ID]
	// // 	if !ok {
	// // 		return nil, fmt.Errorf("task not created: %w", errdefs.ErrNotFound)
	// // 	}
	// // 	return proc.doneCtx, nil
	// // }()

	// // if err != nil {
	// // 	return nil, err
	// // }

	// // select {
	// // case <-ctx.Done():
	// // 	return nil, ctx.Err()
	// // case <-doneCtx.Done():
	// // }

	// return &taskAPI.DeleteResponse{
	// 	Pid:       uint32(proc.pid),
	// 	ExitStatus: uint32(proc.exitStatus),
	// 	ExitedAt:   protobuf.ToTimestamp(proc.exitTime),
	// }, nil
}

// Exec an additional process inside the container
func (s *exampleTaskService) Exec(ctx context.Context, r *taskAPI.ExecProcessRequest) (*ptypes.Empty, error) {
	log.G(ctx).Infof("exec id:%s execid:%s", r.ID, r.ExecID)
	return nil, NewErrNotImplementedMsg("Exec (task)")
}

// ResizePty of a process
func (s *exampleTaskService) ResizePty(ctx context.Context, r *taskAPI.ResizePtyRequest) (*ptypes.Empty, error) {
	log.G(ctx).Infof("resizepty id:%s execid:%s width:%d height:%d", r.ID, r.ExecID, r.Width, r.Height)
	// return nil, NewErrNotImplementedMsg("ResizePty (task)")
	return &ptypes.Empty{}, nil
}

// State returns runtime state of a process
func (s *exampleTaskService) State(ctx context.Context, r *taskAPI.StateRequest) (*taskAPI.StateResponse, error) {
	log.G(ctx).Infof("state id:%s execid:%s", r.ID, r.ExecID)

	s.m.RLock()
	defer s.m.RUnlock()
	proc, ok := s.procs[r.ID]
	if !ok {
		return nil, fmt.Errorf("task not created: %w", errdefs.ErrNotFound)
	}

	status := tasktypes.Status_RUNNING
	if proc.doneCtx.Err() != nil {
		status = tasktypes.Status_STOPPED
	}

	resp := &taskAPI.StateResponse{
		ID:         r.ID,
		Pid:        uint32(proc.pid),
		Status:     status,
		Stdout:     proc.stdout,
		ExitStatus: uint32(proc.exitStatus),
		ExitedAt:   protobuf.ToTimestamp(proc.exitTime),
	}
	log.G(ctx).Infof("state resp: %+v", resp)
	return resp, nil
}

// Pause the container
func (s *exampleTaskService) Pause(ctx context.Context, r *taskAPI.PauseRequest) (*ptypes.Empty, error) {
	log.G(ctx).Infof("pause id:%s", r.ID)
	return nil, NewErrNotImplementedMsg("Pause (task)")
}

// Resume the container
func (s *exampleTaskService) Resume(ctx context.Context, r *taskAPI.ResumeRequest) (*ptypes.Empty, error) {
	log.G(ctx).Infof("resume id:%s", r.ID)
	return nil, NewErrNotImplementedMsg("Resume (task)")
}

// Kill a process
func (s *exampleTaskService) Kill(ctx context.Context, r *taskAPI.KillRequest) (*ptypes.Empty, error) {
	already_exited, err := func() (bool, error) {
		s.m.RLock()
		defer s.m.RUnlock()

		proc, ok := s.procs[r.ID]
		if !ok {
			return false, fmt.Errorf("task not created: %w", errdefs.ErrNotFound)
		}

		// Check if the process is already done
		if proc.doneCtx.Err() != nil {
			return true, nil
		}

		log.G(ctx).Infof("proc: %+v", proc)

		if proc.pid > 0 {
			p, _ := os.FindProcess(proc.pid)
			// The POSIX standard specifies that a null-signal can be sent to check
			// whether a PID is valid.
			if err := p.Signal(syscall.Signal(0)); err == nil {
				log.G(ctx).Infof("kill id:%s execid:%s pid:%d sig:%d", r.ID, r.ExecID, proc.pid, r.Signal)
				// TODO: use the signal from the request
				// sig := syscall.Signal(r.Signal)
				sig := syscall.Signal(9)
				if err := p.Signal(sig); err != nil {
					return false, fmt.Errorf("sending %s to init process: %w", sig, err)
				}
			}
		}
		return false, nil
	}()

	if err != nil {
		log.G(ctx).WithError(err).Errorf("failed to send kill syscall to init process %s", r.ID)
		return nil, err
	}

	if already_exited {
		log.G(ctx).Warnf("task already exited: %s", r.ID)
	} else {

		grab_context := func(id string) (context.Context, error) {
			s.m.RLock()
			defer s.m.RUnlock()
			proc, ok := s.procs[id]
			if !ok {
				return nil, fmt.Errorf("task not created: %w", errdefs.ErrNotFound)
			}
			return proc.doneCtx, nil
		}

		doneCtx, err := grab_context(r.ID)
		if err != nil {
			return nil, err
		}
			
		log.G(ctx).Infof("waiting for process to exit")
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-doneCtx.Done():
		}
		log.G(ctx).Infof("process exited")

	}
	
	// // Check if all the tasks have exited
	// all_exited := func() bool {
	// 	s.m.RLock()
	// 	defer s.m.RUnlock()
	// 	for _, proc := range s.procs {
	// 		if !(proc.doneCtx.Err() != nil) {
	// 			return false
	// 		}
	// 	}
	// 	return true
	// }()

	// if all_exited {
	// 	log.G(ctx).Infof("all tasks exited. shutting down the shim")
	// 	s.ss.Shutdown()
	// }	

	return &ptypes.Empty{}, nil
}

// Pids returns all pids inside the container
func (s *exampleTaskService) Pids(ctx context.Context, r *taskAPI.PidsRequest) (*taskAPI.PidsResponse, error) {
	log.G(ctx).Infof("pids id:%s", r.ID)
	return nil, NewErrNotImplementedMsg("Pids (task)")
}

// CloseIO of a process
func (s *exampleTaskService) CloseIO(ctx context.Context, r *taskAPI.CloseIORequest) (*ptypes.Empty, error) {
	log.G(ctx).Infof("closeio id:%s execid:%s", r.ID, r.ExecID)
	return nil, NewErrNotImplementedMsg("CloseIO (task)")
}

// Checkpoint the container
func (s *exampleTaskService) Checkpoint(ctx context.Context, r *taskAPI.CheckpointTaskRequest) (*ptypes.Empty, error) {
	log.G(ctx).Infof("checkpoint id:%s", r.ID)
	return nil, NewErrNotImplementedMsg("Checkpoint (task)")
}

// Connect returns shim information of the underlying service
func (s *exampleTaskService) Connect(ctx context.Context, r *taskAPI.ConnectRequest) (*taskAPI.ConnectResponse, error) {
	log.G(ctx).Infof("connect id:%s", r.ID)
	s.m.RLock()
	defer s.m.RUnlock()

	proc, ok := s.procs[r.ID]
	if !ok {
		return nil, fmt.Errorf("task not created: %w", errdefs.ErrNotFound)
	}
	
	return &taskAPI.ConnectResponse{
		ShimPid: uint32(os.Getpid()),
		TaskPid: uint32(proc.pid),
	}, nil
}

// Shutdown is called after the underlying resources of the shim are cleaned up and the service can be stopped
func (s *exampleTaskService) Shutdown(ctx context.Context, r *taskAPI.ShutdownRequest) (*ptypes.Empty, error) {
	log.G(ctx).Infof("shutdown id:%s", r.ID)
	
	s.ss.Shutdown()
	return &ptypes.Empty{}, nil
}

// Stats returns container level system stats for a container and its processes
func (s *exampleTaskService) Stats(ctx context.Context, r *taskAPI.StatsRequest) (*taskAPI.StatsResponse, error) {
	// log.G(ctx).Infof("stats id:%s", r.ID)
	// return nil, NewErrNotImplementedMsg("Stats (task)")
	// return empty stats
	stats := &taskAPI.StatsResponse{
		Stats: &anypb.Any{},
	}
	return stats, nil

}

// Update the live container
func (s *exampleTaskService) Update(ctx context.Context, r *taskAPI.UpdateTaskRequest) (*ptypes.Empty, error) {
	log.G(ctx).Infof("update id:%s", r.ID)
	return nil, NewErrNotImplementedMsg("Update (task)")
}

// Wait for a process to exit
func (s *exampleTaskService) Wait(ctx context.Context, r *taskAPI.WaitRequest) (*taskAPI.WaitResponse, error) {
	log.G(ctx).Infof("wait id:%s", r.ID)
	
	doneCtx, err := func() (context.Context, error) {
		s.m.RLock()
		defer s.m.RUnlock()
		proc, ok := s.procs[r.ID]
		if !ok {
			return nil, fmt.Errorf("task not created: %w", errdefs.ErrNotFound)
		}
		return proc.doneCtx, nil
	}()
	if err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-doneCtx.Done():
	}

	s.m.RLock()
	defer s.m.RUnlock()
	proc, ok := s.procs[r.ID]
	if !ok {
		return nil, fmt.Errorf("task was removed: %w", errdefs.ErrNotFound)
	}

	return &taskAPI.WaitResponse{
		ExitStatus: uint32(proc.exitStatus),
		ExitedAt:   protobuf.ToTimestamp(proc.exitTime),
	}, nil
}
	
