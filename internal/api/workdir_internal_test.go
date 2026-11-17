package api

import (
	"os"
	"testing"
)

// chdir is t.Chdir that goes back by path. On Windows t.Chdir cannot go back
// to a working directory on a mapped network drive, and the panic that follows
// ends every test after it: https://github.com/golang/go/issues/80130
func chdir(t *testing.T, dir string) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			panic("back to " + wd + ": " + err.Error())
		}
	})
}
