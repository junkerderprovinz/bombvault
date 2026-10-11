package keydeposit

import (
	"encoding/json"
	"errors"
)

// Content is what a deposit seals: the recovery kit and, with it, whose kit
// it is and when it was sealed. The keeper cannot change any of it, so the
// install that opens a deposit reads those from inside and not from what the
// keeper says about it.
type Content struct {
	OwnerID   string `json:"owner"`
	OwnerName string `json:"name"`
	// Counter is the one the deposit was signed with, higher for every later
	// deposit of the same install.
	Counter  int64  `json:"counter"`
	SealedAt int64  `json:"sealedAt"`
	Kit      string `json:"kit"`
}

// ErrNotADeposit is returned for sealed bytes that open and hold no Content.
var ErrNotADeposit = errors.New("what the code opened is not a key deposit")

// Pack spells a Content for sealing.
func Pack(c Content) ([]byte, error) {
	return json.Marshal(c)
}

// Unpack reads what Pack wrote.
func Unpack(plain []byte) (Content, error) {
	var c Content
	if err := json.Unmarshal(plain, &c); err != nil || c.OwnerID == "" || c.Kit == "" {
		return Content{}, ErrNotADeposit
	}
	return c, nil
}
