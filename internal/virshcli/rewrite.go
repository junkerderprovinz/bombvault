package virshcli

import "regexp"

var (
	// diskSourceFileRe captures a <disk> <source file='PATH'/> element's path,
	// splitting the tag so only the path is rewritten (quote style preserved).
	// Groups: 1=`<source file=`, 2=quote, 3=path, 4=quote.
	diskSourceFileRe = regexp.MustCompile(`(<source\s+file=)(['"])([^'"]*)(['"])`)
	// nvramInnerRe captures the text path INSIDE a <nvram ...>PATH</nvram> element
	// (groups: 1=open tag, 2=path, 3=close tag). A self-closing / empty <nvram>
	// carries no path and is left untouched.
	nvramInnerRe = regexp.MustCompile(`(<nvram(?:\s[^>]*)?>)([^<]+)(</nvram>)`)
)

// RewriteDiskSources rewrites every <disk> <source file='OLD'/> whose OLD path
// is a key in remap to the mapped NEW path, leaving every other source (e.g. a
// cdrom ISO, or a disk not being restored) untouched. This is how a VM restored
// onto a DIFFERENT host/pool points libvirt at the disks' new location instead
// of the source server's paths. Paths are matched EXACTLY against the strings in
// the XML (the same absolute host paths the backup recorded). An empty remap
// returns the XML unchanged.
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

// RewriteNVRAM rewrites the <nvram>OLD</nvram> host path to newPath so a restored
// UEFI domain reads its var store from the destination location. BIOS domains (no
// <nvram>) and a self-closing/empty <nvram> element are returned unchanged. Only
// the FIRST nvram element is rewritten (a domain has at most one).
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
