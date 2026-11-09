package jobimport

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func dirReader(dir string) Reader {
	return func(name string) ([]byte, error) {
		return os.ReadFile(filepath.Join(dir, filepath.FromSlash(name))) //nolint:gosec // G304: a fixture of this package
	}
}

func TestDuplicacyCLITurnsEveryBackupStorageIntoAJob(t *testing.T) {
	prefDir := filepath.Join("testdata", "duplicacy-cli", ".duplicacy")
	jobs, err := DuplicacyCLI(readFixture(t, "duplicacy-cli/.duplicacy/preferences"), "/mnt/user/music", dirReader(prefDir))
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 2 {
		t.Fatalf("got %d jobs, want 2 (a storage marked no_backup is skipped): %+v", len(jobs), jobs)
	}
	music, nas := jobs[0], jobs[1]
	if music.Tool != ToolDuplicacy || music.Name != "music" || nas.Name != "music-nas" {
		t.Errorf("names = %q, %q", music.Name, nas.Name)
	}
	if want := []string{"/mnt/user/music"}; !reflect.DeepEqual(music.Sources, want) {
		t.Errorf("sources = %q", music.Sources)
	}
	wantEx := []string{"*.tmp", "/mnt/user/music/.cache", "node_modules", "/mnt/user/music/Downloads/incomplete", "*.part"}
	if !reflect.DeepEqual(music.Excludes, wantEx) {
		t.Errorf("excludes = %q, want %q", music.Excludes, wantEx)
	}
	for _, n := range []Note{
		{Code: NoteInclude, Detail: "+Playlists/"},
		{Code: NoteFilter, Detail: "-Playlists/*"},
		{Code: NoteFilter, Detail: `e:\.(?i:bak)$`},
		{Code: NoteInclude, Detail: `i:\.flac$`},
		{Code: NoteFilter, Detail: "-*"},
		{Code: NoteOptions, Detail: "nobackup_file .nobackup"},
		{Code: NoteEncryptionKey},
		{Code: NoteNoSchedule},
	} {
		if !hasNote(music.Notes, n.Code, n.Detail) {
			t.Errorf("missing note %+v in %+v", n, music.Notes)
		}
	}
	want := Destination{Kind: KindB2, Path: "example-music/library/bombvault", KeyID: "0001234567890ab0000000001", Secret: "K001exampleApplicationKeyNotReal"}
	if music.Dest != want {
		t.Errorf("destination = %+v, want %+v", music.Dest, want)
	}
	if want := (Destination{Kind: KindSFTP, Location: "sftp://backup@nas.example.lan:2222/srv/duplicacy/bombvault"}); nas.Dest != want {
		t.Errorf("nas destination = %+v, want %+v", nas.Dest, want)
	}
	if !hasNote(nas.Notes, NoteSSHKey, "") || !hasNote(nas.Notes, NoteOptions, "exclude_by_attribute") {
		t.Errorf("nas notes = %+v", nas.Notes)
	}
	if hasNote(nas.Notes, NoteEncryptionKey, "") {
		t.Errorf("an unencrypted storage has no key to note: %+v", nas.Notes)
	}
	for _, j := range jobs {
		for _, n := range j.Notes {
			for _, secret := range []string{"K001example", "storage password", "not-a-real-password"} {
				if containsFold(n.Detail, secret) {
					t.Errorf("note %+v carries a secret", n)
				}
			}
		}
	}
}

func TestDuplicacyCLIUsesTheRepositoryPathItWasGiven(t *testing.T) {
	prefs := []byte(`[{"name":"default","id":"docs","repository":"/data/docs","storage":"/backups/docs","keys":null,"filters":"/config/docs.filters"}]`)
	var asked []string
	read := func(name string) ([]byte, error) {
		asked = append(asked, name)
		return []byte("-*.swp\n"), nil
	}
	jobs, err := DuplicacyCLI(prefs, "/mnt/user/docs", read)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || !reflect.DeepEqual(jobs[0].Sources, []string{"/data/docs"}) {
		t.Fatalf("jobs = %+v", jobs)
	}
	if !reflect.DeepEqual(asked, []string{"/config/docs.filters"}) || !reflect.DeepEqual(jobs[0].Excludes, []string{"*.swp"}) {
		t.Errorf("asked %q, excludes %q", asked, jobs[0].Excludes)
	}
	if want := (Destination{Kind: KindLocal, Path: "/backups/docs"}); jobs[0].Dest != want {
		t.Errorf("destination = %+v", jobs[0].Dest)
	}
}

func TestDuplicacyCLIWithoutAFiltersFileHasNoExcludes(t *testing.T) {
	prefs := []byte(`[{"name":"default","id":"docs","repository":"","storage":"/backups/docs","keys":null}]`)
	jobs, err := DuplicacyCLI(prefs, "/mnt/user/docs", func(string) ([]byte, error) { return nil, os.ErrNotExist })
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || len(jobs[0].Excludes) != 0 {
		t.Fatalf("jobs = %+v", jobs)
	}
}

func TestDuplicacyCLIRefusesWhatIsNotAPreferencesFile(t *testing.T) {
	for _, data := range []string{`{"storages":[]}`, `[]`, `not json`} {
		if _, err := DuplicacyCLI([]byte(data), "/x", dirReader(t.TempDir())); err == nil {
			t.Errorf("%s was accepted", data)
		}
	}
}

func TestDuplicacyWebReadsBackupsSchedulesAndFilters(t *testing.T) {
	jobs, err := DuplicacyWeb(readFixture(t, "duplicacy-web/duplicacy.json"), dirReader(filepath.Join("testdata", "duplicacy-web")))
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 3 {
		t.Fatalf("got %d jobs, want 3", len(jobs))
	}
	docs, appdata, camera := jobs[0], jobs[1], jobs[2]
	if docs.Tool != ToolDuplicacyWeb || docs.Name != "documents" || !reflect.DeepEqual(docs.Sources, []string{"/backuproot/documents"}) {
		t.Errorf("documents = %+v", docs)
	}
	if want := []string{"/backuproot/documents/Scans/raw", "*.lnk"}; !reflect.DeepEqual(docs.Excludes, want) {
		t.Errorf("excludes = %q, want %q", docs.Excludes, want)
	}
	if docs.Schedule != "daily 02:30" {
		t.Errorf("schedule = %q", docs.Schedule)
	}
	want := Destination{Kind: KindS3, Location: "s3:https://s3.eu-central-1.wasabisys.com/example-backups/unraid/bombvault", Region: "eu-central-1"}
	if docs.Dest != want {
		t.Errorf("destination = %+v, want %+v", docs.Dest, want)
	}
	for _, n := range []Note{
		{Code: NoteSignInEncrypted},
		{Code: NoteEncryptionKey},
		{Code: NoteRetention},
		{Code: NoteOptions, Detail: "-threads 4"},
	} {
		if !hasNote(docs.Notes, n.Code, n.Detail) {
			t.Errorf("missing note %+v in %+v", n, docs.Notes)
		}
	}

	if appdata.Schedule != "0 0/6 * * 1,2,3,4,5" {
		t.Errorf("appdata schedule = %q", appdata.Schedule)
	}
	if want := (Destination{Kind: KindLocal, Path: "/backuproot/usb-disk/duplicacy"}); appdata.Dest != want {
		t.Errorf("appdata destination = %+v", appdata.Dest)
	}
	if hasNote(appdata.Notes, NoteSignInEncrypted, "") {
		t.Errorf("a local storage has no sign-in: %+v", appdata.Notes)
	}

	if camera.Dest.Kind != "" || !hasNote(camera.Notes, NoteDestination, "gcd") || !hasNote(camera.Notes, NoteNoSchedule, "") {
		t.Errorf("camera = %+v", camera)
	}
}

func TestDuplicacyWebRefusesAPreferencesFile(t *testing.T) {
	if _, err := DuplicacyWeb(readFixture(t, "duplicacy-cli/.duplicacy/preferences"), dirReader(t.TempDir())); err == nil {
		t.Fatal("a CLI preferences file was read as a Duplicacy Web configuration")
	}
}

func TestDuplicacyWebDaysAndFrequency(t *testing.T) {
	cases := []struct {
		start, days string
		freq        int
		want        string
	}{
		{"02:30", "1111111", 86400, "daily 02:30"},
		{"02:30", "0011111", 86400, "weekly Mon,Tue,Wed,Thu,Fri 02:30"},
		{"23:00", "1000000", 86400, "weekly Sun 23:00"},
		{"00:10", "1111111", 3600, "10 * * * *"},
		{"00:10", "1111111", 1800, "10/30 * * * *"},
		{"07:05", "1100000", 7200, "5 1/2 * * 0,6"},
		{"02:30", "1111111", 604800, ""},
		{"2:30", "1111111", 86400, ""},
		{"02:30", "0000000", 86400, ""},
	}
	for _, c := range cases {
		got, ok := duplicacyWebCadence(c.start, c.freq, c.days)
		if got != c.want || ok != (c.want != "") {
			t.Errorf("%s %s %d: got %q %v, want %q", c.start, c.days, c.freq, got, ok, c.want)
		}
	}
}

func TestDuplicacyStorageURLs(t *testing.T) {
	keys := map[string]string{"s3_id": "id", "s3_secret": "secret", "wasabi_key": "wk", "wasabi_secret": "ws"}
	cases := []struct {
		url  string
		want Destination
		note Note
	}{
		{url: "s3://us-east-1@amazon/bucket/path", want: Destination{Kind: KindS3, Location: "s3:https://s3.amazonaws.com/bucket/path/bombvault", Region: "us-east-1", KeyID: "id", Secret: "secret"}},
		{url: "s3://s3.example.com/bucket", want: Destination{Kind: KindS3, Location: "s3:https://s3.example.com/bucket/bombvault", KeyID: "id", Secret: "secret"}},
		{url: "minio://us-east-1@minio.lan:9000/backups/music", want: Destination{Kind: KindS3, Location: "s3:http://minio.lan:9000/backups/music/bombvault", Region: "us-east-1", KeyID: "id", Secret: "secret"}},
		{url: "minios://minio.lan/backups", want: Destination{Kind: KindS3, Location: "s3:https://minio.lan/backups/bombvault", KeyID: "id", Secret: "secret"}},
		{url: "wasabi://us-east-2@s3.us-east-2.wasabisys.com/bucket", want: Destination{Kind: KindS3, Location: "s3:https://s3.us-east-2.wasabisys.com/bucket/bombvault", Region: "us-east-2", KeyID: "wk", Secret: "ws"}},
		{url: "b2://bucket", want: Destination{Kind: KindB2, Path: "bucket/bombvault"}},
		{url: "sftp://me@host.lan/backups", want: Destination{Kind: KindSFTP, Location: "sftp://me@host.lan/backups/bombvault"}, note: Note{Code: NoteSSHKey}},
		{url: "flat:///mnt/disks/usb/dup", want: Destination{Kind: KindLocal, Path: "/mnt/disks/usb/dup"}},
		{url: "samba:///mnt/remotes/nas/dup", want: Destination{Kind: KindLocal, Path: "/mnt/remotes/nas/dup"}},
		{url: `D:\Duplicacy`, want: Destination{Kind: KindLocal, Path: "D:/Duplicacy"}},
		{url: "sftp://bob:hunter2@nas.lan/backups", want: Destination{Kind: KindSFTP, Location: "sftp://bob@nas.lan/backups/bombvault"}, note: Note{Code: NoteSSHKey}},
		{url: "s3://id:hunter2@s3.example.com/bucket", want: Destination{Kind: KindS3, Location: "s3:https://s3.example.com/bucket/bombvault", KeyID: "id", Secret: "secret"}},
		{url: "minio://key:hunter2@minio.lan/backups", want: Destination{Kind: KindS3, Location: "s3:http://minio.lan/backups/bombvault", KeyID: "id", Secret: "secret"}},
		{url: "wasabi://wk2:hunter2@s3.wasabisys.com/bucket", want: Destination{Kind: KindS3, Location: "s3:https://s3.wasabisys.com/bucket/bombvault", KeyID: "wk", Secret: "ws"}},
		{url: "webdav://me@dav.example.org/dup", note: Note{Code: NoteDestination, Detail: "webdav"}},
		{url: "b2-custom://endpoint/bucket", note: Note{Code: NoteDestination, Detail: "b2-custom"}},
		{url: "no scheme", note: Note{Code: NoteDestination}},
	}
	for _, c := range cases {
		got, notes := duplicacyDestination(c.url, func(kind string) string { return keys[kind] })
		if containsFold(got.Location+got.Path, "hunter2") {
			t.Errorf("%s: the location carries a password: %+v", c.url, got)
		}
		if got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.url, got, c.want)
		}
		if c.note.Code != "" && !hasNote(notes, c.note.Code, c.note.Detail) {
			t.Errorf("%s: notes %+v lack %+v", c.url, notes, c.note)
		}
	}
}

func TestDuplicacyStorageURLSignInComesFromTheUserPartWhenNothingIsSaved(t *testing.T) {
	got, _ := duplicacyDestination("s3://id:hunter2@s3.example.com/bucket", func(string) string { return "" })
	want := Destination{Kind: KindS3, Location: "s3:https://s3.example.com/bucket/bombvault", KeyID: "id", Secret: "hunter2"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
