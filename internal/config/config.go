// Package config loads and validates process configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// appKeyRe validates that APP_KEY is exactly 64 lowercase hex characters.
var appKeyRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Config holds all process-level configuration for bombvault.
type Config struct {
	AppKey            string
	DataDir           string
	HostMountRoot     string
	HostSourceRoot    string
	HostRunRoot       string
	NVRAMMountRoot    string
	NVRAMSourceRoot   string
	LibvirtHost       string
	LibvirtSSHUser    string
	Port              int
	HTTPSPort         int
	HTTPOnly          bool
	FlashTemplatesDir string
	DBPath            string
}

// Load reads configuration from the provided env map and applies defaults.
// It returns an error if APP_KEY is missing or does not match [0-9a-f]{64}.
func Load(env map[string]string) (Config, error) {
	key := env["APP_KEY"]
	if !appKeyRe.MatchString(key) {
		return Config{}, fmt.Errorf("APP_KEY must be exactly 64 lowercase hex characters")
	}

	c := Config{
		AppKey:         key,
		DataDir:        stringOr(env["DATA_DIR"], "/config"),
		HostMountRoot:  stringOr(env["HOST_MOUNT_ROOT"], "/host/user"),
		HostSourceRoot: stringOr(env["HOST_SOURCE_ROOT"], "/mnt"),
		// HostRunRoot is where the host /var/run is mounted (for libvirt access).
		// BombVault mounts the run PARENT, never /var/run/libvirt directly —
		// pinning that dir stops the Unraid VM Manager from recreating it on
		// toggle. The libvirt socket is symlinked from here at startup.
		HostRunRoot: stringOr(env["HOST_RUN_ROOT"], "/host/run"),
		// NVRAM translation is OFF by default (empty roots). Mounting under
		// /etc/libvirt — a loopback (libvirt.img) mount point on Unraid — from a
		// container races and blocks the host libvirt.img mount, taking down the
		// VM Manager. Left empty unless an operator points these at a SAFE source
		// (e.g. a /mnt path populated by a host script). UEFI VMs still boot via
		// virshcli.EnsureNVRAMTemplate (firmware var store regenerated on restore).
		NVRAMMountRoot:  env["NVRAM_MOUNT_ROOT"],
		NVRAMSourceRoot: env["NVRAM_SOURCE_ROOT"],
		// libvirt is reached over SSH (qemu+ssh://) — no filesystem mount.
		LibvirtHost:       stringOr(env["LIBVIRT_HOST"], "host.docker.internal"),
		LibvirtSSHUser:    stringOr(env["LIBVIRT_SSH_USER"], "root"),
		Port:              intOr(env["PORT"], 3000),
		HTTPSPort:         intOr(env["HTTPS_PORT"], 3443),
		HTTPOnly:          strings.EqualFold(env["HTTP_ONLY"], "true"),
		FlashTemplatesDir: stringOr(env["FLASH_TEMPLATES_DIR"], "/host/boot/config/plugins/dockerMan/templates-user"),
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
