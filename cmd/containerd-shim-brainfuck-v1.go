package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	bf_shim "runbf/shim"

	"github.com/containerd/containerd/v2/pkg/shim"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	brainfuck := false
	for _, arg := range os.Args[1:] {
		if arg == "brainfuck" {
			brainfuck = true
			break
		}
	}

	if brainfuck {
		// Run as brainfuck interpreter
	} else {
		shim.Run(ctx, bf_shim.NewManager("io.containerd.bf.v1"))
	}
}
