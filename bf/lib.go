package bf

import (
	"context"
	"io"
	"os"
)

func RunContext(ctx context.Context, source string, input io.Reader, output io.StringWriter) {
	source = PreLex(source)
	lexer := NewLexer(source)

	commands := lexer.Lex()

	interpreter := NewInterpreter(commands, input, output, false)
	interpreter.RunContext(ctx)
}

func Run(source string) {
	RunContext(context.Background(), source, os.Stdin, os.Stdout)
}
