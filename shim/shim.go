package shim

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
	"strings"
	"sync"
	"syscall"
	"time"

	taskAPI "github.com/containerd/containerd/api/runtime/task/v2"
	apitypes "github.com/containerd/containerd/api/types"
	tasktypes "github.com/containerd/containerd/api/types/task"
	"github.com/containerd/containerd/protobuf"
	ptypes "github.com/containerd/containerd/v2/pkg/protobuf/types"
	"github.com/containerd/containerd/v2/pkg/shim"
	"github.com/containerd/containerd/v2/pkg/shutdown"
	"github.com/containerd/containerd/v2/plugins"
	"github.com/containerd/errdefs"
	"github.com/containerd/fifo"
	"github.com/containerd/log"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/containerd/plugin"
	"github.com/containerd/plugin/registry"
	"github.com/containerd/ttrpc"
)

// https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_21_18
const exitCodeSignal = 128
const initPidFile = "bf.pid"

// comptime override for debug flag
// set with `-ldflags="-X 'github.com/MarcinKonowalczyk/runbf/shim.debug=true'"`
var debug string

func init() {
	registry.Register(&plugin.Registration{
		Type: plugins.TTRPCPlugin,
		ID:   "task",
		Requires: []plugin.Type{
			plugins.InternalPlugin,
		},
		InitFn: func(ic *plugin.InitContext) (interface{}, error) {
			ss, err := ic.GetByID(plugins.InternalPlugin, "shutdown")
			if err != nil {
				return nil, err
			}
			return newTaskService(ic.Context, ss.(shutdown.Service))
		},
	})
}

type bfManager struct {
	name string
}

func NewManager(name string) shim.Manager {
	return bfManager{name: name}
}

func (m bfManager) Name() string {
	return m.name
}

func (m bfManager) Start(ctx context.Context, id string, opts shim.StartOpts) (retShim shim.BootstrapParams, retErr error) {
	log.G(ctx).Debug("Start (manager)")

	self, err := os.Executable()
	if err != nil {
		return retShim, fmt.Errorf("getting executable of current process: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return retShim, fmt.Errorf("getting current working directory: %w", err)
	}

	var args []string
	if opts.Debug || debug != "" {
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

	// Start the shim command
	retErr = func() error {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		if err := cmd.Start(); err != nil {
			sockF.Close()
			return fmt.Errorf("starting shim command: %w", err)
		}
		return nil
	}()

	if retErr != nil {
		return retShim, retErr
	}

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
		Version:  2,
		Address:  sockAddr,
		Protocol: "ttrpc",
	}

	return retShim, nil
}

func (m bfManager) Stop(ctx context.Context, id string) (shim.StopStatus, error) {
	log.G(ctx).Debug("Stop (manager)")

	pid, err := readPidFile(id)
	if err != nil {
		return shim.StopStatus{}, fmt.Errorf("reading pid file: %w", err)
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
		Pid:        pid,
		ExitedAt:   time.Now(),
		ExitStatus: int(exitCodeSignal + syscall.SIGKILL),
	}, nil
}

func readPidFile(id string) (int, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return -1, fmt.Errorf("getting current working directory: %w", err)
	}
	path := filepath.Join(filepath.Join(filepath.Dir(cwd), id), initPidFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return -1, err
	}
	return strconv.Atoi(string(data))
}

// If containerd needs to resort to calling the shim's "stop" command to
// clean things up, having the process' pid readable from a file is the
// only way for it to know what init process is associated with the task.
func writePidFile(id string, pid int) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current working directory: %w", err)
	}

	path := filepath.Join(filepath.Join(filepath.Dir(cwd), id), initPidFile)
	if err := shim.WritePidFile(path, pid); err != nil {
		return fmt.Errorf("writing pid file of init process: %w", err)
	}

	// 644 == rw-r--r--
	// aka: owner can read/write, group/other can read
	if err := os.Chmod(path, 0644); err != nil {
		return fmt.Errorf("changing pid file permissions: %w", err)
	}
	if err := os.Chown(path, 0, 0); err != nil {

		return fmt.Errorf("changing pid file ownership: %w", err)
	}

	return nil
}

func (m bfManager) Info(ctx context.Context, optionsR io.Reader) (*apitypes.RuntimeInfo, error) {
	log.G(ctx).Debug("Info (manager)")
	info := &apitypes.RuntimeInfo{
		Name: m.name,
		Version: &apitypes.RuntimeVersion{
			Version: "v1.2.0",
		},
	}
	return info, nil
}

var (
	_ = shim.Manager(&bfManager{})
)

type proc struct {
	pid int

	done       context.Context
	exitTime   time.Time
	exitStatus int

	stdout string
	stdin  string
}

func (pid *proc) String() string {
	if pid.done.Err() != nil {
		return fmt.Sprintf("pid:%d, exitTime:%s, exitStatus:%d", pid.pid, pid.exitTime.Format(time.RFC3339), pid.exitStatus)
	} else {
		return fmt.Sprintf("pid:%d running", pid.pid)
	}
}

type bfTaskService struct {
	mu       sync.RWMutex
	procs    map[string]*proc
	shutdown shutdown.Service
}

func newTaskService(ctx context.Context, sd shutdown.Service) (taskAPI.TaskService, error) {
	return &bfTaskService{
		procs:    make(map[string]*proc, 1),
		shutdown: sd,
	}, nil
}

// RegisterTTRPC allows TTRPC services to be registered with the underlying server
func (s *bfTaskService) RegisterTTRPC(server *ttrpc.Server) error {
	taskAPI.RegisterTaskService(server, s)
	return nil
}

var (
	_ = shim.TTRPCService(&bfTaskService{})
)

func (s *bfTaskService) grab_context(id string) (context.Context, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	proc, ok := s.procs[id]
	if !ok {
		return nil, fmt.Errorf("task not created: %w", errdefs.ErrNotFound)
	}
	return proc.done, nil
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
	Env []string `json:"env"`
	// Cwd is the working directory
	// Cwd string `json:"cwd"`
}

type config struct {
	// RootPath is the path to the rootfs
	Root    root    `json:"root"`
	Process process `json:"process"`
}

type Config struct {
	Root       string
	Entrypoint string
	Path       []string
}

// /var/run/desktop-containerd/daemon/io.containerd.runtime.v2.task/moby/

// ReadOptions reads the option information from the path.
// When the file does not exist, ReadOptions returns nil without an error.
func ReadConfig(path string) (*Config, error) {
	filePath := filepath.Join(path, configFilename)
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file %s not found", configFilename)
		}
		return nil, err
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var config config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	if config.Root.Path == "" {
		return nil, fmt.Errorf("root path not found in config file %s", configFilename)
	}

	if len(config.Process.Args) != 1 {
		return nil, fmt.Errorf("incorrect number of args in the CMD. Expected 1, got %d", len(config.Process.Args))
	}

	arg0 := config.Process.Args[0]

	// check if the extension is .bf
	if !(filepath.Ext(arg0) == ".bf" || filepath.Ext(arg0) == ".brainfuck") {
		return nil, fmt.Errorf("entry point (%s) is not a .bf file", arg0)
	}

	// check if the script exists
	script := filepath.Join(config.Root.Path, arg0)
	if _, err := os.Stat(script); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("script %s does not exist: %w", arg0, err)
		}
		return nil, fmt.Errorf("checking script %s: %w", arg0, err)
	}

	// Get the PATH environment variable
	split_path := []string{}
	for _, env := range config.Process.Env {
		if env[0:5] == "PATH=" {
			// Split the PATH variable into a slice
			path := env[5:]
			split_path = strings.Split(path, ":")
			break
		}
	}

	return &Config{
		Root:       config.Root.Path,
		Entrypoint: arg0,
		Path:       split_path,
	}, nil
}

func (c *Config) FullPath() string {
	return filepath.Join(c.Root, c.Entrypoint)
}

type finalizer struct {
	done func()
	cmd  *exec.Cmd
	pid  int
	s    *bfTaskService
	id   string
}

func (fc *finalizer) schedule(ctx context.Context) {
	ready_ch := make(chan struct{})
	go finalize(ctx, ready_ch, fc.done, fc.cmd, fc.pid, fc.s, fc.id)
	<-ready_ch
}

func finalize(
	ctx context.Context,
	ready_ch chan struct{},
	done func(),
	cmd *exec.Cmd,
	pid int,
	s *bfTaskService,
	id string,
) {
	ready_ch <- struct{}{}

	log.G(ctx).Debug("finalizer (service)")
	if err := cmd.Wait(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			log.G(ctx).WithError(err).Errorf("failed to wait for init process %d", pid)
		}
	}
	log.G(ctx).Debugf("init process %d exited", pid)

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

	s.mu.Lock()
	defer s.mu.Unlock()

	proc, ok := s.procs[id]
	if !ok {
		log.G(ctx).Errorf("failed to write final status of done init process: task was removed")
	}

	proc.exitStatus = exitStatus
	proc.exitTime = time.Now()
	done()

	// Check if all the procs have exited
	all_exited := func() bool {
		for _, proc := range s.procs {
			if !(proc.done.Err() != nil) {
				return false
			}
		}
		return true
	}()

	if all_exited {
		log.G(ctx).Debug("all procs exited. shutting down the shim")
		s.shutdown.Shutdown()
	}
}

const start_stopped_script = `
#!/bin/sh
kill -STOP $$
exec $@
`

const command_wait_delay = 100 * time.Millisecond

// Create a new container
func (s *bfTaskService) Create(ctx context.Context, r *taskAPI.CreateTaskRequest) (_ *taskAPI.CreateTaskResponse, retErr error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.procs[r.ID]; ok {
		return nil, errdefs.ErrAlreadyExists
	}

	config, err := ReadConfig(r.Bundle)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	start_stopped_script_path := filepath.Join(r.Bundle, "start-stopped.sh")
	if err := os.WriteFile(start_stopped_script_path, []byte(start_stopped_script), 0755); err != nil {
		return nil, fmt.Errorf("writing start-stopped.sh: %w", err)
	}

	self, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("getting executable of current process: %w", err)
	}

	cmd := exec.CommandContext(ctx, "/bin/sh", start_stopped_script_path, self, "brainfuck", "-file", config.FullPath())

	// DEBUG script to run a long running process
	// cmd := exec.CommandContext(ctx, "sh", "-c",
	// 	"while date --rfc-3339=seconds; do "+
	// 		"sleep 5; "+
	// 		"done",
	// )

	// STDOUT
	ok, err := fifo.IsFifo(r.Stdout)
	if err != nil {
		return nil, fmt.Errorf("checking whether file %s is a fifo: %w", r.Stdout, err)
	}
	if !ok {
		return nil, fmt.Errorf("file %s is not a fifo", r.Stdout)
	}

	var fw io.WriteCloser
	if fw, err = fifo.OpenFifo(ctx, r.Stdout, syscall.O_WRONLY, 0); err != nil {
		return nil, fmt.Errorf("opening write only fifo %s: %w", r.Stdout, err)
	}

	stdout_pipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("getting stdout pipe: %w", err)
	}

	// Connect the stdout pipe to the fifo
	go func() {
		if _, err := io.Copy(fw, stdout_pipe); err != nil {
			log.G(ctx).WithError(err).Errorf("failed to copy stdout pipe to fifo %s", r.Stdout)
		}
	}()

	// STDIN

	ok, err = fifo.IsFifo(r.Stdin)
	if err != nil {
		return nil, fmt.Errorf("checking whether file %s is a fifo: %w", r.Stdin, err)
	}
	if !ok {
		return nil, fmt.Errorf("file %s is not a fifo", r.Stdin)
	}
	var fr io.ReadCloser
	if fr, err = fifo.OpenFifo(ctx, r.Stdin, syscall.O_RDONLY, 0); err != nil {
		return nil, fmt.Errorf("opening read only fifo %s: %w", r.Stdin, err)
	}

	stdin_pipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("getting stdin pipe: %w", err)
	}

	// Connect the stdin pipe to the fifo
	go func() {
		if _, err := io.Copy(stdin_pipe, fr); err != nil {
			log.G(ctx).WithError(err).Errorf("failed to copy fifo %s to stdin pipe", r.Stdin)
		}
	}()

	// STDERR
	stderr := r.Stderr
	if stderr == "" {
		stderr = r.Stdout
	}

	ok, err = fifo.IsFifo(stderr)
	if err != nil {
		return nil, fmt.Errorf("checking whether file %s is a fifo: %w", stderr, err)
	}
	if !ok {
		return nil, fmt.Errorf("file %s is not a fifo", stderr)
	}

	stderr_pipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("getting stderr pipe: %w", err)
	}

	var fe io.WriteCloser
	if fe, err = fifo.OpenFifo(ctx, stderr, syscall.O_WRONLY, 0); err != nil {
		return nil, fmt.Errorf("opening read only fifo %s: %w", stderr, err)
	}

	// Connect the stderr pipe to the fifo
	go func() {
		if _, err := io.Copy(fe, stderr_pipe); err != nil {
			log.G(ctx).WithError(err).Errorf("failed to copy stderr pipe to fifo %s", stderr)
		}
	}()

	cmd.WaitDelay = command_wait_delay

	// Start the process (in a suspended state)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("running init command: %w", err)
	}

	pid := cmd.Process.Pid

	doneCtx, mark_done := context.WithCancel(context.Background())

	finalizer := &finalizer{
		done: mark_done,
		cmd:  cmd,
		pid:  pid,
		s:    s,
		id:   r.ID,
	}

	finalizer.schedule(ctx)

	writePidFile(r.ID, pid)

	s.procs[r.ID] = &proc{
		pid:    pid,
		done:   doneCtx,
		stdout: r.Stdout,
		stdin:  r.Stdin,
	}

	return &taskAPI.CreateTaskResponse{
		Pid: uint32(pid),
	}, nil
}

// Start the primary user process inside the container
func (s *bfTaskService) Start(ctx context.Context, r *taskAPI.StartRequest) (*taskAPI.StartResponse, error) {
	log.G(ctx).Debug("start (service)")

	s.mu.RLock()
	defer s.mu.RUnlock()
	proc, ok := s.procs[r.ID]
	if !ok {
		return nil, fmt.Errorf("task not created: %w", errdefs.ErrNotFound)
	}

	cmd := exec.CommandContext(ctx, "kill", "-CONT", strconv.Itoa(proc.pid))
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting init command: %w", err)
	}

	return &taskAPI.StartResponse{
		Pid: uint32(proc.pid),
	}, nil
}

// Delete a process or container
func (s *bfTaskService) Delete(ctx context.Context, r *taskAPI.DeleteRequest) (*taskAPI.DeleteResponse, error) {
	log.G(ctx).Debug("delete (service)")

	s.mu.Lock()
	defer s.mu.Unlock()

	proc, ok := s.procs[r.ID]
	if !ok {
		return nil, fmt.Errorf("task not created: %w", errdefs.ErrNotFound)
	}

	if proc.done.Err() != nil {
		delete(s.procs, r.ID)
	} else {
		return nil, errdefs.ErrFailedPrecondition.WithMessage(fmt.Sprintf("init process %d is not done yet", proc.pid))
	}

	return &taskAPI.DeleteResponse{
		Pid:        uint32(proc.pid),
		ExitStatus: uint32(proc.exitStatus),
		ExitedAt:   protobuf.ToTimestamp(proc.exitTime),
	}, nil
}

// Exec an additional process inside the container
func (s *bfTaskService) Exec(ctx context.Context, r *taskAPI.ExecProcessRequest) (*ptypes.Empty, error) {
	log.G(ctx).Debug("exec (service)")
	return nil, errdefs.ErrNotImplemented.WithMessage("Exec (task)")
}

// ResizePty of a process
func (s *bfTaskService) ResizePty(ctx context.Context, r *taskAPI.ResizePtyRequest) (*ptypes.Empty, error) {
	log.G(ctx).Debug("resizepty (service)")
	return &ptypes.Empty{}, nil
}

// State returns runtime state of a process
func (s *bfTaskService) State(ctx context.Context, r *taskAPI.StateRequest) (*taskAPI.StateResponse, error) {
	log.G(ctx).Debug("state (service)")

	s.mu.RLock()
	defer s.mu.RUnlock()
	proc, ok := s.procs[r.ID]
	if !ok {
		return nil, fmt.Errorf("task not created: %w", errdefs.ErrNotFound)
	}

	status := tasktypes.Status_RUNNING
	if proc.done.Err() != nil {
		status = tasktypes.Status_STOPPED
	}

	return &taskAPI.StateResponse{
		ID:         r.ID,
		Pid:        uint32(proc.pid),
		Status:     status,
		Stdout:     proc.stdout,
		Stdin:      proc.stdin,
		ExitStatus: uint32(proc.exitStatus),
		ExitedAt:   protobuf.ToTimestamp(proc.exitTime),
	}, nil
}

// Pause the container
func (s *bfTaskService) Pause(ctx context.Context, r *taskAPI.PauseRequest) (*ptypes.Empty, error) {
	log.G(ctx).Debug("pause (service)")
	return nil, errdefs.ErrNotImplemented.WithMessage("Pause (task)")
}

// Resume the container
func (s *bfTaskService) Resume(ctx context.Context, r *taskAPI.ResumeRequest) (*ptypes.Empty, error) {
	log.G(ctx).Debug("resume (service)")
	return nil, errdefs.ErrNotImplemented.WithMessage("Resume (task)")
}

// Kill a process
func (s *bfTaskService) Kill(ctx context.Context, r *taskAPI.KillRequest) (*ptypes.Empty, error) {
	log.G(ctx).Debug("kill (service)")

	already_exited, err := func() (bool, error) {
		s.mu.RLock()
		defer s.mu.RUnlock()

		proc, ok := s.procs[r.ID]
		if !ok {
			return false, fmt.Errorf("task not created: %w", errdefs.ErrNotFound)
		}

		// Check if the process is already done
		if proc.done.Err() != nil {
			return true, nil
		}

		if proc.pid > 0 {
			p, err := os.FindProcess(proc.pid)
			log.G(ctx).Debugf("kill id:%s execid:%s pid:%d sig:%d err:%v", r.ID, r.ExecID, proc.pid, r.Signal, err)
			// The POSIX standard specifies that a null-signal can be sent to check
			// whether a PID is valid.
			if err := p.Signal(syscall.Signal(0)); err == nil {
				// log.G(ctx).Debugf("kill id:%s execid:%s pid:%d sig:%d", r.ID, r.ExecID, proc.pid, r.Signal)
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
		done, err := s.grab_context(r.ID)
		if err != nil {
			return nil, err
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-done.Done():
		}
	}

	return &ptypes.Empty{}, nil
}

// Pids returns all pids inside the container
func (s *bfTaskService) Pids(ctx context.Context, r *taskAPI.PidsRequest) (*taskAPI.PidsResponse, error) {
	log.G(ctx).Debug("pids (service)")
	return nil, errdefs.ErrNotImplemented.WithMessage("Pids (task)")
}

// CloseIO of a process
func (s *bfTaskService) CloseIO(ctx context.Context, r *taskAPI.CloseIORequest) (*ptypes.Empty, error) {
	log.G(ctx).Debug("closeio (service)")
	return nil, errdefs.ErrNotImplemented.WithMessage("CloseIO (task)")
}

// Checkpoint the container
func (s *bfTaskService) Checkpoint(ctx context.Context, r *taskAPI.CheckpointTaskRequest) (*ptypes.Empty, error) {
	log.G(ctx).Debug("checkpoint (service)")
	return nil, errdefs.ErrNotImplemented.WithMessage("Checkpoint (task)")
}

// Connect returns shim information of the underlying service
func (s *bfTaskService) Connect(ctx context.Context, r *taskAPI.ConnectRequest) (*taskAPI.ConnectResponse, error) {
	log.G(ctx).Debug("connect (service)")
	s.mu.RLock()
	defer s.mu.RUnlock()

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
func (s *bfTaskService) Shutdown(ctx context.Context, r *taskAPI.ShutdownRequest) (*ptypes.Empty, error) {
	log.G(ctx).Debug("shutdown (service)")

	s.shutdown.Shutdown()
	return &ptypes.Empty{}, nil
}

// Stats returns container level system stats for a container and its processes
func (s *bfTaskService) Stats(ctx context.Context, r *taskAPI.StatsRequest) (*taskAPI.StatsResponse, error) {
	log.G(ctx).Debug("stats (service)")
	// return empty stats
	stats := &taskAPI.StatsResponse{
		Stats: &anypb.Any{},
	}
	return stats, nil

}

// Update the live container
func (s *bfTaskService) Update(ctx context.Context, r *taskAPI.UpdateTaskRequest) (*ptypes.Empty, error) {
	log.G(ctx).Debug("update (service)")
	return nil, errdefs.ErrAborted.WithMessage("Update (task)")
}

// Wait for a process to exit
func (s *bfTaskService) Wait(ctx context.Context, r *taskAPI.WaitRequest) (*taskAPI.WaitResponse, error) {
	log.G(ctx).Debug("wait (service)")

	done, err := s.grab_context(r.ID)
	if err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-done.Done():
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	proc, ok := s.procs[r.ID]
	if !ok {
		return nil, fmt.Errorf("task was removed: %w", errdefs.ErrNotFound)
	}

	return &taskAPI.WaitResponse{
		ExitStatus: uint32(proc.exitStatus),
		ExitedAt:   protobuf.ToTimestamp(proc.exitTime),
	}, nil
}
