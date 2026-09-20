package dbdump_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/dbdump"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name string
		obs  dbdump.Observation
		want string
	}{
		{
			"cancelled wins over a failing exit",
			dbdump.Observation{Cancelled: true, Exit: 1, StderrTail: "access denied for user"},
			dbdump.ReasonCancelled,
		},
		{"observed timeout", dbdump.Observation{TimedOut: true, Exit: 1}, dbdump.ReasonTimeout},
		{"in-container timeout", dbdump.Observation{Exit: 124}, dbdump.ReasonTimeout},
		{"in-container term", dbdump.Observation{Exit: 143}, dbdump.ReasonTimeout},
		{"container not running", dbdump.Observation{NotRunning: true, Exit: 1}, dbdump.ReasonNotRunning},
		{"docker refused", dbdump.Observation{DockerErr: true, Exit: 1}, dbdump.ReasonDocker},
		{"stdout write failed", dbdump.Observation{WriteFailed: true, Exit: 1}, dbdump.ReasonWrite},
		{"no client", dbdump.Observation{Exit: dbdump.ExitNoClient}, dbdump.ReasonNoClient},
		{"secret unreadable", dbdump.Observation{Exit: dbdump.ExitSecret}, dbdump.ReasonSecret},
		{"no credentials", dbdump.Observation{Exit: dbdump.ExitNoCredentials}, dbdump.ReasonNoCredentials},
		{
			"postgres missing role",
			dbdump.Observation{
				Exit:       1,
				StderrTail: `pg_dumpall: error: connection to server on socket "/var/run/postgresql/.s.PGSQL.5432" failed: FATAL:  role "postgres" does not exist`,
			},
			dbdump.ReasonAuth,
		},
		{
			"mysqldump access denied",
			dbdump.Observation{
				Exit:       2,
				StderrTail: `mysqldump: Got error: 1045: "Access denied for user 'root'@'localhost' (using password: YES)" when trying to connect`,
			},
			dbdump.ReasonAuth,
		},
		{
			"socket unreachable",
			dbdump.Observation{
				Exit:       2,
				StderrTail: `mariadb-dump: Got error: 2002: "Can't connect to local server through socket '/run/mysqld/mysqld.sock' (2)" when trying to connect`,
			},
			dbdump.ReasonUnreachable,
		},
		{
			"flush privilege missing",
			dbdump.Observation{
				Exit:       2,
				StderrTail: `mysqldump: Couldn't execute 'FLUSH TABLES WITH READ LOCK': Access denied; you need (at least one of) the RELOAD or FLUSH_TABLES privilege(s) for this operation (1227)`,
			},
			dbdump.ReasonPrivileges,
		},
		{
			"system tables need an upgrade",
			dbdump.Observation{
				Exit:       2,
				Bytes:      33 * 1024,
				StderrTail: `mariadb-dump: Couldn't execute 'SHOW FUNCTION STATUS WHERE Db = 'filerun'': Column count of mysql.proc is wrong. Expected 22, found 21. Created with MariaDB 110302, now running 120303. Please use mariadb-upgrade to fix this error (1558)`,
			},
			dbdump.ReasonNeedsUpgrade,
		},
		{
			"upgrade message behind an auth-looking line",
			dbdump.Observation{
				Exit:       2,
				StderrTail: "Access denied for user 'root'@'localhost'\nColumn count of mysql.proc is wrong, please use mariadb-upgrade",
			},
			dbdump.ReasonNeedsUpgrade,
		},
		{"tool error without a known message", dbdump.Observation{Exit: 3, StderrTail: "mysqldump: unknown variable"}, dbdump.ReasonTool},
		{"empty dump", dbdump.Observation{Exit: 0, Bytes: 0, MarkerSeen: true}, dbdump.ReasonEmpty},
		{"truncated dump", dbdump.Observation{Exit: 0, Bytes: 1024}, dbdump.ReasonIncomplete},
		{"complete dump", dbdump.Observation{Exit: 0, Bytes: 1024, MarkerSeen: true}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := dbdump.Classify(tc.obs); got != tc.want {
				t.Errorf("Classify = %q, want %q", got, tc.want)
			}
		})
	}
}
