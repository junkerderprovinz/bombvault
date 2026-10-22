package remotes

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// fakeRcloneEnv makes the test binary stand in for rclone: the first call
// fails, every later one prints the recorded backends.
const fakeRcloneEnv = "BV_FAKE_RCLONE"

func TestMain(m *testing.M) {
	if dir := os.Getenv(fakeRcloneEnv); dir != "" {
		fakeRclone(dir)
		return
	}
	os.Exit(m.Run())
}

func fakeRclone(dir string) {
	mark := filepath.Join(dir, "failed-once")
	if _, err := os.Stat(mark); err != nil { //nolint:gosec // G703: dir is the test's own temp dir
		_ = os.WriteFile(mark, nil, 0o600) //nolint:gosec // G703: as above
		_, _ = os.Stderr.WriteString("ERROR : out of memory\n")
		os.Exit(1)
	}
	raw, err := os.ReadFile("testdata/rclone-1.75.1-providers.json")
	if err != nil {
		os.Exit(2)
	}
	_, _ = os.Stdout.Write(raw)
	os.Exit(0)
}

func fakeRcloneBinary(t *testing.T, failFirst bool) *Rclone {
	t.Helper()
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if !failFirst {
		if err := os.WriteFile(filepath.Join(dir, "failed-once"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(fakeRcloneEnv, dir)
	return &Rclone{Bin: bin, Config: filepath.Join(dir, "rclone.conf")}
}

func TestAFailedBackendListIsAskedForAgain(t *testing.T) {
	r := fakeRcloneBinary(t, true)

	if _, err := r.Backends(context.Background()); err == nil {
		t.Fatal("the first call succeeded, want the fake's failure")
	}
	backends, err := r.Backends(context.Background())
	if err != nil || backends["s3"].Name == "" {
		t.Fatalf("second call = %d backends, %v; want the list", len(backends), err)
	}
}

func TestACancelledCallerStillGetsTheBackendList(t *testing.T) {
	r := fakeRcloneBinary(t, false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if backends, err := r.Backends(ctx); err != nil || len(backends) == 0 {
		t.Fatalf("Backends = %d, %v; want the list although the caller went away", len(backends), err)
	}
}
