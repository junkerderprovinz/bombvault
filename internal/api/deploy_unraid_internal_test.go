package api

import (
	"encoding/xml"
	"strings"
	"testing"
)

// The Unraid template has to PARSE, and it has to keep the one flag that makes
// the destination immutable ([601]).
//
// Reported as "I couldn't get the scripted deployment of a restic docker
// functional, and it couldn't be edited in the docker UI, so I used one from CA
// instead." The docker-run recipe itself is sound — verified end to end against
// a real rest-server, where `restic init` created a repository through it — so
// what failed was the FORMAT. A container Unraid did not create from a template
// has no Edit form behind it.
//
// A template that does not parse is worse than no template: Unraid drops it from
// the dropdown silently, and the user is back to the bare docker run without
// being told why.
func TestUnraidTemplateParsesAndStaysAppendOnly(t *testing.T) {
	snip, err := buildDeploySnippet("containers")
	if err != nil {
		t.Fatalf("buildDeploySnippet: %v", err)
	}
	if snip.Unraid == "" {
		t.Fatal("no Unraid template generated")
	}

	// The declaration must lead. An XML comment before it is not valid XML, and
	// this text is saved verbatim as a file Unraid parses.
	if !strings.HasPrefix(snip.Unraid, `<?xml version="1.0"?>`) {
		t.Errorf("the XML declaration must come first, got %.40q", snip.Unraid)
	}

	// Parse THE WHOLE STRING, never a prefix of it. The first version of this
	// test truncated at </Container> before parsing, on the assumption that the
	// trailing guidance lines were not part of the document. They were: the
	// generator appended the shared `# ...` notes after the root element, which
	// is not valid XML, and the whole file Unraid reads was broken while this
	// test stayed green. It measured a prefix and reported on a file.
	doc := snip.Unraid
	i := strings.Index(doc, "</Container>")
	if i < 0 {
		t.Fatal("template has no </Container> close tag")
	}
	if tail := strings.TrimSpace(doc[i+len("</Container>"):]); tail != "" {
		t.Errorf("nothing may follow the root element, got %q after </Container>", tail)
	}
	var parsed struct {
		XMLName xml.Name `xml:"Container"`
		Name    string   `xml:"Name"`
		Repo    string   `xml:"Repository"`
		Configs []struct {
			Name   string `xml:"Name,attr"`
			Target string `xml:"Target,attr"`
			Value  string `xml:",chardata"`
		} `xml:"Config"`
	}
	if err := xml.Unmarshal([]byte(doc), &parsed); err != nil {
		t.Fatalf("template does not parse as XML: %v", err)
	}
	if parsed.Name == "" || parsed.Repo == "" {
		t.Errorf("template needs a Name and a Repository, got %q / %q", parsed.Name, parsed.Repo)
	}

	// The OPTIONS field is the whole point: --append-only is what makes this a
	// destination that enforces its own policy rather than trusting the sender.
	var options string
	for _, c := range parsed.Configs {
		if c.Target == "OPTIONS" {
			options = c.Value
		}
	}
	if options == "" {
		t.Fatal("template has no OPTIONS field")
	}
	for _, flag := range []string{"--append-only", "--private-repos", "--htpasswd-file"} {
		if !strings.Contains(options, flag) {
			t.Errorf("OPTIONS must carry %s, got %q", flag, options)
		}
	}

	// The bcrypt line is shown for the user to write by hand and must NOT be a
	// template field: Unraid keeps templates on the flash drive indefinitely,
	// and this is a credential displayed exactly once.
	if !strings.Contains(snip.Unraid, snip.Htpasswd) {
		t.Error("the htpasswd line must appear in the instructions")
	}
	for _, c := range parsed.Configs {
		if strings.Contains(c.Value, snip.Htpasswd) || strings.Contains(c.Value, snip.Password) {
			t.Errorf("the credential must not be baked into a template field (%s)", c.Name)
		}
	}
	// And the one-time plaintext password must not be anywhere in the file.
	if strings.Contains(snip.Unraid, snip.Password) {
		t.Error("the plaintext password must never reach the template")
	}

	// The two shared notes moved INTO the leading comment when the trailing
	// lines were removed. Without this, deleting them would silently satisfy
	// the "nothing follows the root element" check above.
	for _, want := range []string{"terminate TLS", "repo URL for BombVault"} {
		if !strings.Contains(snip.Unraid, want) {
			t.Errorf("the template lost its %q guidance", want)
		}
	}

	// An XML comment may not contain "--". The OPTIONS flags legitimately do,
	// but they live in an element; the comment block must stay clean, or the
	// file stops parsing for a reason nobody would look for.
	if start := strings.Index(snip.Unraid, "<!--"); start >= 0 {
		end := strings.Index(snip.Unraid[start:], "-->")
		if end < 0 {
			t.Fatal("the leading comment is never closed")
		}
		if body := snip.Unraid[start+len("<!--") : start+end]; strings.Contains(body, "--") {
			t.Errorf("the comment block contains a double dash and will not parse: %q", body)
		}
	}
}
