package api

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

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
	Unraid    string `json:"unraid"`    // Unraid container template (XML), same values
}

// bcryptDeployCost is the bcrypt work factor for the generated htpasswd hash.
// rest-server verifies htpasswd bcrypt hashes; cost 12 is a sensible 2026 default.
const bcryptDeployCost = 12

// tlsGuidance is an honest caveat appended to both deploy recipes. restic sends
// the htpasswd credential as HTTP Basic auth, so on plain http:// it travels in
// the clear — fine on a trusted LAN/VPN, but a WAN-reachable box should terminate
// TLS at a reverse proxy so the append-only repository credential is not exposed
// to on-path observers.
const tlsGuidance = "# Plain HTTP is fine on a trusted LAN/VPN. For a WAN-reachable box, terminate TLS (a reverse proxy) so the repository credential is not sent in the clear."

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
	case "containers", "vms", "flash", "config", "files":
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

%s
%s`, htpasswd, tlsGuidance, repoHint)

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

%s
%s`, htpasswd, tlsGuidance, repoHint)

	return DeploySnippet{
		User:      user,
		Password:  password,
		Htpasswd:  htpasswd,
		DockerRun: dockerRun,
		Compose:   compose,
		Unraid:    unraidTemplate(htpasswd, repoHint),
	}, nil
}

// unraidTemplate renders the same rest-server recipe as an Unraid container
// template ([601]).
// ---------------------------------------------------------------------------
// Why this exists, in the reporter's own words: "I couldn't get the scripted
// deployment of a restic docker functional, and it couldn't be edited in the
// docker UI, so I used one from CA instead."
//
// The docker-run recipe is not broken. Verified end to end against a real
// rest-server: the container starts, loads the htpasswd, reports "Append only
// mode enabled" and "Private repositories enabled", and `restic init` creates
// the repository through it. The defect is the FORMAT, not the command.
//
// A container Unraid did not create from a template has no template. It shows
// up in the Docker tab with no Edit form behind it, so the port, the path and
// the options can only ever be changed by deleting it and retyping the whole
// command. BombVault is an Unraid application; handing its users a bare
// `docker run` asks them to give up the one management surface their platform
// has. This is the same rule the project already applies to its OWN containers.
//
// Saved as /boot/config/plugins/dockerMan/templates-user/my-rest-server.xml,
// the Docker tab's "Add Container" template dropdown picks it up, and every
// value below becomes an editable field.
//
// The htpasswd line still has to be written by hand: it carries a bcrypt hash
// of a password shown exactly once, and a template that embedded it would put
// that credential into a file Unraid keeps on the flash drive forever.
//
// EVERY note belongs inside the leading comment, and nothing may follow the root
// element. The first version appended the shared `# ...` guidance lines after
// </Container>, the way the docker-run and compose snippets carry them, which
// made the whole file invalid XML: shell comments are not XML comments, and
// nothing but whitespace may follow a root element. It got that far because the
// test truncated the string at </Container> before parsing, so it proved that a
// PREFIX parsed rather than the file the user actually saves. Caught by asking
// the deployed instance for a real recipe and parsing what came back.
func unraidTemplate(htpasswd, repoHint string) string {
	// The declaration comes FIRST. An XML comment before it is not valid XML,
	// and this text is meant to be saved verbatim as a file that Unraid parses.
	return fmt.Sprintf(`<?xml version="1.0"?>
<!--
  1) create the append-only credential on the storage box FIRST:
     echo '%s' >> /mnt/user/appdata/rest-server/.htpasswd

  2) save this file on the storage box as
     /boot/config/plugins/dockerMan/templates-user/my-rest-server.xml
     then Docker tab, Add Container, and pick "rest-server" from the
     template dropdown. Every field below is editable there.

  3) %s

  4) %s
-->
<Container version="2">
  <Name>rest-server</Name>
  <Repository>restic/rest-server:0.14.0</Repository>
  <Registry>https://hub.docker.com/r/restic/rest-server</Registry>
  <Network>bridge</Network>
  <Privileged>false</Privileged>
  <Support>https://github.com/junkerderprovinz/bombvault</Support>
  <Overview>Append-only restic REST server. Receives immutable off-site copies from BombVault: the DESTINATION refuses deletes and overwrites, so a compromised or misconfigured sender cannot reach the copy that is meant to be the last line of defence.</Overview>
  <Category>Backup:</Category>
  <WebUI/>
  <Icon>https://raw.githubusercontent.com/restic/restic/master/doc/logo/logo.png</Icon>
  <Config Name="Port" Target="8000" Default="8000" Mode="tcp" Description="Port BombVault connects to." Type="Port" Display="always" Required="true" Mask="false">8000</Config>
  <Config Name="Data" Target="/data" Default="/mnt/user/appdata/rest-server" Mode="rw" Description="Where the repositories and the .htpasswd file live." Type="Path" Display="always" Required="true" Mask="false">/mnt/user/appdata/rest-server</Config>
  <Config Name="OPTIONS" Target="OPTIONS" Default="--append-only --private-repos --htpasswd-file /data/.htpasswd" Description="Leave --append-only in place: it is what makes this an immutable destination. Removing it turns the box back into an ordinary share." Type="Variable" Display="always" Required="true" Mask="false">--append-only --private-repos --htpasswd-file /data/.htpasswd</Config>
</Container>
`, xmlCommentSafe(htpasswd), xmlCommentSafe(noteText(tlsGuidance)), xmlCommentSafe(noteText(repoHint)))
}

// noteText strips the leading "# " the shared guidance lines carry for the
// shell snippets. Inside an XML comment the hash is just noise.
func noteText(s string) string { return strings.TrimPrefix(s, "# ") }

// xmlCommentSafe makes text safe to sit inside an XML comment: a comment may
// not contain "--", and may not end with "-" (XML 1.0 §2.5). Neither the
// generated repo URL nor a bcrypt hash produces those today, which is exactly
// why this belongs in the code rather than in a reviewer's memory. A value that
// grew a double dash later would otherwise turn the whole template into a file
// Unraid drops from the dropdown without saying why.
func xmlCommentSafe(s string) string {
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "- -")
	}
	if strings.HasSuffix(s, "-") {
		s += " "
	}
	return s
}
