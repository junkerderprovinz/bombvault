package api

import (
	"encoding/xml"
	"strings"
	"testing"
)

// TestUnraidTemplateParsesAndStaysAppendOnly: the Unraid template must parse
// and must keep --append-only. Unraid drops a template that does not parse from
// its dropdown without a word, leaving the user with a bare docker run that has
// no Edit form.
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

	// Parse the whole string, not a prefix: shell-style notes after
	// </Container> make the file invalid XML.
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

	// --append-only makes the destination enforce its own policy instead of
	// trusting the sender.
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

	// The htpasswd line is for the user to write by hand, never a template
	// field: Unraid keeps templates on the flash drive, and the credential is
	// shown only once.
	if !strings.Contains(snip.Unraid, snip.Htpasswd) {
		t.Error("the htpasswd line must appear in the instructions")
	}
	for _, c := range parsed.Configs {
		if strings.Contains(c.Value, snip.Htpasswd) || strings.Contains(c.Value, snip.Password) {
			t.Errorf("the credential must not be baked into a template field (%s)", c.Name)
		}
	}
	if strings.Contains(snip.Unraid, snip.Password) {
		t.Error("the plaintext password must never reach the template")
	}

	// The shared notes belong in the leading comment. Dropping them would also
	// pass the trailing-content check above.
	for _, want := range []string{"terminate TLS", "repo URL for BombVault"} {
		if !strings.Contains(snip.Unraid, want) {
			t.Errorf("the template lost its %q guidance", want)
		}
	}

	// An XML comment may not contain "--". The flags do, but they live in an
	// element.
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
