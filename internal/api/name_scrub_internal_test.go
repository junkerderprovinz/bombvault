package api

import (
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

const nameWithPassword = "https://bob:hunter2@nas.local/backups"

func TestAPasswordInANameStaysOutOfWhatOthersRead(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	if _, err := in.st.MutateSettings(func(s *store.Settings) error {
		s.InstanceName = nameWithPassword
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := in.st.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: nameWithPassword, Repo: "rest:https://box.example:8000/c", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	vm, err := in.st.UpsertVMTarget(store.VMTarget{Name: "bob:hunter2@nas"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := in.st.StartRun(vm.ID, "backup"); err != nil {
		t.Fatal(err)
	}
	settings, err := in.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}

	read := map[string]string{
		"the name an item shows":       scrubSafeName(nameWithPassword),
		"the name members are told":    instanceDisplayName(settings),
		"the off-site target a member": "",
	}
	repos, err := in.svc.pairableRepos(settings)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range repos {
		read["the off-site target a member"] += r.Name + r.Location
	}
	for _, v := range in.h.runViews(mustRuns(t, in.st)) {
		read["the run list"] += v.Target + v.StartedViaLabel
	}
	for what, text := range read {
		if strings.Contains(text, "hunter2") {
			t.Errorf("%s carries the password: %s", what, text)
		}
	}
}

func mustRuns(t *testing.T, st *store.Repo) []store.Run {
	t.Helper()
	runs, err := st.ListRuns(10)
	if err != nil {
		t.Fatal(err)
	}
	return runs
}
