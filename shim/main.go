package main

import (
	"context"
	"os/signal"
	"syscall"

	"shim/bf"

	"github.com/containerd/containerd/v2/pkg/shim"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	shim.Run(ctx, bf.NewManager("io.containerd.bf.v1"))
}
