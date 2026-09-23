package backup

import "testing"

func TestSummaryPlus(t *testing.T) {
	yes := true
	left := Summary{
		SnapshotID:  "fileSnap1234",
		Bytes:       1024,
		Measured:    true,
		SourceBytes: 4096,
		SourceFiles: 12,
		FilesNew:    3,
		ResticMS:    2400,
		HasParent:   &yes,
	}
	right := Summary{
		SnapshotID:  "zvolSnap5678",
		Bytes:       2048,
		Measured:    true,
		SourceBytes: 8192,
		SourceFiles: 1,
		FilesNew:    1,
		ResticMS:    600,
	}

	got := left.Plus(right)
	want := Summary{
		SnapshotID:  "fileSnap1234",
		Bytes:       3072,
		Measured:    true,
		SourceBytes: 12288,
		SourceFiles: 13,
		FilesNew:    4,
		ResticMS:    3000,
		HasParent:   &yes,
	}
	if got != want {
		t.Fatalf("Plus = %+v, want %+v", got, want)
	}

	t.Run("an unmeasured part leaves the whole run unmeasured", func(t *testing.T) {
		got := left.Plus(Summary{Bytes: 2048, SourceBytes: 8192})
		if got.Measured {
			t.Fatalf("Measured = true, want false so the run records no metrics instead of an undercount")
		}
		if got.Bytes != 3072 || got.SourceBytes != 12288 {
			t.Fatalf("sums = %d/%d, want 3072/12288", got.Bytes, got.SourceBytes)
		}
		if left.Plus(Summary{}).Measured || (Summary{}).Plus(left).Measured {
			t.Fatalf("Measured = true next to an empty summary, want false")
		}
	})
}
