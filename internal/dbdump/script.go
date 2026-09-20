package dbdump

import (
	"fmt"
	"strconv"
)

// Protocol lines the script writes to stderr before it execs the dump tool.
// restic forwards them as "subprocess <argv0>: <line>", which is how the pid
// reaches the BombVault process in time to stop an orphan.
const (
	PIDLinePrefix   = "bombvault-dbdump-pid "
	ScopeLinePrefix = "bombvault-dbdump-scope "
)

// Exit codes the scripts use for the conditions they recognise themselves.
// Everything else is the dump tool's own code.
const (
	ExitNoClient      = 64
	ExitSecret        = 65
	ExitNoCredentials = 66
)

const (
	minMaxSeconds = 60
	maxMaxSeconds = 172800
)

// bvSecret resolves a variable from its own value or from the file conventions
// the images use: <VAR>_FILE (official images and Docker secrets),
// <VAR>__FILE (jc21) and FILE__<VAR> (linuxserver). eval only ever sees the
// variable names written here, never a value.
//
//nolint:gosec // G101: a shell function that reads a password file is not a hardcoded password
const bvSecret = `bv_secret() {
  eval "bv_v=\${$1:-}"; eval "bv_f=\${$1_FILE:-}"; eval "bv_d=\${$1__FILE:-}"; eval "bv_l=\${FILE__$1:-}"
  [ -n "$bv_f" ] || bv_f="$bv_d"
  [ -n "$bv_f" ] || bv_f="$bv_l"
  if [ -z "$bv_v" ] && [ -n "$bv_f" ]; then
    [ -r "$bv_f" ] || exit 65
    bv_v=$(cat "$bv_f") || exit 65
    eval "$1=\$bv_v"; export "$1"
  fi
}
`

const (
	dumpHead  = "umask 077\nmax=\"$1\"\n" + bvSecret
	shellHead = "umask 077\n" + bvSecret
)

const postgresPreamble = `bv_secret POSTGRES_USER
bv_secret POSTGRES_PASSWORD
command -v pg_dumpall >/dev/null 2>&1 || exit 64
PGUSER="${POSTGRES_USER:-postgres}"; export PGUSER
if [ -n "${POSTGRES_PASSWORD:-}" ]; then PGPASSWORD="$POSTGRES_PASSWORD"; export PGPASSWORD; fi
`

// mysqlPreamble also settles the root-or-user decision, because the probe, the
// ready check and the import all have to take the same one as the dump.
// A template that carries a root password and a non-empty random flag at once
// is decided by asking the server: a root login that works means root is real.
const mysqlPreamble = `for v in MARIADB_ROOT_PASSWORD MYSQL_ROOT_PASSWORD MARIADB_USER MYSQL_USER MARIADB_PASSWORD MYSQL_PASSWORD MARIADB_DATABASE MYSQL_DATABASE; do
  bv_secret "$v"
done
tool=$(command -v mariadb-dump 2>/dev/null || command -v mysqldump 2>/dev/null) || exit 64
[ -n "$tool" ] || exit 64
rootpw="${MARIADB_ROOT_PASSWORD:-${MYSQL_ROOT_PASSWORD:-}}"
case "$rootpw" in /*) if [ -f "$rootpw" ] && [ -r "$rootpw" ]; then rootpw=$(cat "$rootpw") || exit 65; fi ;; esac
random="${MARIADB_RANDOM_ROOT_PASSWORD:-${MYSQL_RANDOM_ROOT_PASSWORD:-}}"
empty="${MARIADB_ALLOW_EMPTY_ROOT_PASSWORD:-${MYSQL_ALLOW_EMPTY_PASSWORD:-}}"
common="--quick --routines --events --triggers --hex-blob --default-character-set=utf8mb4 --single-transaction"
client=$(command -v mariadb 2>/dev/null || command -v mysql 2>/dev/null)
if [ -n "$random" ] && [ -n "$rootpw" ] && [ -n "$client" ] &&
   MYSQL_PWD="$rootpw" "$client" --user=root -N -B -e 'SELECT 1' >/dev/null 2>&1; then
  random=""
fi
user=""; db=""
if [ -z "$random" ] && { [ -n "$rootpw" ] || [ -n "$empty" ]; }; then
  scope=all
  if [ -n "$rootpw" ]; then MYSQL_PWD="$rootpw"; export MYSQL_PWD; fi
else
  scope=database
  user="${MARIADB_USER:-${MYSQL_USER:-}}"; db="${MARIADB_DATABASE:-${MYSQL_DATABASE:-}}"
  { [ -n "$user" ] && [ -n "$db" ]; } || exit 66
  MYSQL_PWD="${MARIADB_PASSWORD:-${MYSQL_PASSWORD:-}}"; export MYSQL_PWD
fi
`

// No --clean: a DROP ROLE of the connected role aborts the reload.
const postgresDumpTail = `set -- pg_dumpall --no-password
if timeout 1 true >/dev/null 2>&1; then set -- timeout "$max" "$@"; fi
echo "bombvault-dbdump-scope all" >&2
echo "bombvault-dbdump-pid $$" >&2
exec "$@"
`

// $common stays unquoted so the shell splits it into words. --set-gtid-purged
// keeps Oracle's mysqldump from taking a global read lock under
// --single-transaction; mariadb-dump and the mysqldump of MariaDB 10.x reject
// the option, so its --help decides.
const mysqlDumpTail = `if [ "$scope" = all ]; then
  set -- "$tool" $common --user=root --all-databases
else
  set -- "$tool" $common --no-tablespaces --user="$user" --databases "$db"
fi
case "$tool" in *mysqldump) "$tool" --help 2>/dev/null | grep -q -- --set-gtid-purged && set -- "$@" --set-gtid-purged=OFF ;; esac
echo "bombvault-dbdump-scope $scope" >&2
if timeout 1 true >/dev/null 2>&1; then set -- timeout "$max" "$@"; fi
echo "bombvault-dbdump-pid $$" >&2
exec "$@"
`

const postgresProbeTail = `echo "bombvault-dbdump-version $(pg_dumpall --version 2>/dev/null)"
psql -XAtq --no-password -d postgres \
  -c "select datname from pg_database where datallowconn and not datistemplate order by 1" 2>/dev/null |
  while IFS= read -r n; do echo "bombvault-dbdump-db $n"; done
exit 0
`

const mysqlProbeTail = `echo "bombvault-dbdump-version $("$tool" --version 2>/dev/null)"
if [ "$scope" = database ]; then echo "bombvault-dbdump-db $db"
elif [ -n "$client" ]; then "$client" -N -B --user=root -e 'SHOW DATABASES' 2>/dev/null |
  while IFS= read -r n; do echo "bombvault-dbdump-db $n"; done
fi
exit 0
`

const postgresReadyTail = `exec pg_isready -q -d postgres
`

const mysqlReadyTail = `admin=$(command -v mariadb-admin 2>/dev/null || command -v mysqladmin 2>/dev/null) || exit 64
if [ "$scope" = all ]; then exec "$admin" --user=root ping; fi
exec "$admin" --user="$user" ping
`

// ON_ERROR_STOP stays off: pg_dumpall's role section always collides with the
// roles a fresh cluster already has, and the caller counts the ERROR lines.
const postgresImportTail = `exec psql -X --no-password -v ON_ERROR_STOP=0 -d postgres
`

const mysqlImportTail = `[ -n "$client" ] || exit 64
if [ "$scope" = all ]; then exec "$client" --user=root; fi
exec "$client" --user="$user" "$db"
`

// orphanStopScript signals a dump that outlived its helper. The case makes a
// recycled pid harmless.
const orphanStopScript = `case "$(tr '\0' ' ' </proc/$1/cmdline 2>/dev/null)" in *dump*) kill -TERM "$1";; esac`

func buildScript(e Engine, head, postgresTail, mysqlTail string) (string, error) {
	switch e {
	case EnginePostgres:
		return head + postgresPreamble + postgresTail, nil
	case EngineMySQL, EngineMariaDB:
		return head + mysqlPreamble + mysqlTail, nil
	}
	return "", fmt.Errorf("dbdump: engine %q has no script", e)
}

// Script returns the dump script of an engine. It is a constant per engine:
// container configuration reaches it only as the container shell's own
// variable expansions.
func Script(e Engine) (string, error) {
	return buildScript(e, dumpHead, postgresDumpTail, mysqlDumpTail)
}

// ExecArgv is the argv of a dump exec. maxSeconds is the in-container time
// limit and arrives as $1.
func ExecArgv(e Engine, maxSeconds int) ([]string, error) {
	if maxSeconds < minMaxSeconds || maxSeconds > maxMaxSeconds {
		return nil, fmt.Errorf("dbdump: max runtime %ds is outside %d..%d", maxSeconds, minMaxSeconds, maxMaxSeconds)
	}
	script, err := Script(e)
	if err != nil {
		return nil, err
	}
	return []string{"sh", "-c", script, "bombvault-dbdump", strconv.Itoa(maxSeconds)}, nil
}

// ProbeArgv is the argv of the read-only exec that reports the server version
// and the database names right before a dump.
func ProbeArgv(e Engine) ([]string, error) {
	script, err := buildScript(e, shellHead, postgresProbeTail, mysqlProbeTail)
	if err != nil {
		return nil, err
	}
	return []string{"sh", "-c", script, "bombvault-dbdump-probe"}, nil
}

// ReadyArgv is the argv of the readiness check an import waits on.
func ReadyArgv(e Engine) ([]string, error) {
	script, err := buildScript(e, shellHead, postgresReadyTail, mysqlReadyTail)
	if err != nil {
		return nil, err
	}
	return []string{"sh", "-c", script, "bombvault-dbdump-ready"}, nil
}

// ImportArgv is the argv of the exec that reads a dump from stdin into the
// container's database.
func ImportArgv(e Engine) ([]string, error) {
	script, err := buildScript(e, shellHead, postgresImportTail, mysqlImportTail)
	if err != nil {
		return nil, err
	}
	return []string{"sh", "-c", script, "bombvault-dbdump-import"}, nil
}

// OrphanStopArgv signals the dump process a helper left behind.
func OrphanStopArgv(pid int) ([]string, error) {
	if pid <= 1 {
		return nil, fmt.Errorf("dbdump: %d is not the pid of a dump", pid)
	}
	return []string{"sh", "-c", orphanStopScript, "bv", strconv.Itoa(pid)}, nil
}
