package api

// White-box table for the browse read-error classifier (BROWSE-02).
//
// classifyReadDirError is unexported, so this table lives in package api
// (house convention for white-box tests, cf. selection_readers_internal_test.go
// and path_internal_test.go) and runs on EVERY OS: the endpoint-level
// permission fixture in browse_contract_test.go must skip on Windows dev, but
// the KIND mapping itself must not go untested there — it is pinned here over
// constructed *fs.PathError values instead of real filesystem fixtures.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"testing"
)

func TestClassifyReadDirError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "permission denied maps to restricted",
			err:  &fs.PathError{Op: "open", Path: "[path]", Err: fs.ErrPermission},
			want: "restricted",
		},
		{
			name: "wrapped os.ErrPermission still classifies",
			err:  fmt.Errorf("browse: open: %w", &fs.PathError{Op: "open", Path: "[path]", Err: os.ErrPermission}),
			want: "restricted",
		},
		{
			name: "not exist maps to missing",
			err:  &fs.PathError{Op: "open", Path: "[path]", Err: fs.ErrNotExist},
			want: "missing",
		},
		{
			name: "os.Root escape rejection stays opaque",
			// The stdlib escape error is its own value (not fs.ErrNotExist /
			// fs.ErrPermission) — it must land in the generic bucket: an
			// escape attempt is indistinguishable from any other failure.
			err:  errors.New("path escapes from parent"),
			want: "error",
		},
		{
			name: "unclassified errors land in the generic bucket",
			err:  &fs.PathError{Op: "readdir", Path: "[path]", Err: errors.New("not a directory")},
			want: "error",
		},
	}
	for _, tc := range tests {
		if got := classifyReadDirError(tc.err); got != tc.want {
			t.Errorf("%s: classifyReadDirError(%v) = %q, want %q", tc.name, tc.err, got, tc.want)
		}
	}
}
