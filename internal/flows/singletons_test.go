//go:build flows

package flows

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

// singleton is a section with one thing to back up, which prunes inline after
// each backup instead of in a batch.
type singleton struct {
	domain string
	tag    string
	backup string
	listed string
}

var singletons = []singleton{
	{domain: "flash", tag: "flash", backup: "/api/flash/backup", listed: "/api/flash/snapshots"},
	{domain: "config", tag: "config", backup: "/api/config/backup", listed: "/api/config/snapshots"},
}

func TestFlashAndSelfBackupPruneByTheirRule(t *testing.T) {
	t.Parallel()
	for _, sec := range singletons {
		t.Run(sec.domain, func(t *testing.T) {
			t.Parallel()
			for _, st := range starts {
				t.Run(st.name, func(t *testing.T) {
					t.Parallel()
					for _, pl := range ownFolderPlaces {
						t.Run(pl.name, func(t *testing.T) {
							t.Parallel()
							f := newFlow(t)
							st.boot(f, func() { f.own = pl.prepare(f.b, sec.domain) })
							f.flowSingleton(sec)
							f.finish()
						})
					}
				})
			}
		})
	}
}

func (f *flow) backupSingleton(sec singleton, round int) {
	f.t.Helper()
	// Both read a real file of their own that changes, so no backup is skipped
	// as unchanged.
	if sec.domain == "flash" {
		f.b.writeFlash(round)
	} else {
		f.b.changeSettings(func(s map[string]any) { s["instanceName"] = instanceName(round) })
	}
	f.b.call("POST", sec.backup, nil, nil)
	f.b.settle()
}

func instanceName(round int) string { return fmt.Sprintf("flows-%d", round) }

func (b *bombvault) writeFlash(round int) {
	b.t.Helper()
	p := filepath.Join(b.flash, "config", "go")
	if err := os.WriteFile(p, []byte("round "+instanceName(round)), 0o644); err != nil {
		b.t.Fatal(err)
	}
}

func (f *flow) flowSingleton(sec singleton) {
	f.t.Helper()
	rule := keep{Last: 1}
	f.own.card(rule)
	f.backupSingleton(sec, 0)
	f.agrees(sec.domain, rule, false)
	f.backupSingleton(sec, 1)
	f.wantSnapshots(f.own.repo, "", 1)

	var listed struct {
		Snapshots []resticSnapshot `json:"snapshots"`
	}
	f.b.call("GET", sec.listed, nil, &listed)
	if got, want := ids(listed.Snapshots), ids(f.b.snapshotsIn(f.own.repo, "")); !slices.Equal(got, want) {
		f.errorf("the API lists %v, restic %v", got, want)
	}

	section := keep{Last: 2}
	f.b.changeSettings(func(s map[string]any) { s["ownRetention"] = map[string]keep{sec.domain: section} })
	f.agrees(sec.domain, section, true)
	f.backupSingleton(sec, 2)
	f.wantSnapshots(f.own.repo, "", 2)
	f.b.restic(f.b.hostPath(f.own.repo), "check")
	f.wantNoFailedRuns()
}

func TestSelfBackupRestoreBringsBackTheSettings(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("BombVault ships for Linux, and restoring its own database does not complete on Windows")
	}
	t.Parallel()
	for _, st := range starts {
		t.Run(st.name, func(t *testing.T) {
			t.Parallel()
			f := newFlow(t)
			st.boot(f, func() { f.own = typedPath.prepare(f.b, "config") })
			sec := singletons[1]
			f.backupSingleton(sec, 0)
			sn := f.newestOf(f.own.repo, "")

			f.b.changeSettings(func(s map[string]any) { s["instanceName"] = "changed since" })
			f.b.call("POST", "/api/config/restore", map[string]any{"snapshot": sn.ID}, nil)
			f.b.settle()
			f.b.stop()
			f.b.start(currentBinary)

			settings, _ := f.b.settings()
			if settings["instanceName"] != instanceName(0) {
				f.errorf("the instance is called %v after the restore, want %s", settings["instanceName"], instanceName(0))
			}
			f.finish()
		})
	}
}
