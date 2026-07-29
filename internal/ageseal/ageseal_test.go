package ageseal

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"
)

// TestWrapWriterRoundTrip encrypts bytes to an in-test identity's recipient and
// decrypts them back with the identity.
func TestWrapWriterRoundTrip(t *testing.T) {
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("bombvault export payload \x00\x01\x02 with binary")

	var cipher bytes.Buffer
	w, err := WrapWriter(&cipher, []age.Recipient{id.Recipient()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(plaintext); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(cipher.Bytes(), plaintext) {
		t.Fatal("ciphertext must differ from plaintext")
	}

	r, err := age.Decrypt(bytes.NewReader(cipher.Bytes()), id)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("round-trip mismatch: got %q", got)
	}
}

// TestEncryptFileRoundTrip streams a plaintext file to an age file and decrypts it.
func TestEncryptFileRoundTrip(t *testing.T) {
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "plain.bin")
	dst := filepath.Join(dir, "plain.bin.age")
	payload := bytes.Repeat([]byte("A"), 4096)
	if err := os.WriteFile(src, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EncryptFile(src, dst, []age.Recipient{id.Recipient()}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatal("EncryptFile must not remove the source")
	}
	cipher, err := os.ReadFile(dst) //nolint:gosec // test file
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(cipher, payload) {
		t.Fatal("ciphertext must differ from plaintext")
	}
	r, err := age.Decrypt(bytes.NewReader(cipher), id)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("decrypted file mismatch")
	}
}

// TestParseRecipients covers the empty, garbage, comment and multi-recipient cases.
func TestParseRecipients(t *testing.T) {
	id1, _ := age.GenerateX25519Identity()
	id2, _ := age.GenerateX25519Identity()

	t.Run("empty yields no recipients, no error", func(t *testing.T) {
		rs, err := ParseRecipients("   \n\t  ")
		if err != nil {
			t.Fatalf("empty must not error: %v", err)
		}
		if len(rs) != 0 {
			t.Fatalf("empty must yield no recipients, got %d", len(rs))
		}
	})

	t.Run("garbage errors", func(t *testing.T) {
		if _, err := ParseRecipients("garbage"); err == nil {
			t.Fatal("garbage must error")
		}
	})

	t.Run("multiple recipients with comments and blank lines", func(t *testing.T) {
		in := "# my keys\n" + id1.Recipient().String() + "\n\n" + id2.Recipient().String() + "\n"
		rs, err := ParseRecipients(in)
		if err != nil {
			t.Fatal(err)
		}
		if len(rs) != 2 {
			t.Fatalf("expected 2 recipients, got %d", len(rs))
		}
	})
}

// TestWrapWriterNoRecipients: wrapping with no recipients is an error (never a
// silent plaintext passthrough).
func TestWrapWriterNoRecipients(t *testing.T) {
	if _, err := WrapWriter(io.Discard, nil); err == nil {
		t.Fatal("WrapWriter with no recipients must error")
	}
	if err := EncryptFile("x", "y", nil); err == nil {
		t.Fatal("EncryptFile with no recipients must error")
	}
}
