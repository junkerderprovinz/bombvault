package restic

import "testing"

// What counts as a sign of life in a restic status line.
//
// This decides when a backup is declared stuck, so being wrong in either
// direction is expensive: too strict and a healthy multi-terabyte run is killed
// at the worst possible moment, too loose and a wedged run holds its domain
// forever.
//
// The trap is the SCAN phase. Before restic writes its first pack it walks the
// tree, and during that walk bytes_done stays at 0 for minutes on a large
// appdata tree while total_bytes climbs steadily. A guard that watched
// bytes_done alone would treat that as a stall and kill exactly the run the
// 48-hour cap was once raised for.
//
// The opposite trap is seconds_elapsed, which advances on its own whether or
// not anything is happening. Treating it as progress would mean no run is ever
// stuck, which is a guard that cannot fire.

func TestProgressIsAnyCounterThatAdvanced(t *testing.T) {
	cases := []struct {
		name       string
		prev, next Progress
		want       bool
	}{
		{
			name: "bytes moved: the ordinary case",
			prev: Progress{BytesDone: 100, TotalBytes: 1000},
			next: Progress{BytesDone: 200, TotalBytes: 1000},
			want: true,
		},
		{
			// The scan phase. restic has written nothing yet and is still
			// discovering the tree; this is the case that decides the design.
			name: "scanning: no bytes written, but the total is still growing",
			prev: Progress{BytesDone: 0, TotalBytes: 1000, TotalFiles: 10},
			next: Progress{BytesDone: 0, TotalBytes: 4000, TotalFiles: 42},
			want: true,
		},
		{
			name: "files finished even though the byte counter is flat",
			prev: Progress{BytesDone: 500, FilesDone: 3},
			next: Progress{BytesDone: 500, FilesDone: 4},
			want: true,
		},
		{
			name: "nothing moved at all",
			prev: Progress{BytesDone: 500, TotalBytes: 1000, FilesDone: 3, TotalFiles: 10},
			next: Progress{BytesDone: 500, TotalBytes: 1000, FilesDone: 3, TotalFiles: 10},
			want: false,
		},
		{
			// The clock is not progress. If it counted, a wedged run would look
			// alive forever and the guard could never fire.
			name: "only the elapsed clock advanced",
			prev: Progress{BytesDone: 500, SecondsElapsed: 10},
			next: Progress{BytesDone: 500, SecondsElapsed: 900},
			want: false,
		},
		{
			// restic can revise total_bytes DOWNWARD when its scan over-estimated.
			// That is still the scanner working, not a stall.
			name: "a revised total still counts as life",
			prev: Progress{BytesDone: 0, TotalBytes: 9000},
			next: Progress{BytesDone: 0, TotalBytes: 4000},
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.next.MovedSince(tc.prev); got != tc.want {
				t.Fatalf("MovedSince = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParseProgressReadsEveryCounter(t *testing.T) {
	line := []byte(`{"message_type":"status","percent_done":0.25,"total_files":42,"files_done":7,` +
		`"total_bytes":4000,"bytes_done":1000,"seconds_elapsed":12}`)
	got, ok := ParseProgress(line)
	if !ok {
		t.Fatal("a status line must parse")
	}
	if got.BytesDone != 1000 || got.TotalBytes != 4000 || got.FilesDone != 7 || got.TotalFiles != 42 {
		t.Fatalf("counters lost in parsing: %+v", got)
	}
	if got.Percent != 25 {
		t.Fatalf("percent = %v, want 25", got.Percent)
	}
}

func TestParseProgressIgnoresEverythingElse(t *testing.T) {
	for _, line := range []string{
		`{"message_type":"summary","files_new":3}`,
		`not json at all`,
		``,
		`{"message_type":"error","error":{"message":"boom"}}`,
	} {
		if _, ok := ParseProgress([]byte(line)); ok {
			t.Fatalf("non-status line parsed as progress: %q", line)
		}
	}
}
