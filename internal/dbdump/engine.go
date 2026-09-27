// Package dbdump recognises database containers and describes how a logical
// dump of one is taken. It is pure: no Docker, no store, no restic.
package dbdump

import (
	"path"
	"strings"
)

// Engine is the database server a container runs.
type Engine string

const (
	EngineNone     Engine = ""
	EnginePostgres Engine = "postgres"
	EngineMySQL    Engine = "mysql"
	EngineMariaDB  Engine = "mariadb"
)

// ParseEngine reads an engine in the spelling used on the wire and in the store.
func ParseEngine(s string) (Engine, bool) {
	switch e := Engine(s); e {
	case EnginePostgres, EngineMySQL, EngineMariaDB:
		return e, true
	}
	return EngineNone, false
}

// CompletionMarker is the line the engine's dump tool writes once it has
// written everything, so a truncated dump with exit code 0 can be told apart
// from a complete one.
func (e Engine) CompletionMarker() string {
	switch e {
	case EnginePostgres:
		return "-- PostgreSQL database cluster dump complete"
	case EngineMySQL, EngineMariaDB:
		return "-- Dump completed"
	}
	return ""
}

// DataDirs lists the container paths that can hold the engine's data, the most
// specific first.
func (e Engine) DataDirs(env []string) []string {
	switch e {
	case EnginePostgres:
		var dirs []string
		if d := envValue(env, "PGDATA"); strings.HasPrefix(d, "/") {
			dirs = append(dirs, path.Clean(d))
		}
		return append(dirs, "/var/lib/postgresql/data", "/var/lib/postgresql")
	case EngineMySQL, EngineMariaDB:
		return []string{"/var/lib/mysql", "/config"}
	}
	return nil
}

// Mount is one entry of a container's mount table.
type Mount struct{ Source, Destination string }

// DataMount returns the host path of the engine's data directory. Of the
// mounts that hold the directory the deepest one decides, and the part of the
// directory below that mount is appended to its source.
func DataMount(e Engine, env []string, mounts []Mount) (string, bool) {
	for _, dir := range e.DataDirs(env) {
		var best Mount
		for _, m := range mounts {
			dest := path.Clean(m.Destination)
			if m.Source == "" || !holds(dest, dir) {
				continue
			}
			if len(dest) > len(best.Destination) {
				best = Mount{Source: m.Source, Destination: dest}
			}
		}
		if best.Source != "" {
			return path.Join(best.Source, strings.TrimPrefix(dir, best.Destination)), true
		}
	}
	return "", false
}

// holds reports whether dest is dir or an ancestor of it.
func holds(dest, dir string) bool {
	if !strings.HasPrefix(dest, "/") {
		return false
	}
	return dest == dir || strings.HasPrefix(dir, strings.TrimSuffix(dest, "/")+"/")
}

func envValue(env []string, name string) string {
	prefix := name + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(e, prefix))
		}
	}
	return ""
}
