package virshcli

import (
	"regexp"
	"strings"
)

var (
	// diskSourceFileRe captures a <disk> <source file='PATH'/> element's path,
	// splitting the tag so only the path is rewritten (quote style preserved).
	// Groups: 1=`<source file=`, 2=quote, 3=path, 4=quote.
	diskSourceFileRe = regexp.MustCompile(`(<source\s+file=)(['"])([^'"]*)(['"])`)
	// diskSourceDevRe is diskSourceFileRe for a block device, <source dev='PATH'/>.
	diskSourceDevRe = regexp.MustCompile(`(<source\s+dev=)(['"])([^'"]*)(['"])`)
	// nvramInnerRe captures the path inside a <nvram ...>PATH</nvram> element
	// (groups: 1=open tag, 2=path, 3=close tag). A self-closing or empty
	// <nvram> has no path and does not match.
	nvramInnerRe = regexp.MustCompile(`(<nvram(?:\s[^>]*)?>)([^<]+)(</nvram>)`)

	backingStoreTagRe = regexp.MustCompile(`</?backingStore\b[^>]*>`)
	diskElementRe     = regexp.MustCompile(`(?s)<disk\b.*?</disk>`)
	// diskDriverTypeRe groups: 1=up to `type=`, 2=quote, 3=type, 4=quote.
	diskDriverTypeRe = regexp.MustCompile(`(<driver\b[^>]*?\stype=)(['"])([^'"]*)(['"])`)
)

// RewriteDiskSources replaces each <disk> <source file='OLD'/> whose OLD path
// is a key in remap with the mapped path, so a VM restored onto another host or
// pool finds its disks. Paths are matched exactly against the host paths the
// backup recorded; other sources, such as a cdrom ISO or a disk not being
// restored, stay as they are.
func RewriteDiskSources(domainXML string, remap map[string]string) string {
	return rewriteSources(diskSourceFileRe, domainXML, remap)
}

// RewriteBlockSources is RewriteDiskSources for block-device disks, so a VM
// whose zvols were restored onto another pool boots from them.
func RewriteBlockSources(domainXML string, remap map[string]string) string {
	return rewriteSources(diskSourceDevRe, domainXML, remap)
}

func rewriteSources(re *regexp.Regexp, domainXML string, remap map[string]string) string {
	if len(remap) == 0 {
		return domainXML
	}
	return re.ReplaceAllStringFunc(domainXML, func(m string) string {
		sub := re.FindStringSubmatch(m)
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

// StripBackingStore removes every <backingStore> from a domain XML, so libvirt
// reads the chain from the image headers. A definition that lists a chain makes
// qemu read through it whatever the image says, and a restored image is the
// only thing that knows its chain as it was backed up.
func StripBackingStore(domainXML string) string {
	const open, closeTag = "<backingStore", "</backingStore>"
	var b strings.Builder
	rest := domainXML
	for {
		i := strings.Index(rest, open)
		if i < 0 {
			b.WriteString(rest)
			return b.String()
		}
		b.WriteString(strings.TrimRight(rest[:i], " \t\r\n"))
		end, ok := backingStoreEnd(rest[i:])
		if !ok {
			b.WriteString(rest[i:])
			return b.String()
		}
		rest = rest[i+end:]
	}
}

// backingStoreEnd returns where the element s starts with ends. The elements
// nest, one per image of the chain, and the innermost is often self-closed.
func backingStoreEnd(s string) (int, bool) {
	depth, at := 0, 0
	for _, m := range backingStoreTagRe.FindAllString(s, -1) {
		at += strings.Index(s[at:], m) + len(m)
		switch {
		case strings.HasPrefix(m, "</"):
			depth--
		case !strings.HasSuffix(m, "/>"):
			depth++
		}
		if depth == 0 {
			return at, true
		}
	}
	return 0, false
}

// RewriteDiskFormat sets the driver type of the file disk at source, for a disk
// that is restored as a file of another format than the one it has now, such
// as a raw base under a qcow2 snapshot.
func RewriteDiskFormat(domainXML, source, format string) string {
	return diskElementRe.ReplaceAllStringFunc(domainXML, func(disk string) string {
		m := diskSourceFileRe.FindStringSubmatch(disk)
		if m == nil || m[3] != source {
			return disk
		}
		return diskDriverTypeRe.ReplaceAllString(disk, "${1}${2}"+format+"${4}")
	})
}
