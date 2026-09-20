package dbdump_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/dbdump"
)

func TestParseEngine(t *testing.T) {
	tests := []struct {
		in     string
		want   dbdump.Engine
		wantOK bool
	}{
		{"postgres", dbdump.EnginePostgres, true},
		{"mysql", dbdump.EngineMySQL, true},
		{"mariadb", dbdump.EngineMariaDB, true},
		{"Postgres", dbdump.EngineNone, false},
		{" mysql", dbdump.EngineNone, false},
		{"pg", dbdump.EngineNone, false},
		{"", dbdump.EngineNone, false},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, ok := dbdump.ParseEngine(tc.in)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("ParseEngine(%q) = %q, %v, want %q, %v", tc.in, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestCompletionMarker(t *testing.T) {
	tests := []struct {
		engine dbdump.Engine
		want   string
	}{
		{dbdump.EnginePostgres, "-- PostgreSQL database cluster dump complete"},
		{dbdump.EngineMySQL, "-- Dump completed"},
		{dbdump.EngineMariaDB, "-- Dump completed"},
		{dbdump.EngineNone, ""},
	}
	for _, tc := range tests {
		t.Run(string(tc.engine), func(t *testing.T) {
			if got := tc.engine.CompletionMarker(); got != tc.want {
				t.Errorf("CompletionMarker() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDataMountMatchesAncestor(t *testing.T) {
	tests := []struct {
		name   string
		engine dbdump.Engine
		env    []string
		mounts []dbdump.Mount
		want   string
		wantOK bool
	}{
		{
			name:   "pgdata below the volume",
			engine: dbdump.EnginePostgres,
			env:    []string{"PGDATA=/var/lib/postgresql/data/pgdata"},
			mounts: []dbdump.Mount{{Source: "/mnt/user/appdata/immich-pg", Destination: "/var/lib/postgresql/data"}},
			want:   "/mnt/user/appdata/immich-pg/pgdata",
			wantOK: true,
		},
		{
			name:   "pgdata below a bind outside the image layout",
			engine: dbdump.EnginePostgres,
			env:    []string{"PGDATA=/data/pg"},
			mounts: []dbdump.Mount{{Source: "/mnt/user/databases", Destination: "/data"}},
			want:   "/mnt/user/databases/pg",
			wantOK: true,
		},
		{
			name:   "volume on the data directory itself",
			engine: dbdump.EnginePostgres,
			mounts: []dbdump.Mount{{Source: "/mnt/user/appdata/pg", Destination: "/var/lib/postgresql/data"}},
			want:   "/mnt/user/appdata/pg",
			wantOK: true,
		},
		{
			name:   "deepest of two candidates wins",
			engine: dbdump.EnginePostgres,
			mounts: []dbdump.Mount{
				{Source: "/mnt/user/appdata/pg-home", Destination: "/var/lib/postgresql"},
				{Source: "/mnt/user/appdata/pg-data", Destination: "/var/lib/postgresql/data/"},
			},
			want:   "/mnt/user/appdata/pg-data",
			wantOK: true,
		},
		{
			name:   "mysql data directory",
			engine: dbdump.EngineMariaDB,
			mounts: []dbdump.Mount{{Source: "/mnt/user/appdata/nextcloud-db", Destination: "/var/lib/mysql"}},
			want:   "/mnt/user/appdata/nextcloud-db",
			wantOK: true,
		},
		{
			name:   "linuxserver config directory",
			engine: dbdump.EngineMariaDB,
			mounts: []dbdump.Mount{{Source: "/mnt/user/appdata/mariadb", Destination: "/config"}},
			want:   "/mnt/user/appdata/mariadb",
			wantOK: true,
		},
		{
			name:   "no mount holds the data",
			engine: dbdump.EnginePostgres,
			mounts: []dbdump.Mount{{Source: "/mnt/user/appdata/pg-backup", Destination: "/backup"}},
		},
		{
			name:   "no mounts at all",
			engine: dbdump.EngineMySQL,
		},
		{
			name:   "no engine",
			mounts: []dbdump.Mount{{Source: "/mnt/user/appdata/pg", Destination: "/var/lib/postgresql/data"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := dbdump.DataMount(tc.engine, tc.env, tc.mounts)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("DataMount() = %q, %v, want %q, %v", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
