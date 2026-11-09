package jobimport

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/schedule"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name)) //nolint:gosec // G304: a fixture of this package
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func hasNote(notes []Note, code, detail string) bool {
	return slices.Contains(notes, Note{Code: code, Detail: detail})
}

func TestDuplicatiExportCarriesSourcesFiltersScheduleAndS3(t *testing.T) {
	job, err := Duplicati(readFixture(t, "duplicati-photos.json"), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if job.Tool != ToolDuplicati || job.Name != "Family photos" || !job.Enabled {
		t.Fatalf("job = %+v", job)
	}
	if want := []string{"/source/photos", "/source/videos"}; !reflect.DeepEqual(job.Sources, want) {
		t.Errorf("sources = %q, want %q", job.Sources, want)
	}
	if want := []string{"*.tmp", "*thumbnails*", "/source/photos/.trash"}; !reflect.DeepEqual(job.Excludes, want) {
		t.Errorf("excludes = %q, want %q", job.Excludes, want)
	}
	if job.Schedule != "weekly Mon,Tue,Wed,Thu,Fri 01:30" {
		t.Errorf("schedule = %q", job.Schedule)
	}
	want := Destination{
		Kind:         KindS3,
		Location:     "s3:https://s3.eu-central-1.amazonaws.com/example-photos/duplicati/bombvault",
		Region:       "eu-central-1",
		StorageClass: "STANDARD_IA",
		KeyID:        "example-access-key-id",
		Secret:       "exampleSecretKey/not-real",
	}
	if job.Dest != want {
		t.Errorf("destination = %+v, want %+v", job.Dest, want)
	}
	for _, n := range []Note{
		{Code: NoteFilter, Detail: `[.*\.bak$]`},
		{Code: NoteFilter, Detail: "{TemporaryFiles}"},
		{Code: NoteInclude, Detail: "/source/videos/keep.tmp"},
		{Code: NoteOptions, Detail: "--skip-files-larger-than, --exclude-files-attributes"},
		{Code: NoteRetention},
		{Code: NoteEncryptionKey},
	} {
		if !hasNote(job.Notes, n.Code, n.Detail) {
			t.Errorf("missing note %+v in %+v", n, job.Notes)
		}
	}
}

func TestDuplicatiNotesNeverCarryTheSecrets(t *testing.T) {
	job, err := Duplicati(readFixture(t, "duplicati-photos.json"), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range job.Notes {
		for _, secret := range []string{"correct horse", "exampleSecretKey", "example-access-key"} {
			if n.Detail != "" && containsFold(n.Detail, secret) {
				t.Errorf("note %+v carries a secret", n)
			}
		}
	}
}

func TestDuplicatiScheduleFollowsTheServerTimeZone(t *testing.T) {
	job, err := Duplicati(readFixture(t, "duplicati-photos.json"), time.FixedZone("CET", 3600))
	if err != nil {
		t.Fatal(err)
	}
	if job.Schedule != "weekly Mon,Tue,Wed,Thu,Fri 02:30" {
		t.Errorf("schedule = %q", job.Schedule)
	}
}

func TestDuplicatiB2WithoutSignInAsksForTheEndpoint(t *testing.T) {
	job, err := Duplicati(readFixture(t, "duplicati-documents.json"), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if job.Enabled || !hasNote(job.Notes, NoteDisabled, "") {
		t.Errorf("a switched off job should stay off with a note: %+v", job)
	}
	if want := (Destination{Kind: KindB2, Path: "example-docs/duplicati/documents/bombvault"}); job.Dest != want {
		t.Errorf("destination = %+v, want %+v", job.Dest, want)
	}
	if job.Schedule != "15 0/6 * * *" {
		t.Errorf("schedule = %q", job.Schedule)
	}
	if len(job.Excludes) != 0 {
		t.Errorf("an exclude of everything must not be carried over: %q", job.Excludes)
	}
	if !hasNote(job.Notes, NoteFilter, "*") || !hasNote(job.Notes, NoteInclude, "/source/documents/Work/") {
		t.Errorf("notes = %+v", job.Notes)
	}
	if !hasNote(job.Notes, NoteRetention, "") {
		t.Errorf("keep-time should leave a retention note: %+v", job.Notes)
	}
	if hasNote(job.Notes, NoteEncryptionKey, "") {
		t.Errorf("an export without the passphrase has nothing to note about it: %+v", job.Notes)
	}
}

func TestDuplicatiLocalFolderAndWeeklyRepeat(t *testing.T) {
	job, err := Duplicati(readFixture(t, "duplicati-appdata.json"), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Destination{Kind: KindLocal, Path: "/backups/duplicati/appdata"}); job.Dest != want {
		t.Errorf("destination = %+v, want %+v", job.Dest, want)
	}
	if job.Schedule != "weekly Sat 03:00" {
		t.Errorf("schedule = %q", job.Schedule)
	}
	if hasNote(job.Notes, NoteOptions, "--no-encryption") {
		t.Errorf("--no-encryption needs no note: %+v", job.Notes)
	}
}

func TestDuplicatiEncryptedExportIsRefused(t *testing.T) {
	_, err := Duplicati(readFixture(t, "duplicati-encrypted.json.aes"), time.UTC)
	if !errors.Is(err, ErrEncryptedExport) {
		t.Fatalf("err = %v, want ErrEncryptedExport", err)
	}
}

func TestDuplicatiFileWithoutABackupIsRefused(t *testing.T) {
	if _, err := Duplicati([]byte(`{"CreatedByVersion":"2.1.0.5"}`), time.UTC); err == nil {
		t.Fatal("a file without a backup job was accepted")
	}
	if _, err := Duplicati([]byte(`[1,2]`), time.UTC); err == nil {
		t.Fatal("a file that is not an export was accepted")
	}
}

func TestDuplicatiRepeatToCadence(t *testing.T) {
	base := time.Date(2026, 3, 4, 5, 15, 0, 0, time.UTC) // a Wednesday
	every := []time.Weekday{0, 1, 2, 3, 4, 5, 6}
	cases := []struct {
		repeat string
		days   []time.Weekday
		want   string
	}{
		{"1D", every, "daily 05:15"},
		{"1D", nil, "daily 05:15"},
		{"1D", []time.Weekday{time.Saturday, time.Sunday}, "weekly Sun,Sat 05:15"},
		{"1W", nil, "weekly Wed 05:15"},
		{"1h", every, "15 * * * *"},
		{"12h", every, "15 5/12 * * *"},
		{"6h", []time.Weekday{time.Monday, time.Friday}, "15 5/6 * * 1,5"},
		{"30m", every, "15/30 * * * *"},
		{"1M", every, "15 5 4 * *"},
		{"1Y", every, "15 5 4 3 *"},
		{"2D", every, ""},
		{"7h", every, ""},
		{"1D12h", every, ""},
		{"", every, ""},
	}
	for _, c := range cases {
		got, ok := duplicatiCadence(c.repeat, base, c.days)
		if got != c.want || ok != (c.want != "") {
			t.Errorf("%s %v: got %q %v, want %q", c.repeat, c.days, got, ok, c.want)
		}
	}
}

func TestDuplicatiTargetURLs(t *testing.T) {
	cases := []struct {
		url  string
		want Destination
		note Note
	}{
		{
			url:  "s3://bucket?s3-location-constraint=eu-west-1&auth-username=k&auth-password=s",
			want: Destination{Kind: KindS3, Location: "s3:https://s3.eu-west-1.amazonaws.com/bucket/bombvault", Region: "eu-west-1", KeyID: "k", Secret: "s"},
		},
		{
			url:  "s3://bucket/prefix?s3-server-name=minio.lan:9000&aws-access-key-id=k&aws-secret-access-key=s",
			want: Destination{Kind: KindS3, Location: "s3:http://minio.lan:9000/bucket/prefix/bombvault", KeyID: "k", Secret: "s"},
		},
		{
			url:  "b2://bucket/folder?auth-username=id&auth-password=key",
			want: Destination{Kind: KindB2, Path: "bucket/folder/bombvault", KeyID: "id", Secret: "key"},
		},
		{
			url:  "ssh://nas.lan:2222/volume1/backup?auth-username=me&auth-password=pw&ssh-fingerprint=x",
			want: Destination{Kind: KindSFTP, Location: "sftp://me@nas.lan:2222/volume1/backup/bombvault"},
			note: Note{Code: NoteSSHKey},
		},
		{
			url:  "rclone://onedrive/Backups/duplicati?rclone-local-repository=local",
			want: Destination{Kind: KindRclone, Location: "rclone:onedrive:Backups/duplicati/bombvault"},
			note: Note{Code: NoteRcloneRemote, Detail: "onedrive"},
		},
		{
			url:  `file://C:\Backups\Duplicati`,
			want: Destination{Kind: KindLocal, Path: "C:/Backups/Duplicati"},
		},
		{
			url:  "ssh://bob:hunter2@nas.lan:22/backups?auth-username=bob",
			want: Destination{Kind: KindSFTP, Location: "sftp://bob@nas.lan:22/backups/bombvault"},
			note: Note{Code: NoteSSHKey},
		},
		{
			url:  "ssh://bob:hunter2@nas.lan/backups",
			want: Destination{Kind: KindSFTP, Location: "sftp://bob@nas.lan/backups/bombvault"},
			note: Note{Code: NoteSSHKey},
		},
		{
			url:  "s3://AKIA:topsecret@bucket/folder",
			want: Destination{Kind: KindS3, Location: "s3:https://s3.amazonaws.com/bucket/folder/bombvault", KeyID: "AKIA", Secret: "topsecret"},
		},
		{
			url:  "s3://old:hunter2@bucket?auth-username=k&auth-password=s",
			want: Destination{Kind: KindS3, Location: "s3:https://s3.amazonaws.com/bucket/bombvault", KeyID: "k", Secret: "s"},
		},
		{
			url:  "s3://old:hunter2@bucket?auth-username=k",
			want: Destination{Kind: KindS3, Location: "s3:https://s3.amazonaws.com/bucket/bombvault", KeyID: "k"},
		},
		{
			url:  "b2://id:hunter2@bucket/folder",
			want: Destination{Kind: KindB2, Path: "bucket/folder/bombvault", KeyID: "id", Secret: "hunter2"},
		},
		{
			url:  "rclone://bob:hunter2@onedrive/Backups",
			want: Destination{Kind: KindRclone, Location: "rclone:onedrive:Backups/bombvault"},
			note: Note{Code: NoteRcloneRemote, Detail: "onedrive"},
		},
		{
			url:  "webdav://cloud.example.org/remote.php/dav?auth-username=me&auth-password=pw",
			want: Destination{},
			note: Note{Code: NoteDestination, Detail: "webdav"},
		},
	}
	for _, c := range cases {
		got, notes := duplicatiDestination(c.url)
		if containsFold(got.Location+got.Path, "hunter2") || containsFold(got.Location+got.Path, "topsecret") {
			t.Errorf("%s: the location carries a password: %+v", c.url, got)
		}
		if got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.url, got, c.want)
		}
		if c.note.Code != "" && !hasNote(notes, c.note.Code, c.note.Detail) {
			t.Errorf("%s: notes %+v lack %+v", c.url, notes, c.note)
		}
		for _, n := range notes {
			if containsFold(n.Detail, "pw") {
				t.Errorf("%s: note %+v carries the password", c.url, n)
			}
		}
	}
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

func TestEveryWrittenCadenceIsOneAFolderSetTakes(t *testing.T) {
	base := time.Date(2026, 3, 4, 5, 15, 0, 0, time.UTC)
	var written []string
	for _, repeat := range []string{"1D", "1W", "1h", "2h", "12h", "15m", "30m", "1M", "1Y"} {
		c, _ := duplicatiCadence(repeat, base, []time.Weekday{time.Monday, time.Thursday})
		written = append(written, c)
	}
	for _, f := range []int{900, 3600, 7200, 21600, 86400} {
		c, _ := duplicacyWebCadence("07:45", f, "0010101")
		written = append(written, c)
	}
	for _, c := range written {
		cad, err := schedule.ParseCadence(c)
		if err != nil || !cad.Enabled || cad.IntervalDays != 0 {
			t.Errorf("%q: %+v, %v", c, cad, err)
		}
	}
}
