package virshcli

import (
	"path"
	"regexp"
	"strings"
)

var (
	// nvramOpenRe matches the opening <nvram ...> tag (with optional attributes).
	nvramOpenRe = regexp.MustCompile(`<nvram(\s[^>]*)?>`)
	// loaderRe captures the firmware loader (CODE) path from <loader ...>PATH</loader>.
	loaderRe = regexp.MustCompile(`<loader\b[^>]*>([^<]+)</loader>`)
	// safeAbsolutePathRe matches an absolute path with no quote, angle bracket
	// or whitespace. Any host path this package splices into domain XML or
	// trusts as a device path must match it.
	safeAbsolutePathRe = regexp.MustCompile(`^/[A-Za-z0-9._/-]+$`)
)

// defaultOVMFVars is the Unraid stock OVMF master var store, used when the
// loader path cannot be mapped to a CODE→VARS template.
const defaultOVMFVars = "/usr/share/qemu/ovmf-x64/OVMF_VARS-pure-efi.fd"

// EnsureNVRAMTemplate makes a restored UEFI domain bootable when its NVRAM var
// store is missing. libvirt creates the per-VM nvram from a master var store
// only when <nvram> has a template= attribute or a firmware descriptor maps
// the loader; without either, a fresh host fails with "unable to find any
// master var store for loader". This adds template= pointing at the OVMF
// master derived from <loader>. libvirt reads the template only when the nvram
// file is absent, so a restored nvram and its boot entries are kept.
//
// BIOS domains (no <nvram>) and domains that already set template= are
// returned unchanged.
func EnsureNVRAMTemplate(domainXML string) string {
	loc := nvramOpenRe.FindStringIndex(domainXML)
	if loc == nil {
		return domainXML // BIOS
	}
	openTag := domainXML[loc[0]:loc[1]]
	if strings.Contains(openTag, "template=") {
		return domainXML
	}
	tmpl := deriveVarsTemplate(domainXML)
	newOpen := "<nvram template='" + tmpl + "'" + openTag[len("<nvram"):]
	return domainXML[:loc[0]] + newOpen + domainXML[loc[1]:]
}

// deriveVarsTemplate maps the firmware loader (CODE) to its var-store (VARS)
// master by basename substitution, e.g.
// /usr/share/qemu/ovmf-x64/OVMF_CODE-pure-efi.fd →
// /usr/share/qemu/ovmf-x64/OVMF_VARS-pure-efi.fd. Falls back to the Unraid
// stock master when no loader is present or it carries no "CODE" marker.
func deriveVarsTemplate(domainXML string) string {
	m := loaderRe.FindStringSubmatch(domainXML)
	if m == nil {
		return defaultOVMFVars
	}
	loader := strings.TrimSpace(m[1])
	base := path.Base(loader)
	if !strings.Contains(base, "CODE") {
		return defaultOVMFVars
	}
	cand := path.Dir(loader) + "/" + strings.Replace(base, "CODE", "VARS", 1)
	// cand ends up inside template='...', so a loader path with quotes,
	// brackets or whitespace falls back to the stock master.
	if !safeAbsolutePathRe.MatchString(cand) {
		return defaultOVMFVars
	}
	return cand
}
