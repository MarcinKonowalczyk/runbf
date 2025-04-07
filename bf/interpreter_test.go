package bf_test

import (
	"testing"

	"github.com/MarcinKonowalczyk/runbf/bf"
	"github.com/MarcinKonowalczyk/runbf/utils"
)

func TestInterpreter_OutputEmptyInterpreter(t *testing.T) {
	program := []bf.Command{bf.Output}
	interpreter := bf.NewInterpreter(program, nil, nil, false)
	interpreter.Run()
}

func TestInterpreter_InputEmptyInterpreter(t *testing.T) {
	program := []bf.Command{bf.Input}
	interpreter := bf.NewInterpreter(program, nil, nil, false)
	interpreter.Run()
}

func TestInterpreter_Increment(t *testing.T) {
	program := []bf.Command{bf.Increment}
	interpreter := bf.NewInterpreter(program, nil, nil, false)
	utils.AssertEqual(t, interpreter.At(0), 0)
	interpreter.Run()
	utils.AssertEqual(t, interpreter.At(0), 1)
}

func TestInterpreter_Decrement(t *testing.T) {
	program := []bf.Command{bf.Decrement}
	interpreter := bf.NewInterpreter(program, nil, nil, false)
	utils.AssertEqual(t, interpreter.At(0), 0)
	interpreter.Run()
	utils.AssertEqual(t, interpreter.At(0), 255)
}

func TestInterpreter_MoveRight(t *testing.T) {
	program := []bf.Command{bf.Right, bf.Increment}
	interpreter := bf.NewInterpreter(program, nil, nil, false)
	utils.AssertEqual(t, interpreter.At(0), 0)
	utils.AssertEqual(t, interpreter.At(1), 0)
	interpreter.Run()
	utils.AssertEqual(t, interpreter.At(0), 0)
	utils.AssertEqual(t, interpreter.At(1), 1)
}

func TestInterpreter_MoveLeft(t *testing.T) {
	program := []bf.Command{bf.Left, bf.Increment}
	interpreter := bf.NewInterpreter(program, nil, nil, false)
	utils.AssertEqual(t, interpreter.At(0), 0)
	utils.AssertEqual(t, interpreter.At(-1), 0)
	interpreter.Run()
	utils.AssertEqual(t, interpreter.At(0), 0)
	utils.AssertEqual(t, interpreter.At(-1), 1)
}

func TestInterpreter_Loop(t *testing.T) {
	// +++[->+<]
	program := []bf.Command{
		bf.Increment,
		bf.Increment,
		bf.Increment,
		bf.LoopStart,
		bf.Decrement,
		bf.Right,
		bf.Increment,
		bf.Left,
		bf.LoopEnd,
	}
	interpreter := bf.NewInterpreter(program, nil, nil, false)
	utils.AssertEqual(t, interpreter.At(0), 0)
	utils.AssertEqual(t, interpreter.At(1), 0)
	interpreter.Run()
	utils.AssertEqual(t, interpreter.At(0), 0)
	utils.AssertEqual(t, interpreter.At(1), 3)
}
