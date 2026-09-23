//go:build !windows

package dbdump_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/dbdump"
)

// shellCase runs a generated script against stub tools on a PATH of its own.
// Every stub records how it was called, so a test can see the argv and the
// environment the script really handed to the dump tool.
type shellCase struct {
	t         *testing.T
	stubs     string
	records   string
	onlyStubs bool
}

type shellResult struct {
	stdout, stderr string
	exit           int
}

// call is what a stub wrote down about the last time it ran.
type call struct {
	path string
	args []string
	env  map[string]string
	pid  string
}

func newShellCase(t *testing.T) *shellCase {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh on PATH")
	}
	root := t.TempDir()
	c := &shellCase{t: t, stubs: filepath.Join(root, "bin"), records: filepath.Join(root, "rec")}
	for _, dir := range []string{c.stubs, c.records} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("create %s: %v", dir, err)
		}
	}
	return c
}

func (c *shellCase) write(name, script string) {
	c.t.Helper()
	path := filepath.Join(c.stubs, name)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil { //nolint:gosec // G306: a stub tool has to be executable
		c.t.Fatalf("write stub %s: %v", name, err)
	}
}

// stub writes a tool that records its call and then runs body.
func (c *shellCase) stub(name, body string) {
	c.t.Helper()
	record := filepath.Join(c.records, name)
	c.write(name, "#!/bin/sh\n"+
		"{ echo \"path=$0\"; for a in \"$@\"; do echo \"arg=$a\"; done\n"+
		"  echo \"MYSQL_PWD=$MYSQL_PWD\"; echo \"PGPASSWORD=$PGPASSWORD\"; echo \"PGUSER=$PGUSER\"\n"+
		"  echo \"pid=$$\"; } > "+record+"\n"+body+"\n")
}

func (c *shellCase) secretFile(name, content string) string {
	c.t.Helper()
	path := filepath.Join(c.t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		c.t.Fatalf("write secret %s: %v", name, err)
	}
	return path
}

func (c *shellCase) run(argv, env []string, stdin string) shellResult {
	c.t.Helper()
	path := c.stubs
	if !c.onlyStubs {
		path += ":" + os.Getenv("PATH")
	}
	cmd := exec.Command(argv[0], argv[1:]...) //nolint:gosec // G204: the argv is this package's own constant script
	cmd.Env = append([]string{"PATH=" + path}, env...)
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	res := shellResult{}
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			c.t.Fatalf("run script: %v", err)
		}
		res.exit = exit.ExitCode()
	}
	res.stdout, res.stderr = stdout.String(), stderr.String()
	return res
}

func (c *shellCase) dump(engine dbdump.Engine, env []string) shellResult {
	c.t.Helper()
	argv, err := dbdump.ExecArgv(engine, 3600)
	if err != nil {
		c.t.Fatalf("ExecArgv: %v", err)
	}
	return c.run(argv, env, "")
}

func (c *shellCase) call(name string) call {
	c.t.Helper()
	data, err := os.ReadFile(filepath.Join(c.records, name)) //nolint:gosec // G304: the path is this test's own temp directory
	if err != nil {
		c.t.Fatalf("stub %s was never called: %v", name, err)
	}
	got := call{env: map[string]string{}}
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		key, value, _ := strings.Cut(line, "=")
		switch key {
		case "path":
			got.path = value
		case "arg":
			got.args = append(got.args, value)
		case "pid":
			got.pid = value
		default:
			got.env[key] = value
		}
	}
	return got
}

func (c *shellCase) ran(name string) bool {
	c.t.Helper()
	_, err := os.Stat(filepath.Join(c.records, name))
	return err == nil
}

func (c *shellCase) stdinOf(name string) string {
	c.t.Helper()
	data, err := os.ReadFile(filepath.Join(c.records, name+".stdin")) //nolint:gosec // G304: the path is this test's own temp directory
	if err != nil {
		c.t.Fatalf("stub %s read no stdin: %v", name, err)
	}
	return string(data)
}

func (c call) hasArg(want string) bool {
	for _, arg := range c.args {
		if arg == want {
			return true
		}
	}
	return false
}

func scopeOf(stderr string) string {
	return dbdump.ParseScope(strings.Split(stderr, "\n"))
}

func TestPostgresScriptReadsPasswordFile(t *testing.T) {
	c := newShellCase(t)
	c.stub("pg_dumpall", "exit 0")
	secret := c.secretFile("pgpass", "s3cr3t\n")

	res := c.dump(dbdump.EnginePostgres, []string{"POSTGRES_PASSWORD_FILE=" + secret})
	if res.exit != 0 {
		t.Fatalf("exit %d, stderr %q", res.exit, res.stderr)
	}
	got := c.call("pg_dumpall")
	if got.env["PGPASSWORD"] != "s3cr3t" {
		t.Errorf("PGPASSWORD = %q, want the file content without its newline", got.env["PGPASSWORD"])
	}
	if got.env["PGUSER"] != "postgres" {
		t.Errorf("PGUSER = %q, want postgres", got.env["PGUSER"])
	}
	if strings.Contains(strings.Join(got.args, " "), "s3cr3t") {
		t.Errorf("the password reached argv: %q", got.args)
	}
	if !got.hasArg("--no-password") {
		t.Errorf("argv = %q, want --no-password", got.args)
	}
	if scopeOf(res.stderr) != "all" {
		t.Errorf("scope = %q, want all", scopeOf(res.stderr))
	}
}

func TestMySQLScriptRootPath(t *testing.T) {
	t.Run("mariadb-dump as root", func(t *testing.T) {
		c := newShellCase(t)
		c.stub("mariadb-dump", "exit 0")
		c.stub("mysqldump", "exit 0")

		res := c.dump(dbdump.EngineMariaDB, []string{"MYSQL_ROOT_PASSWORD=r"})
		if res.exit != 0 {
			t.Fatalf("exit %d, stderr %q", res.exit, res.stderr)
		}
		if c.ran("mysqldump") {
			t.Error("mysqldump ran although mariadb-dump was there")
		}
		got := c.call("mariadb-dump")
		if filepath.Base(got.path) != "mariadb-dump" {
			t.Errorf("the script ran %q", got.path)
		}
		for _, want := range []string{"--user=root", "--all-databases", "--single-transaction"} {
			if !got.hasArg(want) {
				t.Errorf("argv = %q, want %s", got.args, want)
			}
		}
		if got.env["MYSQL_PWD"] != "r" {
			t.Errorf("MYSQL_PWD = %q, want r", got.env["MYSQL_PWD"])
		}
		if got.hasArg("--set-gtid-purged=OFF") {
			t.Error("mariadb-dump was given --set-gtid-purged=OFF")
		}
		if scopeOf(res.stderr) != "all" {
			t.Errorf("scope = %q, want all", scopeOf(res.stderr))
		}
	})

	gtid := "--set-gtid-purged=OFF"
	for _, tc := range []struct {
		name string
		help string
		want bool
	}{
		{"mysqldump that knows gtid-purged", "  --set-gtid-purged=name", true},
		{"mysqldump that does not", "  --single-transaction", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newShellCase(t)
			c.stub("mysqldump", "case \"$1\" in --help) echo \""+tc.help+"\"; exit 0 ;; esac\nexit 0")

			res := c.dump(dbdump.EngineMySQL, []string{"MYSQL_ROOT_PASSWORD=r"})
			if res.exit != 0 {
				t.Fatalf("exit %d, stderr %q", res.exit, res.stderr)
			}
			got := c.call("mysqldump")
			if got.hasArg(gtid) != tc.want {
				t.Errorf("argv = %q, want %s present: %v", got.args, gtid, tc.want)
			}
			if tc.want && got.args[len(got.args)-1] != gtid {
				t.Errorf("argv = %q, want %s last", got.args, gtid)
			}
		})
	}
}

// mysqlClient writes a client stub that records every login attempt as
// "<user> <socket|tcp> <none|given> <query>" and then answers the way decide,
// an sh fragment, says. It returns the path of the record.
func (c *shellCase) mysqlClient(decide string) string {
	c.t.Helper()
	attempts := filepath.Join(c.records, "attempts")
	c.write("mariadb", "#!/bin/sh\n"+
		"proto=socket; user=; query=\n"+
		"while [ $# -gt 0 ]; do\n"+
		"  case \"$1\" in\n"+
		"    --protocol=TCP) proto=tcp ;;\n"+
		"    --user=*) user=${1#--user=} ;;\n"+
		"    -e) query=$2; shift ;;\n"+
		"  esac\n"+
		"  shift\n"+
		"done\n"+
		"pw=none; [ -n \"$MYSQL_PWD\" ] && pw=given\n"+
		"echo \"$user $proto $pw $query\" >> "+attempts+"\n"+
		decide+"\n")
	return attempts
}

func (c *shellCase) attempts() []string {
	c.t.Helper()
	data, err := os.ReadFile(filepath.Join(c.records, "attempts")) //nolint:gosec // G304: the path is this test's own temp directory
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

// TestMySQLScriptSettlesTheLoginTheImageGrants runs the login order against the
// account layouts the tier-1 images really create: linuxserver/mariadb puts
// the root password on root@'%' and leaves the local root accounts without
// one, yobasystems/alpine-mariadb creates the app user as user@'%' behind an
// anonymous local account.
func TestMySQLScriptSettlesTheLoginTheImageGrants(t *testing.T) {
	const (
		refuseEveryPassword = `[ "$pw" = given ] && exit 1
case "$query" in *CURRENT_USER*) echo "root@localhost"; exit 0 ;; esac
exit 0`
		anonymousAnswers = `[ "$pw" = given ] && exit 1
case "$query" in *CURRENT_USER*) echo "@localhost"; exit 0 ;; esac
exit 0`
		tcpOnly = `[ "$proto" = tcp ] && exit 0
exit 1`
	)
	appEnv := []string{"MARIADB_USER=app", "MARIADB_PASSWORD=p", "MARIADB_DATABASE=appdb"}

	t.Run("the socket login works", func(t *testing.T) {
		c := newShellCase(t)
		c.stub("mariadb-dump", "exit 0")
		c.mysqlClient("exit 0")

		res := c.dump(dbdump.EngineMariaDB, []string{"MYSQL_ROOT_PASSWORD=r"})
		if res.exit != 0 {
			t.Fatalf("exit %d, stderr %q", res.exit, res.stderr)
		}
		if want := []string{"root socket given SELECT 1"}; !slices.Equal(c.attempts(), want) {
			t.Errorf("attempts = %q, want %q", c.attempts(), want)
		}
		got := c.call("mariadb-dump")
		if got.hasArg("--protocol=TCP") {
			t.Errorf("argv = %q, want the socket", got.args)
		}
		if got.env["MYSQL_PWD"] != "r" {
			t.Errorf("MYSQL_PWD = %q, want r", got.env["MYSQL_PWD"])
		}
	})

	t.Run("the local root account has no password", func(t *testing.T) {
		c := newShellCase(t)
		c.stub("mariadb-dump", "exit 0")
		c.mysqlClient(refuseEveryPassword)

		res := c.dump(dbdump.EngineMariaDB, []string{"MYSQL_ROOT_PASSWORD=r"})
		if res.exit != 0 {
			t.Fatalf("exit %d, stderr %q", res.exit, res.stderr)
		}
		want := []string{
			"root socket given SELECT 1",
			"root tcp given SELECT 1",
			"root socket none SELECT CURRENT_USER()",
		}
		if !slices.Equal(c.attempts(), want) {
			t.Errorf("attempts = %q, want %q", c.attempts(), want)
		}
		got := c.call("mariadb-dump")
		if got.env["MYSQL_PWD"] != "" {
			t.Errorf("MYSQL_PWD = %q, want the password dropped", got.env["MYSQL_PWD"])
		}
		if got.hasArg("--protocol=TCP") || !got.hasArg("--all-databases") {
			t.Errorf("argv = %q, want the whole server over the socket", got.args)
		}
	})

	t.Run("an anonymous local account does not pass for root", func(t *testing.T) {
		c := newShellCase(t)
		c.stub("mariadb-dump", "exit 0")
		c.mysqlClient(anonymousAnswers)

		res := c.dump(dbdump.EngineMariaDB, []string{"MYSQL_ROOT_PASSWORD=r"})
		if res.exit != 0 {
			t.Fatalf("exit %d, stderr %q", res.exit, res.stderr)
		}
		got := c.call("mariadb-dump")
		if got.env["MYSQL_PWD"] != "r" {
			t.Errorf("MYSQL_PWD = %q, want the configured password, so the dump fails with the server's own message", got.env["MYSQL_PWD"])
		}
		if got.hasArg("--protocol=TCP") {
			t.Errorf("argv = %q, want the socket", got.args)
		}
	})

	t.Run("the app user exists for TCP only", func(t *testing.T) {
		c := newShellCase(t)
		c.stub("mariadb-dump", "exit 0")
		c.mysqlClient(tcpOnly)

		res := c.dump(dbdump.EngineMariaDB, appEnv)
		if res.exit != 0 {
			t.Fatalf("exit %d, stderr %q", res.exit, res.stderr)
		}
		want := []string{"app socket given SELECT 1", "app tcp given SELECT 1"}
		if !slices.Equal(c.attempts(), want) {
			t.Errorf("attempts = %q, want %q", c.attempts(), want)
		}
		got := c.call("mariadb-dump")
		for _, arg := range []string{"--protocol=TCP", "--host=127.0.0.1", "--user=app", "appdb"} {
			if !got.hasArg(arg) {
				t.Errorf("argv = %q, want %s", got.args, arg)
			}
		}
		if got.env["MYSQL_PWD"] != "p" {
			t.Errorf("MYSQL_PWD = %q, want p", got.env["MYSQL_PWD"])
		}
	})

	t.Run("a refused app user keeps the socket and its own error", func(t *testing.T) {
		c := newShellCase(t)
		c.stub("mariadb-dump", "exit 0")
		c.mysqlClient("exit 1")

		res := c.dump(dbdump.EngineMariaDB, appEnv)
		if res.exit != 0 {
			t.Fatalf("exit %d, stderr %q", res.exit, res.stderr)
		}
		got := c.call("mariadb-dump")
		if got.hasArg("--protocol=TCP") {
			t.Errorf("argv = %q, want the socket", got.args)
		}
		if len(c.attempts()) != 2 {
			t.Errorf("attempts = %q, want the socket and TCP and no more", c.attempts())
		}
	})

	t.Run("the import takes the login the dump settled on", func(t *testing.T) {
		c := newShellCase(t)
		c.stub("mariadb-dump", "exit 0")
		c.write("mariadb", "#!/bin/sh\n"+
			"case \"$*\" in\n"+
			"  *'SELECT 1'*) case \"$*\" in *--protocol=TCP*) exit 0 ;; esac; exit 1 ;;\n"+
			"esac\n"+
			"{ for a in \"$@\"; do echo \"arg=$a\"; done; } > "+filepath.Join(c.records, "import")+"\nexit 0\n")

		argv, err := dbdump.ImportArgv(dbdump.EngineMariaDB)
		if err != nil {
			t.Fatalf("ImportArgv: %v", err)
		}
		res := c.run(argv, appEnv, "")
		if res.exit != 0 {
			t.Fatalf("exit %d, stderr %q", res.exit, res.stderr)
		}
		if got := c.call("import"); !got.hasArg("--protocol=TCP") {
			t.Errorf("import argv = %q, want the login the dump settles on", got.args)
		}
	})
}

func TestMySQLScriptRandomRootFallsBackToAppUser(t *testing.T) {
	appEnv := []string{"MARIADB_USER=app", "MARIADB_PASSWORD=p", "MARIADB_DATABASE=appdb"}

	tests := []struct {
		name      string
		env       []string
		clientRun string
		wantScope string
	}{
		{"random root password", append([]string{"MARIADB_RANDOM_ROOT_PASSWORD=yes"}, appEnv...), "", "database"},
		{"no root variable at all", appEnv, "", "database"},
		{
			"random flag but the root login works",
			append([]string{"MARIADB_RANDOM_ROOT_PASSWORD=yes", "MARIADB_ROOT_PASSWORD=r"}, appEnv...),
			"exit 0", "all",
		},
		{
			"random flag and the root login is refused",
			append([]string{"MARIADB_RANDOM_ROOT_PASSWORD=yes", "MARIADB_ROOT_PASSWORD=r"}, appEnv...),
			"exit 1", "database",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newShellCase(t)
			c.stub("mariadb-dump", "exit 0")
			if tc.clientRun != "" {
				c.stub("mariadb", tc.clientRun)
			}

			res := c.dump(dbdump.EngineMariaDB, tc.env)
			if res.exit != 0 {
				t.Fatalf("exit %d, stderr %q", res.exit, res.stderr)
			}
			if got := scopeOf(res.stderr); got != tc.wantScope {
				t.Fatalf("scope = %q, want %q", got, tc.wantScope)
			}
			got := c.call("mariadb-dump")
			if tc.wantScope == "all" {
				if !got.hasArg("--user=root") || !got.hasArg("--all-databases") {
					t.Errorf("argv = %q, want the root path", got.args)
				}
				if got.env["MYSQL_PWD"] != "r" {
					t.Errorf("MYSQL_PWD = %q, want r", got.env["MYSQL_PWD"])
				}
				return
			}
			for _, want := range []string{"--no-tablespaces", "--user=app", "--databases", "appdb"} {
				if !got.hasArg(want) {
					t.Errorf("argv = %q, want %s", got.args, want)
				}
			}
			if got.hasArg("--all-databases") {
				t.Errorf("argv = %q, want no --all-databases", got.args)
			}
			if got.env["MYSQL_PWD"] != "p" {
				t.Errorf("MYSQL_PWD = %q, want p", got.env["MYSQL_PWD"])
			}
		})
	}
}

func TestFileSecretConventions(t *testing.T) {
	tests := []struct {
		name     string
		variable string
	}{
		{"linuxserver", "FILE__MYSQL_ROOT_PASSWORD"},
		{"jc21", "MYSQL_ROOT_PASSWORD__FILE"},
		{"official image", "MARIADB_ROOT_PASSWORD_FILE"},
		{"docker secret in the value itself", "MYSQL_ROOT_PASSWORD"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newShellCase(t)
			c.stub("mariadb-dump", "exit 0")
			secret := c.secretFile("rootpw", "fromfile\n")

			res := c.dump(dbdump.EngineMariaDB, []string{tc.variable + "=" + secret})
			if res.exit != 0 {
				t.Fatalf("exit %d, stderr %q", res.exit, res.stderr)
			}
			if got := c.call("mariadb-dump"); got.env["MYSQL_PWD"] != "fromfile" {
				t.Errorf("MYSQL_PWD = %q, want fromfile", got.env["MYSQL_PWD"])
			}
		})
	}
}

func TestScriptExitCodes(t *testing.T) {
	t.Run("no dump tool in the container", func(t *testing.T) {
		c := newShellCase(t)
		c.onlyStubs = true
		for _, engine := range []dbdump.Engine{dbdump.EnginePostgres, dbdump.EngineMariaDB} {
			if res := c.dump(engine, nil); res.exit != dbdump.ExitNoClient {
				t.Errorf("%s exit = %d, want %d", engine, res.exit, dbdump.ExitNoClient)
			}
		}
	})

	t.Run("secret file unreadable", func(t *testing.T) {
		c := newShellCase(t)
		c.stub("mariadb-dump", "exit 0")
		missing := filepath.Join(c.records, "there-is-no-such-file")

		if res := c.dump(dbdump.EngineMariaDB, []string{"MARIADB_ROOT_PASSWORD_FILE=" + missing}); res.exit != dbdump.ExitSecret {
			t.Errorf("exit = %d, want %d", res.exit, dbdump.ExitSecret)
		}
	})

	t.Run("no usable credentials", func(t *testing.T) {
		c := newShellCase(t)
		c.stub("mariadb-dump", "exit 0")

		if res := c.dump(dbdump.EngineMariaDB, nil); res.exit != dbdump.ExitNoCredentials {
			t.Errorf("exit = %d, want %d", res.exit, dbdump.ExitNoCredentials)
		}
	})
}

func TestScriptHostileValuesAreNotEvaluated(t *testing.T) {
	for _, fromFile := range []bool{false, true} {
		name := "in the variable"
		if fromFile {
			name = "in the secret file"
		}
		t.Run(name, func(t *testing.T) {
			c := newShellCase(t)
			c.stub("mariadb-dump", "exit 0")
			pwned := filepath.Join(c.records, "pwned")

			for _, password := range []string{"$(touch " + pwned + ")", "; touch " + pwned + " #"} {
				env := []string{"MYSQL_ROOT_PASSWORD=" + password}
				if fromFile {
					env = []string{"MYSQL_ROOT_PASSWORD_FILE=" + c.secretFile("rootpw", password+"\n")}
				}
				res := c.dump(dbdump.EngineMariaDB, env)
				if res.exit != 0 {
					t.Fatalf("exit %d, stderr %q", res.exit, res.stderr)
				}
				if _, err := os.Stat(pwned); err == nil {
					t.Fatalf("the script evaluated %q", password)
				}
				if got := c.call("mariadb-dump"); got.env["MYSQL_PWD"] != password {
					t.Errorf("MYSQL_PWD = %q, want the literal %q", got.env["MYSQL_PWD"], password)
				}
			}
		})
	}
}

func TestScriptPrintsPidOfTheExecedProcess(t *testing.T) {
	pidOf := func(t *testing.T, res shellResult) int {
		t.Helper()
		pid, ok := dbdump.ParsePID(strings.Split(res.stderr, "\n"))
		if !ok {
			t.Fatalf("no pid line in %q", res.stderr)
		}
		return pid
	}

	t.Run("without timeout the shell becomes the dump tool", func(t *testing.T) {
		c := newShellCase(t)
		c.onlyStubs = true
		c.stub("pg_dumpall", "exit 0")

		res := c.dump(dbdump.EnginePostgres, nil)
		if res.exit != 0 {
			t.Fatalf("exit %d, stderr %q", res.exit, res.stderr)
		}
		if got := c.call("pg_dumpall").pid; got != strconv.Itoa(pidOf(t, res)) {
			t.Errorf("dump tool ran as pid %s, the pid line says %d", got, pidOf(t, res))
		}
	})

	t.Run("a timeout that execs keeps the pid", func(t *testing.T) {
		c := newShellCase(t)
		c.onlyStubs = true
		c.stub("pg_dumpall", "exit 0")
		c.write("timeout", "#!/bin/sh\nif [ \"$1\" = 1 ]; then exit 0; fi\nshift\nexec \"$@\"\n")

		res := c.dump(dbdump.EnginePostgres, nil)
		if res.exit != 0 {
			t.Fatalf("exit %d, stderr %q", res.exit, res.stderr)
		}
		if got := c.call("pg_dumpall").pid; got != strconv.Itoa(pidOf(t, res)) {
			t.Errorf("dump tool ran as pid %s, the pid line says %d", got, pidOf(t, res))
		}
	})

	t.Run("a timeout that forks is what the pid line names", func(t *testing.T) {
		c := newShellCase(t)
		c.onlyStubs = true
		c.stub("pg_dumpall", "exit 0")
		c.write("timeout", "#!/bin/sh\nif [ \"$1\" = 1 ]; then exit 0; fi\n"+
			"echo \"pid=$$\" > "+filepath.Join(c.records, "timeout")+"\nshift\n\"$@\"\n")

		res := c.dump(dbdump.EnginePostgres, nil)
		if res.exit != 0 {
			t.Fatalf("exit %d, stderr %q", res.exit, res.stderr)
		}
		line := strconv.Itoa(pidOf(t, res))
		if got := c.call("timeout").pid; got != line {
			t.Errorf("timeout ran as pid %s, the pid line says %s", got, line)
		}
		if got := c.call("pg_dumpall").pid; got == line {
			t.Error("the pid line names the dump tool, not the process the orphan stop can signal")
		}
	})
}

func TestScriptSkipsBrokenTimeout(t *testing.T) {
	record := "timeout"

	t.Run("a timeout that fails its check is not used", func(t *testing.T) {
		c := newShellCase(t)
		c.onlyStubs = true
		c.stub("pg_dumpall", "exit 0")
		c.write("timeout", "#!/bin/sh\nif [ \"$1\" = 1 ]; then exit 1; fi\n"+
			"echo \"pid=$$\" > "+filepath.Join(c.records, record)+"\nshift\nexec \"$@\"\n")

		res := c.dump(dbdump.EnginePostgres, nil)
		if res.exit != 0 {
			t.Fatalf("exit %d, stderr %q", res.exit, res.stderr)
		}
		if c.ran(record) {
			t.Error("the script wrapped the dump in a timeout that cannot run it")
		}
		if !c.ran("pg_dumpall") {
			t.Error("the dump tool never ran")
		}
	})

	t.Run("a working timeout wraps the tool with the seconds", func(t *testing.T) {
		c := newShellCase(t)
		c.onlyStubs = true
		c.stub("pg_dumpall", "exit 0")
		c.write("timeout", "#!/bin/sh\nif [ \"$1\" = 1 ]; then exit 0; fi\n"+
			"{ for a in \"$@\"; do echo \"arg=$a\"; done; echo \"pid=$$\"; } > "+
			filepath.Join(c.records, record)+"\nshift\nexec \"$@\"\n")

		res := c.dump(dbdump.EnginePostgres, nil)
		if res.exit != 0 {
			t.Fatalf("exit %d, stderr %q", res.exit, res.stderr)
		}
		want := []string{"3600", "pg_dumpall", "--no-password"}
		if got := c.call(record).args; !slices.Equal(got, want) {
			t.Errorf("timeout argv = %q, want %q", got, want)
		}
	})
}

func TestProbeScriptPrintsVersionAndDatabases(t *testing.T) {
	t.Run("postgres", func(t *testing.T) {
		c := newShellCase(t)
		c.stub("pg_dumpall", "case \"$1\" in --version) echo \"pg_dumpall (PostgreSQL) 16.4\" ;; esac\nexit 0")
		c.stub("psql", "printf 'postgres\\nimmich\\n'\nexit 0")

		argv, err := dbdump.ProbeArgv(dbdump.EnginePostgres)
		if err != nil {
			t.Fatalf("ProbeArgv: %v", err)
		}
		res := c.run(argv, nil, "")
		if res.exit != 0 {
			t.Fatalf("exit %d, stderr %q", res.exit, res.stderr)
		}
		probe := dbdump.ParseProbe(res.stdout)
		if probe.Version != "16.4" {
			t.Errorf("version = %q, want 16.4", probe.Version)
		}
		if want := []string{"postgres", "immich"}; !slices.Equal(probe.Databases, want) {
			t.Errorf("databases = %q, want %q", probe.Databases, want)
		}
	})

	t.Run("mariadb as root", func(t *testing.T) {
		c := newShellCase(t)
		c.stub("mariadb-dump", "case \"$1\" in --version) echo \"mariadb-dump from 11.4.5-MariaDB, client 15.2 for debian-linux-gnu (x86_64)\" ;; esac\nexit 0")
		c.stub("mariadb", "printf 'information_schema\\nnextcloud\\nperformance_schema\\nsys\\n'\nexit 0")

		argv, err := dbdump.ProbeArgv(dbdump.EngineMariaDB)
		if err != nil {
			t.Fatalf("ProbeArgv: %v", err)
		}
		res := c.run(argv, []string{"MARIADB_ROOT_PASSWORD=r"}, "")
		if res.exit != 0 {
			t.Fatalf("exit %d, stderr %q", res.exit, res.stderr)
		}
		probe := dbdump.ParseProbe(res.stdout)
		if probe.Version != "11.4.5" {
			t.Errorf("version = %q, want 11.4.5", probe.Version)
		}
		if want := []string{"nextcloud"}; !slices.Equal(probe.Databases, want) {
			t.Errorf("databases = %q, want %q", probe.Databases, want)
		}
	})

	t.Run("one database only", func(t *testing.T) {
		c := newShellCase(t)
		c.stub("mariadb-dump", "case \"$1\" in --version) echo \"mariadb-dump from 11.4.5-MariaDB\" ;; esac\nexit 0")

		argv, err := dbdump.ProbeArgv(dbdump.EngineMariaDB)
		if err != nil {
			t.Fatalf("ProbeArgv: %v", err)
		}
		res := c.run(argv, []string{"MARIADB_USER=app", "MARIADB_PASSWORD=p", "MARIADB_DATABASE=appdb"}, "")
		if res.exit != 0 {
			t.Fatalf("exit %d, stderr %q", res.exit, res.stderr)
		}
		if want := []string{"appdb"}; !slices.Equal(dbdump.ParseProbe(res.stdout).Databases, want) {
			t.Errorf("databases = %q, want %q", dbdump.ParseProbe(res.stdout).Databases, want)
		}
	})
}

func TestReadyScriptWaitsForTheRealServer(t *testing.T) {
	ready := func(t *testing.T, c *shellCase, engine dbdump.Engine, env []string) shellResult {
		t.Helper()
		argv, err := dbdump.ReadyArgv(engine)
		if err != nil {
			t.Fatalf("ReadyArgv: %v", err)
		}
		return c.run(argv, env, "")
	}
	// The entrypoint's temporary server answers on the socket and not over TCP,
	// and so does a server configured to listen on the socket alone.
	const socketOnly = `for a in "$@"; do case "$a" in -h|--protocol=TCP) exit 2 ;; esac; done
exit 0`
	rootEnv := []string{"MARIADB_ROOT_PASSWORD=r"}

	t.Run("postgres during initialisation", func(t *testing.T) {
		c := newShellCase(t)
		c.stub("pg_dumpall", "exit 0")
		c.stub("pg_isready", socketOnly)
		runEntrypoint(t)

		if res := ready(t, c, dbdump.EnginePostgres, nil); res.exit == 0 {
			t.Error("the temporary server passed the ready check")
		}
	})

	t.Run("postgres once the real server listens", func(t *testing.T) {
		c := newShellCase(t)
		c.stub("pg_dumpall", "exit 0")
		c.stub("pg_isready", "exit 0")

		if res := ready(t, c, dbdump.EnginePostgres, nil); res.exit != 0 {
			t.Fatalf("exit %d, stderr %q", res.exit, res.stderr)
		}
		if got := c.call("pg_isready"); !got.hasArg("-h") {
			t.Errorf("pg_isready argv = %q, want a TCP host", got.args)
		}
	})

	t.Run("postgres that listens on the socket alone", func(t *testing.T) {
		c := newShellCase(t)
		c.stub("pg_dumpall", "exit 0")
		c.stub("pg_isready", socketOnly)

		if res := ready(t, c, dbdump.EnginePostgres, nil); res.exit != 0 {
			t.Fatalf("exit %d, stderr %q", res.exit, res.stderr)
		}
	})

	t.Run("mariadb during initialisation", func(t *testing.T) {
		c := newShellCase(t)
		c.stub("mariadb-dump", "exit 0")
		c.stub("mariadb-admin", socketOnly)
		c.stub("mariadb", "exit 0")
		runEntrypoint(t)

		if res := ready(t, c, dbdump.EngineMariaDB, rootEnv); res.exit == 0 {
			t.Error("the temporary server passed the ready check")
		}
	})

	t.Run("mariadb that listens on the socket alone", func(t *testing.T) {
		c := newShellCase(t)
		c.stub("mariadb-dump", "exit 0")
		c.stub("mariadb-admin", socketOnly)
		c.stub("mariadb", "exit 0")

		if res := ready(t, c, dbdump.EngineMariaDB, rootEnv); res.exit != 0 {
			t.Fatalf("exit %d, stderr %q", res.exit, res.stderr)
		}
		if got := c.call("mariadb"); !got.hasArg("SELECT 1") {
			t.Errorf("mariadb argv = %q, want the query that proves the login", got.args)
		}
	})

	t.Run("mariadb that answers but refuses the login", func(t *testing.T) {
		c := newShellCase(t)
		c.stub("mariadb-dump", "exit 0")
		c.stub("mariadb-admin", "exit 0")
		c.stub("mariadb", "echo 'ERROR 1045 (28000): Access denied' >&2; exit 1")

		if res := ready(t, c, dbdump.EngineMariaDB, rootEnv); res.exit == 0 {
			t.Error("a refused login passed the ready check")
		}
	})

	t.Run("mariadb ready", func(t *testing.T) {
		c := newShellCase(t)
		c.stub("mariadb-dump", "exit 0")
		c.stub("mariadb-admin", "exit 0")
		c.stub("mariadb", "exit 0")

		if res := ready(t, c, dbdump.EngineMariaDB, rootEnv); res.exit != 0 {
			t.Fatalf("exit %d, stderr %q", res.exit, res.stderr)
		}
		got := c.call("mariadb")
		if !got.hasArg("--user=root") || !got.hasArg("SELECT 1") {
			t.Errorf("mariadb argv = %q, want a query as root", got.args)
		}
		if got.env["MYSQL_PWD"] != "r" {
			t.Errorf("MYSQL_PWD = %q, want r", got.env["MYSQL_PWD"])
		}
	})
}

// runEntrypoint keeps a process named like the official images' entrypoint
// script running until the test ends, as one runs while it initialises a data
// folder.
func runEntrypoint(t *testing.T) {
	t.Helper()
	if _, err := os.Stat("/proc/self/comm"); err != nil {
		t.Skip("no /proc to find the entrypoint in")
	}
	script := filepath.Join(t.TempDir(), "docker-entrypoint.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nwhile :; do sleep 1; done\n"), 0o755); err != nil { //nolint:gosec // G306: the script has to be executable
		t.Fatal(err)
	}
	cmd := exec.Command(script) //nolint:gosec // G204: a script this test wrote
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
}

func TestImportScriptReadsStdin(t *testing.T) {
	dump := "-- PostgreSQL database cluster dump\nCREATE ROLE immich;\n-- PostgreSQL database cluster dump complete\n"

	t.Run("postgres", func(t *testing.T) {
		c := newShellCase(t)
		c.stub("pg_dumpall", "exit 0")
		c.stub("psql", "cat > "+filepath.Join(c.records, "psql.stdin")+"\nexit 0")

		argv, err := dbdump.ImportArgv(dbdump.EnginePostgres)
		if err != nil {
			t.Fatalf("ImportArgv: %v", err)
		}
		res := c.run(argv, nil, dump)
		if res.exit != 0 {
			t.Fatalf("exit %d, stderr %q", res.exit, res.stderr)
		}
		want := []string{"-X", "--no-password", "-v", "ON_ERROR_STOP=0", "-d", "postgres"}
		if got := c.call("psql").args; !slices.Equal(got, want) {
			t.Errorf("psql argv = %q, want %q", got, want)
		}
		if got := c.stdinOf("psql"); got != dump {
			t.Errorf("psql read %q, want the dump unchanged", got)
		}
	})

	t.Run("mariadb as root", func(t *testing.T) {
		c := newShellCase(t)
		c.stub("mariadb-dump", "exit 0")
		c.stub("mariadb", "cat > "+filepath.Join(c.records, "mariadb.stdin")+"\nexit 0")

		argv, err := dbdump.ImportArgv(dbdump.EngineMariaDB)
		if err != nil {
			t.Fatalf("ImportArgv: %v", err)
		}
		res := c.run(argv, []string{"MARIADB_ROOT_PASSWORD=r"}, dump)
		if res.exit != 0 {
			t.Fatalf("exit %d, stderr %q", res.exit, res.stderr)
		}
		got := c.call("mariadb")
		if want := []string{"--user=root"}; !slices.Equal(got.args, want) {
			t.Errorf("mariadb argv = %q, want %q", got.args, want)
		}
		if got.env["MYSQL_PWD"] != "r" {
			t.Errorf("MYSQL_PWD = %q, want r", got.env["MYSQL_PWD"])
		}
		if got := c.stdinOf("mariadb"); got != dump {
			t.Errorf("mariadb read %q, want the dump unchanged", got)
		}
	})

	t.Run("mariadb as the app user", func(t *testing.T) {
		c := newShellCase(t)
		c.stub("mariadb-dump", "exit 0")
		c.stub("mariadb", "cat > "+filepath.Join(c.records, "mariadb.stdin")+"\nexit 0")

		argv, err := dbdump.ImportArgv(dbdump.EngineMariaDB)
		if err != nil {
			t.Fatalf("ImportArgv: %v", err)
		}
		res := c.run(argv, []string{"MARIADB_USER=app", "MARIADB_PASSWORD=p", "MARIADB_DATABASE=appdb"}, dump)
		if res.exit != 0 {
			t.Fatalf("exit %d, stderr %q", res.exit, res.stderr)
		}
		got := c.call("mariadb")
		if want := []string{"--user=app", "appdb"}; !slices.Equal(got.args, want) {
			t.Errorf("mariadb argv = %q, want %q", got.args, want)
		}
		if got.env["MYSQL_PWD"] != "p" {
			t.Errorf("MYSQL_PWD = %q, want p", got.env["MYSQL_PWD"])
		}
	})
}
