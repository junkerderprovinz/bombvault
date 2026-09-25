package places

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
)

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
