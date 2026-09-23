package api

import (
	"os"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/template"
)

// templateFilePrefix and templateFileSuffix reverse template.FileName's
// "my-<name>.xml", which only goes from name to file.
const (
	templateFilePrefix = "my-"
	templateFileSuffix = ".xml"
)

// containerNameFromTemplateFile returns the container name of an Unraid user
// template file, "radarr" for "my-radarr.xml". Any other file reports false.
func containerNameFromTemplateFile(fileName string) (string, bool) {
	if !strings.HasPrefix(fileName, templateFilePrefix) || !strings.HasSuffix(fileName, templateFileSuffix) {
		return "", false
	}
	name := strings.TrimSuffix(strings.TrimPrefix(fileName, templateFilePrefix), templateFileSuffix)
	if name == "" {
		return "", false
	}
	return name, true
}

// readTemplateLineage builds the templateLineage from one listing of the Unraid
// user-template directory. The flash is a USB stick, so one listing per
// suggestion pass is far cheaper than a lookup per name. A missing or
// unreadable directory (not Unraid, or /host/boot not mounted) yields an empty
// lineage, because a rename hint must never break the container list.
func (s *Service) readTemplateLineage() templateLineage {
	entries, err := os.ReadDir(s.cfg.FlashTemplatesDir)
	if err != nil {
		return templateLineage{}
	}
	lineage := templateLineage{
		Present:   make(map[string]bool, len(entries)),
		XMLByName: make(map[string]string, len(entries)),
	}
	for _, e := range entries {
		if e.IsDir() {
			continue // templates-user holds files only
		}
		name, ok := containerNameFromTemplateFile(e.Name())
		if !ok {
			continue // not a my-<name>.xml file
		}
		lineage.Present[name] = true
		// A file that fails to read still counts as present, since signal 4
		// needs only the old template's absence; without content the name
		// just never matches as the new side.
		if xml, found, rerr := template.Read(s.cfg.FlashTemplatesDir, name); rerr == nil && found {
			lineage.XMLByName[name] = xml
		}
	}
	return lineage
}
