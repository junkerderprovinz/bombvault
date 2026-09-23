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

// mysqlPreamble also settles the root-or-user decision and how that account
// reaches the server, because the probe, the ready check and the import all
// have to take the same ones as the dump. A template that carries a root
// password and a non-empty random flag at once is decided by asking the
// server: a root login that works means root is real.
//
// The images disagree about which accounts they create. linuxserver/mariadb
// puts MYSQL_ROOT_PASSWORD on root@'%' and leaves the local root accounts
// without a password; yobasystems/alpine-mariadb creates the app user as
// user@'%' and keeps the anonymous local account that wins over it on the
// socket. Both refuse the socket login with 1045 and take the same credentials
// one step to the side, so bv_resolve settles the login before the dump
// starts: the socket, then TCP where a '%' grant applies, then, for root
// alone, the local account without a password, which has to answer as root so
// an anonymous account cannot pass for it. A refused local login costs about
// ten milliseconds, and an account nobody grants keeps the configured password
// on the socket, so the dump fails with the server's own message.
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
conn=""; pw=""
bv_try() { MYSQL_PWD="$pw" "$client" $conn --user="$1" -N -B -e 'SELECT 1' </dev/null >/dev/null 2>&1; }
bv_resolve() {
  pw="$2"; conn=""
  bv_try "$1" && return 0
  conn="--protocol=TCP --host=127.0.0.1"
  bv_try "$1" && return 0
  conn=""
  if [ "$1" = root ]; then
    case "$(MYSQL_PWD= "$client" --user=root -N -B -e 'SELECT CURRENT_USER()' </dev/null 2>/dev/null)" in
      root@*) pw=""; return 0 ;;
    esac
  fi
  pw="$2"
  return 1
}
user=""; db=""; scope=database
if [ -n "$rootpw" ] || [ -n "$empty" ]; then
  pw="$rootpw"
  if [ -n "$client" ] && bv_resolve root "$rootpw"; then
    scope=all
  elif [ -z "$random" ]; then
    scope=all
  fi
fi
if [ "$scope" = database ]; then
  user="${MARIADB_USER:-${MYSQL_USER:-}}"; db="${MARIADB_DATABASE:-${MYSQL_DATABASE:-}}"
  { [ -n "$user" ] && [ -n "$db" ]; } || exit 66
  conn=""; pw="${MARIADB_PASSWORD:-${MYSQL_PASSWORD:-}}"
  if [ -n "$client" ]; then bv_resolve "$user" "$pw"; fi
fi
MYSQL_PWD="$pw"; export MYSQL_PWD
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
  set -- "$tool" $common $conn --user=root --all-databases
else
  set -- "$tool" $common $conn --no-tablespaces --user="$user" --databases "$db"
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
elif [ -n "$client" ]; then "$client" $conn -N -B --user=root -e 'SHOW DATABASES' 2>/dev/null |
  while IFS= read -r n; do echo "bombvault-dbdump-db $n"; done
fi
exit 0
`

// The ready checks try TCP first because the official entrypoints initialise a
// fresh data folder under a temporary server that listens on the socket alone
// and is shut down again before the real one starts. A server set up to listen
// on the socket or on another address only is asked over the socket once no
// entrypoint script runs any more; the kernel names a script's process after
// the script, cut to 15 bytes. ping answers 0 even when the login is refused,
// so a real query proves the credentials are in place.
const entrypointRunning = `entrypoint_running() {
  for comm in /proc/[0-9]*/comm; do
    read -r name 2>/dev/null <"$comm" || continue
    case "$name" in docker-entrypoi*) return 0 ;; esac
  done
  return 1
}
`

const postgresReadyTail = entrypointRunning + `pg_isready -q -h localhost -d postgres && exit 0
entrypoint_running && exit 1
exec pg_isready -q -d postgres
`

const mysqlReadyTail = entrypointRunning + `admin=$(command -v mariadb-admin 2>/dev/null || command -v mysqladmin 2>/dev/null) || exit 64
[ -n "$client" ] || exit 64
if ! "$admin" --protocol=TCP --host=127.0.0.1 ping >/dev/null 2>&1; then
  entrypoint_running && exit 1
  "$admin" ping >/dev/null 2>&1 || exit 1
fi
if [ "$scope" = all ]; then exec "$client" $conn --user=root -N -B -e 'SELECT 1'; fi
exec "$client" $conn --user="$user" -N -B -e 'SELECT 1' "$db"
`

// ON_ERROR_STOP stays off: pg_dumpall's role section always collides with the
// roles a fresh cluster already has, and the caller counts the ERROR lines.
const postgresImportTail = `exec psql -X --no-password -v ON_ERROR_STOP=0 -d postgres
`

const mysqlImportTail = `[ -n "$client" ] || exit 64
if [ "$scope" = all ]; then exec "$client" $conn --user=root; fi
exec "$client" $conn --user="$user" "$db"
`

// orphanStopScript signals a dump that outlived its helper, deepest process
// first and each one only once the one below it has been reaped. The first
// case makes a recycled pid harmless.
//
// The signal must reach the dump's processes and no others. pg_dumpall runs a
// pg_dump per database through a shell, and coreutils timeout, which the dump
// runs under, is its own process-group leader and forwards a TERM to the whole
// group. Whoever dies first leaves children behind that PID 1 adopts, and PID 1
// in the official image is the postmaster: it takes a signalled child it never
// started for a crashed backend, drops every connection and reinitialises the
// cluster. So no process is signalled while it still has a child of the dump
// under it, and in practice the first signal is enough, because pg_dumpall
// ends when its pg_dump does and timeout ends with the tool it wraps.
const orphanStopScript = `case "$(tr '\0' ' ' </proc/$1/cmdline 2>/dev/null)" in *dump*) ;; *) exit 0 ;; esac
bv_parent() {
  [ -r "/proc/$1/stat" ] || return 1
  read -r bv_stat <"/proc/$1/stat" || return 1
  bv_stat=${bv_stat##*") "}
  set -- $bv_stat
  parent=$2
}
tree=" $1 "
order="$1"
depth=0
while [ "$depth" -lt 8 ]; do
  depth=$((depth + 1)); grew=""
  for entry in /proc/[0-9]*; do
    pid=${entry#/proc/}
    case "$tree" in *" $pid "*) continue ;; esac
    bv_parent "$pid" || continue
    case "$tree" in *" $parent "*) tree="$tree$pid "; order="$pid $order"; grew=y ;; esac
  done
  [ -n "$grew" ] || break
done
budget=60
below=""
for pid in $order; do
  while [ -n "$below" ] && [ "$budget" -gt 0 ] && [ -e "/proc/$below" ]; do
    budget=$((budget - 1))
    sleep 0.05
  done
  kill -TERM "$pid" 2>/dev/null
  below="$pid"
done
exit 0
`

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
