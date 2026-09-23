// Package template reads and writes Unraid container template XML files.
// Templates are stored one-per-container as `my-<Name>.xml` (the Unraid
// dockerMan user-template convention), with the container name's casing
// preserved verbatim.
package template

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// FileName returns the canonical template filename for a container name,
// e.g. FileName("Plex") == "my-Plex.xml". Casing is preserved.
func FileName(name string) string {
	return "my-" + name + ".xml"
}

// Read returns the template XML for name from dir. A missing file reports
// found == false with a nil error; any other read error is returned, so a
// failure is never mistaken for a missing template.
func Read(dir, name string) (string, bool, error) {
	path := filepath.Join(dir, FileName(name))
	data, err := os.ReadFile(path) //nolint:gosec // G304: dir is an operator-configured templates dir; name is a docker container name, not attacker-controlled free path
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("template: read %s: %w", path, err)
	}
	return string(data), true, nil
}

// Write writes the template XML for name into dir, creating dir (and parents)
// if needed. An existing template is overwritten.
func Write(dir, name, xml string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // G301: Unraid flash templates dir must be world-readable by the docker daemon
		return fmt.Errorf("template: mkdir %s: %w", dir, err)
	}
	path := filepath.Join(dir, FileName(name))
	if err := os.WriteFile(path, []byte(xml), 0o644); err != nil { //nolint:gosec // G306: template XML is non-sensitive and read by the host docker manager
		return fmt.Errorf("template: write %s: %w", path, err)
	}
	return nil
}
