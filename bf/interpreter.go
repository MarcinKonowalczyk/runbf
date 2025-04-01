package bf

import (
	"io"
)

type Interpreter struct {
	program []Command
	program_ptr uint32
	mem   []uint8
	mem_ptr uint32
	input  io.Reader
	output io.StringWriter
}

func NewInterpreter(program []Command, input io.Reader, output io.StringWriter) *Interpreter {
	return &Interpreter{
		program: program,
		program_ptr: 0,
		mem:   make([]uint8, 30_000),
		mem_ptr: 0,
		input:   input,
		output:  output,
	}
}

func (i *Interpreter) Run() {
	for {
		c := i.program[i.program_ptr]
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
			i.output.WriteString(string(i.mem[i.mem_ptr]))
		case Input:
			buff := make([]byte, 1)
			b, err := i.input.Read(buff)
			if err != nil {
				panic(err)
			}
			if b == 0 {
				i.mem[i.mem_ptr] = 0
			} else if b == 1 {
				i.mem[i.mem_ptr] = buff[0]
			} else {
				panic("Input buffer is too large")
			}
		case LoopStart:
			v := i.mem[i.mem_ptr]
			if v == 0 {
				// Find the matching LoopEnd
				depth := 1
				for j := i.program_ptr + 1; j < uint32(len(i.program)); j++ {
					if i.program[j] == LoopStart {
						depth++
					} else if i.program[j] == LoopEnd {
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
					if i.program[j] == LoopEnd {
						depth++
					} else if i.program[j] == LoopStart {
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
		if i.program_ptr >= uint32(len(i.program)) {
			break
		}
	}
}