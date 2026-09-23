package api

import (
	"errors"
	"strings"
	"testing"
)

// TestEveryAppendOnlyFlagGetsItsOwnSentence: each flag has its own refusal,
// naming the card where that toggle lives.
func TestEveryAppendOnlyFlagGetsItsOwnSentence(t *testing.T) {
	cases := []struct {
		flag appendOnlyFlag
		want error
		card string
	}{
		{appendOnlyNamedRepo, errOffsiteAppendOnly, "Settings, Repositories"},
		{appendOnlyPrimaryRemote, errAppendOnlyPrimaryRemote, "Remote safety settings"},
		{appendOnlyUnreadable, errAppendOnlyUnknown, "could not be read"},
	}
	seen := map[string]appendOnlyFlag{}
	for _, c := range cases {
		got := appendOnlyRefusal(c.flag)
		if !errors.Is(got, c.want) {
			t.Errorf("appendOnlyRefusal(%v) = %v, want %v", c.flag, got, c.want)
			continue
		}
		msg := got.Error()
		if !strings.Contains(msg, c.card) {
			t.Errorf("the refusal for %v does not name where to act: %q, want it to mention %q.\n"+
				"Sending somebody to a card their repository is not listed on costs a whole diagnosis.", c.flag, msg, c.card)
		}
		if prev, dup := seen[msg]; dup {
			t.Errorf("%v and %v produce the SAME sentence: %q.\n"+
				"The split exists precisely because one sentence cannot answer for three toggles.", prev, c.flag, msg)
		}
		seen[msg] = c.flag
	}

	// appendOnlyNone is a programming error, but it still has to refuse: nil
	// would let a forget through.
	if appendOnlyRefusal(appendOnlyNone) == nil {
		t.Error("appendOnlyRefusal(appendOnlyNone) returned nil.\n" +
			"Every call site treats a non-nil answer as the refusal; nil would open the gate.")
	}
}

// TestNoAppendOnlyRefusalCarriesASlash: scrubError redacts any token that
// starts with a slash, so a remedy containing a path would arrive as "[path]".
func TestNoAppendOnlyRefusalCarriesASlash(t *testing.T) {
	for _, err := range []error{errOffsiteAppendOnly, errAppendOnlyOffsiteTarget, errAppendOnlyPrimaryRemote, errAppendOnlyUnknown} {
		if strings.Contains(err.Error(), "/") {
			t.Errorf("this refusal carries a slash and will be redacted on the way out: %q", err.Error())
		}
	}
}

// TestOffsiteDestinationRefusalIsItsOwnSentence: the off-site call sites return
// errAppendOnlyOffsiteTarget directly, so the flag table above does not reach
// it.
func TestOffsiteDestinationRefusalIsItsOwnSentence(t *testing.T) {
	msg := errAppendOnlyOffsiteTarget.Error()
	if !strings.Contains(msg, "Off-site") {
		t.Errorf("the off-site destination refusal does not name the off-site card: %q", msg)
	}
	for _, other := range []error{errOffsiteAppendOnly, errAppendOnlyPrimaryRemote, errAppendOnlyUnknown} {
		if other.Error() == msg {
			t.Errorf("the off-site refusal is identical to another one: %q", msg)
		}
	}
}
