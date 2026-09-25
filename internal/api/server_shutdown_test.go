package api_test

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"testing"
	"testing/fstest"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
)

// A request can wait on work only the shutdown ends, such as an MCP listing of
// a repository that stopped answering. The server has to end that work before
// it waits for its requests, or every such stop runs out the grace and the
// process exits with an error.
func TestShutdownEndsDetachedWorkBeforeWaitingForRequests(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}

	release := make(chan struct{})
	entered := make(chan struct{})
	router := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		w.WriteHeader(http.StatusOK)
	})
	srv := api.NewServer(config.Config{HTTPOnly: true, Port: port}, fstest.MapFS{}, router)
	srv.BeforeShutdown = func() { close(release) }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx) }()

	url := "http://127.0.0.1:" + strconv.Itoa(port) + "/api/slow"
	go func() {
		for {
			resp, err := http.Get(url) //nolint:gosec // G107: a local test server
			if err == nil {
				_ = resp.Body.Close()
				return
			}
			select {
			case <-release:
				return
			case <-time.After(20 * time.Millisecond):
			}
		}
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the request never reached the handler")
	}

	start := time.Now()
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run = %v, want a clean stop", err)
	}
	if took := time.Since(start); took > time.Second {
		t.Fatalf("the stop took %s, so it waited for the request instead of releasing it", took)
	}
}
