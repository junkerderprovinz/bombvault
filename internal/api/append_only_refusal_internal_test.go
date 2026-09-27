package api

import (
	"errors"
	"strings"
	"testing"
)

// TestEveryAppendOnlyFlagGetsItsOwnSentence: each flag has its own refusal,
// naming the place where that toggle lives.
func TestEveryAppendOnlyFlagGetsItsOwnSentence(t *testing.T) {
	cases := []struct {
		flag  appendOnlyFlag
		want  error
		where string
	}{
		{appendOnlyNamedRepo, errOffsiteAppendOnly, "the place it lies at, under Settings, Storage"},
		{appendOnlyPrimaryRemote, errAppendOnlyPrimaryRemote, "the place this domain is stored in, under Settings, Storage"},
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
		if !strings.Contains(msg, c.where) {
			t.Errorf("the refusal for %v does not name where to act: %q, want it to mention %q.\n"+
				"Sending somebody to a place their repository does not lie at costs a whole diagnosis.", c.flag, msg, c.where)
		}
		if prev, dup := seen[msg]; dup {
			t.Errorf("%v and %v produce the same sentence: %q.\n"+
				"One sentence cannot answer for three toggles.", prev, c.flag, msg)
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

// A row whose address fits no place has no details; its switch sits in its
// own row of that list.
func TestAnAppendOnlyRefusalSaysWhereARowWithoutAPlaceGetsItsSwitch(t *testing.T) {
	for _, c := range []struct {
		err  error
		what string
	}{
		{errOffsiteAppendOnly, "A repository"},
		{errAppendOnlyOffsiteTarget, "A destination"},
		{errAppendOnlyPrimaryRemote, "A path"},
	} {
		want := c.what + " listed under Without a place has that switch in its own row there"
		if !strings.Contains(c.err.Error(), want) {
			t.Errorf("refusal %q does not say %q", c.err.Error(), want)
		}
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
	if !strings.Contains(msg, "the place it copies to, under Settings, Storage") {
		t.Errorf("the off-site destination refusal does not name the place it copies to: %q", msg)
	}
	for _, other := range []error{errOffsiteAppendOnly, errAppendOnlyPrimaryRemote, errAppendOnlyUnknown} {
		if other.Error() == msg {
			t.Errorf("the off-site refusal is identical to another one: %q", msg)
		}
	}
}
