package api

// Private container-registry credentials (#106): the post-backup update pull
// (updateContainerAfterBackup) previously always pulled anonymously, so an
// image living in a private/sponsor-gated registry (e.g. a ghcr.io sponsor
// image) could never be update-checked. The user stores per-registry
// credentials in Settings; the update pull resolves the registry host from the
// image ref and, when a credential matches, sends it as the Docker Engine
// API's RegistryAuth. No credential → the existing anonymous behavior.
//
// At rest the list follows the house pattern for nested secret configs
// (notify_conf / cloud_conf): one AES-256-GCM-encrypted JSON blob (base64),
// keyed by the APP_KEY via internal/secret, in settings.registry_auths.

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// RegistryAuth is one stored private-registry credential. Token is the registry
// password / personal access token and is write-only over the API: the settings
// GET exposes only host + username + a tokenSet flag (see registryAuthView).
type RegistryAuth struct {
	Host     string `json:"host"`
	Username string `json:"username"`
	Token    string `json:"token"`
}

// RegistryAuths returns the stored registry credentials (empty when none).
func (s *Service) RegistryAuths() ([]RegistryAuth, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, err
	}
	return s.decodeRegistryAuths(settings)
}

// decodeRegistryAuths decrypts the stored credential list from the given
// settings (an empty/blank registry_auths yields nil, no error).
func (s *Service) decodeRegistryAuths(settings store.Settings) ([]RegistryAuth, error) {
	if strings.TrimSpace(settings.RegistryAuths) == "" {
		return nil, nil
	}
	enc, err := base64.StdEncoding.DecodeString(settings.RegistryAuths)
	if err != nil {
		return nil, err
	}
	plain, err := secret.Decrypt(s.cfg.AppKey, enc)
	if err != nil {
		return nil, err
	}
	var list []RegistryAuth
	if err := json.Unmarshal(plain, &list); err != nil {
		return nil, err
	}
	return list, nil
}

// EncodeRegistryAuths encrypts the credential list into the value stored in
// settings.registry_auths. An empty list encodes to "" (credentials cleared).
func (s *Service) EncodeRegistryAuths(list []RegistryAuth) (string, error) {
	if len(list) == 0 {
		return "", nil
	}
	blob, err := json.Marshal(list)
	if err != nil {
		return "", fmt.Errorf("marshal registry auths: %w", err)
	}
	enc, err := secret.Encrypt(s.cfg.AppKey, blob)
	if err != nil {
		return "", fmt.Errorf("encrypt registry auths: %w", err)
	}
	return base64.StdEncoding.EncodeToString(enc), nil
}

// registryAuthFor resolves the stored credential for an image ref's registry
// host and returns it encoded for the Docker Engine API, or "" (anonymous)
// when no credential matches. Any error (store, decrypt, encode) is logged and
// degrades to anonymous, so a broken credential store can never break the pull
// path that worked before credentials existed.
func (s *Service) registryAuthFor(ref string) string {
	auths, err := s.RegistryAuths()
	if err != nil {
		log.Printf("api: registry auth: %v (pulling anonymously)", err)
		return ""
	}
	host := registryHost(ref)
	for _, a := range auths {
		if normalizeRegistryHost(a.Host) != host {
			continue
		}
		enc, eErr := dockercli.EncodeRegistryAuth(a.Username, a.Token, host)
		if eErr != nil {
			log.Printf("api: registry auth for %s: %v (pulling anonymously)", host, eErr)
			return ""
		}
		return enc
	}
	return ""
}

// registryHost resolves the registry host an image ref pulls from, using the
// standard docker reference heuristic: the part before the first "/" is a
// registry host only when it contains a "." or ":" (a domain or a port) or is
// "localhost" — otherwise the whole ref is a Docker Hub path ("nginx",
// "library/nginx"). Hub aliases normalize to "docker.io".
func registryHost(ref string) string {
	first, _, found := strings.Cut(ref, "/")
	if !found {
		return "docker.io" // bare image, e.g. "nginx:latest"
	}
	if !strings.ContainsAny(first, ".:") && first != "localhost" {
		return "docker.io" // namespaced Hub path, e.g. "library/nginx"
	}
	return normalizeRegistryHost(first)
}

// normalizeRegistryHost canonicalizes a user-entered or ref-derived registry
// host for matching: lowercase, no scheme, no trailing slash, and the Docker
// Hub endpoint aliases collapse to "docker.io".
func normalizeRegistryHost(host string) string {
	h := strings.ToLower(strings.TrimSpace(host))
	h = strings.TrimPrefix(h, "https://")
	h = strings.TrimPrefix(h, "http://")
	h = strings.TrimSuffix(h, "/")
	if h == "index.docker.io" || h == "registry-1.docker.io" {
		return "docker.io"
	}
	return h
}

// mergeRegistryAuths turns the submitted settings-view entries into the list to
// store, applying the house write-only-secret contract: the GET never echoes
// tokens, so a submitted entry with a blank token KEEPS the stored token for
// that host, a non-blank token replaces it, and a host absent from the
// submitted list is deleted. Returned errors are user-facing.
func mergeRegistryAuths(submitted []registryAuthView, stored []RegistryAuth) ([]RegistryAuth, error) {
	prev := make(map[string]string, len(stored))
	for _, a := range stored {
		prev[normalizeRegistryHost(a.Host)] = a.Token
	}
	out := make([]RegistryAuth, 0, len(submitted))
	seen := make(map[string]bool, len(submitted))
	for _, v := range submitted {
		host := normalizeRegistryHost(v.Host)
		if host == "" {
			return nil, errors.New("registry host is required")
		}
		if strings.ContainsAny(host, " /") {
			return nil, fmt.Errorf("invalid registry host %q — use just the host, e.g. ghcr.io", host)
		}
		if seen[host] {
			return nil, fmt.Errorf("duplicate registry %q", host)
		}
		seen[host] = true
		token := strings.TrimSpace(v.Token)
		if token == "" {
			token = prev[host] // blank = keep the stored token (never echoed by GET)
		}
		if token == "" {
			return nil, fmt.Errorf("a token is required for registry %q", host)
		}
		out = append(out, RegistryAuth{Host: host, Username: strings.TrimSpace(v.Username), Token: token})
	}
	return out, nil
}
