//go:build linux

package hostsnap

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// localShell runs the scripts the way the host does, with stand-ins for the
// tools on the PATH.
type localShell struct{ path string }

func (l localShell) RunCapture(ctx context.Context, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Env = append(os.Environ(), "PATH="+l.path)
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	return strings.TrimSpace(out.String()), strings.TrimSpace(errb.String()), err
}

func writeTool(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body), 0o755); err != nil { //nolint:gosec // a test stand-in has to be executable
		t.Fatal(err)
	}
}

func toolsFor(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	logf := filepath.Join(dir, "zfs.log")
	writeTool(t, dir, "zpool", `case "$1" in
status) printf '  pool: tank\n config:\n\ttank\tONLINE\n\t  /dev/fakea1\tONLINE\n  pool: media\n config:\n\tmedia\tONLINE\n\t  /dev/fakeb1\tONLINE\n\tspares\n\t  /dev/fakec1\tAVAIL\n' ;;
list) printf 'tank\nmedia\n' ;;
esac
`)
	writeTool(t, dir, "hdparm", `case "$2" in
*fakeb*|*fakec*) printf '\n%s:\n drive state is:  standby\n' "$2" ;;
*) printf '\n%s:\n drive state is:  active/idle\n' "$2" ;;
esac
`)
	writeTool(t, dir, "zfs", `echo "$*" >> `+logf+`
case "$*" in
*"-t filesystem,volume"*) printf 'tank\tfilesystem\t/mnt/tank\t100\t1\n' ;;
*"-t snapshot"*) printf 'tank@one\t1700000000\t10\t90\t0\t-\n' ;;
esac
`)
	return dir + ":/usr/bin:/bin", logf
}

func TestTheSurveyLeavesASleepingPoolUnread(t *testing.T) {
	path, logf := toolsFor(t)
	s := New(localShell{path: path}, nil).Survey(context.Background(), false)
	states := map[string]Pool{}
	for _, p := range s.Pools {
		states[p.Name] = p
	}
	if states["tank"].State != StateAwake || !states["tank"].Read {
		t.Errorf("tank = %+v, want awake and read", states["tank"])
	}
	if states["media"].State != StateAsleep || states["media"].Read {
		t.Errorf("media = %+v, want asleep and not read", states["media"])
	}
	log, _ := os.ReadFile(logf) //nolint:gosec // the test's own file
	if strings.Contains(string(log), "media") {
		t.Errorf("zfs was asked about the sleeping pool: %s", log)
	}
	if len(s.Snapshots) != 1 || s.Snapshots[0].Name != "one" {
		t.Errorf("snapshots = %+v", s.Snapshots)
	}
}

func TestASpareThatSleepsDoesNotPutThePoolToSleep(t *testing.T) {
	path, _ := toolsFor(t)
	out, _, err := localShell{path: path}.RunCapture(context.Background(), "sh", "-c", surveyScript, "x", "0", "states")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "fakec") {
		t.Errorf("the spare was asked: %s", out)
	}
}

func TestReadingSleepingPoolsListsThemToo(t *testing.T) {
	path, logf := toolsFor(t)
	s := New(localShell{path: path}, nil).Survey(context.Background(), true)
	for _, p := range s.Pools {
		if !p.Read {
			t.Errorf("%s was not read", p.Name)
		}
	}
	log, _ := os.ReadFile(logf) //nolint:gosec // the test's own file
	if !strings.Contains(string(log), "tank media") {
		t.Errorf("zfs log %q, want both pools listed", log)
	}
}
