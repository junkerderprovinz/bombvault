// Package config loads and validates process configuration from environment variables.
package config

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var appKeyRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Config holds all process-level configuration for bombvault.
type Config struct {
	AppKey         string
	DataDir        string
	HostMountRoot  string
	HostSourceRoot string
	// DataRootSegments (env DATA_ROOT_SEGMENTS) are the path segments that
	// mark a bind mount's host source as container data, such as "appdata" or
	// "config". A bind is kept when any of them is a whole segment of its
	// source. The default is ["appdata"], the Unraid layout.
	DataRootSegments []string
	// PlatformOverride (env PLATFORM) forces the platform.Kind instead of
	// detecting it; empty means detect. platform.Detect validates the value
	// and falls back to generic with a warning when it is unknown.
	PlatformOverride string
	LibvirtHost      string
	LibvirtSSHUser   string
	LibvirtSSHPort   string
	// LibvirtURI (env LIBVIRT_URI), when set, replaces the qemu+ssh URI built
	// from LibvirtHost, LibvirtSSHUser and LibvirtSSHPort. TrueNAS Scale needs
	// it because its libvirtd listens on a non-standard socket; the value is
	// in docs/vm-backup-ssh-setup.md.
	LibvirtURI        string
	Port              int
	HTTPSPort         int
	HTTPOnly          bool
	FlashTemplatesDir string
	FlashDir          string
	DBPath            string
	// TrustedProxies (env TRUSTED_PROXY, comma-separated addresses or CIDR
	// ranges) are the hops whose X-Forwarded-For is believed. The login
	// throttle counts failures per client, and behind a reverse proxy every
	// request carries the proxy's address, so without this an attacker's
	// failures lock out every client. Empty trusts nobody; otherwise a caller
	// could pick its own throttle bucket with a forged header.
	TrustedProxies []net.IPNet
}

// Load reads configuration from the provided env map and applies defaults.
// It returns an error if APP_KEY is missing or does not match [0-9a-f]{64}.
func Load(env map[string]string) (Config, error) {
	key := env["APP_KEY"]
	if !appKeyRe.MatchString(key) {
		return Config{}, fmt.Errorf("APP_KEY must be exactly 64 lowercase hex characters")
	}

	c := Config{
		AppKey:           key,
		DataDir:          stringOr(env["DATA_DIR"], "/config"),
		HostMountRoot:    stringOr(env["HOST_MOUNT_ROOT"], "/host/user"),
		HostSourceRoot:   stringOr(env["HOST_SOURCE_ROOT"], "/mnt"),
		DataRootSegments: dataRootSegments(env["DATA_ROOT_SEGMENTS"]),
		PlatformOverride: env["PLATFORM"],
		// libvirt is reached over SSH, not through a mounted socket.
		LibvirtHost:       stringOr(env["LIBVIRT_HOST"], "host.docker.internal"),
		LibvirtSSHUser:    stringOr(env["LIBVIRT_SSH_USER"], "root"),
		LibvirtSSHPort:    stringOr(env["LIBVIRT_SSH_PORT"], "22"),
		LibvirtURI:        env["LIBVIRT_URI"],
		Port:              intOr(env["PORT"], 3000),
		HTTPSPort:         intOr(env["HTTPS_PORT"], 3443),
		HTTPOnly:          strings.EqualFold(env["HTTP_ONLY"], "true"),
		FlashTemplatesDir: stringOr(env["FLASH_TEMPLATES_DIR"], "/host/boot/config/plugins/dockerMan/templates-user"),
		// the Unraid flash, /boot mounted at /host/boot
		FlashDir:       stringOr(env["FLASH_DIR"], "/host/boot"),
		TrustedProxies: trustedProxies(env["TRUSTED_PROXY"]),
	}
	c.DBPath = filepath.Join(c.DataDir, "bombvault.sqlite")
	return c, nil
}

// LoadFromEnv reads configuration from the process environment.
func LoadFromEnv() (Config, error) {
	env := make(map[string]string)
	for _, kv := range os.Environ() {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			env[kv[:i]] = kv[i+1:]
		}
	}
	return Load(env)
}

func stringOr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func intOr(v string, def int) int {
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// defaultDataRootSegments matches Unraid's appdata share.
var defaultDataRootSegments = []string{"appdata"}

// trustedProxies parses TRUSTED_PROXY. Each entry is a CIDR ("10.0.0.0/8") or
// a single address, which becomes a /32 or /128. An entry that does not parse
// is logged and skipped rather than failing startup, which leaves that proxy
// untrusted.
func trustedProxies(raw string) []net.IPNet {
	var out []net.IPNet
	for _, part := range strings.Split(raw, ",") {
		entry := strings.TrimSpace(part)
		if entry == "" {
			continue
		}
		if _, netw, err := net.ParseCIDR(entry); err == nil {
			out = append(out, *netw)
			continue
		}
		ip := net.ParseIP(entry)
		if ip == nil {
			log.Printf("config: TRUSTED_PROXY: ignoring unparseable entry %q", entry)
			continue
		}
		bits := 32
		if ip.To4() == nil {
			bits = 128
		}
		out = append(out, net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
	}
	return out
}

// dataRootSegments parses DATA_ROOT_SEGMENTS as a comma-separated list of
// path-segment names (each trimmed, lower-cased, empty entries dropped).
// Unset, empty, or all-empty-after-trim input falls back to
// defaultDataRootSegments.
func dataRootSegments(raw string) []string {
	if raw == "" {
		return defaultDataRootSegments
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if seg := strings.ToLower(strings.TrimSpace(part)); seg != "" {
			out = append(out, seg)
		}
	}
	if len(out) == 0 {
		return defaultDataRootSegments
	}
	return out
}
