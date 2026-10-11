package api

import (
	"errors"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"filippo.io/age/armor"

	"github.com/junkerderprovinz/bombvault/internal/restickey"
)

var (
	// errKitSealed is a kit downloaded while export encryption was on. It is
	// sealed for the person's own age key, of which the server holds only the
	// public half, so nobody but them can open it.
	errKitSealed    = errors.New("this recovery kit is sealed with your age key, which BombVault does not have: decrypt it first (age -d -i <key file> bombvault-recovery-kit.md.age) and upload the result")
	errNotAKit      = errors.New("this is not a BombVault recovery kit")
	errKitKeyBroken = errors.New("the APP_KEY in this recovery kit does not give the repository password beside it, so one of the two was changed or mistyped")
	errKitHasNoKey  = errors.New("this recovery kit names no APP_KEY, because its instance ran without encryption")
)

// The kinds of place a recovery kit lists.
const (
	kitLocal       = "local"
	kitOffsite     = "offsite"
	kitNamed       = "named"
	kitDestination = "destination"
)

// kitLocation is one repository a recovery kit names.
type kitLocation struct {
	// Domain is the section the repository belongs to, empty for a named
	// repository, which can hold entries of several.
	Domain string `json:"domain,omitempty"`
	Kind   string `json:"kind"`
	// Name is what a named repository or a destination was called.
	Name     string `json:"name,omitempty"`
	Location string `json:"location"`
	// Folder is a local location below the host mount of the instance that
	// wrote the kit, the form POST /api/foreign/open takes one in. A remote
	// location has none.
	Folder string `json:"folder,omitempty"`
}

// recoveryKit is what a recovery kit says. The key stays on the server.
type recoveryKit struct {
	// Encrypted says the repositories carry a password. A kit of an instance
	// without encryption names no key, so its repositories can be read and its
	// containers and VMs cannot be recreated from them.
	Encrypted bool          `json:"encrypted"`
	Locations []kitLocation `json:"locations"`
	appKey    string
}

var (
	kitLineRe        = regexp.MustCompile(`^- (.+?): (.+?)(?: \(compression: [a-z]+\))?$`)
	kitDomainRe      = regexp.MustCompile(`^([a-z]+) \((local|off-site)\)$`)
	kitDerivedLineRe = regexp.MustCompile(`^  ([a-z]+): (\S+)$`)
	kitMountRe       = regexp.MustCompile(`under the host data mount \((.+)\)\.$`)
	kitSecretRe      = regexp.MustCompile(`^ {4}([0-9a-f]{64})$`)
)

// readRecoveryKit reads the document RecoveryKit writes.
func readRecoveryKit(text string) (recoveryKit, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if head := strings.TrimSpace(text); strings.HasPrefix(head, armor.Header) || strings.HasPrefix(head, "age-encryption.org/") {
		return recoveryKit{}, errKitSealed
	}
	if !strings.Contains(text, "# BombVault encryption-key recovery kit") {
		return recoveryKit{}, errNotAKit
	}
	kit := recoveryKit{Locations: []kitLocation{}}
	var section, mount, destination string
	var secrets []string
	for line := range strings.SplitSeq(text, "\n") {
		if heading, ok := strings.CutPrefix(line, "## "); ok {
			section = heading
			continue
		}
		switch section {
		case "Encryption":
			if line == "Status: ENABLED" {
				kit.Encrypted = true
			}
			if m := kitSecretRe.FindStringSubmatch(line); m != nil {
				secrets = append(secrets, m[1])
			}
		case "Repository locations", "BombVault settings backup (config domain)":
			if m := kitMountRe.FindStringSubmatch(line); m != nil {
				mount = m[1]
			}
			m := kitLineRe.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			if d := kitDomainRe.FindStringSubmatch(m[1]); d != nil && m[2] != orNone("") {
				kind := kitLocal
				if d[2] == "off-site" {
					kind = kitOffsite
				}
				kit.Locations = append(kit.Locations, kitLocation{Domain: d[1], Kind: kind, Location: m[2]})
			}
		case "Named repositories (per-item)":
			if m := kitLineRe.FindStringSubmatch(line); m != nil {
				kit.Locations = append(kit.Locations, kitLocation{Kind: kitNamed, Name: m[1], Location: m[2]})
			}
		case "Destinations":
			if m := kitLineRe.FindStringSubmatch(line); m != nil {
				destination = m[1]
			} else if m := kitDerivedLineRe.FindStringSubmatch(line); m != nil {
				kit.Locations = append(kit.Locations, kitLocation{Domain: m[1], Kind: kitDestination, Name: destination, Location: m[2]})
			}
		}
	}
	if kit.Encrypted {
		// The kit prints the key and the password restic takes, which is
		// derived from it. A kit typed off a printout is caught by the pair.
		if len(secrets) != 2 || restickey.Derive(secrets[0]) != secrets[1] {
			return recoveryKit{}, errKitKeyBroken
		}
		kit.appKey = secrets[0]
	}
	mount = strings.TrimRight(filepath.ToSlash(mount), "/")
	for i, loc := range kit.Locations {
		if rest, ok := strings.CutPrefix(filepath.ToSlash(loc.Location), mount+"/"); ok && mount != "" {
			kit.Locations[i].Folder = rest
		}
	}
	return kit, nil
}

// foreignKeyOf returns the key a request to open a foreign repository carries:
// the one typed, or the one in the recovery kit sent in its place.
func foreignKeyOf(typed, kitText string) (string, error) {
	if typed != "" || kitText == "" {
		return typed, nil
	}
	kit, err := readRecoveryKit(kitText)
	if err != nil {
		return "", err
	}
	if kit.appKey == "" {
		return "", errKitHasNoKey
	}
	return kit.appKey, nil
}

func kitFail(w http.ResponseWriter, err error) {
	code := ""
	switch {
	case errors.Is(err, errKitSealed):
		code = "kit-sealed"
	case errors.Is(err, errNotAKit):
		code = "not-a-kit"
	case errors.Is(err, errKitKeyBroken):
		code = "kit-key-broken"
	case errors.Is(err, errKitHasNoKey):
		code = "kit-no-key"
	default:
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	// These sentences name no path of this server, and the scrubber would cut
	// the file name out of the hint on how to open a sealed kit.
	writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error(), "code": code})
}

// handleReadRecoveryKit reads an uploaded recovery kit and answers with the
// repositories it names. The key in it is checked and not sent back: a request
// to open one of the repositories carries the kit again.
// POST /api/recovery-kit/read  body {kit}
func (h *Handler) handleReadRecoveryKit(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kit string `json:"kit"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	kit, err := readRecoveryKit(body.Kit)
	if err != nil {
		kitFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"kit": kit}))
}
