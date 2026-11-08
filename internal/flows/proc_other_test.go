//go:build flows && !unix

package flows

import (
	"os"
	"os/exec"
)

func ownGroup(*exec.Cmd) {}

func endGroup(*os.Process) {}
