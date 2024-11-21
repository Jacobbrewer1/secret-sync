package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func getRootContext() context.Context {
	// Listen for ctrl+c and kill signals
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		got := <-sig
		slog.Info("Received signal, shutting down", slog.String("signal", got.String()))
		cancel()
	}()

	return ctx
}
