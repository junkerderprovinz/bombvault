package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/jobimport"
	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// How a job's destination signs in, as the preview tells it.
const (
	signInCarried = "carried"
	signInNeeded  = "needed"
	signInNone    = "none"
)

// maxToolFile caps a settings file read from the host mount. The files are a
// few kilobytes; anything near this is not one of them.
const maxToolFile = 1 << 20

type toolImportFile struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

// toolImportInput is where the other tool's settings come from: Duplicati
// exports sent from the browser, or a Duplicacy folder under the host mount.
// The apply sends it again, so a credential in the files never has to make a
// round trip through the preview.
type toolImportInput struct {
	Tool  string           `json:"tool"`
	Files []toolImportFile `json:"files"`
	Path  string           `json:"path"`
}

type toolJobView struct {
	Key      string           `json:"key"`
	Tool     string           `json:"tool"`
	Name     string           `json:"name"`
	Enabled  bool             `json:"enabled"`
	Sets     []toolSetView    `json:"sets"`
	Schedule string           `json:"schedule"`
	Dest     toolDestView     `json:"dest"`
	Notes    []jobimport.Note `json:"notes"`
}

type toolSetView struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	// Source is the folder as the other tool named it.
	Source string `json:"source"`
	// Path is where it lies under the host mount, empty when it was not found.
	Path string `json:"path"`
	// Taken names a folder set that backs the same folder up already.
	Taken    string   `json:"taken,omitempty"`
	Excludes []string `json:"excludes"`
}

type toolDestView struct {
	Kind string `json:"kind"`
	// Location is the storage location the import proposes, empty when it
	// needs an address from the person.
	Location string `json:"location"`
	// B2Path is the bucket and folder of a B2 destination, which BombVault
	// reaches over its S3 address.
	B2Path string `json:"b2Path,omitempty"`
	SignIn string `json:"signIn"`
	// Existing names a storage location that sits there already and is used
	// instead of a new one.
	Existing string `json:"existing,omitempty"`
}

type toolJobChoice struct {
	Key  string          `json:"key"`
	Sets []toolSetChoice `json:"sets"`
	// Location replaces the proposed one; the empty string leaves the
	// destination out, so the sets go where new folder sets go.
	Location *string `json:"location"`
	KeyID    string  `json:"keyId"`
	Secret   string  `json:"secret"`
}

type toolSetChoice struct {
	Key string `json:"key"`
	// Path replaces the folder the preview found.
	Path string `json:"path"`
}

type toolJobResult struct {
	Key      string   `json:"key"`
	Sets     []string `json:"sets"`
	Location string   `json:"location"`
	Error    string   `json:"error,omitempty"`
}

// handleToolImportPreview reads the jobs of Duplicati or Duplicacy and says
// what an import would make of each. It writes nothing.
// POST /api/import/tools/preview
func (h *Handler) handleToolImportPreview(w http.ResponseWriter, r *http.Request) {
	var in toolImportInput
	if !decodeBody(w, r, &in) {
		return
	}
	views, _, _, err := h.toolImportPlan(in)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"jobs": views}))
}

// handleToolImport creates the folder sets, storage locations and sign-ins of
// the jobs and sets the request picks. Each job stands on its own, so one
// that fails leaves the others in place. POST /api/import/tools
func (h *Handler) handleToolImport(w http.ResponseWriter, r *http.Request) {
	var body struct {
		toolImportInput
		Jobs []toolJobChoice `json:"jobs"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	views, jobs, finder, err := h.toolImportPlan(body.toolImportInput)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	results := make([]toolJobResult, 0, len(body.Jobs))
	for _, choice := range body.Jobs {
		i := slices.IndexFunc(views, func(v toolJobView) bool { return v.Key == choice.Key })
		if i < 0 {
			results = append(results, toolJobResult{Key: choice.Key, Sets: []string{}, Error: "no such job in these settings"})
			continue
		}
		results = append(results, h.applyToolJob(r.Context(), finder, jobs[i], views[i], choice))
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"jobs": results}))
}

func (h *Handler) toolImportPlan(in toolImportInput) ([]toolJobView, []jobimport.Job, *folderFinder, error) {
	finder := newFolderFinder(h.cfg.HostMountRoot, h.cfg.HostSourceRoot)
	jobs, err := h.readToolJobs(in, finder)
	if err != nil {
		return nil, nil, nil, err
	}
	sets, err := h.store.ListFileSets()
	if err != nil {
		return nil, nil, nil, err
	}
	repos, err := h.store.ListNamedRepos()
	if err != nil {
		return nil, nil, nil, err
	}
	names := map[string]bool{}
	for _, fs := range sets {
		names[strings.ToLower(fs.Name)] = true
	}
	views := make([]toolJobView, len(jobs))
	for i, job := range jobs {
		v := toolJobView{
			Key: strconv.Itoa(i), Tool: job.Tool, Name: job.Name, Enabled: job.Enabled,
			Sets: []toolSetView{}, Schedule: job.Schedule, Notes: job.Notes,
		}
		if v.Notes == nil {
			v.Notes = []jobimport.Note{}
		}
		for j, source := range job.Sources {
			name := cleanSetName(job.Name)
			if len(job.Sources) > 1 {
				name += "-" + cleanSetName(path.Base(source))
			}
			set := toolSetView{Key: strconv.Itoa(j), Name: freeName(names, name), Source: finder.shown(source)}
			if rel, ok := finder.find(source); ok {
				set.Path = rel
				for _, fs := range sets {
					if path.Clean(fs.Path) == rel {
						set.Taken = fs.Name
						break
					}
				}
			}
			set.Excludes = finder.excludes(job, source, set.Path)
			v.Sets = append(v.Sets, set)
		}
		v.Dest = h.toolDest(job.Dest, finder, repos)
		views[i] = v
	}
	return views, jobs, finder, nil
}

func (h *Handler) readToolJobs(in toolImportInput, finder *folderFinder) ([]jobimport.Job, error) {
	switch in.Tool {
	case "duplicati":
		if len(in.Files) == 0 {
			return nil, errors.New("choose one or more jobs exported from Duplicati")
		}
		jobs := make([]jobimport.Job, 0, len(in.Files))
		for _, f := range in.Files {
			job, err := jobimport.Duplicati([]byte(f.Text), time.Local)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", f.Name, err)
			}
			jobs = append(jobs, job)
		}
		return jobs, nil
	case "duplicacy":
		return finder.readDuplicacy(in.Path)
	}
	return nil, errors.New("choose Duplicati or Duplicacy")
}

func (h *Handler) toolDest(d jobimport.Destination, finder *folderFinder, repos []store.OffsiteTarget) toolDestView {
	v := toolDestView{Kind: d.Kind, SignIn: signInNone}
	switch d.Kind {
	case jobimport.KindLocal:
		if rel, ok := finder.find(d.Path); ok {
			v.Location = rel + "/" + jobimport.Subfolder
		}
	case jobimport.KindB2:
		v.B2Path = d.Path
	default:
		v.Location = d.Location
	}
	if d.Kind == jobimport.KindS3 || d.Kind == jobimport.KindB2 {
		v.SignIn = signInNeeded
		if d.KeyID != "" && d.Secret != "" {
			v.SignIn = signInCarried
		}
	}
	if repo, ok := h.namedRepoAt(v.Location, repos); ok {
		v.Existing = repo.Name
	}
	return v
}

// namedRepoAt finds the storage location that sits at loc already.
func (h *Handler) namedRepoAt(loc string, repos []store.OffsiteTarget) (store.OffsiteTarget, bool) {
	if loc == "" {
		return store.OffsiteTarget{}, false
	}
	want, err := h.svc.resolveRepo(loc)
	if err != nil {
		return store.OffsiteTarget{}, false
	}
	for _, r := range repos {
		if got, err := h.svc.resolveRepo(r.Repo); err == nil && sameRepoLocation(got, want) {
			return r, true
		}
	}
	return store.OffsiteTarget{}, false
}

func (h *Handler) applyToolJob(ctx context.Context, finder *folderFinder, job jobimport.Job, view toolJobView, choice toolJobChoice) toolJobResult {
	res := toolJobResult{Key: view.Key, Sets: []string{}}
	loc := view.Dest.Location
	if choice.Location != nil {
		loc = strings.TrimSpace(*choice.Location)
	} else if loc == "" && view.Dest.Kind != "" {
		res.Error = "this job's destination needs its address: type it in, or leave the destination out"
		return res
	}

	var repoID string
	undo := func() {}
	if loc != "" {
		repo, created, err := h.toolLocation(job, loc, choice)
		if err != nil {
			res.Error = err.Error()
			return res
		}
		repoID, res.Location, undo = repo.ID, repo.Name, created
	}

	var problems []string
	for _, c := range choice.Sets {
		i := slices.IndexFunc(view.Sets, func(s toolSetView) bool { return s.Key == c.Key })
		if i < 0 {
			continue
		}
		set := view.Sets[i]
		rel := strings.Trim(path.Clean("/"+strings.TrimSpace(c.Path)), "/")
		if strings.TrimSpace(c.Path) == "" {
			rel = set.Path
		}
		if rel == "" {
			problems = append(problems, fmt.Sprintf("%s: choose the folder that %s is on this server", set.Name, set.Source))
			continue
		}
		fs := store.FileSet{Name: set.Name, Path: rel, Excludes: finder.excludes(job, job.Sources[i], rel), Enabled: job.Enabled}
		if loc != "" {
			fs.Repo, fs.RepoChosen = repoID, store.RepoChosen
		}
		created, err := h.createToolFileSet(ctx, fs)
		if err != nil {
			problems = append(problems, set.Name+": "+err.Error())
			continue
		}
		res.Sets = append(res.Sets, created.Name)
		if job.Schedule != "" {
			if err := h.svc.SetFileSetScheduleCadence(ctx, created.ID, job.Schedule); err != nil {
				problems = append(problems, set.Name+": "+err.Error())
			}
		}
	}
	if len(res.Sets) == 0 {
		undo()
		res.Location = ""
	}
	res.Error = strings.Join(problems, "; ")
	return res
}

// createToolFileSet puts a folder set through the checks a new one from the
// Folders page gets.
func (h *Handler) createToolFileSet(ctx context.Context, fs store.FileSet) (store.FileSet, error) {
	if err := h.svc.validateFileSet(fs, true); err != nil {
		return store.FileSet{}, err
	}
	if _, err := h.store.GetFileSetByName(fs.Name); err == nil {
		return store.FileSet{}, errors.New("a file set with this name already exists")
	}
	if err := h.svc.fileSetNameAdoptable(ctx, fs.Name, fs.Repo, fs.Path); err != nil {
		return store.FileSet{}, err
	}
	return h.svc.createFileSet(fs, nil)
}

// toolLocation returns the storage location at loc, made with its sign-in
// when there is none yet. The function it returns removes what it made, for
// a job none of whose sets could be created.
func (h *Handler) toolLocation(job jobimport.Job, loc string, choice toolJobChoice) (store.OffsiteTarget, func(), error) {
	repos, err := h.store.ListNamedRepos()
	if err != nil {
		return store.OffsiteTarget{}, nil, err
	}
	if repo, ok := h.namedRepoAt(loc, repos); ok {
		return repo, func() {}, nil
	}
	row := store.OffsiteTarget{
		Role: store.RoleRepo, Enabled: true, Name: freeRepoName(repos, job.Name),
		Repo: loc, OffPremises: restic.IsRemoteRepo(loc),
	}
	var addedSignIn bool
	if strings.HasPrefix(loc, "s3:") {
		creds := CloudCreds{S3KeyID: job.Dest.KeyID, S3Secret: job.Dest.Secret, S3Region: job.Dest.Region}
		if choice.KeyID != "" || choice.Secret != "" {
			creds.S3KeyID, creds.S3Secret = strings.TrimSpace(choice.KeyID), strings.TrimSpace(choice.Secret)
		}
		if creds.S3KeyID == "" || creds.S3Secret == "" {
			return store.OffsiteTarget{}, nil, errors.New("this storage location needs a key ID and a secret to sign in")
		}
		if row.CredsRef, addedSignIn, err = h.svc.toolSignIn(job.Name, creds); err != nil {
			return store.OffsiteTarget{}, nil, err
		}
		if class := strings.ToUpper(job.Dest.StorageClass); class != "" && restic.StorageClassAllowed(class) {
			row.StorageClass = class
		}
	}
	dropSignIn := func() {
		if addedSignIn {
			h.svc.removeCredSet(row.CredsRef)
		}
	}
	if err := h.validateNamedRepo(row, row.StorageClass != "", true); err != nil {
		dropSignIn()
		return store.OffsiteTarget{}, nil, err
	}
	saved, err := h.store.UpsertOffsiteTarget(row)
	if err != nil {
		dropSignIn()
		return store.OffsiteTarget{}, nil, err
	}
	h.svc.locationAddedAt(saved.Repo)
	return saved, func() {
		if _, err := h.store.DeleteNamedRepoIfUnused(saved.ID); err == nil {
			dropSignIn()
		}
	}, nil
}

// toolSignIn returns the saved sign-in with these keys, adding one named
// after the job when there is none. It reports whether it added one.
func (s *Service) toolSignIn(name string, creds CloudCreds) (string, bool, error) {
	if err := validCloudCreds(&creds); err != nil {
		return "", false, err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return "", false, err
	}
	sets, err := s.decodeCloudCredSets(settings)
	if err != nil {
		return "", false, err
	}
	for _, c := range sets {
		if c.S3KeyID == creds.S3KeyID && c.S3Secret == creds.S3Secret && c.S3Region == creds.S3Region {
			return c.ID, false, nil
		}
	}
	id, err := s.addCredSet(freeSetName(sets, name), creds)
	return id, err == nil, err
}

func freeRepoName(repos []store.OffsiteTarget, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Imported"
	}
	taken := func(n string) bool {
		return slices.ContainsFunc(repos, func(r store.OffsiteTarget) bool { return strings.EqualFold(r.Name, n) })
	}
	out := name
	for i := 2; taken(out); i++ {
		out = fmt.Sprintf("%s %d", name, i)
	}
	return out
}

var setNameJunk = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// cleanSetName turns a job name into one a folder set can carry.
func cleanSetName(s string) string {
	s = setNameJunk.ReplaceAllString(s, "-")
	for strings.Contains(s, "..") {
		s = strings.ReplaceAll(s, "..", ".")
	}
	s = strings.Trim(s, "-._")
	if len(s) > 100 {
		s = strings.TrimRight(s[:100], "-._")
	}
	if s == "" {
		return "imported"
	}
	return s
}

// freeName is name, or name with a number when it is taken, and marks the
// answer taken.
func freeName(taken map[string]bool, name string) string {
	out := name
	for i := 2; taken[strings.ToLower(out)]; i++ {
		out = fmt.Sprintf("%s-%d", name, i)
	}
	taken[strings.ToLower(out)] = true
	return out
}

// folderFinder finds the folders another tool names under the host mount.
// The tools often run in a container of their own, with the shares mounted
// under a name of its choosing, so a path is first read as it is and then by
// its tails: below the mount, and below each folder at the top of it, the
// user shares first.
type folderFinder struct {
	mount  string
	source string
	tops   []string
}

func newFolderFinder(mount, source string) *folderFinder {
	f := &folderFinder{mount: path.Clean(filepath.ToSlash(mount)), source: path.Clean(source)}
	entries, _ := os.ReadDir(f.mount)
	for _, e := range entries {
		if e.IsDir() {
			f.tops = append(f.tops, e.Name())
		}
	}
	if i := slices.Index(f.tops, "user"); i > 0 {
		f.tops = append([]string{"user"}, slices.Delete(f.tops, i, i+1)...)
	}
	return f
}

func (f *folderFinder) exists(rel string) bool {
	full, err := paths.Resolve(f.mount, rel)
	if err != nil {
		return false
	}
	_, err = os.Stat(full)
	return err == nil
}

func cleanForeign(p string) string {
	return path.Clean(strings.ReplaceAll(p, `\`, "/"))
}

// below returns p's part under base, "" for base itself.
func below(p, base string) (string, bool) {
	if p == base {
		return "", true
	}
	prefix := strings.TrimSuffix(base, "/") + "/"
	rest, ok := strings.CutPrefix(p, prefix)
	return rest, ok
}

func (f *folderFinder) find(foreign string) (string, bool) {
	p := cleanForeign(foreign)
	for _, base := range []string{f.mount, f.source} {
		if rel, ok := below(p, base); ok && rel != "" && f.exists(rel) {
			return rel, true
		}
	}
	if len(p) >= 2 && p[1] == ':' {
		p = p[2:]
	}
	parts := strings.FieldsFunc(p, func(r rune) bool { return r == '/' })
	for i := range parts {
		tail := strings.Join(parts[i:], "/")
		if f.exists(tail) {
			return tail, true
		}
		for _, top := range f.tops {
			if f.exists(top + "/" + tail) {
				return top + "/" + tail, true
			}
		}
	}
	return "", false
}

// shown is a source as the preview names it: a folder BombVault itself read
// by the host's name for it, any other as the tool named it.
func (f *folderFinder) shown(source string) string {
	if rel, ok := below(cleanForeign(source), f.mount); ok {
		return path.Join(f.source, rel)
	}
	return source
}

// excludes are the job's exclusions for the set made from source, with the
// anchored ones that lie inside it moved to the host's path for rel. Until
// the folder is known they keep the tool's spelling.
func (f *folderFinder) excludes(job jobimport.Job, source, rel string) []string {
	out := []string{}
	src := cleanForeign(source)
	for _, e := range job.Excludes {
		if !jobimport.IsAnchored(e) {
			out = append(out, e)
			continue
		}
		rest, ok := below(cleanForeign(e), src)
		switch {
		case !ok:
		case rel == "":
			out = append(out, e)
		default:
			out = append(out, path.Join(f.source, rel, rest))
		}
	}
	return out
}

// readDuplicacy reads the Duplicacy settings in a folder under the mount: a
// Duplicacy Web folder with duplicacy.json, a repository with .duplicacy in
// it, or the .duplicacy folder itself. The files are opened through a root on
// the mount, so a link cannot lead the reads out of it.
func (f *folderFinder) readDuplicacy(sub string) ([]jobimport.Job, error) {
	if _, err := paths.Resolve(f.mount, sub); err != nil || strings.TrimSpace(sub) == "" {
		return nil, errors.New("choose a folder below the host mount")
	}
	root, err := os.OpenRoot(f.mount)
	if err != nil {
		return nil, fmt.Errorf("open the host mount: %w", err)
	}
	defer root.Close() //nolint:errcheck // a directory handle
	dir := strings.Trim(path.Clean("/"+filepath.ToSlash(sub)), "/")
	read := func(rel string) ([]byte, error) {
		if _, err := paths.Resolve(f.mount, rel); err != nil {
			return nil, fs.ErrNotExist
		}
		file, err := root.Open(filepath.FromSlash(rel))
		if err != nil {
			return nil, err
		}
		defer file.Close() //nolint:errcheck // read only
		data, err := io.ReadAll(io.LimitReader(file, maxToolFile+1))
		if err == nil && len(data) > maxToolFile {
			err = fmt.Errorf("%s is too large to be a settings file", path.Base(rel))
		}
		return data, err
	}
	readerAt := func(base string) jobimport.Reader {
		return func(name string) ([]byte, error) {
			if !jobimport.IsAnchored(name) {
				return read(path.Join(base, name))
			}
			rel, ok := f.find(name)
			if !ok {
				return nil, fs.ErrNotExist
			}
			return read(rel)
		}
	}
	if data, err := read(path.Join(dir, "duplicacy.json")); err == nil {
		return jobimport.DuplicacyWeb(data, readerAt(dir))
	}
	full := f.mount + "/" + dir
	if data, err := read(path.Join(dir, ".duplicacy", "preferences")); err == nil {
		return jobimport.DuplicacyCLI(data, full, readerAt(path.Join(dir, ".duplicacy")))
	}
	if path.Base(dir) == ".duplicacy" {
		if data, err := read(path.Join(dir, "preferences")); err == nil {
			return jobimport.DuplicacyCLI(data, path.Dir(full), readerAt(dir))
		}
	}
	return nil, errors.New("this folder holds no Duplicacy settings: choose the folder with duplicacy.json, or a folder with .duplicacy in it")
}
