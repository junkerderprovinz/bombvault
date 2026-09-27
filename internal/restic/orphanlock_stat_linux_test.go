package restic

import "testing"

func TestParseProcStatReadsPastACommandNameWithParentheses(t *testing.T) {
	stat := "4242 (rest) 1 (c) S 77 4242 4242 0 -1 4194560 100 0 0 0 1 2 0 0 20 0 1 0 918273 0 0"
	got, err := parseProcStat(stat)
	if err != nil {
		t.Fatal(err)
	}
	if got.parent != 77 || got.startTicks != 918273 {
		t.Fatalf("parsed %+v, want parent 77 and start 918273", got)
	}
	if _, err := parseProcStat("4242 (restic) S 77"); err == nil {
		t.Fatal("a cut-off stat line parsed")
	}
}
