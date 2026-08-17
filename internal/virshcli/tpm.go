// Package virshcli — vTPM state parsing/path support (v8.0.0 TrueNAS platform
// expansion, Task 11). Mirrors nvram.go's role: NVRAM's raw path is extracted
// straight off the domain XML in virshcli.go's ParseDomain (d.OS.NVRAM), and
// nvram.go layers a documented fallback (the OVMF master template) on top for
// when the captured bytes are missing. TPM follows the same shape: the raw
// domain-XML extraction lives in ParseDomain too (see domainXML.Devices.TPM
// below), and this file holds the extraction helper plus a documented,
// EXPLICITLY-NOT-AUTOMATIC fallback for when the XML doesn't carry a path.
//
// libvirt's <tpm> element supports several backend types. Only ONE of them —
// "passthrough" — is documented to carry an explicit, safe filesystem path in
// the domain XML itself (<backend type='passthrough'><device path='...'/>
// </backend>), and even then that path is a real hardware TPM character
// device (e.g. /dev/tpm0), not virtualized state to back up. An EMULATED
// (software) vTPM — the kind TrueNAS Scale actually provisions for a
// Secure-Boot/Windows-11-class guest — is backed by a swtpm process that
// libvirt manages itself; per libvirt's public documentation, a
// <backend type='emulator'> device does NOT expose its state directory as an
// attribute of <tpm> in the domain XML. A newer <backend type='external'>
// shape (recent libvirt releases) CAN carry a UNIX socket path for an
// externally-managed swtpm, which is plausibly how TrueNAS's own middleware
// wires its vTPM given the design's own note that TPM state "lives outside
// the domain's disk/NVRAM, at TrueNAS-specific fixed paths" — but the exact
// attribute layout as actually rendered by TrueNAS's middleware is NOT
// confirmed against real hardware anywhere in this project. Rather than guess
// at either the emulator or the external shape, tpmPathFromXML below
// recognizes ONLY the well-documented passthrough case and otherwise reports
// "no usable path in the XML" — the same clean, non-erroring degrade
// ParseDomain already gives a BIOS domain with no <nvram>. See
// TPMFixedPath's doc comment below for the documented (but NOT
// automatically-applied) TrueNAS fallback this leaves room for.
package virshcli

import (
	"regexp"
	"strings"
)

// tpmXML models the <tpm> element as BombVault understands it — only the
// passthrough backend's device path, the one shape this package trusts (see
// this file's package doc comment). Every other backend type (emulator,
// external, or none of the above) decodes into a zero-value Backend, which
// tpmPathFromXML deliberately treats as "no usable path" rather than
// guessing.
type tpmXML struct {
	Backend struct {
		Type   string `xml:"type,attr"`
		Device struct {
			Path string `xml:"path,attr"`
		} `xml:"device"`
	} `xml:"backend"`
}

// tpmSafePathRe mirrors nvram.go's safeFirmwarePathRe discipline exactly: a
// clean absolute path only. A <tpm> device path this package parses may later
// reach an SSH file read/write (mirroring how NVRAM's path is used) or get
// spliced into a rewritten domain XML, so a path carrying quotes, angle
// brackets, or whitespace is never trusted — it degrades to "no usable path"
// exactly like an unrecognized backend shape, never a guess at what the
// attacker (or a malformed/hostile XML) meant.
var tpmSafePathRe = regexp.MustCompile(`^/[A-Za-z0-9._/-]+$`)

// tpmPathFromXML extracts a usable TPM device path from a parsed <tpm>
// element, or "" if there is no element, the backend isn't the recognized
// "passthrough" shape, or the device path isn't a clean absolute path. Never
// errors and never guesses — see this file's package doc comment for exactly
// which shapes are recognized and why the rest degrade cleanly.
func tpmPathFromXML(tpm *tpmXML) string {
	if tpm == nil {
		return "" // no <tpm> element at all — no vTPM on this domain
	}
	if tpm.Backend.Type != "passthrough" {
		// Emulator/external/unset backend: the domain XML does not (per
		// available documentation) carry a directly usable path for these —
		// see the package doc comment. Degrade cleanly rather than guess.
		return ""
	}
	p := strings.TrimSpace(tpm.Backend.Device.Path)
	if p == "" || !tpmSafePathRe.MatchString(p) {
		return ""
	}
	return p
}

// TPMFixedPath returns TrueNAS Scale's documented, fixed vTPM state-file
// naming convention for a VM, given its numeric TrueNAS VM id and libvirt
// domain name: "/var/db/system/vm/tpm/{id}_{name}_tpm_state" (the same
// convention TrueNAS uses for NVRAM at "/var/db/system/vm/nvram/
// {id}_{name}_VARS.fd" — see the design spec's §5 audit). TrueNAS 25.10's own
// libvirt domain-naming scheme already joins id and name this way
// ("{id}_{name}"); a caller with that raw domain name can split it itself
// (Task 12, tracked separately, is where BombVault teaches itself to do that
// split generically — not yet implemented on this branch).
//
// This function is a pure string builder — it is NOT called anywhere in this
// package or from internal/backup/vm_orchestrator.go, and deliberately so:
// per this file's package doc comment, ParseDomain already prefers a path
// sourced directly from the domain XML when one is genuinely discoverable
// there, and reconstructing this fixed-path guess is only ever correct when
// a caller (a) already knows for certain it is talking to TrueNAS Scale
// specifically (this convention is meaningless on Unraid or a generic
// libvirt host) and (b) has the VM's real numeric id from that platform — a
// standalone helper here would otherwise invite exactly the kind of silent,
// possibly-wrong path guess this task was explicitly asked not to build. A
// future TrueNAS-aware integration point (which today would live in
// internal/api/service.go, alongside the platform detection added in Task 9
// — see internal/platform/truenas.go) is where calling this with a
// confirmed id/name belongs.
func TPMFixedPath(id, name string) string {
	return "/var/db/system/vm/tpm/" + id + "_" + name + "_tpm_state"
}
