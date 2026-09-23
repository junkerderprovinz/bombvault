package api

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"testing"
)

// TestClassifyReadDirError uses constructed errors, so it also runs on Windows,
// where browse_contract_test.go cannot build a permission-denied fixture.
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
			// os.Root's escape error is a value of its own, and an escape
			// attempt has to look like any other failure.
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
