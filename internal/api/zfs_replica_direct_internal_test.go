package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
	"github.com/junkerderprovinz/bombvault/internal/zfsrepl"
)

// pipeConn is a host connection that notes the pipelines it is asked to run
// and runs none of them.
type pipeConn struct{ ran [][]string }

func (c *pipeConn) RunCapture(context.Context, ...string) (string, string, error) { return "", "", nil }

func (c *pipeConn) StreamCommand(context.Context, ...string) (io.ReadCloser, func() error, error) {
	return nil, nil, errors.New("pipeConn streams nothing")
}

func (c *pipeConn) RunWithStdin(context.Context, io.Reader, ...string) error {
	return errors.New("pipeConn streams nothing")
}

func (c *pipeConn) RunStreams(_ context.Context, _ io.Reader, _, _ io.Writer, args ...string) error {
	c.ran = append(c.ran, args)
	return errors.New("pipeConn runs no pipeline")
}

const directPub = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIBombVault bombvault"

// directRig is the replica rig on Unraid's layout: the data folder at
// /config, Host Data at /host/user, both served by shfs, so the data folder
// shows below Host Data too, where aliasPub is the key it holds. Both ends
// are reached over SSH, and the host's own connection can run pipelines. src
// and dst are what the engine was handed.
type directRig struct {
	*replicaRig
	conn     *pipeConn
	src, dst zfsrepl.End
}

func newDirectRig(t *testing.T, aliasPub string) *directRig {
	t.Helper()
	r := &directRig{replicaRig: newReplicaRig(t), conn: &pipeConn{}}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root = filepath.ToSlash(root)
	data, host := root+"/config", root+"/host/user"
	for dir, pub := range map[string]string{data: directPub, host + "/user/appdata/bombvault": aliasPub} {
		if err := os.MkdirAll(filepath.FromSlash(dir+"/ssh-replica"), 0o700); err != nil {
			t.Fatal(err)
		}
		for name, body := range map[string]string{"id_ed25519": "private", "id_ed25519.pub": pub + "\n"} {
			if err := os.WriteFile(filepath.FromSlash(dir+"/ssh-replica/"+name), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	esc := func(p string) string { return strings.ReplaceAll(p, " ", `\040`) }
	mountinfo := filepath.Join(root, "mountinfo")
	table := "36 25 0:44 /appdata/bombvault " + esc(data) + " rw,relatime - fuse.shfs shfs rw\n" +
		"37 25 0:44 / " + esc(host+"/user") + " rw,relatime - fuse.shfs shfs rw\n"
	if err := os.WriteFile(mountinfo, []byte(table), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(SetMountinfoPath(mountinfo))
	r.s.cfg.DataDir, r.s.cfg.HostMountRoot, r.s.cfg.HostSourceRoot = data, host, "/mnt"

	r.s.replica.hostEnd = func() (zfsrepl.End, error) { return zfsrepl.NewSSHEnd(zfs.NewSSHHost(r.conn), r.conn), nil }
	r.s.replica.serverEnd = func(store.ZFSReplicaServer, string) (zfsrepl.End, error) {
		return zfsrepl.NewSSHEnd(zfs.NewSSHHost(r.server), &pipeConn{}), nil
	}
	r.s.replica.run = func(_ context.Context, src, dst zfsrepl.End, _ zfsrepl.Entry) (zfsrepl.Result, error) {
		r.src, r.dst = src, dst
		return zfsrepl.Result{Snapshot: replicaSnap}, nil
	}
	return r
}

func (r *directRig) runOnce() {
	r.t.Helper()
	if _, m, _ := r.call(http.MethodPost, "/api/zfs/datasets/"+r.item.ID+"/replica/run", ""); m["ok"] != true {
		r.t.Fatalf("run = %v", m)
	}
	r.s.replica.work.Wait()
	if r.src == nil {
		r.t.Fatalf("the run stopped before the engine: %+v", r.lastRun())
	}
}

// RunCapture lets a replicaHost stand behind a zfs.SSHHost.
func (h *replicaHost) RunCapture(ctx context.Context, args ...string) (string, string, error) {
	out, err := h.Run(ctx, args)
	var ce *zfs.CmdError
	if errors.As(err, &ce) {
		return out, ce.Stderr, err
	}
	return out, "", err
}

func TestTheHostStreamsToAZFSServerWithTheKeyAndPinHostDataShows(t *testing.T) {
	r := newDirectRig(t, directPub)
	r.runOnce()
	p, ok := r.src.(zfsrepl.Piper)
	if !ok {
		t.Fatal("the engine got a source that cannot stream to the server itself")
	}
	if _, piped, _ := p.Pipe(context.Background(), []string{"zfs", "send", "a@b"}, r.dst, []string{"zfs", "receive", "t"}, nil); piped {
		t.Fatal("a host whose check failed piped anyway")
	}
	if len(r.conn.ran) != 1 {
		t.Fatalf("the host ran %q, want one check", r.conn.ran)
	}
	key := "/mnt/user/appdata/bombvault/ssh-replica/id_ed25519"
	pin := "/mnt/user/appdata/bombvault/ssh-replica/hosts/" + r.srv.ID + "/known_hosts"
	if check := r.conn.ran[0]; !slices.Contains(check, key) || !slices.Contains(check, pin) || !slices.Contains(check, directPub) || !slices.Contains(check, "root@backup.lan") {
		t.Errorf("the check ran %q, want the key %s and the pin %s", check, key, pin)
	}
}

func TestARunStreamsThroughBombVaultWhenHostDataDoesNotShowTheKey(t *testing.T) {
	for _, c := range []struct {
		name     string
		aliasPub string
		noMount  bool
	}{
		{"another key below Host Data", "ssh-ed25519 AAAAsomeoneelse bombvault", false},
		{"no mount that shows the data folder", directPub, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := newDirectRig(t, c.aliasPub)
			if c.noMount {
				t.Cleanup(SetMountinfoPath(filepath.Join(t.TempDir(), "none")))
			}
			r.runOnce()
			if _, ok := r.src.(zfsrepl.Piper); ok {
				t.Fatal("the host was handed a key folder that is not this instance's")
			}
		})
	}
}
