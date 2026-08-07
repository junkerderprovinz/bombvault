package template

import (
	"path"
	"regexp"
	"strings"
)

var (
	// configInnerRe captures an Unraid <Config …>HOST_PATH</Config> element's text
	// content (groups: 1=open tag, 2=inner text, 3=close tag). A self-closing
	// <Config …/> element carries no text and is left untouched.
	configInnerRe = regexp.MustCompile(`(<Config\b[^>]*>)([^<]*)(</Config>)`)
	// defaultAttrRe captures a Default="HOST_PATH" attribute value (groups:
	// 1=`Default="`, 2=value, 3=closing quote). Rewritten only on an exact match,
	// so empty defaults and non-path values are left alone.
	defaultAttrRe = regexp.MustCompile(`(Default=")([^"]*)(")`)
)

// RewriteHostPaths rewrites every Unraid template host path that is a key in
// remap to its mapped value — both the <Config …>HOST_PATH</Config> element text
// (Unraid stores a volume bind's host path there) and any Default="HOST_PATH"
// attribute. Matching is EXACT against the recorded absolute host paths, so env
// Variables, container-side Target paths and unrelated values are untouched. This
// is how a container restored onto a DIFFERENT pool gets its flashed template to
// point at the appdata's new location instead of the source host's path (issue
// #125), mirroring virshcli.RewriteDiskSources for VMs. An empty remap returns
// the XML unchanged.
func RewriteHostPaths(xml string, remap map[string]string) string {
	if len(remap) == 0 {
		return xml
	}
	// Canonicalize the matched host path before the lookup (remap keys are
	// path.Clean'd), so a non-canonical template path (trailing/doubled slash)
	// is rewritten the same way rewriteBinds handles the docker bind.
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
