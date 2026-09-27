package dbdump

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// maxProbeDatabases is how many database names a dump snapshot can carry as
// tags.
const maxProbeDatabases = 32

const (
	probeVersionPrefix  = "bombvault-dbdump-version "
	probeDatabasePrefix = "bombvault-dbdump-db "
)

// Probe is what the read-only exec before a dump found out about the server.
type Probe struct {
	Version   string
	Databases []string
	Dropped   int
}

// versionAnchors precede the server version in the --version formats of
// pg_dumpall, mariadb-dump and mysqldump, most specific first. The older
// format of both MySQL and MariaDB prints the dump tool's own version after
// "Ver " and the server's after "Distrib ", so "Ver " is the last resort: it
// is the server version only where the tool shares its number, as MySQL 8 does.
var versionAnchors = []string{"(PostgreSQL) ", "from ", "Distrib ", "Ver "}

var versionRe = regexp.MustCompile(`\d+\.\d+(\.\d+)?`)

// systemSchemas belong to the server, not to an application, so a dump does
// not get a tag for them.
var systemSchemas = map[string]bool{
	"information_schema": true,
	"performance_schema": true,
	"sys":                true,
}

// ParseProbe reads the probe's protocol lines. Names that could not be a tag
// and names beyond the cap are counted rather than dropped silently, so the
// log can say what the snapshot does not mention.
func ParseProbe(out string) Probe {
	var p Probe
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, probeVersionPrefix):
			if p.Version == "" {
				p.Version = versionIn(line[len(probeVersionPrefix):])
			}
		case strings.HasPrefix(line, probeDatabasePrefix):
			name := strings.TrimSpace(line[len(probeDatabasePrefix):])
			if name == "" || systemSchemas[name] {
				continue
			}
			if !tagSafe(name) || len(p.Databases) == maxProbeDatabases {
				p.Dropped++
				continue
			}
			p.Databases = append(p.Databases, name)
		}
	}
	return p
}

func versionIn(banner string) string {
	for _, anchor := range versionAnchors {
		at := strings.Index(banner, anchor)
		if at < 0 {
			continue
		}
		if v := versionRe.FindString(banner[at+len(anchor):]); v != "" {
			return v
		}
	}
	return ""
}

func tagSafe(name string) bool {
	if len(name) > 255 || strings.ContainsRune(name, ',') {
		return false
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// VersionMajorOK reports whether a server of version server can take a dump
// written by version dump. PostgreSQL reads older dumps, MySQL and MariaDB
// want the major version they were dumped from. An unknown version on either
// side is no reason to refuse.
func VersionMajorOK(e Engine, dump, server string) bool {
	dumpMajor, haveDump := majorOf(dump)
	serverMajor, haveServer := majorOf(server)
	if !haveDump || !haveServer {
		return true
	}
	if e == EnginePostgres {
		return serverMajor >= dumpMajor
	}
	return serverMajor == dumpMajor
}

func majorOf(version string) (int, bool) {
	head, _, _ := strings.Cut(version, ".")
	major, err := strconv.Atoi(head)
	if err != nil {
		return 0, false
	}
	return major, true
}

// CountImportErrors counts the errors an import reported. A cluster the image
// just initialised already holds the role and the database the container was
// created with, so pg_dumpall's CREATE for each of them collides once and is
// not an error of the import.
func CountImportErrors(e Engine, stderr, pguser, pgdb string) int {
	var expected []string
	if e == EnginePostgres {
		if pguser != "" {
			expected = append(expected, `role "`+pguser+`" already exists`)
		}
		if pgdb != "" {
			expected = append(expected, `database "`+pgdb+`" already exists`)
		}
	}
	count := 0
	for _, line := range strings.Split(stderr, "\n") {
		if !strings.Contains(line, "ERROR") {
			continue
		}
		if i := slices.IndexFunc(expected, func(e string) bool { return strings.Contains(line, e) }); i >= 0 {
			expected = slices.Delete(expected, i, i+1)
			continue
		}
		count++
	}
	return count
}
