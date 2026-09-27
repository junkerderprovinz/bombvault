package dbdump_test

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/dbdump"
)

var dumpEngines = []dbdump.Engine{dbdump.EnginePostgres, dbdump.EngineMySQL, dbdump.EngineMariaDB}

func TestScriptIsConstant(t *testing.T) {
	for _, e := range dumpEngines {
		script, err := dbdump.Script(e)
		if err != nil {
			t.Fatalf("Script(%q): %v", e, err)
		}
		again, err := dbdump.Script(e)
		if err != nil {
			t.Fatalf("Script(%q) again: %v", e, err)
		}
		if script != again {
			t.Errorf("Script(%q) is not constant", e)
		}
		if !strings.HasPrefix(script, "umask 077\n") {
			t.Errorf("Script(%q) does not start with umask 077", e)
		}
		if strings.Contains(script, "%") {
			t.Errorf("Script(%q) contains %%, so it could be used as a format string", e)
		}
		if strings.Contains(script, "set -x") {
			t.Errorf("Script(%q) traces its commands", e)
		}
	}
	if _, err := dbdump.Script(dbdump.EngineNone); err == nil {
		t.Error("Script(EngineNone) returned a script")
	}
}

func TestMySQLScriptFlags(t *testing.T) {
	script, err := dbdump.Script(dbdump.EngineMySQL)
	if err != nil {
		t.Fatalf("Script: %v", err)
	}
	mariaScript, err := dbdump.Script(dbdump.EngineMariaDB)
	if err != nil {
		t.Fatalf("Script(mariadb): %v", err)
	}
	if script != mariaScript {
		t.Error("MySQL and MariaDB do not share one script")
	}
	for _, flag := range []string{
		"--single-transaction", "--routines", "--events", "--triggers",
		"--hex-blob", "--quick", "--default-character-set=utf8mb4",
	} {
		if !strings.Contains(script, flag) {
			t.Errorf("script lacks %s", flag)
		}
	}
	maria, mysql := strings.Index(script, "mariadb-dump"), strings.Index(script, "mysqldump")
	if maria < 0 || mysql < 0 || maria > mysql {
		t.Errorf("mariadb-dump must be probed before mysqldump, got %d and %d", maria, mysql)
	}
	gtidCase := strings.Index(script, `case "$tool" in *mysqldump)`)
	help := strings.Index(script, `"$tool" --help`)
	gtid := strings.Index(script, "--set-gtid-purged=OFF")
	if gtidCase < 0 || help < gtidCase || gtid < help {
		t.Errorf("--set-gtid-purged=OFF must sit in the *mysqldump) case after the --help probe, got %d, %d, %d", gtidCase, help, gtid)
	}
	if strings.Count(script, "--set-gtid-purged=OFF") != 1 {
		t.Error("--set-gtid-purged=OFF appears outside the mysqldump case")
	}
}

func TestScriptsKeepPasswordsOffArgv(t *testing.T) {
	builders := []func(dbdump.Engine) ([]string, error){dbdump.ProbeArgv, dbdump.ReadyArgv, dbdump.ImportArgv}
	for _, e := range dumpEngines {
		scripts := map[string]string{}
		script, err := dbdump.Script(e)
		if err != nil {
			t.Fatalf("Script(%q): %v", e, err)
		}
		scripts["dump"] = script
		for i, build := range builders {
			argv, err := build(e)
			if err != nil {
				t.Fatalf("argv builder %d for %q: %v", i, e, err)
			}
			scripts[strconv.Itoa(i)] = argv[2]
		}
		for name, s := range scripts {
			for _, token := range []string{" -p", "--password", " -W"} {
				if strings.Contains(s, token) {
					t.Errorf("%s script of %q passes %q on argv", name, e, token)
				}
			}
		}
		want := "export MYSQL_PWD"
		if e == dbdump.EnginePostgres {
			want = "export PGPASSWORD"
		}
		if !strings.Contains(script, want) {
			t.Errorf("%q dump script does not %s", e, want)
		}
	}
}

func TestExecArgvShapeAndBounds(t *testing.T) {
	script, err := dbdump.Script(dbdump.EnginePostgres)
	if err != nil {
		t.Fatalf("Script: %v", err)
	}
	argv, err := dbdump.ExecArgv(dbdump.EnginePostgres, 3600)
	if err != nil {
		t.Fatalf("ExecArgv: %v", err)
	}
	want := []string{"sh", "-c", script, "bombvault-dbdump", "3600"}
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("ExecArgv = %q, want %q", argv, want)
	}
	for _, seconds := range []int{-1, 0, 59, 172801} {
		if _, err := dbdump.ExecArgv(dbdump.EnginePostgres, seconds); err == nil {
			t.Errorf("ExecArgv accepted %d seconds", seconds)
		}
	}
	for _, seconds := range []int{60, 172800} {
		if _, err := dbdump.ExecArgv(dbdump.EnginePostgres, seconds); err != nil {
			t.Errorf("ExecArgv rejected %d seconds: %v", seconds, err)
		}
	}
	if _, err := dbdump.ExecArgv(dbdump.Engine("redis"), 3600); err == nil {
		t.Error("ExecArgv accepted an unknown engine")
	}
}

func TestOrphanStopArgvRejectsBadPid(t *testing.T) {
	for _, pid := range []int{-5, 0, 1} {
		if _, err := dbdump.OrphanStopArgv(pid); err == nil {
			t.Errorf("OrphanStopArgv accepted pid %d", pid)
		}
	}
	argv, err := dbdump.OrphanStopArgv(42)
	if err != nil {
		t.Fatalf("OrphanStopArgv: %v", err)
	}
	if len(argv) != 5 || argv[0] != "sh" || argv[1] != "-c" || argv[3] != "bv" || argv[4] != "42" {
		t.Fatalf("OrphanStopArgv = %q", argv)
	}
	for _, want := range []string{"/proc/$1/cmdline", `kill -TERM "$pid"`, "*dump*"} {
		if !strings.Contains(argv[2], want) {
			t.Errorf("orphan stop script lacks %q: %s", want, argv[2])
		}
	}
}
