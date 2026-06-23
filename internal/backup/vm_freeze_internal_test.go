package backup

import (
	"errors"
	"testing"
)

func TestIsFreezeErr(t *testing.T) {
	freeze := []error{
		errors.New("guest agent command failed: child process has failed to execute fsfreeze hook"),
		errors.New("unable to execute QEMU agent command 'guest-fsfreeze-freeze'"),
		errors.New("Quiesce request failed"),
	}
	for _, e := range freeze {
		if !isFreezeErr(e) {
			t.Fatalf("expected freeze error to match: %v", e)
		}
	}
	for _, e := range []error{nil, errors.New("snapshot device busy"), errors.New("no space left")} {
		if isFreezeErr(e) {
			t.Fatalf("did not expect a freeze match for: %v", e)
		}
	}
}
