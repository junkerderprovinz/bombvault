//go:build !windows

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// notifyHelperSignals cancels the dump when the process is asked to stop and
// keeps SIGPIPE from killing it outright: the dump goes to stdout, so a restic
// that closed its end would otherwise take the helper down before it can say
// why the dump ended.
func notifyHelperSignals(cancel context.CancelFunc) func() {
	pipes := make(chan os.Signal, 1)
	signal.Notify(pipes, syscall.SIGPIPE)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-ctx.Done()
		cancel()
	}()
	return func() {
		signal.Stop(pipes)
		stop()
	}
}
