package backup

import (
	"errors"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// ExitTempFail is the exit code with which a pre-backup hook says the app is
// busy and the backup should be tried again later. It is EX_TEMPFAIL from
// sysexits.h.
const ExitTempFail = 75

// StatusDeferred marks the run of a backup that did not run because the app
// was busy. It is no failure: the backup is tried again on its own.
const StatusDeferred = "deferred"

// DeferredError is a backup that did not run because the container's app was
// busy, either by its pre-backup hook's word or by its activity check.
type DeferredError struct {
	Reason string
}

func (e *DeferredError) Error() string { return "backup deferred: " + e.Reason }

// Deferred marks the error for callers that cannot import this package.
func (e *DeferredError) Deferred() bool { return true }

// IsDeferred reports whether err is a deferred backup, and why.
func IsDeferred(err error) (string, bool) {
	var d *DeferredError
	if errors.As(err, &d) {
		return d.Reason, true
	}
	return "", false
}

// exitCoder is an error that carries the exit code of a command and what it
// wrote.
type exitCoder interface {
	ExitCode() int
	Output() string
}

// hookDeferral says why a pre-hook that failed with ExitTempFail deferred the
// backup, and reports false for any other failure.
func hookDeferral(err error) (string, bool) {
	var ec exitCoder
	if !errors.As(err, &ec) || ec.ExitCode() != ExitTempFail {
		return "", false
	}
	if out := strings.TrimSpace(ec.Output()); out != "" {
		return store.ReasonDeferredHook + ": " + out, true
	}
	return store.ReasonDeferredHook, true
}
