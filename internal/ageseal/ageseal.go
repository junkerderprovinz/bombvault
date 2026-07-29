// Package ageseal wraps filippo.io/age to optionally encrypt BombVault's PLAIN
// export artifacts (the tool-free tar.gz / xml / zip exports) to one or more age
// public-key recipients. The restic repository is already encrypted; this covers
// only the deliberately-unencrypted export paths so they are safe to store or move
// off the box. Encryption is asymmetric: the server holds only recipient PUBLIC
// keys, and decryption happens off-box with the user's private key (age or SSH).
package ageseal

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"filippo.io/age"
)

// ParseRecipients parses a whitespace/newline-separated list of age recipients.
// Each non-blank, non-comment (#) token is parsed with age.ParseRecipients, which
// accepts native age recipients (age1...) AND SSH public keys (ssh-ed25519 ...,
// ssh-rsa ...). Multiple recipients are supported (the export is encrypted to all
// of them, so any one private key can decrypt it). An empty/blank string yields no
// recipients and no error; a NON-empty string that yields zero valid recipients is
// an error, so a caller that requires encryption never silently proceeds with none.
func ParseRecipients(s string) ([]age.Recipient, error) {
	var recipients []age.Recipient
	for _, line := range strings.Split(s, "\n") {
		// A '#' comment is line-level (matching age's own recipients-file format),
		// so a whole line beginning with '#' is skipped before tokenizing.
		if trimmed := strings.TrimSpace(line); trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		for _, tok := range strings.Fields(line) {
			rs, err := age.ParseRecipients(strings.NewReader(tok))
			if err != nil {
				return nil, fmt.Errorf("parse age recipient %q: %w", tok, err)
			}
			recipients = append(recipients, rs...)
		}
	}
	if strings.TrimSpace(s) != "" && len(recipients) == 0 {
		return nil, errors.New("no valid age recipients found")
	}
	return recipients, nil
}

// WrapWriter returns a WriteCloser that age-encrypts everything written to it to
// recipients, emitting ciphertext into dst. The caller MUST Close the returned
// writer to flush and finalize the age stream (Close does not close dst). It
// errors on an empty recipient set so encryption is never a no-op.
func WrapWriter(dst io.Writer, recipients []age.Recipient) (io.WriteCloser, error) {
	if len(recipients) == 0 {
		return nil, errors.New("ageseal: no recipients")
	}
	return age.Encrypt(dst, recipients...)
}

// EncryptFile streams the plaintext file at srcPath into a new age-encrypted file
// at dstPath (created/truncated, 0600). It does NOT remove srcPath (the caller
// owns the temp-file lifecycle). On any failure the partial dstPath is removed so
// no half-written ciphertext is left behind.
func EncryptFile(srcPath, dstPath string, recipients []age.Recipient) (err error) {
	if len(recipients) == 0 {
		return errors.New("ageseal: no recipients")
	}
	src, err := os.Open(srcPath) //nolint:gosec // G304: srcPath is a BombVault-owned export temp file
	if err != nil {
		return err
	}
	defer src.Close() //nolint:errcheck // read-only source

	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //nolint:gosec // G304: dstPath is under the operator-configured export dir
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = dst.Close()
			_ = os.Remove(dstPath)
		}
	}()

	w, err := age.Encrypt(dst, recipients...)
	if err != nil {
		return err
	}
	if _, err = io.Copy(w, src); err != nil {
		return err
	}
	if err = w.Close(); err != nil { // flush the age stream
		return err
	}
	return dst.Close()
}
