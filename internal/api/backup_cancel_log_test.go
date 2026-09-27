package api_test

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// The cancel mark is gone by the time the started backup's goroutine logs how
// it ended, and a cancel read as a failure sends whoever reads the log looking
// for a fault.
func TestACancelledBackupIsLoggedAsCancelled(t *testing.T) {
	eng := &fakeResticEngine{block: make(chan struct{}), backupEntered: make(chan struct{}, 1)}
	h, _, svc, _ := newFilesTestRouter(t, eng)

	w, m := doJSON(t, h, http.MethodPost, "/api/files/sets", `{"name":"docs","path":"data/docs"}`)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("create: %d %v", w.Code, m)
	}
	id, _ := m["id"].(string)

	var out lockedBuffer
	prev, flags := log.Writer(), log.Flags()
	log.SetOutput(&out)
	t.Cleanup(func() { log.SetOutput(prev); log.SetFlags(flags) })

	if started, err := svc.StartBackupFileSet(context.Background(), id); err != nil || !started {
		t.Fatalf("backup should start: started=%v err=%v", started, err)
	}
	select {
	case <-eng.backupEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("backup never reached the engine")
	}
	if !svc.CancelBackupRun("files:docs", "") {
		t.Fatal("the running backup could not be cancelled")
	}
	waitForBackupDone(t, svc)

	var line string
	for _, l := range strings.Split(out.String(), "\n") {
		if strings.Contains(l, "api: backup file set:") {
			line = l
		}
	}
	if !strings.Contains(line, "cancelled") || strings.Contains(line, "failed") {
		t.Fatalf("the cancelled backup was logged as %q", line)
	}
}
