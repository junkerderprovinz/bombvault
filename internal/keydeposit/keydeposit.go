// Package keydeposit seals what brings an install's backups back, so that a
// member of its pairing group can keep it without being able to read it.
//
// The deposit is sealed to an X25519 key pair derived from a recovery code the
// person is shown once. The install keeps the public half and the salt, which
// lets it seal a fresh deposit whenever its storage locations change. The
// private half exists only while the code is being typed on the day it is
// needed, and is derived again there.
package keydeposit

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	"golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/scrypt"
)

// Version is the form of a deposit this package writes and reads.
const Version = 1

const (
	// codeAlphabet is Crockford's base32: no I, L, O or U, so a code read off
	// paper has no two characters that look alike.
	codeAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	codeLen      = 20
	codeGroup    = 5
	saltLen      = 16

	// A member that keeps a deposit can try codes against it for as long as it
	// likes. These cost it about a second and 256 MiB a guess on a desktop
	// processor, against the hundred bits of a code.
	scryptN = 1 << 18
	scryptR = 8
	scryptP = 2
)

var (
	// ErrCode is returned for a code that is not twenty characters of the
	// code alphabet once spaces, dashes and look-alikes are set right.
	ErrCode = errors.New("the recovery code has twenty letters and digits, in four groups of five")
	// ErrWrongCode is returned for a deposit the code does not open.
	ErrWrongCode = errors.New("the recovery code does not open this deposit")
	// ErrBusy is returned while as many codes wait to be stretched as are
	// let wait.
	ErrBusy = errors.New("too many recovery codes are being tried at once. Try again in a moment")
)

// Stretching a code takes 256 MiB, so it is done for one code at a time, and
// no more than maxWaiting others wait their turn. A server in a container
// with a memory limit would otherwise be brought down by a handful of
// attempts at once.
const maxWaiting = 4

var (
	stretching = make(chan struct{}, 1)
	waiting    atomic.Int32
)

// NewCode returns a fresh recovery code as it is shown: four groups of five.
func NewCode() (string, error) {
	raw := make([]byte, codeLen)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("keydeposit: code: %w", err)
	}
	var b strings.Builder
	for i, x := range raw {
		if i > 0 && i%codeGroup == 0 {
			b.WriteByte('-')
		}
		b.WriteByte(codeAlphabet[int(x)%len(codeAlphabet)])
	}
	return b.String(), nil
}

// NewSalt returns the salt a code is stretched with.
func NewSalt() ([]byte, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("keydeposit: salt: %w", err)
	}
	return salt, nil
}

// normalize brings a typed code to the twenty characters it was made of.
func normalize(code string) (string, error) {
	code = strings.NewReplacer(" ", "", "-", "", "\t", "", "O", "0", "I", "1", "L", "1").Replace(strings.ToUpper(code))
	if len(code) != codeLen || strings.Trim(code, codeAlphabet) != "" {
		return "", ErrCode
	}
	return code, nil
}

// CheckCode reports ErrCode for a code that cannot be one, without the cost of
// stretching it.
func CheckCode(code string) error {
	_, err := normalize(code)
	return err
}

// keyPair derives the X25519 key pair of a code. The private half is the
// stretched code and is never stored: whoever holds a deposit and its salt
// would open the deposit with it.
func keyPair(code string, salt []byte) (public, private *[32]byte, err error) {
	code, err = normalize(code)
	if err != nil {
		return nil, nil, err
	}
	if waiting.Add(1) > maxWaiting+1 {
		waiting.Add(-1)
		return nil, nil, ErrBusy
	}
	stretching <- struct{}{}
	seed, err := scrypt.Key([]byte(code), salt, scryptN, scryptR, scryptP, 32)
	<-stretching
	waiting.Add(-1)
	if err != nil {
		return nil, nil, fmt.Errorf("keydeposit: stretch the code: %w", err)
	}
	public, private, err = box.GenerateKey(bytes.NewReader(seed))
	if err != nil {
		return nil, nil, fmt.Errorf("keydeposit: derive the key pair: %w", err)
	}
	return public, private, nil
}

// Recipient returns the public key deposits for code are sealed to.
func Recipient(code string, salt []byte) ([]byte, error) {
	public, _, err := keyPair(code, salt)
	if err != nil {
		return nil, err
	}
	return public[:], nil
}

// Seal seals plain to a recipient. Nothing but the code the recipient was
// derived from opens the result.
func Seal(plain, recipient []byte) ([]byte, error) {
	if len(recipient) != 32 {
		return nil, errors.New("keydeposit: the recipient is not an X25519 public key")
	}
	sealed, err := box.SealAnonymous(nil, plain, (*[32]byte)(recipient), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("keydeposit: seal: %w", err)
	}
	return sealed, nil
}

// Open opens a deposit with the code and the salt kept beside it.
func Open(sealed []byte, code string, salt []byte) ([]byte, error) {
	public, private, err := keyPair(code, salt)
	if err != nil {
		return nil, err
	}
	plain, ok := box.OpenAnonymous(nil, sealed, public, private)
	if !ok {
		return nil, ErrWrongCode
	}
	return plain, nil
}
