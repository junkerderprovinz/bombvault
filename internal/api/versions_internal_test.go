package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func printing(outputs map[string]string) versionCommand {
	return func(_ context.Context, tool string) ([]byte, error) {
		out, ok := outputs[tool]
		if !ok {
			return nil, errors.New("exec: \"" + tool + "\": executable file not found in $PATH")
		}
		return []byte(out), nil
	}
}

func TestToolVersionsAreReadFromTheVersionCommands(t *testing.T) {
	got := probeToolVersions(printing(map[string]string{
		"restic": "restic 0.18.1 compiled with go1.25.1 on linux/amd64\n",
		"rclone": "rclone v1.71.1\n- os/version: alpine 3.22.1 (64 bit)\n- go/version: go1.25.1\n",
	}))
	if got.Restic == nil || *got.Restic != "0.18.1" {
		t.Fatalf("restic = %v, want 0.18.1", got.Restic)
	}
	if got.Rclone == nil || *got.Rclone != "1.71.1" {
		t.Fatalf("rclone = %v, want 1.71.1", got.Rclone)
	}
}

func TestToolWithAFailingProbeIsUnknown(t *testing.T) {
	got := probeToolVersions(printing(map[string]string{
		"restic": "restic 0.18.1 compiled with go1.25.1 on linux/amd64\n",
	}))
	if got.Rclone != nil {
		t.Fatalf("rclone = %q although its command failed", *got.Rclone)
	}
	if got.Restic == nil {
		t.Fatal("a missing rclone took restic's version with it")
	}

	got = probeToolVersions(printing(map[string]string{"restic": "usage: restic [command]\n", "rclone": ""}))
	if got.Restic != nil || got.Rclone != nil {
		t.Fatalf("output without a version number = %v / %v, want both unknown", got.Restic, got.Rclone)
	}
}

func TestVersionsPayload(t *testing.T) {
	probed, running := toolVersions, Version
	t.Cleanup(func() { toolVersions, Version = probed, running })
	Version = "v10.0.0"
	toolVersions = func() ComponentVersions {
		return probeToolVersions(printing(map[string]string{"restic": "restic 0.18.1 compiled with go1.25.1 on linux/amd64\n"}))
	}

	rec := httptest.NewRecorder()
	(&Handler{}).handleVersions(rec, httptest.NewRequest(http.MethodGet, "/api/versions", nil))

	want := `{"ok":true,"versions":{"bombvault":"v10.0.0","restic":"0.18.1","rclone":null}}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("payload = %s\nwant      %s", got, want)
	}
}
