package api

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// DeploySnippet is a one-time recipe for an append-only rest-server holding a
// domain's off-site repo. Password is the plaintext htpasswd password, shown
// once and never stored; Htpasswd is its bcrypt line.
type DeploySnippet struct {
	User      string `json:"user"`      // htpasswd user "bombvault-<domain>"
	Password  string `json:"password"`  // one-time plaintext password (never stored)
	Htpasswd  string `json:"htpasswd"`  // "bombvault-<domain>:<bcrypt-hash>"
	DockerRun string `json:"dockerRun"` // docker run recipe (+ echo pre-step + repo-URL hint)
	Compose   string `json:"compose"`   // docker-compose equivalent, same values
	Unraid    string `json:"unraid"`    // Unraid container template (XML), same values
}

// bcryptDeployCost is the work factor of the generated htpasswd hash.
const bcryptDeployCost = 12

// tlsGuidance ends each recipe: restic sends the htpasswd credential as HTTP
// Basic auth, so over plain http it travels in the clear.
const tlsGuidance = "# Plain HTTP is fine on a trusted LAN/VPN. For a WAN-reachable box, terminate TLS (a reverse proxy) so the repository credential is not sent in the clear."

// randomDeployPassword returns 24 URL-safe characters (18 random bytes in
// unpadded base64url), safe to paste into a shell or an htpasswd line.
func randomDeployPassword() (string, error) {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate password: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// buildDeploySnippet builds a fresh recipe with a new password for one of the
// fixed backup domains. Nothing is stored, so the password appears only in the
// response that returns it.
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

	// A placeholder address, never a real one.
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

// unraidTemplate renders the rest-server recipe as an Unraid container
// template. A container started with a bare docker run has no Edit form in
// Unraid, so its port, path and options could only be changed by recreating it.
//
// The htpasswd line stays out of the template fields because Unraid keeps
// templates on the flash drive and the credential is shown only once. All notes
// go in the leading comment: nothing but whitespace may follow the root element.
func unraidTemplate(htpasswd, repoHint string) string {
	// The XML declaration has to come before the comment.
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

// noteText strips the "# " the shared notes carry for the shell snippets.
func noteText(s string) string { return strings.TrimPrefix(s, "# ") }

// xmlCommentSafe keeps text valid inside an XML comment, which may not contain
// "--" or end with "-" (XML 1.0 §2.5). The repo URL and a bcrypt hash never do,
// but a template that stopped parsing would vanish from Unraid's dropdown
// without a word.
func xmlCommentSafe(s string) string {
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "- -")
	}
	if strings.HasSuffix(s, "-") {
		s += " "
	}
	return s
}
