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
	// safeFirmwarePathRe matches a clean absolute firmware path. The derived
	// template is spliced into a template='…' XML attribute, so it must contain
	// no quote, angle bracket or whitespace that could break out of (or inject
	// into) the attribute (CWE-91). Any loader path that does not map to such a
	// clean path falls back to the trusted defaultOVMFVars constant.
	safeFirmwarePathRe = regexp.MustCompile(`^/[A-Za-z0-9._/-]+$`)
)

// defaultOVMFVars is the Unraid stock OVMF master var store, used when the
// loader path cannot be mapped to a CODE→VARS template.
const defaultOVMFVars = "/usr/share/qemu/ovmf-x64/OVMF_VARS-pure-efi.fd"

// EnsureNVRAMTemplate makes a restored UEFI domain bootable even when its
// backed-up NVRAM var store is missing. libvirt instantiates the per-VM nvram
// from a "master var store" only when the <nvram> element carries a template=
// attribute (or a host-side firmware descriptor maps the loader). A restored
// domain XML often lacks template=, so a fresh host — or a backup that never
// captured the nvram — fails to start with "unable to find any master var store
// for loader". This adds template= pointing at the OVMF master derived from the
// <loader> (CODE→VARS). libvirt uses the template ONLY when the nvram file is
// absent, so an existing (restored) nvram — with its boot entries — is kept.
//
// BIOS domains (no <nvram>) and domains that already specify template= are
// returned unchanged.
func EnsureNVRAMTemplate(domainXML string) string {
	loc := nvramOpenRe.FindStringIndex(domainXML)
	if loc == nil {
		return domainXML // BIOS — no nvram element
	}
	openTag := domainXML[loc[0]:loc[1]]
	if strings.Contains(openTag, "template=") {
		return domainXML // already mapped
	}
	tmpl := deriveVarsTemplate(domainXML)
	// Inject template= immediately after "<nvram".
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
	// Never splice an unclean path into the template='…' attribute — fall back
	// to the trusted master if the loader carried quotes/brackets/whitespace.
	if !safeFirmwarePathRe.MatchString(cand) {
		return defaultOVMFVars
	}
	return cand
}
