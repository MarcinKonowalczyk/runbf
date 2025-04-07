package bf

import (
	"context"
	"fmt"
	"io"
	"os"
)

// comptime override for debug flag
// set with `-ldflags="-X 'github.com/MarcinKonowalczyk/runbf/bf.debug=true'"`
var debug string

type Interpreter struct {
	Program     []Command
	program_ptr uint32
	mem         []uint8
	mem_ptr     uint32
	Input       io.Reader
	Output      io.StringWriter
	debug       bool
}

func NewInterpreter(program []Command, input io.Reader, output io.StringWriter, debug bool) *Interpreter {
	return &Interpreter{
		Program:     program,
		program_ptr: 0,
		mem:         make([]uint8, 30_000),
		mem_ptr:     0,
		Input:       input,
		Output:      output,
		debug:       debug,
	}
}

func (i *Interpreter) Reset() {
	i.program_ptr = 0
	i.mem_ptr = 0
	for j := range i.mem {
		i.mem[j] = 0
	}
}

func (i *Interpreter) MemoryLength() int {
	return len(i.mem)
}

func wrap_index(i int32, N int32) int32 {
	for i > N {
		i -= N
	}
	for i < 0 {
		i += N
	}
	return i
}

// Index the memory
func (i *Interpreter) At(j int32) uint8 {
	return i.mem[wrap_index(j, int32(i.MemoryLength()))]
}

// Slice the memory
// func (i *Interpreter) Slice(start, end int32) []uint8 {
// 	N := int32(i.MemoryLength())
// 	// return i.mem[wrap_index(start, N):wrap_index(end, N)]
// }

// Write a debug message to stderr if debug is enabled
func logf(format string, args ...interface{}) {
	if debug != "" {
		fmt.Fprintf(os.Stderr, format, args...)
	}
}

// Run the program in a loop until it finishes or an error occurs
func (i *Interpreter) RunContext(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		c := i.Program[i.program_ptr]
		switch c {
		case Increment:
			i.mem[i.mem_ptr]++
		case Decrement:
			i.mem[i.mem_ptr]--
		case Right:
			i.mem_ptr++
			if i.mem_ptr >= uint32(len(i.mem)) {
				i.mem_ptr = 0
			}
		case Left:
			if i.mem_ptr == 0 {
				i.mem_ptr = uint32(len(i.mem) - 1)
			} else {
				i.mem_ptr--
			}
		case Output:
			if i.Output != nil {
				to_write := i.mem[i.mem_ptr]
				if to_write == '\n' {
					// Patch for Windows and, from some reason, docker
					i.Output.WriteString("\r\n")
				} else {
					i.Output.WriteString(string(to_write))
				}
			}
		case Input:
			if i.Input != nil {
				// read a byte from stdin
				buff := make([]byte, 1)
				_, err := i.Input.Read(buff)
				if err != nil {
					if err == io.EOF {
						logf("EOF")
						return
					}
					logf("Error reading input: %v", err)
					panic(err)
				}
				i.mem[i.mem_ptr] = buff[0]
			}
		case LoopStart:
			v := i.mem[i.mem_ptr]
			if v == 0 {
				// Find the matching LoopEnd
				depth := 1
				for j := i.program_ptr + 1; j < uint32(len(i.Program)); j++ {
					if i.Program[j] == LoopStart {
						depth++
					} else if i.Program[j] == LoopEnd {
						depth--
						if depth == 0 {
							i.program_ptr = j
							break
						}
					}
				}
			} else {
				// Continue to the next command
			}
		case LoopEnd:
			v := i.mem[i.mem_ptr]
			if v != 0 {
				// Find the matching LoopStart
				depth := 1
				for j := i.program_ptr - 1; j > 0; j-- {
					if i.Program[j] == LoopEnd {
						depth++
					} else if i.Program[j] == LoopStart {
						depth--
						if depth == 0 {
							i.program_ptr = j
							break
						}
					}
				}
			} else {
				// Continue to the next command
			}
		case Ignore:
			// Ignore the command
		default:
			panic("Unknown command")
		}
		i.program_ptr++
		if i.program_ptr >= uint32(len(i.Program)) {
			break
		}
	}
}

func (i *Interpreter) Run() {
	i.RunContext(context.Background())
}
