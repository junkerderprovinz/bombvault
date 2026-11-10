package restic

import "testing"

func TestTheRepositoryIDIsReadFromItsConfig(t *testing.T) {
	id, err := repoIDOf([]byte(`{"version":2,"id":"5f3c0a1e9b2d4c6f8a7e1d3b5c9f0e2a4b6d8c1e3f5a7b9d0c2e4f6a8b1d3c5e","chunker_polynomial":"3e8d7f1a2b4c6d"}` + "\n"))
	if err != nil || id != "5f3c0a1e9b2d4c6f8a7e1d3b5c9f0e2a4b6d8c1e3f5a7b9d0c2e4f6a8b1d3c5e" {
		t.Fatalf("id %q, %v", id, err)
	}
	for _, out := range []string{"", "{}", `{"version":2}`, "not json"} {
		if _, err := repoIDOf([]byte(out)); err == nil {
			t.Errorf("%q read as a config with an id", out)
		}
	}
}
