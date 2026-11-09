package store

import "testing"

func migratedRepo(t *testing.T) *Repo {
	t.Helper()
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	return New(db)
}

func TestConvertingTheNotifyConfKeepsTheOldOneReadable(t *testing.T) {
	r := migratedRepo(t)
	if _, err := r.MutateSettings(func(s *Settings) error { s.NotifyConf = "old-sealed"; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := r.ConvertNotifyConf("old-sealed", "new-sealed", 1700000000); err != nil {
		t.Fatal(err)
	}
	s, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.NotifyConf != "new-sealed" {
		t.Errorf("settings hold %q, want the converted conf", s.NotifyConf)
	}
	kept, at, err := r.NotifyConfBeforeChannels()
	if err != nil || kept != "old-sealed" || at != 1700000000 {
		t.Errorf("kept %q at %d (%v), want the old conf", kept, at, err)
	}
}

func TestAConversionLeavesAConfChangedMeanwhileAlone(t *testing.T) {
	r := migratedRepo(t)
	if _, err := r.MutateSettings(func(s *Settings) error { s.NotifyConf = "saved-since"; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := r.ConvertNotifyConf("old-sealed", "new-sealed", 1700000000); err == nil {
		t.Fatal("a conversion of a conf that was replaced went through")
	}
	s, _ := r.GetSettings()
	if s.NotifyConf != "saved-since" {
		t.Errorf("settings hold %q", s.NotifyConf)
	}
	if kept, _, _ := r.NotifyConfBeforeChannels(); kept != "" {
		t.Errorf("kept %q for a conversion that did not happen", kept)
	}
}
