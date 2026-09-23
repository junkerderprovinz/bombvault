package template

import (
	"path"
	"regexp"
	"strings"
)

var (
	// configInnerRe captures the text of a <Config ...>HOST_PATH</Config> element
	// (groups: 1=open tag, 2=text, 3=close tag). A self-closing <Config .../> has
	// no text and does not match.
	configInnerRe = regexp.MustCompile(`(<Config\b[^>]*>)([^<]*)(</Config>)`)
	// defaultAttrRe captures a Default="HOST_PATH" attribute (groups:
	// 1=`Default="`, 2=value, 3=closing quote).
	defaultAttrRe = regexp.MustCompile(`(Default=")([^"]*)(")`)
)

// RewriteHostPaths replaces every template host path that is a key in remap
// with its value: the text of a <Config> element, where Unraid keeps a bind's
// host path, and any Default attribute. Only exact matches change, so
// variables, container-side targets and other values stay as they are. A
// container restored onto another pool uses this to point its template at the
// new appdata location, as virshcli.RewriteDiskSources does for VMs.
func RewriteHostPaths(xml string, remap map[string]string) string {
	if len(remap) == 0 {
		return xml
	}
	// remap keys are cleaned, so a trailing or doubled slash in the template
	// still matches, as it does in rewriteBinds.
	xml = configInnerRe.ReplaceAllStringFunc(xml, func(m string) string {
		sub := configInnerRe.FindStringSubmatch(m)
		if v := strings.TrimSpace(sub[2]); v != "" {
			if nw, ok := remap[path.Clean(v)]; ok {
				return sub[1] + nw + sub[3]
			}
		}
		return m
	})
	xml = defaultAttrRe.ReplaceAllStringFunc(xml, func(m string) string {
		sub := defaultAttrRe.FindStringSubmatch(m)
		if sub[2] != "" {
			if nw, ok := remap[path.Clean(sub[2])]; ok {
				return sub[1] + nw + sub[3]
			}
		}
		return m
	})
	return xml
}
