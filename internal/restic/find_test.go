package restic

import (
	"context"
	"errors"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestFileNamePatternFindsPlainTextAnywhereInTheName(t *testing.T) {
	got, err := FileNamePattern("  Invoice ")
	if err != nil || got != "*invoice*" {
		t.Fatalf("pattern %q, %v, want *invoice*", got, err)
	}
}

func TestFileNamePatternKeepsTheWildcardsTyped(t *testing.T) {
	got, err := FileNamePattern("Report-*.PDF")
	if err != nil || got != "report-*.pdf" {
		t.Fatalf("pattern %q, %v, want report-*.pdf", got, err)
	}
}

func TestFileNamePatternTakesBracketsAndBangLiterally(t *testing.T) {
	for text, want := range map[string]string{
		"Season [01]": `*season \[01\]*`,
		`back\slash`:  `*back\\slash*`,
		"!*.txt":      `\!*.txt`,
	} {
		got, err := FileNamePattern(text)
		if err != nil || got != want {
			t.Errorf("%q: pattern %q, %v, want %q", text, got, err, want)
		}
	}
}

func TestFileNamePatternRefusesWhatIsNoName(t *testing.T) {
	for _, text := range []string{"", "   ", "docs/report", "a\x00b", "tab\tname", strings.Repeat("a", 256)} {
		if _, err := FileNamePattern(text); !errors.Is(err, ErrFileNamePattern) {
			t.Errorf("%q: err %v, want ErrFileNamePattern", text, err)
		}
	}
}

func TestMatchesNameLooksAtTheLastPartOfThePath(t *testing.T) {
	cases := map[string]bool{
		"/data/Invoices":          true,
		"/data/Invoices/2024.pdf": false,
		"/data/old-invoice.PDF":   true,
		"/data/notes.txt":         false,
	}
	for p, want := range cases {
		if got := MatchesName("*invoice*", p); got != want {
			t.Errorf("%s: %v, want %v", p, got, want)
		}
	}
}

func TestFindArgsSearchOnlyTheGivenSnapshotsWithoutALock(t *testing.T) {
	got := FindArgs("/repo", []string{"aaa", "bbb"}, "*-x*", Mode{Encrypted: true})
	want := []string{"-r", "/repo", "find", "--no-lock", "--json", "--ignore-case", "--snapshot=aaa", "--snapshot=bbb", "--", "*-x*"}
	if !slices.Equal(got, want) {
		t.Fatalf("args %q, want %q", got, want)
	}
	if got := FindArgs("/repo", []string{"aaa"}, "*x*", Mode{}); !slices.Contains(got, insecureFlag) {
		t.Fatalf("args %q open a password-less repository without %s", got, insecureFlag)
	}
}

// findOutput is what restic 0.19 prints for find --json: one array, one object
// per snapshot, its matches before its id.
const findOutput = `[{"matches":[` +
	`{"path":"/data/Invoices","permissions":"drwxr-xr-x","type":"dir","mode":2147484141,"mtime":"2026-11-02T10:00:00Z","uid":0,"gid":0},` +
	`{"path":"/data/Invoices/a.txt","permissions":"-rw-r--r--","type":"file","mode":420,"mtime":"2026-11-02T10:00:00Z","uid":0,"gid":0,"size":6},` +
	`{"path":"/data/invoice-7.pdf","permissions":"-rw-r--r--","type":"file","mode":420,"mtime":"2026-11-03T09:30:00Z","uid":0,"gid":0,"size":2048}` +
	`],"hits":3,"snapshot":"aaaa"},` +
	`{"matches":[{"path":"/data/Invoices/b.txt","type":"file","mode":420,"mtime":"2026-11-01T10:00:00Z","uid":0,"gid":0,"size":1}],"hits":1,"snapshot":"bbbb"}]` + "\n"

func TestDecodeFindHandsOverTheNameMatchesOfEachSnapshot(t *testing.T) {
	found := map[string][]FoundFile{}
	err := decodeFind(strings.NewReader(findOutput), "*invoice*", func(id string, files []FoundFile) { found[id] = files })
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 {
		t.Fatalf("found %+v, want aaaa alone: bbbb only holds a file inside a matching folder", found)
	}
	files := found["aaaa"]
	if len(files) != 2 || files[0].Path != "/data/Invoices" || files[0].Type != "dir" {
		t.Fatalf("aaaa = %+v", files)
	}
	if f := files[1]; f.Path != "/data/invoice-7.pdf" || f.Size != 2048 || !f.ModTime.Equal(time.Date(2026, 11, 3, 9, 30, 0, 0, time.UTC)) {
		t.Fatalf("file = %+v", f)
	}
}

func TestDecodeFindReadsAnEmptyResult(t *testing.T) {
	err := decodeFind(strings.NewReader("[]\n"), "*x*", func(string, []FoundFile) { t.Fatal("called for no snapshot") })
	if err != nil {
		t.Fatal(err)
	}
}

func TestDecodeFindFailsOnCutOffOutput(t *testing.T) {
	cut := findOutput[:len(findOutput)/2]
	if err := decodeFind(strings.NewReader(cut), "*invoice*", func(string, []FoundFile) {}); err == nil {
		t.Fatal("a cut-off result decoded without an error")
	}
}

// TestFindFindsAFileInEverySnapshotThatHoldsIt runs the real binary.
func TestFindFindsAFileInEverySnapshotThatHoldsIt(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("restic not on PATH")
	}
	tmp := t.TempDir()
	repo, src := filepath.Join(tmp, "repo"), filepath.Join(tmp, "src")
	if err := os.MkdirAll(filepath.Join(src, "Invoices"), 0o750); err != nil {
		t.Fatal(err)
	}
	put := func(name, body string) {
		if err := os.WriteFile(filepath.Join(src, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// restic matches with filepath.Match, which has no escape character on
	// Windows, so the brackets are only taken literally elsewhere.
	name, search := "Invoice [7].PDF", "invoice [7]"
	if runtime.GOOS == "windows" {
		name, search = "Invoice 7.PDF", "invoice 7"
	}
	put("Invoices/a.txt", "one")
	put(name, "first")
	r := Restic{Bin: "restic", CacheDir: filepath.Join(tmp, "cache")}
	m := Mode{Encrypted: true, Password: "contract"}
	ctx := context.Background()
	if err := r.Init(ctx, repo, m); err != nil {
		t.Fatal(err)
	}
	backup := func() string {
		sum, err := r.Backup(ctx, repo, []string{src}, []string{"t"}, m)
		if err != nil {
			t.Fatal(err)
		}
		return sum.SnapshotID
	}
	first := backup()
	put(name, "second, longer")
	second := backup()

	pattern, err := FileNamePattern(search)
	if err != nil {
		t.Fatal(err)
	}
	sizes := map[string]int64{}
	err = r.Find(ctx, repo, []string{first, second}, pattern, m, func(id string, files []FoundFile) {
		if len(files) != 1 || filepath.Base(filepath.FromSlash(files[0].Path)) != name {
			t.Errorf("%s: %+v", id, files)
			return
		}
		sizes[id] = files[0].Size
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int64{first: int64(len("first")), second: int64(len("second, longer"))}
	if !maps.Equal(sizes, want) {
		t.Fatalf("sizes %v, want %v", sizes, want)
	}
}
