package virshcli

import (
	"strings"
)

// tpmXML is the part of a <tpm> element that can name a path. Only the
// passthrough backend puts one in the domain XML, and it is a host TPM device
// such as /dev/tpm0. An emulated vTPM keeps its swtpm state in a directory
// libvirt does not expose there, and the external backend's socket layout on
// TrueNAS is unconfirmed, so neither yields a path.
type tpmXML struct {
	Backend struct {
		Type   string `xml:"type,attr"`
		Device struct {
			Path string `xml:"path,attr"`
		} `xml:"device"`
	} `xml:"backend"`
}

// tpmPathFromXML returns the device path of a passthrough <tpm>, or "" when
// there is no <tpm>, the backend is another type, or the path does not match
// safeAbsolutePathRe. The path may later reach an SSH file transfer or a
// rewritten domain XML, so an unclean one is dropped rather than trusted.
func tpmPathFromXML(tpm *tpmXML) string {
	if tpm == nil {
		return ""
	}
	if tpm.Backend.Type != "passthrough" {
		return ""
	}
	p := strings.TrimSpace(tpm.Backend.Device.Path)
	if p == "" || !safeAbsolutePathRe.MatchString(p) {
		return ""
	}
	return p
}

// TPMFixedPath returns where TrueNAS Scale keeps a VM's vTPM state,
// /var/db/system/vm/tpm/{id}_{name}_tpm_state, named like its NVRAM files.
// The path is only right on a confirmed TrueNAS host and with the VM's real
// numeric id; elsewhere it names nothing.
func TPMFixedPath(id, name string) string {
	return "/var/db/system/vm/tpm/" + id + "_" + name + "_tpm_state"
}
