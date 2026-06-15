package virshcli

import "testing"

func TestClientUsesConnectionURI(t *testing.T) {
	c := New("qemu+ssh://root@h/system")
	got := c.baseArgs("list", "--all")
	want := []string{"-c", "qemu+ssh://root@h/system", "list", "--all"}
	if len(got) != len(want) {
		t.Fatalf("baseArgs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("baseArgs[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestClientEmptyURIHasNoConnFlag(t *testing.T) {
	c := New("")
	got := c.baseArgs("list")
	if len(got) != 1 || got[0] != "list" {
		t.Fatalf("baseArgs = %v, want [list]", got)
	}
}
