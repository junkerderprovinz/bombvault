package store

import (
	"strings"
	"testing"
)

func TestFinishedRunsComeOldestFirstFromACursor(t *testing.T) {
	r := migratedRepo(t)
	db := r.db
	for _, run := range []struct {
		id, status string
		finished   any
	}{
		{"b", "success", 200},
		{"a", "failed", 200},
		{"c", "success", 100},
		{"d", "running", nil},
		{"e", "skipped", 300},
	} {
		if _, err := db.Exec(`INSERT INTO runs (id, target_id, kind, status, started_at, finished_at) VALUES (?, 't1', 'backup', ?, 50, ?)`,
			run.id, run.status, run.finished); err != nil {
			t.Fatal(err)
		}
	}
	var got []string
	var at int64
	id := ""
	for range 10 {
		page, err := r.FinishedRunsAfter(at, id, 2)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		for _, run := range page {
			got = append(got, run.ID)
		}
		last := page[len(page)-1]
		at, id = *last.FinishedAt, last.ID
	}
	if want := "c a b e"; strings.Join(got, " ") != want {
		t.Fatalf("paged %q, want %q", strings.Join(got, " "), want)
	}
}
