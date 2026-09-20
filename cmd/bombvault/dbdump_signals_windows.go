//go:build windows

package main

import (
	"context"
	"os"
	"os/signal"
)

// notifyHelperSignals cancels the dump on the one stop signal Windows delivers.
// The helper's real home is the Linux image; this keeps `go run` on a developer
// machine behaving the same way.
func notifyHelperSignals(cancel context.CancelFunc) func() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	go func() {
		<-ctx.Done()
		cancel()
	}()
	return stop
}
