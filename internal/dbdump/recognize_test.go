package dbdump_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/dbdump"
)

func TestCanonicalRepo(t *testing.T) {
	tests := []struct {
		name  string
		image string
		want  string
	}{
		{"bare hub name", "postgres", "docker.io/library/postgres"},
		{"tag dropped", "postgres:16-alpine", "docker.io/library/postgres"},
		{"digest dropped", "docker.io/library/postgres@sha256:2f1a", "docker.io/library/postgres"},
		{"tag and digest dropped", "postgres:16@sha256:2f1a", "docker.io/library/postgres"},
		{"hub mirror folded", "index.docker.io/postgres", "docker.io/library/postgres"},
		{"pull mirror folded", "registry-1.docker.io/library/mysql:8.4", "docker.io/library/mysql"},
		{"other registry kept", "lscr.io/linuxserver/mariadb:11.4.5", "lscr.io/linuxserver/mariadb"},
		{"registry port kept", "registry.example:5000/team/postgres:16", "registry.example:5000/team/postgres"},
		{"registry port without a tag", "registry.example:5000/team/postgres", "registry.example:5000/team/postgres"},
		{"localhost lower cased", "LOCALHOST/x", "localhost/x"},
		{"surrounding space trimmed", "  mariadb:11.4  ", "docker.io/library/mariadb"},
		{"empty", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := dbdump.CanonicalRepo(tc.image); got != tc.want {
				t.Errorf("CanonicalRepo(%q) = %q, want %q", tc.image, got, tc.want)
			}
		})
	}
}

func TestEngineForCuratedList(t *testing.T) {
	tests := []struct {
		image string
		want  dbdump.Engine
	}{
		{"postgres", dbdump.EnginePostgres},
		{"postgres:16-alpine", dbdump.EnginePostgres},
		{"docker.io/library/postgres@sha256:2f1a", dbdump.EnginePostgres},
		{"index.docker.io/postgres:17", dbdump.EnginePostgres},
		{"postgis/postgis:16-3.4", dbdump.EnginePostgres},
		{"docker.io/postgis/postgis", dbdump.EnginePostgres},
		{"timescale/timescaledb:2.17.2-pg16", dbdump.EnginePostgres},
		{"pgvector/pgvector:pg17", dbdump.EnginePostgres},
		{"ghcr.io/immich-app/postgres:14-vectorchord0.4.3", dbdump.EnginePostgres},
		{"tensorchord/pgvecto-rs:pg14-v0.2.0", dbdump.EnginePostgres},
		{"pgautoupgrade/pgautoupgrade:17-alpine", dbdump.EnginePostgres},
		{"mysql", dbdump.EngineMySQL},
		{"mysql:8.4", dbdump.EngineMySQL},
		{"mysql/mysql-server:8.0", dbdump.EngineMySQL},
		{"mariadb:11.4", dbdump.EngineMariaDB},
		{"docker.io/library/mariadb", dbdump.EngineMariaDB},
		{"lscr.io/linuxserver/mariadb:11.4.5", dbdump.EngineMariaDB},
		{"ghcr.io/linuxserver/mariadb", dbdump.EngineMariaDB},
		{"linuxserver/mariadb:latest", dbdump.EngineMariaDB},
		{"yobasystems/alpine-mariadb:11.4.5", dbdump.EngineMariaDB},
		{"jc21/mariadb-aria:latest", dbdump.EngineMariaDB},
	}
	for _, tc := range tests {
		t.Run(tc.image, func(t *testing.T) {
			if got := dbdump.EngineFor(tc.image); got != tc.want {
				t.Errorf("EngineFor(%q) = %q, want %q", tc.image, got, tc.want)
			}
		})
	}
}

func TestEngineForRejectsLookalikes(t *testing.T) {
	tests := []struct {
		image    string
		envNames []string
	}{
		{"prometheuscommunity/postgres-exporter:v0.16.0", []string{"POSTGRES_PASSWORD"}},
		{"prom/mysqld-exporter", []string{"MYSQL_ROOT_PASSWORD"}},
		{"bitnami/postgresql:17", []string{"POSTGRES_PASSWORD"}},
		{"bitnami/mariadb:11.4", []string{"MARIADB_ROOT_PASSWORD"}},
		{"bitnami/mysql:8.4", []string{"MYSQL_ROOT_PASSWORD"}},
		{"arm64v8/mysql:8.4", []string{"MYSQL_ROOT_PASSWORD"}},
		{"dpage/pgadmin4", []string{"POSTGRES_PASSWORD"}},
		{"phpmyadmin:5", []string{"MYSQL_ROOT_PASSWORD"}},
		{"adminer", []string{"MYSQL_ROOT_PASSWORD"}},
		{"ghcr.io/immich-app/immich-server:v1.119.0", []string{"POSTGRES_PASSWORD"}},
		{"timescale/timescaledb-ha:pg16", []string{"POSTGRES_PASSWORD"}},
		{"prodrigestivill/postgres-backup-local:16", []string{"POSTGRES_PASSWORD"}},
		{"", []string{"POSTGRES_PASSWORD"}},
		{"sha256:c0ffee0000000000000000000000000000000000000000000000000000000000", []string{"POSTGRES_PASSWORD"}},
	}
	for _, tc := range tests {
		t.Run(tc.image, func(t *testing.T) {
			if got := dbdump.EngineFor(tc.image); got != dbdump.EngineNone {
				t.Errorf("EngineFor(%q) = %q, want none", tc.image, got)
			}
			if got := dbdump.LookalikeEngine(tc.image, tc.envNames); got != dbdump.EngineNone {
				t.Errorf("LookalikeEngine(%q) = %q, want none", tc.image, got)
			}
		})
	}
}

func TestLookalikeEngine(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		envNames []string
		want     dbdump.Engine
	}{
		{
			name:     "private mirror with the official variables",
			image:    "myregistry.example:5000/postgres:16",
			envNames: []string{"PATH", "POSTGRES_PASSWORD"},
			want:     dbdump.EnginePostgres,
		},
		{
			name:     "private mirror without the official variables",
			image:    "myregistry.example:5000/postgres:16",
			envNames: []string{"PATH", "PGDATA"},
			want:     dbdump.EngineNone,
		},
		{
			name:     "bitnami keeps its own variables",
			image:    "bitnami/postgresql:17",
			envNames: []string{"POSTGRESQL_PASSWORD"},
			want:     dbdump.EngineNone,
		},
		{
			name:     "custom mariadb build",
			image:    "somebody/mariadb-custom:1",
			envNames: []string{"MYSQL_ROOT_PASSWORD"},
			want:     dbdump.EngineMariaDB,
		},
		{
			name:     "curated image is tier one, not a lookalike",
			image:    "postgres:16",
			envNames: []string{"POSTGRES_PASSWORD"},
			want:     dbdump.EngineNone,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := dbdump.LookalikeEngine(tc.image, tc.envNames); got != tc.want {
				t.Errorf("LookalikeEngine(%q, %v) = %q, want %q", tc.image, tc.envNames, got, tc.want)
			}
		})
	}
}

func TestLabelDecision(t *testing.T) {
	tests := []struct {
		name       string
		labels     map[string]string
		want       dbdump.LabelDecision
		wantEngine dbdump.Engine
	}{
		{"false", map[string]string{dbdump.LabelKey: "false"}, dbdump.LabelOff, dbdump.EngineNone},
		{"zero", map[string]string{dbdump.LabelKey: "0"}, dbdump.LabelOff, dbdump.EngineNone},
		{"no", map[string]string{dbdump.LabelKey: "no"}, dbdump.LabelOff, dbdump.EngineNone},
		{"off padded and upper case", map[string]string{dbdump.LabelKey: "  OFF "}, dbdump.LabelOff, dbdump.EngineNone},
		{"postgres", map[string]string{dbdump.LabelKey: "postgres"}, dbdump.LabelForceEngine, dbdump.EnginePostgres},
		{"mysql mixed case", map[string]string{dbdump.LabelKey: "MySQL"}, dbdump.LabelForceEngine, dbdump.EngineMySQL},
		{"mariadb padded", map[string]string{dbdump.LabelKey: " mariadb "}, dbdump.LabelForceEngine, dbdump.EngineMariaDB},
		{"true", map[string]string{dbdump.LabelKey: "true"}, dbdump.LabelDefault, dbdump.EngineNone},
		{"empty", map[string]string{dbdump.LabelKey: ""}, dbdump.LabelDefault, dbdump.EngineNone},
		{"yes", map[string]string{dbdump.LabelKey: "yes"}, dbdump.LabelDefault, dbdump.EngineNone},
		{"abbreviation", map[string]string{dbdump.LabelKey: "pg"}, dbdump.LabelDefault, dbdump.EngineNone},
		{"other labels only", map[string]string{"net.unraid.docker.icon": "x"}, dbdump.LabelDefault, dbdump.EngineNone},
		{"no labels", nil, dbdump.LabelDefault, dbdump.EngineNone},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, engine := dbdump.LabelDecisionFor(tc.labels)
			if got != tc.want || engine != tc.wantEngine {
				t.Errorf("LabelDecisionFor(%v) = %v, %q, want %v, %q", tc.labels, got, engine, tc.want, tc.wantEngine)
			}
		})
	}
}

func TestResolvePrecedence(t *testing.T) {
	off := map[string]string{dbdump.LabelKey: "off"}
	tests := []struct {
		name     string
		image    string
		envNames []string
		labels   map[string]string
		chosen   dbdump.Engine
		want     dbdump.Recognition
	}{
		{
			name:  "curated image dumps by default",
			image: "ghcr.io/immich-app/postgres:14-vectorchord0.4.3",
			want:  dbdump.Recognition{Engine: dbdump.EnginePostgres, Tier: dbdump.TierCurated, Default: true},
		},
		{
			name:   "label off beats the curated list",
			image:  "postgres:16",
			labels: off,
			want:   dbdump.Recognition{Engine: dbdump.EnginePostgres, Tier: dbdump.TierCurated, LabelOff: true},
		},
		{
			name:     "label off beats a chosen engine",
			image:    "somebody/mariadb-custom:1",
			envNames: []string{"MYSQL_ROOT_PASSWORD"},
			labels:   off,
			chosen:   dbdump.EngineMariaDB,
			want: dbdump.Recognition{
				Engine:    dbdump.EngineMariaDB,
				Suggested: dbdump.EngineMariaDB,
				Tier:      dbdump.TierLookalike,
				LabelOff:  true,
			},
		},
		{
			name:   "label engine dumps an unlisted image",
			image:  "acme/ledger-db:3",
			labels: map[string]string{dbdump.LabelKey: "postgres"},
			want:   dbdump.Recognition{Engine: dbdump.EnginePostgres, Tier: dbdump.TierLabel, Default: true},
		},
		{
			name:   "label engine beats the curated list",
			image:  "mysql:8.4",
			labels: map[string]string{dbdump.LabelKey: "mariadb"},
			want:   dbdump.Recognition{Engine: dbdump.EngineMariaDB, Tier: dbdump.TierLabel, Default: true},
		},
		{
			name:     "chosen engine switches a lookalike on",
			image:    "somebody/mariadb-custom:1",
			envNames: []string{"MYSQL_ROOT_PASSWORD"},
			chosen:   dbdump.EngineMariaDB,
			want: dbdump.Recognition{
				Engine:    dbdump.EngineMariaDB,
				Suggested: dbdump.EngineMariaDB,
				Tier:      dbdump.TierLookalike,
				Default:   true,
			},
		},
		{
			name:     "lookalike without a choice is only offered",
			image:    "somebody/mariadb-custom:1",
			envNames: []string{"MYSQL_ROOT_PASSWORD"},
			want:     dbdump.Recognition{Suggested: dbdump.EngineMariaDB, Tier: dbdump.TierLookalike},
		},
		{
			name:   "chosen engine beats the curated list",
			image:  "postgres:16",
			chosen: dbdump.EngineMariaDB,
			want:   dbdump.Recognition{Engine: dbdump.EngineMariaDB, Tier: dbdump.TierCurated, Default: true},
		},
		{
			name:     "exporter is no database",
			image:    "prometheuscommunity/postgres-exporter:v0.16.0",
			envNames: []string{"POSTGRES_PASSWORD"},
			want:     dbdump.Recognition{},
		},
		{
			name:  "plain image is no database",
			image: "nginx:1.27",
			want:  dbdump.Recognition{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := dbdump.Resolve(tc.image, tc.envNames, tc.labels, tc.chosen)
			if got != tc.want {
				t.Errorf("Resolve(%q, %v, %v, %q) = %+v, want %+v", tc.image, tc.envNames, tc.labels, tc.chosen, got, tc.want)
			}
		})
	}
}
