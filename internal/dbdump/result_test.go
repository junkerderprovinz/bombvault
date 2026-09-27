package dbdump_test

import (
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/dbdump"
)

const resticPrefix = "subprocess /usr/local/bin/bombvault: "

func TestResultLineRoundTrip(t *testing.T) {
	stopped := true
	want := dbdump.Result{
		V:             1,
		OK:            false,
		Reason:        dbdump.ReasonTimeout,
		Exit:          124,
		Bytes:         4096,
		Scope:         "database",
		Detail:        "mariadb-dump: Got error: 1045",
		OrphanStopped: &stopped,
	}
	line := want.Line()
	if strings.ContainsAny(line, "\r\n") {
		t.Fatalf("Line() spans more than one line: %q", line)
	}
	if !strings.HasPrefix(line, dbdump.ResultLinePrefix) {
		t.Fatalf("Line() = %q, want the result prefix", line)
	}
	got, ok := dbdump.ParseResult([]string{"noise", line, "more noise"})
	if !ok {
		t.Fatal("ParseResult did not find the line")
	}
	if got.OrphanStopped == nil || *got.OrphanStopped != stopped {
		t.Errorf("OrphanStopped = %v", got.OrphanStopped)
	}
	got.OrphanStopped, want.OrphanStopped = nil, nil
	if got != want {
		t.Errorf("ParseResult = %+v, want %+v", got, want)
	}
}

func TestParseResultBehindResticPrefix(t *testing.T) {
	first := dbdump.Result{V: 1, OK: false, Reason: dbdump.ReasonEmpty}
	last := dbdump.Result{V: 1, OK: true, Bytes: 1024, Scope: "all"}
	lines := []string{
		resticPrefix + first.Line(),
		"restic: chatter in between",
		resticPrefix + last.Line(),
	}
	got, ok := dbdump.ParseResult(lines)
	if !ok {
		t.Fatal("ParseResult did not find a line behind the restic prefix")
	}
	if got != last {
		t.Errorf("ParseResult = %+v, want the last line %+v", got, last)
	}
}

func TestParseResultRejectsGarbage(t *testing.T) {
	tests := []struct {
		name string
		line string
	}{
		{"no line at all", "restic: nothing to see"},
		{"not json", dbdump.ResultLinePrefix + "{not json"},
		{"wrong version", dbdump.ResultLinePrefix + `{"v":2,"ok":true}`},
		{"unknown reason", dbdump.ResultLinePrefix + `{"v":1,"reason":"bogus"}`},
		{"negative bytes", dbdump.ResultLinePrefix + `{"v":1,"ok":true,"bytes":-1}`},
		{"over-long detail", dbdump.ResultLinePrefix + `{"v":1,"detail":"` + strings.Repeat("x", 5000) + `"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got, ok := dbdump.ParseResult([]string{tc.line}); ok {
				t.Errorf("ParseResult accepted %s: %+v", tc.name, got)
			}
		})
	}

	good := dbdump.Result{V: 1, OK: true, Bytes: 7}
	lines := []string{good.Line(), dbdump.ResultLinePrefix + `{"v":9}`}
	if got, ok := dbdump.ParseResult(lines); !ok || got != good {
		t.Errorf("ParseResult = %+v, %v, want the last valid line", got, ok)
	}
}

func TestReasonIDsCoverEveryReason(t *testing.T) {
	seen := map[string]bool{}
	for _, id := range dbdump.ReasonIDs {
		if id == "" {
			t.Error("ReasonIDs carries an empty id")
		}
		if seen[id] {
			t.Errorf("ReasonIDs lists %q twice", id)
		}
		seen[id] = true
	}
	for _, id := range []string{
		dbdump.ReasonAuth, dbdump.ReasonPrivileges, dbdump.ReasonUnreachable,
		dbdump.ReasonNoClient, dbdump.ReasonSecret, dbdump.ReasonNoCredentials,
		dbdump.ReasonNotRunning, dbdump.ReasonTimeout, dbdump.ReasonEmpty,
		dbdump.ReasonIncomplete, dbdump.ReasonTool, dbdump.ReasonDocker,
		dbdump.ReasonWrite, dbdump.ReasonCancelled, dbdump.ReasonUsage,
		dbdump.ReasonNeedsUpgrade,
	} {
		if !seen[id] {
			t.Errorf("ReasonIDs does not list %q", id)
		}
	}
}

func TestScrubDetail(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"control characters go", "a\x00b\x07c\x7f", "a b c"},
		{"whitespace collapses", "  two   words \t\n here  ", "two words here"},
		{"password redacted", "psql: connection failed password=hunter2 and on", "psql: connection failed password=*** and on"},
		{"pwd redacted", "env MYSQL_PWD=s3cr3t set", "env MYSQL_PWD=*** set"},
		{"pass redacted", "url pass:hunter2", "url pass:***"},
		{"pg password redacted", "PGPASSWORD=hunter2", "PGPASSWORD=***"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := dbdump.ScrubDetail(tc.in); got != tc.want {
				t.Errorf("ScrubDetail(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}

	long := dbdump.ScrubDetail(strings.Repeat("ä", 400))
	if len(long) > 300 {
		t.Errorf("ScrubDetail kept %d bytes", len(long))
	}
	if !strings.HasSuffix(long, "ä") {
		t.Errorf("ScrubDetail cut inside a rune: %q", long[len(long)-4:])
	}
}

func TestParsePIDAndScope(t *testing.T) {
	lines := []string{
		resticPrefix + dbdump.PIDLinePrefix + "abc",
		resticPrefix + dbdump.PIDLinePrefix + "0",
		dbdump.PIDLinePrefix + "1",
		dbdump.PIDLinePrefix + "-3",
		resticPrefix + dbdump.PIDLinePrefix + "17",
		resticPrefix + dbdump.PIDLinePrefix + "4242",
		resticPrefix + dbdump.ScopeLinePrefix + "all",
		dbdump.ScopeLinePrefix + "database",
	}
	pid, ok := dbdump.ParsePID(lines)
	if !ok || pid != 4242 {
		t.Errorf("ParsePID = %d, %v, want 4242", pid, ok)
	}
	if got := dbdump.ParseScope(lines); got != "database" {
		t.Errorf("ParseScope = %q, want database", got)
	}

	none := []string{dbdump.PIDLinePrefix + "1", "restic: chatter"}
	if pid, ok := dbdump.ParsePID(none); ok {
		t.Errorf("ParsePID = %d, want no pid", pid)
	}
	if got := dbdump.ParseScope(none); got != "" {
		t.Errorf("ParseScope = %q, want empty", got)
	}
}
