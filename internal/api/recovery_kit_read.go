package api

import (
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"filippo.io/age"
	"filippo.io/age/armor"

	"github.com/junkerderprovinz/bombvault/internal/restickey"
)

var (
	// errKitSealed is a kit downloaded while export encryption was on. It is
	// sealed for the person's own age key, of which the server holds only the
	// public half, so nobody but them can open it.
	errKitSealed    = errors.New("this recovery kit is sealed with your age key, which BombVault does not have. Paste the age identity it was sealed for with it, or decrypt it first (age -d -i <key file> bombvault-recovery-kit.md.age) and upload the result")
	errNotAKit      = errors.New("this is not a BombVault recovery kit")
	errKitKeyBroken = errors.New("the APP_KEY in this recovery kit does not give the repository password beside it, so one of the two was changed or mistyped")
	errKitHasNoKey  = errors.New("this recovery kit names no key. A version that left the key out wrote it for a server without encryption")
	// errKitAmbiguous is a kit with two keys of a kind or the same section
	// twice. BombVault writes neither, and choosing one would let whoever
	// added the second decide which key is taken.
	errKitAmbiguous = errors.New("this recovery kit names a key or a section twice, so it is unclear which one counts. Use the kit as BombVault wrote it")
	// errKitLineBreak is a value that would break out of its line in the kit.
	errKitLineBreak = errors.New("the recovery kit cannot be written: a name, a location or a sign-in stored on this server spans more than one line. Correct it and try again")
)

// kitFence opens and closes the block a kit prints the rclone config in.
const kitFence = "```"

// spansLines reports whether one of the values would take more than its own
// line in a recovery kit.
func spansLines(values ...string) bool {
	return slices.ContainsFunc(values, func(v string) bool { return strings.ContainsAny(v, "\r\n") })
}

// breaksKitFence reports whether an rclone config has a line that would end
// the block the kit prints it in, after which the rest would read as kit.
func breaksKitFence(conf string) bool {
	for line := range strings.SplitSeq(conf, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), kitFence) {
			return true
		}
	}
	return false
}

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

// recoveryKit is what a recovery kit says. The keys stay on the server.
type recoveryKit struct {
	// Encrypted says the repositories carry a password.
	Encrypted bool `json:"encrypted"`
	// HasKey is false for the kit an install without encryption wrote before
	// kits named the key in that case. Its repositories can be read, and its
	// containers and VMs cannot be recreated from them.
	HasKey    bool          `json:"hasKey"`
	Locations []kitLocation `json:"locations"`
	// Server is the name of the install that wrote the kit and WrittenAt when
	// it did, in Unix seconds. A kit of an older version names no server.
	Server    string `json:"server,omitempty"`
	WrittenAt int64  `json:"writtenAt,omitempty"`
	// appKey is the key of the backups. sealingKey is the APP_KEY the install
	// sealed its settings with, the same key unless it had adopted another
	// install's backups.
	appKey, sealingKey string
	// creds and rcloneConf are the shared sign-ins the kit prints for its
	// remote repositories.
	creds      CloudCreds
	rcloneConf string
}

// The secrets the Encryption section of a kit prints, by the line that
// introduces each.
const (
	kitBackupKey = iota
	kitPassword
	kitSealingKey
)

var (
	kitLineRe        = regexp.MustCompile(`^- (.+?): (.+?)(?: \(compression: [a-z]+\))?$`)
	kitDomainRe      = regexp.MustCompile(`^([a-z]+) \((local|off-site)\)$`)
	kitDerivedLineRe = regexp.MustCompile(`^  ([a-z]+): (\S+)$`)
	kitMountRe       = regexp.MustCompile(`under the host data mount \((.+)\)\.$`)
	kitSecretRe      = regexp.MustCompile(`^ {4}([0-9a-f]{64})$`)
	kitEnvRe         = regexp.MustCompile(`^ {4}([A-Z0-9_]+)=(.*)$`)
)

// readRecoveryKit reads the document RecoveryKit writes.
func readRecoveryKit(text string) (recoveryKit, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if kitSealed(text) {
		return recoveryKit{}, errKitSealed
	}
	if !strings.Contains(text, "# BombVault encryption-key recovery kit") {
		return recoveryKit{}, errNotAKit
	}
	kit := recoveryKit{Locations: []kitLocation{}}
	var section, mount, destination string
	secrets, next := map[int]string{}, kitBackupKey
	seen := map[string]bool{}
	var conf []string
	fenced := false
	for line := range strings.SplitSeq(text, "\n") {
		// Inside the block everything is the rclone config, a line that looks
		// like a heading included.
		if line == kitFence {
			fenced = !fenced
			continue
		}
		if fenced {
			if section == "Repository credentials" {
				conf = append(conf, line)
			}
			continue
		}
		if heading, ok := strings.CutPrefix(line, "## "); ok {
			if seen[heading] {
				return recoveryKit{}, errKitAmbiguous
			}
			seen[heading] = true
			section = heading
			continue
		}
		switch section {
		case "":
			if at, ok := strings.CutPrefix(line, "Generated: "); ok && kit.WrittenAt == 0 {
				if t, err := time.Parse(time.RFC1123, at); err == nil {
					kit.WrittenAt = t.Unix()
				}
			} else if name, ok := strings.CutPrefix(line, "Server: "); ok && kit.Server == "" {
				kit.Server = name
			}
		case "Encryption":
			switch {
			case line == "Status: ENABLED":
				kit.Encrypted = true
			case strings.HasPrefix(line, "APP_KEY of this server"):
				next = kitSealingKey
			case strings.HasPrefix(line, "APP_KEY"), strings.HasPrefix(line, "Backup key"):
				next = kitBackupKey
			case strings.HasPrefix(line, "restic repository password"):
				next = kitPassword
			default:
				if m := kitSecretRe.FindStringSubmatch(line); m != nil {
					if _, twice := secrets[next]; twice {
						return recoveryKit{}, errKitAmbiguous
					}
					secrets[next] = m[1]
				}
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
		case "Repository credentials":
			if m := kitEnvRe.FindStringSubmatch(line); m != nil {
				kit.creds.set(m[1], m[2])
			}
		case "Destinations":
			if m := kitLineRe.FindStringSubmatch(line); m != nil {
				destination = m[1]
			} else if m := kitDerivedLineRe.FindStringSubmatch(line); m != nil {
				kit.Locations = append(kit.Locations, kitLocation{Domain: m[1], Kind: kitDestination, Name: destination, Location: m[2]})
			}
		}
	}
	kit.appKey = secrets[kitBackupKey]
	// With encryption on the kit prints the key and the password restic takes,
	// which is derived from it. A kit typed off a printout is caught by the pair.
	if kit.Encrypted && (kit.appKey == "" || restickey.Derive(kit.appKey) != secrets[kitPassword]) {
		return recoveryKit{}, errKitKeyBroken
	}
	kit.HasKey = kit.appKey != ""
	kit.rcloneConf = strings.Join(conf, "\n")
	kit.sealingKey = secrets[kitSealingKey]
	if kit.sealingKey == "" {
		kit.sealingKey = kit.appKey
	}
	mount = strings.TrimRight(filepath.ToSlash(mount), "/")
	for i, loc := range kit.Locations {
		if rest, ok := strings.CutPrefix(filepath.ToSlash(loc.Location), mount+"/"); ok && mount != "" {
			kit.Locations[i].Folder = rest
		}
	}
	return kit, nil
}

// set takes one of the variables the kit prints restic's sign-ins as.
func (c *CloudCreds) set(name, value string) {
	switch name {
	case "RESTIC_REST_USERNAME":
		c.RESTUser = value
	case "RESTIC_REST_PASSWORD":
		c.RESTPassword = value
	case "AWS_ACCESS_KEY_ID":
		c.S3KeyID = value
	case "AWS_SECRET_ACCESS_KEY":
		c.S3Secret = value
	case "AWS_DEFAULT_REGION":
		c.S3Region = value
	}
}

var (
	errKitIdentity       = errors.New("that is not an age identity. Paste the line that starts with AGE-SECRET-KEY-1, or decrypt the kit yourself and upload the result")
	errKitNotForIdentity = errors.New("this age identity does not open the kit")
)

// openSealedKit decrypts a kit sealed with age using the X25519 identity the
// person pasted. The identity is used for this call and kept nowhere, and no
// error carries anything age said about it.
func openSealedKit(sealed, identity string) (string, error) {
	id, err := age.ParseX25519Identity(strings.TrimSpace(identity))
	if err != nil {
		return "", errKitIdentity
	}
	r, err := age.Decrypt(armor.NewReader(strings.NewReader(strings.TrimSpace(sealed))), id)
	if err != nil {
		return "", errKitNotForIdentity
	}
	plain, err := io.ReadAll(io.LimitReader(r, 1<<20))
	if err != nil {
		return "", errKitNotForIdentity
	}
	return string(plain), nil
}

// kitSealed reports whether text is a kit sealed with age.
func kitSealed(text string) bool {
	head := strings.TrimSpace(text)
	return strings.HasPrefix(head, armor.Header) || strings.HasPrefix(head, "age-encryption.org/")
}

// uploadedKit reads a kit as it was uploaded: plain, or sealed with age
// together with the identity that opens it.
func uploadedKit(text, identity string) (recoveryKit, error) {
	if kitSealed(text) && strings.TrimSpace(identity) != "" {
		plain, err := openSealedKit(text, identity)
		if err != nil {
			return recoveryKit{}, err
		}
		text = plain
	}
	return readRecoveryKit(text)
}

// foreignKeyOf returns the key a request to open a foreign repository carries:
// the one typed, or the one in the recovery kit sent in its place.
func foreignKeyOf(typed, kitText, identity string) (string, error) {
	if typed != "" || kitText == "" {
		return typed, nil
	}
	kit, err := uploadedKit(kitText, identity)
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
	case errors.Is(err, errKitAmbiguous):
		code = "kit-ambiguous"
	case errors.Is(err, errKitIdentity):
		code = "kit-identity"
	case errors.Is(err, errKitNotForIdentity):
		code = "kit-not-for-identity"
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
// POST /api/recovery-kit/read  body {kit, identity}
func (h *Handler) handleReadRecoveryKit(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kit      string `json:"kit"`
		Identity string `json:"identity"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	kit, err := uploadedKit(body.Kit, body.Identity)
	if err != nil {
		kitFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"kit": kit}))
}
