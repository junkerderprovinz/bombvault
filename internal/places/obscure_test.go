package places

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"os/exec"
	"strings"
	"testing"
)

// reveal undoes Obscure the way rclone reads the variable back.
func reveal(t *testing.T, obscured string) string {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(obscured)
	if err != nil || len(raw) < aes.BlockSize {
		t.Fatalf("%q is not in rclone's obscure form: %v", obscured, err)
	}
	block, err := aes.NewCipher(rcloneObscureKey)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]byte, len(raw)-aes.BlockSize)
	cipher.NewCTR(block, raw[:aes.BlockSize]).XORKeyStream(out, raw[aes.BlockSize:])
	return string(out)
}

func TestObscureMatchesRclonesOwnVectors(t *testing.T) {
	// From rclone's fs/config/obscure/obscure_test.go, with the IVs it feeds in.
	for _, c := range []struct{ plain, iv, want string }{
		{"", "aaaaaaaaaaaaaaaa", "YWFhYWFhYWFhYWFhYWFhYQ"},
		{"potato", "aaaaaaaaaaaaaaaa", "YWFhYWFhYWFhYWFhYWFhYXMaGgIlEQ"},
		{"potato", "bbbbbbbbbbbbbbbb", "YmJiYmJiYmJiYmJiYmJiYp3gcEWbAw"},
	} {
		got, err := obscure(c.plain, []byte(c.iv))
		if err != nil || got != c.want {
			t.Errorf("obscure(%q) = %q, %v; want %q", c.plain, got, err, c.want)
		}
	}
}

func TestObscureGivesOnePasswordOneForm(t *testing.T) {
	a, err := Obscure("app-pass")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Obscure("app-pass")
	c, _ := Obscure("other-pass")
	if a != b {
		t.Fatalf("one password rendered twice as %q and %q", a, b)
	}
	if a == c {
		t.Fatal("two passwords rendered alike")
	}
	if got := reveal(t, a); got != "app-pass" {
		t.Fatalf("revealed %q", got)
	}
}

func TestRcloneRevealsWhatObscureWrote(t *testing.T) {
	bin, err := exec.LookPath("rclone")
	if err != nil {
		t.Skip("no rclone")
	}
	obscured, err := Obscure("app pass with spaces")
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(bin, "reveal", obscured).Output() //nolint:gosec // G204: the binary comes from LookPath and the argument from Obscure
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got != "app pass with spaces" {
		t.Fatalf("rclone revealed %q", got)
	}
}
