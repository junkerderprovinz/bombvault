// Package seedphrase turns the secret that groups a person's BombVault
// instances into twelve words, and back.
//
// The words come from the BIP39 English list because its 2048 words differ in
// their first four letters, have no near-homophones and no accents, which
// makes a phrase safe to read aloud and to type on a phone. 128 bits keep the
// phrase at twelve words; the BIP39 checksum catches a mistyped word before it
// turns into a group that silently never forms.
package seedphrase

import (
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"errors"
	"fmt"
	"strings"
)

//go:embed english.txt
var wordlistFile string

const (
	// SecretLen is the secret's size in bytes.
	SecretLen = 16
	// WordCount is what SecretLen encodes to: 128 bits of secret plus a
	// 4-bit checksum, in 11-bit groups.
	WordCount = 12

	bitsPerWord  = 11
	checksumBits = SecretLen * 8 / 32
)

var (
	words     []string
	wordIndex map[string]int
)

func init() {
	words = strings.Fields(wordlistFile)
	if len(words) != 1<<bitsPerWord {
		panic(fmt.Sprintf("seedphrase: wordlist has %d words, need %d", len(words), 1<<bitsPerWord))
	}
	wordIndex = make(map[string]int, len(words))
	for i, w := range words {
		wordIndex[w] = i
	}
}

// New mints a fresh secret and returns it with its phrase.
func New() (secret []byte, phrase string, err error) {
	secret = make([]byte, SecretLen)
	if _, err := rand.Read(secret); err != nil {
		return nil, "", fmt.Errorf("seedphrase: reading randomness: %w", err)
	}
	phrase, err = Encode(secret)
	if err != nil {
		return nil, "", err
	}
	return secret, phrase, nil
}

// Encode renders a secret as its phrase. It is a pure function of the secret,
// so the phrase shown again later is the one written down at the start.
func Encode(secret []byte) (string, error) {
	if len(secret) != SecretLen {
		return "", fmt.Errorf("seedphrase: secret is %d bytes, need %d", len(secret), SecretLen)
	}
	sum := sha256.Sum256(secret)
	full := make([]byte, 0, SecretLen+1)
	full = append(full, secret...)
	full = append(full, sum[0])

	out := make([]string, WordCount)
	for i := range out {
		out[i] = words[bitsAt(full, i*bitsPerWord, bitsPerWord)]
	}
	return strings.Join(out, " "), nil
}

// ErrChecksum is returned when every word is on the list but the phrase as a
// whole does not check out, which is what a typo landing on another real word
// or two swapped words look like.
var ErrChecksum = errors.New("seedphrase: the phrase is not valid; check for a mistyped or swapped word")

// Reason names what is wrong with a phrase, so the browser can write the
// sentence in the reader's language.
type Reason string

const (
	// ReasonWordCount is the wrong number of words.
	ReasonWordCount Reason = "word_count"
	// ReasonUnknownWord is a word that is not on the list.
	ReasonUnknownWord Reason = "unknown_word"
	// ReasonChecksum is a phrase of real words that does not check out.
	ReasonChecksum Reason = "checksum"
)

// DecodeError is every way Decode can reject a phrase, with the specifics a
// caller needs to point at the mistake.
type DecodeError struct {
	Reason Reason
	// Word and Position name the unknown word and its 1-based place.
	Word     string
	Position int
	// Count is how many words there were, for ReasonWordCount.
	Count int
}

func (e *DecodeError) Error() string {
	switch e.Reason {
	case ReasonWordCount:
		return fmt.Sprintf("seedphrase: the phrase has %d words, it needs %d", e.Count, WordCount)
	case ReasonUnknownWord:
		return fmt.Sprintf("seedphrase: word %d (%q) is not one of the accepted words", e.Position, e.Word)
	default:
		return ErrChecksum.Error()
	}
}

// Unwrap lets errors.Is(err, ErrChecksum) match the checksum case.
func (e *DecodeError) Unwrap() error {
	if e.Reason == ReasonChecksum {
		return ErrChecksum
	}
	return nil
}

// Decode parses a phrase back into its secret. Case is ignored and any run of
// whitespace separates words, since the input comes from a paste or from
// somebody typing what was read to them.
func Decode(phrase string) ([]byte, error) {
	got := strings.Fields(strings.ToLower(phrase))
	if len(got) != WordCount {
		return nil, &DecodeError{Reason: ReasonWordCount, Count: len(got)}
	}

	full := make([]byte, SecretLen+1)
	for i, w := range got {
		idx, ok := wordIndex[w]
		if !ok {
			return nil, &DecodeError{Reason: ReasonUnknownWord, Word: w, Position: i + 1}
		}
		setBits(full, i*bitsPerWord, bitsPerWord, idx)
	}

	secret := full[:SecretLen]
	sum := sha256.Sum256(secret)
	want := bitsAt(sum[:1], 0, checksumBits)
	have := bitsAt(full, SecretLen*8, checksumBits)
	if want != have {
		return nil, &DecodeError{Reason: ReasonChecksum}
	}
	return secret, nil
}

// bitsAt reads count bits from b starting at bit offset, most significant bit
// first.
func bitsAt(b []byte, offset, count int) int {
	v := 0
	for i := 0; i < count; i++ {
		bit := offset + i
		v <<= 1
		if b[bit/8]&(1<<(7-bit%8)) != 0 {
			v |= 1
		}
	}
	return v
}

// setBits writes the low count bits of v into b at bit offset, most
// significant bit first. The target bits must start clear.
func setBits(b []byte, offset, count, v int) {
	for i := 0; i < count; i++ {
		if v&(1<<(count-1-i)) != 0 {
			bit := offset + i
			b[bit/8] |= 1 << (7 - bit%8)
		}
	}
}
