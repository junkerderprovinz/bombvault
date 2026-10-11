package store

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// The list of items runs these on every load over a table that keeps every run
// of every item, so neither may fall back to reading the table row by row.
func TestItemListQueriesStayOnTheRunIndexes(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name, query string
		args        []any
	}{
		{"RecentRunsByTarget", recentRunsByTargetQuery, []any{14}},
		{"RecentRunsOf", recentRunsOfTargetQuery, []any{14, "t"}},
		{"LatestSourceBytesByTarget", latestSourceBytesByTargetQuery, nil},
		{"LatestSourceBytesOf", latestSourceBytesOfTargetQuery, []any{"t"}},
		{"LatestBackupsByTarget", latestBackupsByTargetQuery, []any{saneStampCutoff()}},
		{"LatestBackupsOf", latestBackupsOfTargetQuery, []any{saneStampCutoff(), "t"}},
	}
	for _, c := range cases {
		rows, err := db.Query("EXPLAIN QUERY PLAN "+c.query, c.args...)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		var plan []string
		for rows.Next() {
			var id, parent, unused int
			var detail string
			if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
			plan = append(plan, detail)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		_ = rows.Close()

		seeks := false
		for _, step := range plan {
			if strings.HasPrefix(step, "SCAN runs") && !strings.Contains(step, "COVERING INDEX") {
				t.Fatalf("%s reads the runs table itself: %q in\n%s", c.name, step, strings.Join(plan, "\n"))
			}
			if strings.HasPrefix(step, "SEARCH runs USING") && strings.Contains(step, "idx_runs_target_kind_started (target_id=? AND kind=?)") {
				seeks = true
			}
		}
		if !seeks {
			t.Fatalf("%s does not seek a target's runs by kind:\n%s", c.name, strings.Join(plan, "\n"))
		}
	}
}

// groupedBackupStamps reads the stamps the plain way, one pass over the whole
// table grouped by target. The bare columns beside max(finished_at) come from
// the row that matched it.
func groupedBackupStamps(t *testing.T, r *Repo) map[string]BackupStamp {
	t.Helper()
	out := map[string]BackupStamp{}
	cutoff := saneStampCutoff()

	good, err := r.db.Query(`
		SELECT target_id, max(finished_at), started_at
		FROM runs
		WHERE kind = 'backup' AND status = 'success' AND finished_at IS NOT NULL AND finished_at <= ?
		GROUP BY target_id`, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	defer good.Close() //nolint:errcheck // test query
	for good.Next() {
		var target string
		var finished, started int64
		if err := good.Scan(&target, &finished, &started); err != nil {
			t.Fatal(err)
		}
		out[target] = BackupStamp{LastSuccessAt: finished, LastDurationSeconds: finished - started}
	}
	if err := good.Err(); err != nil {
		t.Fatal(err)
	}

	last, err := r.db.Query(`
		SELECT target_id, max(finished_at), started_at, status, coalesce(error, '')
		FROM runs
		WHERE kind = 'backup' AND finished_at IS NOT NULL AND finished_at <= ?
		GROUP BY target_id`, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	defer last.Close() //nolint:errcheck // test query
	for last.Next() {
		var target, status, runErr string
		var finished, started int64
		if err := last.Scan(&target, &finished, &started, &status, &runErr); err != nil {
			t.Fatal(err)
		}
		stamp := out[target]
		stamp.LastRunAt, stamp.LastRunStatus = started, status
		if status == "failed" {
			stamp.LastRunError = runErr
		}
		out[target] = stamp
	}
	if err := last.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestLatestBackupsByTargetMatchesAGroupedRead(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := New(db)

	now := time.Now().Unix()
	future := time.Now().AddDate(3, 0, 0).Unix()
	insert := func(id, target, kind, status string, started int64, finished any, runErr string) {
		t.Helper()
		if _, err := db.Exec(`
			INSERT INTO runs (id, target_id, kind, status, started_at, finished_at, error)
			VALUES (?, ?, ?, ?, ?, ?, ?)`, id, target, kind, status, started, finished, runErr); err != nil {
			t.Fatal(err)
		}
	}

	// Forty targets with a history each: backups that succeed, fail or are
	// cancelled, dumps and restores in between, one run still going and one
	// stamped by a clock that was years ahead.
	kinds := []string{"backup", "backup", "dbdump", "backup", "restore", "backup", "prune"}
	statuses := []string{"success", "failed", "success", "cancelled", "success"}
	for target := range 40 {
		name := fmt.Sprintf("t%02d", target)
		for i := range 5 + target {
			started := now - int64(5000-i*100) - int64(target)
			status := statuses[(i*3+target)%len(statuses)]
			runErr := ""
			if status == "failed" {
				runErr = fmt.Sprintf("%s failed at step %d", name, i)
			}
			insert(fmt.Sprintf("%s-%03d", name, i), name, kinds[(i+target)%len(kinds)], status, started, started+int64(20+i), runErr)
		}
		switch target % 4 {
		case 1:
			insert(name+"-running", name, "backup", "running", now-10, nil, "")
		case 2:
			insert(name+"-future", name, "backup", "success", now-5, future, "")
		}
	}
	insert("dumps-only", "dumps", "dbdump", "success", now-50, now-40, "")
	insert("never-finished", "stuck", "backup", "running", now-50, nil, "")

	got, err := r.LatestBackupsByTarget()
	if err != nil {
		t.Fatalf("LatestBackupsByTarget: %v", err)
	}
	want := groupedBackupStamps(t, r)
	if len(want) != 40 {
		t.Fatalf("the grouped read found %d targets, want the 40 with a finished backup", len(want))
	}
	if !reflect.DeepEqual(got, want) {
		for target, w := range want {
			if g := got[target]; g != w {
				t.Errorf("%s: got %+v, want %+v", target, g, w)
			}
		}
		t.Fatalf("LatestBackupsByTarget found %d targets, the grouped read %d", len(got), len(want))
	}
	failed := 0
	for _, stamp := range got {
		if stamp.LastRunError != "" {
			failed++
		}
	}
	if failed == 0 {
		t.Fatal("no seeded target ends on a failed backup, so the error text went unchecked")
	}
}

func TestHistoryOfOneTargetIsItsShareOfTheListing(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := New(db)

	now := time.Now().Unix()
	kinds := []string{"backup", "dbdump", "backup", "restore", "import", "backup", "prune"}
	statuses := []string{"success", "failed", "success", "cancelled"}
	for target := range 12 {
		name := fmt.Sprintf("t%02d", target)
		for i := range 3 + target*2 {
			started := now - int64(5000-i*100)
			var size any
			if (i+target)%3 != 0 {
				size = 1000*target + i
			}
			if _, err := db.Exec(`
				INSERT INTO runs (id, target_id, kind, status, started_at, finished_at, error, source_bytes)
				VALUES (?, ?, ?, ?, ?, ?, '', ?)`,
				fmt.Sprintf("%s-%03d", name, i), name, kinds[(i+target)%len(kinds)], statuses[(i*3+target)%len(statuses)],
				started, started+20, size); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := db.Exec(`INSERT INTO runs (id, target_id, kind, status, started_at) VALUES ('dump', 'dumps', 'dbdump', 'running', ?)`, now); err != nil {
		t.Fatal(err)
	}

	const perTarget = 5
	stamps, err := r.LatestBackupsByTarget()
	if err != nil {
		t.Fatal(err)
	}
	sizes, err := r.LatestSourceBytesByTarget()
	if err != nil {
		t.Fatal(err)
	}
	runs, err := r.RecentRunsByTarget(perTarget)
	if err != nil {
		t.Fatal(err)
	}
	if len(stamps) != 12 || len(sizes) == 0 || len(runs) != 12 {
		t.Fatalf("the listing found %d stamps, %d sizes and %d run strips over 12 targets", len(stamps), len(sizes), len(runs))
	}

	for target := range 13 {
		name := fmt.Sprintf("t%02d", target)
		if target == 12 {
			name = "dumps"
		}
		oneStamp, err := r.LatestBackupsOf(name)
		if err != nil {
			t.Fatal(err)
		}
		oneSize, err := r.LatestSourceBytesOf(name)
		if err != nil {
			t.Fatal(err)
		}
		oneRuns, err := r.RecentRunsOf(name, perTarget)
		if err != nil {
			t.Fatal(err)
		}
		if len(oneStamp) > 1 || len(oneSize) > 1 || len(oneRuns) > 1 {
			t.Fatalf("%s: a read of one target names others: %v %v %v", name, oneStamp, oneSize, oneRuns)
		}
		stamp, hasStamp := oneStamp[name]
		if want, listed := stamps[name]; stamp != want || hasStamp != listed {
			t.Errorf("%s: stamp %+v (present %v), the listing has %+v (present %v)", name, stamp, hasStamp, want, listed)
		}
		size, hasSize := oneSize[name]
		if want, listed := sizes[name]; size != want || hasSize != listed {
			t.Errorf("%s: size %d (present %v), the listing has %d (present %v)", name, size, hasSize, want, listed)
		}
		if !reflect.DeepEqual(oneRuns[name], runs[name]) {
			t.Errorf("%s: runs %+v, the listing has %+v", name, oneRuns[name], runs[name])
		}
	}
}
