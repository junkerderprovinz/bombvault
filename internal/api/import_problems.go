package api

import (
	"errors"
	"fmt"
	"net/http"
)

// importProblem is a refusal of a settings file that the page words itself: a
// code for the kind, the part of the file and the row it is about, a setting's
// key or a repository's name, and for a repository on a taken place, that
// place. cause is the refusal behind it, which carries a code of its own.
type importProblem struct {
	code, part, name, place string
	row                     int
	msg                     string
	cause                   error
}

func (p *importProblem) Error() string { return p.msg }

func (p *importProblem) Unwrap() error { return p.cause }

// The parts of a settings file whose rows a problem can name.
const (
	importPartTarget      = "target"
	importPartRepository  = "repository"
	importPartDestination = "destination"
	importPartZFSServer   = "zfs-server"
	importPartZFSReplica  = "zfs-replica"
)

// rowProblem is a problem of row i, counted from 0, of a part of the file.
func rowProblem(code, part string, i int, cause error, format string, args ...any) *importProblem {
	return &importProblem{code: code, part: part, row: i + 1, cause: cause, msg: fmt.Sprintf(format, args...)}
}

// errImportCheck is a file that could not be held against what is set up here.
var errImportCheck = &importProblem{code: "import-check-failed", msg: "could not check this file against the repositories already set up; try again"}

// importFail writes a refused import: a problem with its code, its parameters
// and the code of its cause, anything else as a placement refusal.
func importFail(w http.ResponseWriter, err error) {
	p, ok := errors.AsType[*importProblem](err)
	if !ok {
		placementFail(w, err, nil)
		return
	}
	env := map[string]any{"ok": false, "error": scrubError(err), "code": p.code}
	for key, value := range map[string]string{"part": p.part, "name": p.name, "place": p.place} {
		if value != "" {
			env[key] = value
		}
	}
	if p.row > 0 {
		env["row"] = p.row
	}
	if p.cause != nil {
		if code := placementCode(p.cause); code != "" {
			env["cause"] = code
		}
		if pathErr, ok := errors.AsType[*repoPathErr](p.cause); ok && pathErr.suggest != "" {
			env["suggest"] = pathErr.suggest
		}
	}
	writeJSON(w, http.StatusOK, env)
}

// importPartStoredRepository is a repository set up here, which the file has
// no row for and the problem names instead.
const importPartStoredRepository = "stored-repository"

// collision is a repository on a place another one or a section takes: row
// counts the file's repositories from 1, and a repository set up here has none
// and goes by name. place is "" for a repository before it in the file.
func collision(row int, name, place, msg string) error {
	part, code := importPartRepository, "import-place-taken"
	if row == 0 {
		part = importPartStoredRepository
	}
	if place == "" {
		code = "import-place-twice"
	}
	return &importProblem{code: code, part: part, row: row, name: name, place: place, msg: msg}
}
