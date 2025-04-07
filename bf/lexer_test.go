package bf_test

import (
	"testing"

	"github.com/MarcinKonowalczyk/runbf/bf"
	"github.com/MarcinKonowalczyk/runbf/utils"
)

func TestPreLex(t *testing.T) {
	input := "++\n\n--<    >.,[hello sailor]"
	expected := "++--<>.,[]"
	result := bf.PreLex(input)
	utils.AssertEqual(t, result, expected)
}

func TestLex(t *testing.T) {
	input := "+-<>.,[]"
	expected := []bf.Command{
		bf.Increment,
		bf.Decrement,
		bf.Left,
		bf.Right,
		bf.Output,
		bf.Input,
		bf.LoopStart,
		bf.LoopEnd,
	}
	result := bf.Lex(input)
	utils.AssertEqualArrays(t, expected, result)
}
