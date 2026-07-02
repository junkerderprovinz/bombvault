package api

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// DeploySnippet is a one-time rest-server deployment recipe for a domain's
// append-only off-site repo. It gives the user everything to stand up a far-side
// `restic/rest-server --append-only` on their storage box, with generated
// credentials. Password is the PLAINTEXT htpasswd password — shown once and never
// persisted server-side; Htpasswd is its bcrypt line for the far-side htpasswd file.
type DeploySnippet struct {
	User      string `json:"user"`      // htpasswd user "bombvault-<domain>"
	Password  string `json:"password"`  // one-time plaintext password (never stored)
	Htpasswd  string `json:"htpasswd"`  // "bombvault-<domain>:<bcrypt-hash>"
	DockerRun string `json:"dockerRun"` // docker run recipe (+ echo pre-step + repo-URL hint)
	Compose   string `json:"compose"`   // docker-compose equivalent, same values
}

// bcryptDeployCost is the bcrypt work factor for the generated htpasswd hash.
// rest-server verifies htpasswd bcrypt hashes; cost 12 is a sensible 2026 default.
const bcryptDeployCost = 12

// randomDeployPassword returns a URL-safe 24-character password. 18 random bytes
// base64url-encode to exactly 24 chars (no padding), all in the URL-safe alphabet
// so the password is safe to paste into a shell/htpasswd line without quoting
// surprises.
func randomDeployPassword() (string, error) {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate password: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// buildDeploySnippet builds a fresh rest-server deployment snippet for a domain's
// off-site repo: a random one-time password, its bcrypt htpasswd line, and the
// docker-run + compose recipes to run an append-only rest-server. Nothing is
// persisted — the caller returns it once and the plaintext password is shown only
// in that response. domain is one of the fixed backup domains.
func buildDeploySnippet(domain string) (DeploySnippet, error) {
	switch domain {
	case "containers", "vms", "flash":
	default:
		return DeploySnippet{}, fmt.Errorf("unknown domain %q", domain)
	}

	password, err := randomDeployPassword()
	if err != nil {
		return DeploySnippet{}, err
	}
	user := "bombvault-" + domain
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptDeployCost)
	if err != nil {
		return DeploySnippet{}, fmt.Errorf("hash password: %w", err)
	}
	htpasswd := user + ":" + string(hash)

	// A generic placeholder IP only (192.168.x.x) — never a real host address.
	repoHint := fmt.Sprintf("# repo URL for BombVault: rest:http://192.168.x.x:8000/%s/%s", user, domain)

	dockerRun := fmt.Sprintf(`# 1) create the append-only credential on the storage box:
echo '%s' >> /path/on/storage-box/restic/.htpasswd

# 2) start the append-only rest-server:
docker run -d --name rest-server -p 8000:8000 -v /path/on/storage-box/restic:/data -e OPTIONS="--append-only --private-repos --htpasswd-file /data/.htpasswd" restic/rest-server:0.14.0

%s`, htpasswd, repoHint)

	compose := fmt.Sprintf(`# 1) create the append-only credential on the storage box:
echo '%s' >> /path/on/storage-box/restic/.htpasswd

# 2) docker-compose.yml for the append-only rest-server:
services:
  rest-server:
    image: restic/rest-server:0.14.0
    container_name: rest-server
    ports:
      - "8000:8000"
    environment:
      OPTIONS: "--append-only --private-repos --htpasswd-file /data/.htpasswd"
    volumes:
      - /path/on/storage-box/restic:/data
    restart: unless-stopped

%s`, htpasswd, repoHint)

	return DeploySnippet{
		User:      user,
		Password:  password,
		Htpasswd:  htpasswd,
		DockerRun: dockerRun,
		Compose:   compose,
	}, nil
}
