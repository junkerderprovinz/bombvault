package restic

import (
	"slices"
	"testing"
)

func TestResticLetsProtonDriveReplaceTheDraftOfAFailedUpload(t *testing.T) {
	r := Restic{Bin: "restic"}
	if env := r.authEnv(Mode{}); !slices.Contains(env, "RCLONE_PROTONDRIVE_REPLACE_EXISTING_DRAFT=true") {
		t.Fatalf("env = %v, want the draft replaced", env)
	}
}
