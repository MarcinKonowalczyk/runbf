package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/MarcinKonowalczyk/runbf/bf"
	bf_shim "github.com/MarcinKonowalczyk/runbf/shim"

	"github.com/containerd/containerd/v2/pkg/shim"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Maybe hijack the shim to run as brainfuck interpreter
	brainfuck, args := isBrainfuckArg(os.Args[1:])

	if brainfuck {
		err := runBrainfuck(ctx, args)
		if err != nil {
			fmt.Println("Error running brainfuck:", err)
		}
	} else {
		shim.Run(ctx, bf_shim.NewManager("io.containerd.bf.v1"))
	}
}

/////////////// 

var filename string

func isBrainfuckArg(args []string) (bool, []string) {
	for i, arg := range args {
		if arg == "brainfuck" {
			return true, append(args[:i], args[i+1:]...)
		}
	}
	return false, args
}

func parseBrainfuckFlags(args []string) error {
	my_flagset := flag.NewFlagSet("brainfuck", flag.ExitOnError)
	my_flagset.StringVar(&filename, "file", "", "brainfuck source file")
	return my_flagset.Parse(args)
}

func runBrainfuck(ctx context.Context, args []string) error {
	// Run as brainfuck interpreter
	if err := parseBrainfuckFlags(args); err != nil {
		return err
	}
	
	if filename == "" {
		return fmt.Errorf("invalid argument: -file is required")
	}
	
	source, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	// Run the brainfuck interpreter
	bf.RunContext(ctx, string(source), os.Stdin, os.Stdout)

	return nil
}