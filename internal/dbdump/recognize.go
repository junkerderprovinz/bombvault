package dbdump

import "strings"

// LabelKey is the container label that overrides recognition.
const LabelKey = "bombvault.dbdump"

// curatedRepos are the images whose entrypoint and variables follow the
// official conventions the dump scripts expect, so a dump runs by default.
// Everything derived from an official image is listed by its own repository,
// never by a substring, or an exporter would be dumped as a database.
var curatedRepos = map[string]Engine{
	"docker.io/library/postgres":            EnginePostgres,
	"docker.io/postgis/postgis":             EnginePostgres,
	"docker.io/timescale/timescaledb":       EnginePostgres,
	"docker.io/pgvector/pgvector":           EnginePostgres,
	"docker.io/tensorchord/pgvecto-rs":      EnginePostgres,
	"docker.io/pgautoupgrade/pgautoupgrade": EnginePostgres,
	"ghcr.io/immich-app/postgres":           EnginePostgres,
	"docker.io/library/mysql":               EngineMySQL,
	"docker.io/mysql/mysql-server":          EngineMySQL,
	"docker.io/library/mariadb":             EngineMariaDB,
	"docker.io/linuxserver/mariadb":         EngineMariaDB,
	"lscr.io/linuxserver/mariadb":           EngineMariaDB,
	"ghcr.io/linuxserver/mariadb":           EngineMariaDB,
	"docker.io/yobasystems/alpine-mariadb":  EngineMariaDB,
	"docker.io/jc21/mariadb-aria":           EngineMariaDB,
}

// lookalikeNegatives carry a database word in their name without being a
// database server that the scripts can dump: exporters, admin interfaces,
// backup sidecars, per-architecture mirrors and packagings whose variables and
// socket paths mean something else.
var lookalikeNegatives = map[string]bool{
	"docker.io/prometheuscommunity/postgres-exporter": true,
	"docker.io/prom/mysqld-exporter":                  true,
	"docker.io/bitnami/postgresql":                    true,
	"docker.io/bitnami/mariadb":                       true,
	"docker.io/bitnami/mysql":                         true,
	"docker.io/arm64v8/mysql":                         true,
	"docker.io/dpage/pgadmin4":                        true,
	"docker.io/library/phpmyadmin":                    true,
	"docker.io/library/adminer":                       true,
	"docker.io/timescale/timescaledb-ha":              true,
	"docker.io/prodrigestivill/postgres-backup-local": true,
	"ghcr.io/immich-app/immich-server":                true,
}

// CanonicalRepo reduces an image reference to "<registry>/<path>", so that
// every spelling of the same image compares equal.
func CanonicalRepo(image string) string {
	ref := strings.ToLower(strings.TrimSpace(image))
	if ref == "" {
		return ""
	}
	if i := strings.Index(ref, "@"); i >= 0 {
		ref = ref[:i]
	}
	// Only the last segment can carry a tag; a colon before that is a
	// registry port.
	slash := strings.LastIndex(ref, "/")
	if i := strings.LastIndex(ref, ":"); i > slash {
		ref = ref[:i]
	}
	registry := "docker.io"
	if i := strings.Index(ref, "/"); i >= 0 {
		if head := ref[:i]; strings.ContainsAny(head, ".:") || head == "localhost" {
			registry, ref = head, ref[i+1:]
		}
	}
	switch registry {
	case "index.docker.io", "registry-1.docker.io":
		registry = "docker.io"
	}
	if registry == "docker.io" && !strings.Contains(ref, "/") {
		ref = "library/" + ref
	}
	return registry + "/" + ref
}

// EngineFor returns the engine of a curated image, tier 1 of the recognition.
func EngineFor(image string) Engine {
	if image == "" {
		return EngineNone
	}
	return curatedRepos[CanonicalRepo(image)]
}

// LookalikeEngine guesses the engine of an image outside the curated list:
// its repository path names a database and its environment carries a variable
// with that engine's official prefix. Only variable names are read, never
// values.
func LookalikeEngine(image string, envNames []string) Engine {
	repo := CanonicalRepo(image)
	if repo == "" || curatedRepos[repo] != EngineNone || lookalikeNegatives[repo] {
		return EngineNone
	}
	name := repo[strings.Index(repo, "/")+1:]
	switch {
	case strings.Contains(name, "mariadb"):
		return engineIfEnv(EngineMariaDB, envNames, "MARIADB_", "MYSQL_")
	case strings.Contains(name, "postgres"):
		return engineIfEnv(EnginePostgres, envNames, "POSTGRES_")
	case strings.Contains(name, "mysql"):
		return engineIfEnv(EngineMySQL, envNames, "MYSQL_")
	}
	return EngineNone
}

func engineIfEnv(e Engine, envNames []string, prefixes ...string) Engine {
	for _, name := range envNames {
		for _, prefix := range prefixes {
			if strings.HasPrefix(name, prefix) {
				return e
			}
		}
	}
	return EngineNone
}

// LabelDecision is what the bombvault.dbdump label says about a container.
type LabelDecision int

const (
	LabelDefault LabelDecision = iota
	LabelOff
	LabelForceEngine
)

// LabelDecisionFor reads the bombvault.dbdump label. Anything it does not
// understand leaves recognition to the image.
func LabelDecisionFor(labels map[string]string) (LabelDecision, Engine) {
	value := strings.ToLower(strings.TrimSpace(labels[LabelKey]))
	switch value {
	case "false", "0", "no", "off":
		return LabelOff, EngineNone
	}
	if e, ok := ParseEngine(value); ok {
		return LabelForceEngine, e
	}
	return LabelDefault, EngineNone
}

// Tier says how a container was recognised as a database.
type Tier string

const (
	TierNone      Tier = ""
	TierCurated   Tier = "curated"
	TierLookalike Tier = "lookalike"
	TierLabel     Tier = "label"
)

// Recognition is what the container card shows and what a backup plan is built
// from. Engine is what would be dumped, Suggested is the guess for a lookalike
// the user has not decided about, and Default says whether the dump runs while
// the toggle is untouched.
type Recognition struct {
	Engine, Suggested Engine
	Tier              Tier
	LabelOff, Default bool
}

// Resolve applies the precedence: a label that switches the dump off vetoes
// everything, a label naming an engine wins over the image, then the engine
// the user chose, then the curated list, then the lookalike tier.
func Resolve(image string, envNames []string, labels map[string]string, chosen Engine) Recognition {
	decision, labelEngine := LabelDecisionFor(labels)
	if decision == LabelForceEngine {
		return Recognition{Engine: labelEngine, Tier: TierLabel, Default: true}
	}

	r := Recognition{LabelOff: decision == LabelOff}
	if curated := EngineFor(image); curated != EngineNone {
		r.Engine, r.Tier = curated, TierCurated
	} else if r.Suggested = LookalikeEngine(image, envNames); r.Suggested != EngineNone {
		r.Tier = TierLookalike
	}
	if chosen != EngineNone {
		r.Engine = chosen
		if r.Tier == TierNone {
			// A stored engine is the user's word that this is a database, so
			// the card keeps the toggle even after the image was replaced.
			r.Tier = TierLookalike
		}
	}
	r.Default = r.Engine != EngineNone && !r.LabelOff
	return r
}
