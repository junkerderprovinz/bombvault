//go:build flows

package flows

import (
	"encoding/json"
	"net/http"
	"path"
	"slices"
	"strings"
)

// keep is a keep rule as every part of the API spells it.
type keep struct {
	Last    int `json:"keepLast"`
	Daily   int `json:"keepDaily"`
	Weekly  int `json:"keepWeekly"`
	Monthly int `json:"keepMonthly"`
	Yearly  int `json:"keepYearly"`
}

func (k keep) any() bool { return k != keep{} }

type section struct {
	Domain    string   `json:"domain"`
	Use       string   `json:"use"`
	TargetID  string   `json:"targetId"`
	RepoID    string   `json:"repoId"`
	Where     string   `json:"where"`
	Immutable bool     `json:"immutable"`
	Retention keep     `json:"retention"`
	Own       []string `json:"own"`
}

type location struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Where      string    `json:"where"`
	Retention  *keep     `json:"retention"`
	Sections   []section `json:"sections"`
	Protection struct {
		Immutable bool `json:"immutable"`
	} `json:"protection"`
}

func (b *bombvault) locations() []location {
	b.t.Helper()
	var got struct {
		Locations []location `json:"locations"`
	}
	b.call("GET", "/api/storage/locations", nil, &got)
	return got.Locations
}

// ownFolderAt finds the location that lists a section's own folder, and the
// section in it. A home with a repository or a target behind it is another
// kind of place, as on the storage page.
func ownFolderAt(all []location, domain string) (location, section, bool) {
	for _, l := range all {
		for _, s := range l.Sections {
			if s.Domain == domain && s.Use == "home" && s.RepoID == "" && s.TargetID == "" {
				return l, s, true
			}
		}
	}
	return location{}, section{}, false
}

// Who prunes on the Protection card, in the words of its sentences: the far side,
// nobody, BombVault by the card, BombVault by the section's own values.
const (
	prunesFar  = "far"
	prunesNone = "none"
	prunesCard = "card"
	prunesOwn  = "own"
)

// whoPrunesOwn reads the sentence ProtectionCard shows for an own folder from
// the fields it reads it from.
func whoPrunesOwn(s section) string {
	switch {
	case s.Immutable:
		return prunesFar
	case !s.Retention.any():
		return prunesNone
	case slices.Contains(s.Own, "retention"):
		return prunesOwn
	}
	return prunesCard
}

// whoPrunesCopies reads the sentence for the copies at a location, which is also
// the one it shows when no section lies there yet.
func whoPrunesCopies(l location) string {
	switch {
	case l.Protection.Immutable:
		return prunesFar
	case l.Retention == nil || !l.Retention.any():
		return prunesNone
	}
	return prunesCard
}

type previewSnapshot struct {
	ID string `json:"id"`
}

type previewItem struct {
	Remove []previewSnapshot `json:"remove"`
}

type preview struct {
	Policy struct {
		keep
		On  bool `json:"on"`
		Own bool `json:"own"`
	} `json:"policy"`
	Repos []struct {
		Name       string        `json:"name"`
		AppendOnly bool          `json:"appendOnly"`
		Items      []previewItem `json:"items"`
		Error      string        `json:"error"`
	} `json:"repos"`
}

// previewOf reads the Preview, and false when there is no backup to preview yet.
func (b *bombvault) previewOf(domain, source string) (preview, bool) {
	b.t.Helper()
	url := "/api/retention/preview/" + domain
	if source != "" {
		url += "?source=" + source
	}
	status, raw := b.send("GET", url, nil)
	if strings.Contains(string(raw), `"no backups yet"`) {
		return preview{}, false
	}
	var got struct {
		OK      bool    `json:"ok"`
		Preview preview `json:"preview"`
	}
	if err := json.Unmarshal(raw, &got); err != nil || status != http.StatusOK || !got.OK {
		b.t.Fatalf("GET %s = %d: %s", url, status, raw)
	}
	for _, r := range got.Preview.Repos {
		if r.Error != "" {
			b.t.Fatalf("the Preview could not read %s: %s", r.Name, r.Error)
		}
	}
	return got.Preview, true
}

// removals lists the snapshots a preview says go, by id.
func (p preview) removals() []string {
	var out []string
	for _, r := range p.Repos {
		for _, it := range r.Items {
			for _, sn := range it.Remove {
				out = append(out, sn.ID)
			}
		}
	}
	slices.Sort(out)
	return out
}

// agrees checks that every place the API tells the user the rule of a section's
// own folder says the same: the location that lists it, its card, the sentence
// under Who prunes, the Preview, and any other location on the same folder. The
// job itself is the flow's to check, since what it can remove depends on what
// the flow backed up.
func (f *flow) agrees(domain string, want keep, ownRule bool) {
	f.t.Helper()
	all := f.b.locations()
	home, sec, ok := ownFolderAt(all, domain)
	if !ok {
		f.errorf("no location lists the own folder of %s", domain)
		return
	}
	if sec.Retention != want {
		f.errorf("%s: location %q lists its own folder with the rule %+v, want %+v", domain, home.Name, sec.Retention, want)
	}
	if got := slices.Contains(sec.Own, "retention"); got != ownRule {
		f.errorf("%s: location %q says its section holds an own rule: %v, want %v", domain, home.Name, got, ownRule)
	}
	if !ownRule && (home.Retention == nil || *home.Retention != want) {
		f.errorf("%s: the card of location %q shows %+v, want %+v", domain, home.Name, home.Retention, want)
	}
	wantPruner := prunesCard
	switch {
	case !want.any():
		wantPruner = prunesNone
	case ownRule:
		wantPruner = prunesOwn
	}
	if got := whoPrunesOwn(sec); got != wantPruner {
		f.errorf("%s: Who prunes reads %q, want %q", domain, got, wantPruner)
	}
	for _, other := range all {
		if other.ID == home.ID || other.Where != path.Dir(sec.Where) || ownRule {
			continue
		}
		if other.Retention != nil && *other.Retention != sec.Retention {
			f.errorf("%s: its own folder lies at %s and ages by %+v, and the card of the location %q there shows %+v",
				domain, sec.Where, sec.Retention, other.Name, *other.Retention)
		}
		if got := whoPrunesCopies(other); got != whoPrunesOwn(sec) {
			f.errorf("%s: its own folder lies at %s and Who prunes reads %q for it, and on the location %q there %q",
				domain, sec.Where, whoPrunesOwn(sec), other.Name, got)
		}
	}
	// Before the first backup there is no repository for a Preview to read.
	p, read := f.b.previewOf(domain, "")
	if !read {
		return
	}
	if p.Policy.On != want.any() || p.Policy.keep != want {
		f.errorf("%s: the Preview says the rule is on: %v with %+v, want on: %v with %+v", domain, p.Policy.On, p.Policy.keep, want.any(), want)
	}
	if p.Policy.Own != ownRule {
		f.errorf("%s: the Preview says the section's own rule applies: %v, want %v", domain, p.Policy.Own, ownRule)
	}
}
