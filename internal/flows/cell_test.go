//go:build flows

package flows

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

// start is how an install came to be: set up on this build, or set up on the
// previous release and then updated, which is what most users have.
type start struct {
	name string
	// boot starts the server, runs prepare on the binary that sets the install
	// up, and leaves the current build running.
	boot func(f *flow, prepare func())
}

var starts = []start{
	{"fresh", func(f *flow, prepare func()) {
		f.b.start(currentBinary)
		prepare()
	}},
	{"updated", func(f *flow, prepare func()) {
		upgrade(f, previousBinary(f.t), prepare)
	}},
}

// upgrade sets the install up on the binary from, then updates it to this
// build.
func upgrade(f *flow, from string, prepare func()) {
	f.b.start(from)
	prepare()
	f.b.settle()
	f.b.stop()
	f.b.start(currentBinary)
}

var (
	releasesMu sync.Mutex
	releases   = map[string]string{}
)

// previousBinary builds the last release before this tree, or FLOWS_PREVIOUS_TAG
// when it is set. Running that release itself beats a stored database: there is
// no fixture to go stale, and the repositories it leaves behind are its own.
func previousBinary(t *testing.T) string {
	t.Helper()
	tag := os.Getenv("FLOWS_PREVIOUS_TAG")
	if tag == "" {
		var err error
		if tag, err = previousTag(); err != nil {
			t.Fatal(err)
		}
	}
	return releaseBinary(t, tag)
}

// releaseBinary builds one release once per run, whoever asks first.
func releaseBinary(t *testing.T, tag string) string {
	t.Helper()
	releasesMu.Lock()
	defer releasesMu.Unlock()
	if bin, built := releases[tag]; built {
		return bin
	}
	tree := filepath.Join(buildDir, tag+"-tree")
	if err := os.MkdirAll(tree, 0o755); err != nil {
		t.Fatal(err)
	}
	archive, err := exec.Command("git", "-C", moduleRoot, "archive", "--format=tar", tag).Output()
	if err != nil {
		t.Fatalf("export %s: %v", tag, err)
	}
	if err := unpack(archive, tree); err != nil {
		t.Fatalf("unpack %s: %v", tag, err)
	}
	bin, err := buildServer(tree, tag)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("flows: built", tag)
	releases[tag] = bin
	return bin
}

// unpack writes a git archive below dir. On Windows the tar on PATH may read a
// drive letter as a remote host, so it is done here.
func unpack(archive []byte, dir string) error {
	r := tar.NewReader(bytes.NewReader(archive))
	for {
		h, err := r.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		p := filepath.Join(dir, filepath.FromSlash(h.Name))
		switch h.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(p, 0o755)
		case tar.TypeReg:
			var data []byte
			if data, err = io.ReadAll(r); err == nil {
				err = os.WriteFile(p, data, 0o644)
			}
		}
		if err != nil {
			return err
		}
	}
}

// previousTag is the newest version tag behind HEAD. A release commit carries
// its own tag, which is not the release before it.
func previousTag() (string, error) {
	out, err := exec.Command("git", "-C", moduleRoot, "tag", "--merged", "HEAD", "--no-contains", "HEAD", "--sort=-v:refname", "v*").Output()
	if err != nil {
		return "", fmt.Errorf("list the tags: %w", err)
	}
	tags := strings.Fields(string(out))
	if len(tags) == 0 {
		return "", fmt.Errorf("no version tag behind HEAD, a shallow clone needs fetch-depth: 0")
	}
	return tags[0], nil
}

// flow is one cell of the matrix: one install, one place of the own folder, one
// flow from start to end.
type flow struct {
	t *testing.T
	b *bombvault
	// own is the own folder of the section under test.
	own ownFolder
	// known is this cell's entry on the list of known failures, nil when it has none.
	known    *knownFailure
	failures []string
}

func newFlow(t *testing.T) *flow {
	t.Helper()
	f := &flow{t: t, b: newBombvault(t)}
	for i := range knownFailures {
		if ok, err := path.Match(knownFailures[i].cell, t.Name()); err != nil {
			t.Fatal(err)
		} else if ok {
			f.known = &knownFailures[i]
		}
	}
	return f
}

// errorf records that the server broke a promise. A cell on the known list goes
// on to its end and is reported, any other fails.
func (f *flow) errorf(format string, args ...any) {
	f.t.Helper()
	if f.known == nil {
		f.t.Errorf(format, args...)
		return
	}
	f.failures = append(f.failures, fmt.Sprintf(format, args...))
}

// finish turns a known failure into a skip that says why. A listed cell that
// passes, or that fails in some other way, fails the suite.
func (f *flow) finish() {
	f.t.Helper()
	if f.known == nil {
		return
	}
	found := strings.Join(f.failures, "\n  ")
	switch {
	case len(f.failures) == 0:
		f.t.Errorf("this cell passes and is listed as a known failure (%s): take it off the list", f.known.why)
	case !strings.Contains(found, f.known.shows):
		f.t.Errorf("this cell is listed for %q and failed otherwise:\n  %s", f.known.shows, found)
	default:
		knownMu.Lock()
		knownSeen = append(knownSeen, f.t.Name()+": "+f.known.why)
		knownMu.Unlock()
		f.t.Skipf("known failure, %s:\n  %s", f.known.why, found)
	}
}

var (
	knownMu   sync.Mutex
	knownSeen []string
)

func reportKnownFailures() {
	knownMu.Lock()
	defer knownMu.Unlock()
	if len(knownSeen) == 0 {
		return
	}
	slices.Sort(knownSeen)
	fmt.Printf("\nKnown failures, %d cells:\n", len(knownSeen))
	for _, line := range knownSeen {
		fmt.Println("  " + line)
	}
}

// ownFolder is where a section writes its backups first, and the card a user
// changes to say how long they stay there.
type ownFolder struct {
	// repo is the repository, relative to the host mount.
	repo string
	// card sets the How long to keep card of the place the user sees it on.
	card func(k keep)
	// location is the named location involved, "" for a typed path in the
	// built-in place.
	location string
}

// place is one way a section's own folder can have come to be where it is.
type place struct {
	name    string
	prepare func(b *bombvault, domain string) ownFolder
}

// repoFolder is the folder a section's repository lies in below a location,
// which is the section's name but for the self-backup.
func repoFolder(domain string) string {
	if domain == "config" {
		return "selfbackup"
	}
	return domain
}

func typeOwnFolder(b *bombvault, domain, path string) {
	b.t.Helper()
	b.changeSettings(func(s map[string]any) {
		s[domain+"Enabled"] = true
		s[domain+"Path"] = path
	})
}

func (b *bombvault) namedLocation(name, folder string) string {
	b.t.Helper()
	var answer struct {
		Destination struct {
			ID string `json:"id"`
		} `json:"destination"`
	}
	b.call("POST", "/api/offsite/destinations", map[string]any{"provider": "path", "name": name, "settings": map[string]string{"path": folder}}, &answer)
	if answer.Destination.ID == "" {
		b.t.Fatalf("no id in the answer for the new location %s", name)
	}
	return answer.Destination.ID
}

// setLocationCard saves a named location's card as the storage page does.
func (b *bombvault) setLocationCard(id, name string, k keep) {
	b.t.Helper()
	b.call("PUT", "/api/offsite/destinations/"+id, map[string]any{"name": name, "storageClass": "", "immutable": false, "retention": k}, nil)
}

// setSharedCard saves the card of a folder in the built-in local place.
func (b *bombvault) setSharedCard(k keep) {
	b.t.Helper()
	b.changeSettings(func(s map[string]any) {
		s["retentionKeepLast"], s["retentionKeepDaily"], s["retentionKeepWeekly"] = k.Last, k.Daily, k.Weekly
		s["retentionKeepMonthly"], s["retentionKeepYearly"] = k.Monthly, k.Yearly
	})
}

func (b *bombvault) moveOwnFolder(domain, locationID string) {
	b.t.Helper()
	b.call("POST", "/api/storage/own-folders", map[string]any{"domains": []string{domain}, "location": locationID, "copyOver": true}, nil)
	b.settle()
}

var (
	typedPath = place{"typed", func(b *bombvault, domain string) ownFolder {
		repo := "user/backups/" + repoFolder(domain)
		typeOwnFolder(b, domain, repo)
		return ownFolder{repo: repo, card: b.setSharedCard}
	}}

	namedOwnFolder = place{"named", func(b *bombvault, domain string) ownFolder {
		const name = "Named place"
		id := b.namedLocation(name, "user/named")
		b.changeSettings(func(s map[string]any) { s[domain+"Enabled"] = true })
		b.moveOwnFolder(domain, "destination:"+id)
		return ownFolder{repo: "user/named/" + repoFolder(domain), location: id, card: func(k keep) { b.setLocationCard(id, name, k) }}
	}}

	// namedThenTyped has the location and the own folder on one folder without Use
	// as own folder ever being pressed. The other order, a location on a folder
	// that holds an own folder, is refused.
	namedThenTyped = place{"typed-on-named", func(b *bombvault, domain string) ownFolder {
		const name = "Local folder"
		id := b.namedLocation(name, "user/named")
		repo := "user/named/" + repoFolder(domain)
		typeOwnFolder(b, domain, repo)
		return ownFolder{repo: repo, location: id, card: func(k keep) { b.setLocationCard(id, name, k) }}
	}}

	ownFolderPlaces = []place{typedPath, namedOwnFolder, namedThenTyped}
	// restorePlaces leaves out the shapes that open the same repository the same
	// way as a typed path, which is all a restore asks of its place.
	restorePlaces = []place{typedPath, namedOwnFolder}
)
