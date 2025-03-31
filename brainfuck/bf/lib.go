package bf

import (
	"io"
)

func Run(source string, input io.Reader, output io.StringWriter) {
	source = pre_lex(source)
	lexer := NewLexer(source)

	commands := lexer.Lex()

	interpreter := NewInterpreter(commands, input, output)
	interpreter.Run()
}