package store

import (
	"fmt"
	"strings"
)

// sectionPaths are the folders, relative to the host mount, that a fresh
// install gives each section for its own repository. They match the column
// defaults in the schema.
var sectionPaths = map[string]string{
	"containers": "user/bombvault/container",
	"vms":        "user/bombvault/vms",
	"flash":      "user/bombvault/flash",
	"config":     "user/bombvault/config",
	"files":      "user/bombvault/files",
	"zfs":        "user/bombvault/zfs",
}

// DefaultSectionPath returns the folder a section writes its own repository
// to unless somebody chose another place, "" for an unknown section.
func DefaultSectionPath(domain string) string {
	return sectionPaths[domain]
}

// FillSectionPaths gives every switched-on section without a location the
// default folder a fresh install would have given it. A section that is off
// keeps what it has, and a location that is set is never touched. It returns
// the sections it filled.
func (s *Settings) FillSectionPaths() []string {
	sections := []struct {
		domain string
		on     bool
		path   *string
	}{
		{"containers", s.ContainersEnabled, &s.ContainersPath},
		{"vms", s.VMsEnabled, &s.VMsPath},
		{"flash", s.FlashEnabled, &s.FlashPath},
		{"config", s.ConfigEnabled, &s.ConfigPath},
		{"files", s.FilesEnabled, &s.FilesPath},
		{"zfs", s.ZFSEnabled, &s.ZFSPath},
	}
	var filled []string
	for _, sec := range sections {
		if sec.on && strings.TrimSpace(*sec.path) == "" {
			*sec.path = DefaultSectionPath(sec.domain)
			filled = append(filled, sec.domain)
		}
	}
	return filled
}

// RepairSectionPaths fills the location of every switched-on section that has
// none, so an install that switched a section on while its location was empty
// backs up like a fresh one. It returns the sections it filled.
func (r *Repo) RepairSectionPaths() ([]string, error) {
	var filled []string
	if _, err := r.MutateSettings(func(cur *Settings) error {
		filled = cur.FillSectionPaths()
		return nil
	}); err != nil {
		return nil, fmt.Errorf("RepairSectionPaths: %w", err)
	}
	return filled, nil
}
