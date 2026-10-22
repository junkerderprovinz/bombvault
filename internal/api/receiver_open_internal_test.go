package api

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// openErrEngine answers every open with one error and counts the attempts.
type openErrEngine struct {
	ResticEngine
	err   error
	tries int
}

func (e *openErrEngine) RepoOpensErr(context.Context, string, restic.Mode) error {
	e.tries++
	return e.err
}

func TestOpeningAnotherInstancesRepositorySaysWhyItFailed(t *testing.T) {
	cases := []struct {
		name      string
		err       error
		wantTries int
		want      string
	}{
		{
			name:      "nothing there yet",
			err:       errors.New("Fatal: repository does not exist: unable to open config file: Stat: The specified key does not exist."),
			wantTries: 1,
			want:      "does not exist yet",
		},
		{
			name:      "backend out of reach",
			err:       errors.New("Fatal: unable to open config file: Head \"http://10.0.0.9:8000/x/config\": dial tcp 10.0.0.9:8000: connect: no route to host"),
			wantTries: 1,
			want:      "could not reach",
		},
		{
			name:      "key refused",
			err:       errors.New("Fatal: wrong password or no key found"),
			wantTries: 2,
			want:      "wrong restic password",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			eng := &openErrEngine{err: c.err}
			s := &Service{engine: eng}
			_, err := s.openOthersRepo(context.Background(), "rest:http://10.0.0.9:8000/x", restic.Mode{Encrypted: true, Password: "pw"}, restic.Mode{}, "pull source")
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want it to say %q", err, c.want)
			}
			if eng.tries != c.wantTries {
				t.Errorf("opened %d times, want %d", eng.tries, c.wantTries)
			}
		})
	}
}
