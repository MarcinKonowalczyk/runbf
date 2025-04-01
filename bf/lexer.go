package bf


func pre_lex(input string) string {
	var result []rune
	for _, c := range input {
		if c == '+' || c == '-' || c == '>' || c == '<' || c == '.' || c == ',' || c == '[' || c == ']' {
			result = append(result, c)
		}
	}
	return string(result)
}

type Lexer struct {
	chars string
}

func NewLexer(input string) *Lexer {
	return &Lexer{
		chars: input,
	}
}
type Command rune

const (
	Increment Command = '+'
	Decrement Command = '-'
	Left      Command = '<'
	Right     Command = '>'
	Output    Command = '.'
	Input     Command = ','
	LoopStart Command = '['
	LoopEnd   Command = ']'
	Ignore    Command = ' '
)

func parse(c rune) Command {
	switch c {
	case '+':
		return Increment
	case '-':
		return Decrement
	case '>':
		return Right
	case '<':
		return Left
	case '.':
		return Output
	case ',':
		return Input
	case '[':
		return LoopStart
	case ']':
		return LoopEnd
	default:
		return Ignore
	}
}

func (c Command) String() string {
	switch c {
	case Increment:
		return "+"
	case Decrement:
		return "-"
	case Left:
		return "<"
	case Right:
		return ">"
	case Output:
		return "."
	case Input:
		return ","
	case LoopStart:
		return "["
	case LoopEnd:
		return "]"
	default:
		return " "
	}
}

func (l *Lexer) Lex() []Command {
	commands := []Command{}
	for _, c := range l.chars {
		cmd := parse(c)
		if cmd != Ignore {
			commands = append(commands, cmd)
		}
	}
	return commands
}
