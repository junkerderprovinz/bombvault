package seedphrase

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	for i := 0; i < 2000; i++ {
		secret, phrase, err := New()
		if err != nil {
			t.Fatalf("New() = %v", err)
		}
		if got := len(strings.Fields(phrase)); got != WordCount {
			t.Fatalf("phrase has %d words, want %d: %q", got, WordCount, phrase)
		}
		back, err := Decode(phrase)
		if err != nil {
			t.Fatalf("Decode(%q) = %v", phrase, err)
		}
		if !bytes.Equal(secret, back) {
			t.Fatalf("round trip changed the secret:\n in: %x\nout: %x", secret, back)
		}
	}
}

func TestEncodeIsStable(t *testing.T) {
	secret := make([]byte, SecretLen)
	for i := range secret {
		secret[i] = byte(i * 7)
	}
	first, err := Encode(secret)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Encode(secret)
	if err != nil {
		t.Fatal(err)
	}
	if again != first {
		t.Fatalf("Encode is not stable: %q then %q", first, again)
	}
}

// The canonical BIP39 all-zero vector catches an off-by-one in the bit
// packing, since no set bit can hide behind another.
func TestAllZeroSecretMatchesTheBIP39Vector(t *testing.T) {
	secret := make([]byte, SecretLen)
	phrase, err := Encode(secret)
	if err != nil {
		t.Fatal(err)
	}
	want := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	if phrase != want {
		t.Fatalf("Encode(all zero) = %q, want %q", phrase, want)
	}
	back, err := Decode(phrase)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(back, secret) {
		t.Fatalf("Decode = %x, want %x", back, secret)
	}
}

// Four checksum bits catch about fifteen of sixteen substitutions of one real
// word for another, so every alternative first word is tried against one
// fixed phrase instead of a coin flip on a random one.
func TestChecksumCatchesASwappedWord(t *testing.T) {
	phrase, err := Encode(make([]byte, SecretLen))
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Fields(phrase)

	slipped := 0
	for _, w := range words {
		if w == got[0] {
			continue
		}
		try := append([]string{w}, got[1:]...)
		_, err := Decode(strings.Join(try, " "))
		if err == nil {
			slipped++
			continue
		}
		if !errors.Is(err, ErrChecksum) {
			t.Fatalf("substituting %q: Decode error = %v, want ErrChecksum", w, err)
		}
		var de *DecodeError
		if !errors.As(err, &de) || de.Reason != ReasonChecksum {
			t.Fatalf("substituting %q: error %v carries no checksum reason", w, err)
		}
	}
	const expect = 2047 / 16
	if slipped < expect-40 || slipped > expect+40 {
		t.Fatalf("%d of 2047 substitutions went undetected, expected about %d", slipped, expect)
	}
}

func TestUnknownWordIsNamedWithItsPosition(t *testing.T) {
	_, phrase, err := New()
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Fields(phrase)
	got[6] = "recieve"

	_, err = Decode(strings.Join(got, " "))
	var de *DecodeError
	if !errors.As(err, &de) {
		t.Fatalf("Decode error = %v, want a DecodeError", err)
	}
	if de.Reason != ReasonUnknownWord || de.Word != "recieve" || de.Position != 7 {
		t.Fatalf("DecodeError = %+v, want unknown_word recieve at 7", *de)
	}
	if msg := err.Error(); !strings.Contains(msg, "recieve") || !strings.Contains(msg, "7") {
		t.Errorf("error does not name the word and its position: %q", msg)
	}
}

func TestWrongWordCountReportsTheCount(t *testing.T) {
	_, phrase, err := New()
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Fields(phrase)
	for _, c := range []struct {
		name  string
		words []string
	}{
		{"one short", got[:WordCount-1]},
		{"one too many", append(append([]string{}, got...), got[0])},
		{"empty", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := Decode(strings.Join(c.words, " "))
			var de *DecodeError
			if !errors.As(err, &de) || de.Reason != ReasonWordCount || de.Count != len(c.words) {
				t.Fatalf("Decode of %d words = %v, want word_count with that count", len(c.words), err)
			}
		})
	}
}

func TestDecodeNormalisesPastedInput(t *testing.T) {
	secret, phrase, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ws := strings.Fields(phrase)
	for _, c := range []struct {
		name  string
		input string
	}{
		{"as generated", phrase},
		{"upper case", strings.ToUpper(phrase)},
		{"surrounding space", "   " + phrase + "   \n"},
		{"newlines between words", strings.Join(ws, "\n")},
		{"double spaces", strings.Join(ws, "  ")},
		{"tabs", strings.Join(ws, "\t")},
	} {
		t.Run(c.name, func(t *testing.T) {
			back, err := Decode(c.input)
			if err != nil {
				t.Fatalf("Decode(%q) = %v", c.input, err)
			}
			if !bytes.Equal(back, secret) {
				t.Fatalf("Decode(%q) = %x, want %x", c.input, back, secret)
			}
		})
	}
}

func TestEncodeRejectsWrongSecretLength(t *testing.T) {
	for _, n := range []int{0, 1, SecretLen - 1, SecretLen + 1, 32} {
		if _, err := Encode(make([]byte, n)); err == nil {
			t.Errorf("Encode accepted a %d-byte secret", n)
		}
	}
}

// A changed word in the embedded list would silently turn every written-down
// phrase into a different secret.
func TestWordlistIsTheOfficialOne(t *testing.T) {
	const official = "2f5eed53a4727b4bf8880d8f3f199efc90e58503646d9ff8eff3a2ed3b24dbda"
	sum := sha256.Sum256([]byte(wordlistFile))
	if got := hex.EncodeToString(sum[:]); got != official {
		t.Fatalf("wordlist hash = %s, want the official BIP39 English list %s", got, official)
	}
	if words[0] != "abandon" || words[len(words)-1] != "zoo" {
		t.Fatalf("wordlist bounds = %q..%q, want abandon..zoo", words[0], words[len(words)-1])
	}
}

// The Pairing tab checks each typed word against its own copy of the list,
// which has to be this one or it rejects good words and passes bad ones.
func TestTheBrowserWordListIsThisOne(t *testing.T) {
	browser, err := os.ReadFile(filepath.Join("..", "..", "web", "src", "lib", "bip39-english.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(browser, []byte(wordlistFile)) {
		t.Fatal("web/src/lib/bip39-english.txt differs from english.txt; copy english.txt over it")
	}
}
