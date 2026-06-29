package api

import "testing"

// VM (libvirt) domain names commonly contain spaces ("Windows 11", "Home
// Assistant") — they must be accepted, unlike Docker container names. The
// validator still blocks traversal / option-injection.
func TestValidVMName(t *testing.T) {
	good := []string{"Windows 11", "Ubuntu", "Home Assistant", "win10-pro", "VM_1", "pfSense 2.7", "a.b.c"}
	for _, n := range good {
		if !validVMName(n) {
			t.Errorf("validVMName(%q) = false, want true", n)
		}
	}
	bad := []string{
		"",          // empty
		"../etc",    // traversal
		"a/b",       // path separator
		"a\\b",      // backslash
		"-rf",       // leading dash (option injection)
		"x\ty",      // control char
		"bad\nname", // newline
	}
	for _, n := range bad {
		if validVMName(n) {
			t.Errorf("validVMName(%q) = true, want false", n)
		}
	}
}
