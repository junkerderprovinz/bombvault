package api

import (
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// FreshSizeSamples gives the local repository of each domain a size sample
// taken just now. The backups a test then runs find the throttle closed and
// start no measurement in the background, which would otherwise go on reading
// the engine and its fakes while the test swaps or inspects them.
func FreshSizeSamples(t testing.TB, st *store.Repo, domains ...string) {
	t.Helper()
	for _, domain := range domains {
		if err := st.AddRepoStat(store.RepoStat{Domain: domain, Source: "local", At: time.Now().Unix()}); err != nil {
			t.Fatal(err)
		}
	}
}
