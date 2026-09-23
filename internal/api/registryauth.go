package api

// Per-registry credentials let the post-backup update pull reach private
// images. Like notify_conf and cloud_conf they are stored as one encrypted
// JSON blob (internal/secret with APP_KEY, base64) in settings.registry_auths.

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

// RegistryAuth is one stored registry credential. Token is write-only over the
// API; the settings GET only reports whether it is set (see registryAuthView).
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

// decodeRegistryAuths decrypts the credential list stored in settings. A blank
// value yields nil.
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
// settings.registry_auths. An empty list encodes to "".
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

// registryAuthFor returns the credential for ref's registry encoded for the
// Docker Engine API, or "" to pull anonymously. Errors are logged instead of
// returned so a broken credential store cannot break the pull.
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

// registryHost returns the registry an image ref pulls from. As in docker's
// reference parsing, the part before the first "/" is a host if it contains
// "." or ":" or is "localhost"; otherwise the ref is a Docker Hub path such as
// "nginx" or "library/nginx".
func registryHost(ref string) string {
	first, _, found := strings.Cut(ref, "/")
	if !found {
		return "docker.io"
	}
	if !strings.ContainsAny(first, ".:") && first != "localhost" {
		return "docker.io"
	}
	return normalizeRegistryHost(first)
}

// normalizeRegistryHost lowercases host, strips the scheme and trailing slash,
// and maps the Docker Hub endpoint aliases to "docker.io".
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

// mergeRegistryAuths builds the list to store from the submitted entries. The
// settings GET never returns tokens, so a blank token keeps the stored one for
// that host, and hosts missing from the submission are dropped. Errors are
// shown to the user.
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
			return nil, fmt.Errorf("invalid registry host %q: use just the host, e.g. ghcr.io", host)
		}
		if seen[host] {
			return nil, fmt.Errorf("duplicate registry %q", host)
		}
		seen[host] = true
		token := strings.TrimSpace(v.Token)
		if token == "" {
			token = prev[host]
		}
		if token == "" {
			return nil, fmt.Errorf("a token is required for registry %q", host)
		}
		out = append(out, RegistryAuth{Host: host, Username: strings.TrimSpace(v.Username), Token: token})
	}
	return out, nil
}
