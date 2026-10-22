package api

import (
	"errors"
	"strings"
	"testing"
)

func TestAnExpiredSignInIsNotTakenForABackendWithoutAbout(t *testing.T) {
	stderr := "2026/10/22 10:00:00 CRITICAL: Failed to about: about call failed: couldn't get about: oauth2: \"invalid_grant\"\n"

	err := aboutFailure(stderr, errors.New("exit status 1"))

	if errors.Is(err, errAboutUnsupported) {
		t.Fatal("a failed sign-in was filed as a backend that cannot tell its free space")
	}
	if !strings.Contains(err.Error(), "invalid_grant") {
		t.Fatalf("err = %v, want rclone's reason", err)
	}
}

func TestABackendWithoutAboutIsSaidToBeUnmeasurable(t *testing.T) {
	stderr := "2026/10/22 10:00:00 CRITICAL: Failed to about: sftp root '' doesn't support about\n"

	if err := aboutFailure(stderr, errors.New("exit status 1")); !errors.Is(err, errAboutUnsupported) {
		t.Fatalf("err = %v, want the backend named as unmeasurable", err)
	}
}
