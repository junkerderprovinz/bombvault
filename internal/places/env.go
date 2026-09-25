package places

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"strings"
)

// Creds is the decrypted credential set of a place; it mirrors the fields of
// api.CloudCredSet that places use. WebDAVPass is plain text; Env obscures it.
type Creds struct {
	S3KeyID, S3Secret, S3Region, S3StorageClass     string
	RESTUser, RESTPassword                          string
	WebDAVURL, WebDAVVendor, WebDAVUser, WebDAVPass string
	AzureAccount, AzureKey                          string
}

// RemoteName is the rclone remote a WebDAV place's address names. rclone reads
// a remote from RCLONE_CONFIG_<NAME>_* variables, which leaves letters and
// digits for the name.
func RemoteName(placeID string) string {
	var b strings.Builder
	b.WriteString("bvp")
	for _, r := range strings.ToLower(placeID) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// RemotePlace is the place id an address names through a WebDAV place's
// remote, and "" for any other address. A row a save detached from its place
// keeps its address, so the environment follows the address, not the row.
func RemotePlace(repo string) string {
	rest, ok := strings.CutPrefix(repo, "rclone:bvp")
	if !ok {
		return ""
	}
	id, _, ok := strings.Cut(rest, ":")
	if !ok {
		return ""
	}
	return id
}

// Env renders a place's credentials into the variables restic and rclone read,
// leaving out every unset value. local and sftp places need none, since sftp
// signs in with the container's SSH key, and an rclone place's remote comes
// from RCLONE_CONFIG.
func Env(kind Kind, c Creds, placeID string) ([]string, error) {
	var env []string
	add := func(key, value string) {
		if value != "" {
			env = append(env, key+"="+value)
		}
	}
	switch kind {
	case KindS3:
		add("AWS_ACCESS_KEY_ID", c.S3KeyID)
		add("AWS_SECRET_ACCESS_KEY", c.S3Secret)
		add("AWS_DEFAULT_REGION", c.S3Region)
	case KindREST:
		add("RESTIC_REST_USERNAME", c.RESTUser)
		add("RESTIC_REST_PASSWORD", c.RESTPassword)
	case KindWebDAV:
		prefix := "RCLONE_CONFIG_" + strings.ToUpper(RemoteName(placeID)) + "_"
		add(prefix+"TYPE", "webdav")
		add(prefix+"URL", c.WebDAVURL)
		add(prefix+"VENDOR", c.WebDAVVendor)
		add(prefix+"USER", c.WebDAVUser)
		if c.WebDAVPass != "" {
			pass, err := Obscure(c.WebDAVPass)
			if err != nil {
				return nil, err
			}
			add(prefix+"PASS", pass)
		}
	case KindAzure:
		add("AZURE_ACCOUNT_NAME", c.AzureAccount)
		add("AZURE_ACCOUNT_KEY", c.AzureKey)
	}
	return env, nil
}

// Collides reports whether two environments set one variable to different
// values, which one restic process cannot hold at once.
func Collides(a, b []string) bool {
	set := make(map[string]string, len(a))
	for _, kv := range a {
		key, value, _ := strings.Cut(kv, "=")
		set[key] = value
	}
	for _, kv := range b {
		key, value, _ := strings.Cut(kv, "=")
		if have, ok := set[key]; ok && have != value {
			return true
		}
	}
	return false
}

// Needed keeps the variables of env that the backend of repo reads. Only those
// have to agree when two repositories share one restic process.
func Needed(repo string, env []string) []string {
	var prefix string
	switch scheme, _, _ := strings.Cut(repo, ":"); scheme {
	case "s3":
		prefix = "AWS_"
	case "rest":
		prefix = "RESTIC_REST_"
	case "azure":
		prefix = "AZURE_"
	case "rclone":
		prefix = "RCLONE_CONFIG_"
	default:
		return nil
	}
	var out []string
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			out = append(out, kv)
		}
	}
	return out
}

// rcloneObscureKey is the fixed key rclone obscures passwords with, from its
// fs/config/obscure package. Obscuring keeps a password from being read at a
// glance and protects nothing beyond that.
var rcloneObscureKey = []byte{
	0x9c, 0x93, 0x5b, 0x48, 0x73, 0x0a, 0x55, 0x4d,
	0x6b, 0xfd, 0x7c, 0x63, 0xc8, 0x86, 0xa9, 0x2b,
	0xd3, 0x90, 0x19, 0x8e, 0xb8, 0x12, 0x8a, 0xfb,
	0xf4, 0xde, 0x16, 0x2b, 0x8b, 0x95, 0xf6, 0x38,
}

// Obscure returns plain in the form rclone expects for a password set through
// RCLONE_CONFIG_<REMOTE>_PASS: AES-CTR under rclone's key, the IV in front,
// base64 URL encoding without padding. The IV comes from the password rather
// than from chance, so one password always renders the same environment and
// two renderings of one place never look like a clash.
func Obscure(plain string) (string, error) {
	sum := sha256.Sum256([]byte(plain))
	return obscure(plain, sum[:aes.BlockSize])
}

func obscure(plain string, iv []byte) (string, error) {
	block, err := aes.NewCipher(rcloneObscureKey)
	if err != nil {
		return "", err
	}
	out := make([]byte, aes.BlockSize+len(plain))
	copy(out, iv)
	cipher.NewCTR(block, iv).XORKeyStream(out[aes.BlockSize:], []byte(plain))
	return base64.RawURLEncoding.EncodeToString(out), nil
}
