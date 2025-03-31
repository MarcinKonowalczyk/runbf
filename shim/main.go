package main

import (
	"context"
	"os/signal"
	"syscall"

	"shim/foobar"

	"github.com/containerd/containerd/v2/pkg/shim"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	shim.Run(ctx, foobar.NewManager("io.containerd.foobar.v1"))
}
