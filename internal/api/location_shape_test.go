package api_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestABackupToAStoredHostThatReadsAsAnOptionFailsAndSaysWhy(t *testing.T) {
	rig := newFirstBackupRig(t)
	s := mustSettings(t, rig.st)
	s.FilesPath = "sftp:backup@-badhost:/srv/repo"
	if err := rig.st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	set, err := rig.st.CreateFileSet(store.FileSet{Name: "docs", Path: "docs", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := rig.svc.BackupFileSet(context.Background(), set.ID); !errors.Is(err, restic.ErrLocationReadsAsOption) {
		t.Fatalf("BackupFileSet = %v, want the location refused", err)
	}

	runs, err := rig.st.ListRuns(5)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].Status == "success" || !strings.Contains(runs[0].Error, "start with a dash") {
		t.Fatalf("runs = %+v, want one failed run that says why", runs)
	}
}
