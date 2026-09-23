package virshcli

import "regexp"

var (
	// diskSourceFileRe captures a <disk> <source file='PATH'/> element's path,
	// splitting the tag so only the path is rewritten (quote style preserved).
	// Groups: 1=`<source file=`, 2=quote, 3=path, 4=quote.
	diskSourceFileRe = regexp.MustCompile(`(<source\s+file=)(['"])([^'"]*)(['"])`)
	// nvramInnerRe captures the path inside a <nvram ...>PATH</nvram> element
	// (groups: 1=open tag, 2=path, 3=close tag). A self-closing or empty
	// <nvram> has no path and does not match.
	nvramInnerRe = regexp.MustCompile(`(<nvram(?:\s[^>]*)?>)([^<]+)(</nvram>)`)
)

// RewriteDiskSources replaces each <disk> <source file='OLD'/> whose OLD path
// is a key in remap with the mapped path, so a VM restored onto another host or
// pool finds its disks. Paths are matched exactly against the host paths the
// backup recorded; other sources, such as a cdrom ISO or a disk not being
// restored, stay as they are.
func RewriteDiskSources(domainXML string, remap map[string]string) string {
	if len(remap) == 0 {
		return domainXML
	}
	return diskSourceFileRe.ReplaceAllStringFunc(domainXML, func(m string) string {
		sub := diskSourceFileRe.FindStringSubmatch(m)
		if nw, ok := remap[sub[3]]; ok {
			return sub[1] + sub[2] + nw + sub[4]
		}
		return m
	})
}

// RewriteNVRAM sets the <nvram> host path to newPath so a restored UEFI domain
// reads its var store from the destination. BIOS domains (no <nvram>) and an
// empty <nvram> are returned unchanged. A domain has at most one nvram
// element, and only the first match is rewritten.
func RewriteNVRAM(domainXML, newPath string) string {
	done := false
	return nvramInnerRe.ReplaceAllStringFunc(domainXML, func(m string) string {
		if done {
			return m
		}
		done = true
		sub := nvramInnerRe.FindStringSubmatch(m)
		return sub[1] + newPath + sub[3]
	})
}
