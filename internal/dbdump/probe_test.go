package dbdump_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/dbdump"
)

func TestParseProbe(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want dbdump.Probe
	}{
		{
			"postgres",
			"bombvault-dbdump-version pg_dumpall (PostgreSQL) 16.4\n" +
				"bombvault-dbdump-db postgres\n" +
				"bombvault-dbdump-db immich\n",
			dbdump.Probe{Version: "16.4", Databases: []string{"postgres", "immich"}},
		},
		{
			"mariadb drops the system schemas",
			"bombvault-dbdump-version mariadb-dump from 11.4.5-MariaDB, client 15.2 for debian-linux-gnu (x86_64)\n" +
				"bombvault-dbdump-db information_schema\n" +
				"bombvault-dbdump-db nextcloud\n" +
				"bombvault-dbdump-db performance_schema\n" +
				"bombvault-dbdump-db sys\n",
			dbdump.Probe{Version: "11.4.5", Databases: []string{"nextcloud"}},
		},
		{
			"mariadb of the newest format",
			"bombvault-dbdump-version mariadb-dump from 13.0.2-MariaDB, client 10.20\n",
			dbdump.Probe{Version: "13.0.2"},
		},
		{
			"mariadb 10.6, whose banner leads with the dump tool's own version",
			"bombvault-dbdump-version mariadb-dump  Ver 10.19 Distrib 10.6.28-MariaDB\n" +
				"bombvault-dbdump-db nextcloud\n",
			dbdump.Probe{Version: "10.6.28", Databases: []string{"nextcloud"}},
		},
		{
			"mariadb-aria of jc21",
			"bombvault-dbdump-version mariadb-dump  Ver 10.19 Distrib 10.11.5-MariaDB\n",
			dbdump.Probe{Version: "10.11.5"},
		},
		{
			"mysql 5.7, the same two-version banner",
			"bombvault-dbdump-version mysqldump  Ver 10.13 Distrib 5.7.44, for Linux (x86_64)\n",
			dbdump.Probe{Version: "5.7.44"},
		},
		{
			"mysql",
			"bombvault-dbdump-version mysqldump  Ver 8.0.39 for Linux on x86_64 (MySQL Community Server - GPL)\n" +
				"bombvault-dbdump-db wordpress\n",
			dbdump.Probe{Version: "8.0.39", Databases: []string{"wordpress"}},
		},
		{
			"a name that cannot be a tag is dropped and counted",
			"bombvault-dbdump-db one,two\nbombvault-dbdump-db plain\n",
			dbdump.Probe{Databases: []string{"plain"}, Dropped: 1},
		},
		{"no output", "", dbdump.Probe{}},
		{"version without a number", "bombvault-dbdump-version pg_dumpall (PostgreSQL) devel\n", dbdump.Probe{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := dbdump.ParseProbe(tc.out)
			if got.Version != tc.want.Version || got.Dropped != tc.want.Dropped ||
				!reflect.DeepEqual(got.Databases, tc.want.Databases) {
				t.Errorf("ParseProbe = %+v, want %+v", got, tc.want)
			}
		})
	}

	many := ""
	for i := range 40 {
		many += fmt.Sprintf("bombvault-dbdump-db app%02d\n", i)
	}
	got := dbdump.ParseProbe(many)
	if len(got.Databases) != 32 || got.Dropped != 8 {
		t.Errorf("ParseProbe kept %d names and dropped %d, want 32 and 8", len(got.Databases), got.Dropped)
	}
}

func TestVersionMajorOK(t *testing.T) {
	tests := []struct {
		engine       dbdump.Engine
		dump, server string
		want         bool
	}{
		{dbdump.EnginePostgres, "14.2", "16.4", true},
		{dbdump.EnginePostgres, "16.4", "16.4", true},
		{dbdump.EnginePostgres, "16.4", "14.2", false},
		{dbdump.EngineMariaDB, "11.4.5", "11.8.2", true},
		{dbdump.EngineMariaDB, "11.4.5", "12.3.0", false},
		{dbdump.EngineMySQL, "8.0.39", "8.4.0", true},
		{dbdump.EngineMySQL, "8.0.39", "9.1.0", false},
		{dbdump.EnginePostgres, "", "16.4", true},
		{dbdump.EnginePostgres, "16.4", "", true},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s %s into %s", tc.engine, tc.dump, tc.server), func(t *testing.T) {
			if got := dbdump.VersionMajorOK(tc.engine, tc.dump, tc.server); got != tc.want {
				t.Errorf("VersionMajorOK = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCountImportErrors(t *testing.T) {
	psql := `SET
ERROR:  role "immich" already exists
CREATE ROLE
ERROR:  relation "assets" already exists
`
	if got := dbdump.CountImportErrors(dbdump.EnginePostgres, psql, "immich"); got != 1 {
		t.Errorf("CountImportErrors = %d, want 1", got)
	}
	if got := dbdump.CountImportErrors(dbdump.EnginePostgres, psql, "other"); got != 2 {
		t.Errorf("CountImportErrors with another user = %d, want 2", got)
	}
	twice := psql + `ERROR:  role "immich" already exists` + "\n"
	if got := dbdump.CountImportErrors(dbdump.EnginePostgres, twice, "immich"); got != 2 {
		t.Errorf("CountImportErrors tolerated the expected line twice: %d, want 2", got)
	}

	maria := "ERROR 1064 (42000) at line 12: You have an error in your SQL syntax\n"
	if got := dbdump.CountImportErrors(dbdump.EngineMariaDB, maria, "root"); got != 1 {
		t.Errorf("CountImportErrors = %d, want 1", got)
	}
	if got := dbdump.CountImportErrors(dbdump.EnginePostgres, "", ""); got != 0 {
		t.Errorf("CountImportErrors of empty stderr = %d, want 0", got)
	}
}
