package dockercli_test

import (
	"errors"
	"fmt"
	"testing"

	cerrdefs "github.com/containerd/errdefs"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
)

// TestIsNotFound covers how the scheduled backup notices a removed container:
// the typed containerd error, also when wrapped, and the daemon's plain text.
func TestIsNotFound(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"daemon string", errors.New("Error response from daemon: No such container: Nexterm"), true},
		{"typed errdefs", cerrdefs.ErrNotFound, true},
		{"typed wrapped through inspect", fmt.Errorf("inspect container: dockercli: inspect: %w", cerrdefs.ErrNotFound), true},
		{"string wrapped", fmt.Errorf("inspect container: %w", errors.New("no such container: x")), true},
		{"unrelated", errors.New("connection refused"), false},
	}
	for _, c := range cases {
		if got := dockercli.IsNotFound(c.err); got != c.want {
			t.Errorf("%s: IsNotFound(%v) = %v, want %v", c.name, c.err, got, c.want)
		}
	}
}
