package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// A section's own folder is where its backups are written first, the path in
// its settings. Moving it to a storage location gives it that location's
// sign-in, protection and limits through the link on the section's
// remote-primary row; a typed address keeps a sign-in and safety settings of
// its own.

var (
	errOwnFolderNoSection = errors.New("pick at least one section to move")
	errOwnFolderNowhere   = errors.New("pick a storage location or type an address")
	errOwnFolderNotHere   = errors.New("a section's own folder can go to a folder on this host or to a storage location set up once, not to a named repository or a copy")
)

// ownFolderRequest moves the own folders of one or more sections.
type ownFolderRequest struct {
	Domains []string `json:"domains"`
	// Location is the id of the storage location to move to, as the storage
	// pages name it. Address is used when it is empty.
	Location string `json:"location"`
	// Address is a typed place, the folder each section gets one of its own
	// in. CredsRef is the saved sign-in a remote one signs in with.
	Address  string `json:"address"`
	CredsRef string `json:"credsRef"`
	// CopyOver copies the backups already made before the switch.
	CopyOver bool `json:"copyOver"`
}

// ownFolderStep is what moving one section's own folder does.
type ownFolderStep struct {
	Domain string `json:"domain"`
	From   string `json:"from"`
	To     string `json:"to"`
	// Same says the section keeps its own backups there already.
	Same bool `json:"same"`
	// CopyAt names the copy that lies at the new place. It becomes the own
	// folder, and copying there ends.
	CopyAt string `json:"copyAt,omitempty"`
	// Copies are the names of the section's other copies, which go on.
	Copies []string `json:"copies"`
	// Backups is how many backups the old place holds, Pending how many of
	// them the new place does not hold yet.
	Backups int `json:"backups"`
	Pending int `json:"pending"`
	// Key is the progress key a copy before the switch reports under.
	Key string `json:"key"`

	plan   movePlan
	folder store.OwnFolder
	path   string
	// primaryCopy says CopyAt is the copy the section's off-site field holds.
	primaryCopy bool
}

// ownFolderPlace is where the request sends the sections.
type ownFolderPlace struct {
	base string
	dest *store.OffsiteTarget
	// pathID is the id of a folder on this host the storage pages list.
	pathID   string
	credsRef string
}

func (s *Service) ownFolderPlace(req ownFolderRequest) (ownFolderPlace, error) {
	kind, id, _ := strings.Cut(req.Location, ":")
	switch {
	case req.Location == "" && strings.TrimSpace(req.Address) == "":
		return ownFolderPlace{}, errOwnFolderNowhere
	case req.Location == "":
		base := strings.TrimRight(strings.TrimSpace(req.Address), "/")
		if _, err := s.resolveRepo(base); err != nil {
			return ownFolderPlace{}, err
		}
		return ownFolderPlace{base: base, credsRef: req.CredsRef}, nil
	case kind == locationDestination:
		d, ok, err := s.store.GetDestination(id)
		if err != nil {
			return ownFolderPlace{}, err
		}
		if !ok {
			return ownFolderPlace{}, store.ErrNotDestination
		}
		return ownFolderPlace{base: d.Repo, dest: &d}, nil
	case kind == locationPath:
		loc, err := s.StorageLocation(req.Location)
		if err != nil {
			return ownFolderPlace{}, err
		}
		if loc.Kind != "local" {
			return ownFolderPlace{}, errOwnFolderNotHere
		}
		return ownFolderPlace{base: loc.Where, pathID: loc.ID}, nil
	}
	return ownFolderPlace{}, errOwnFolderNotHere
}

// planOwnFolders checks a move of own folders and counts what it would copy.
// Nothing is written.
func (s *Service) planOwnFolders(ctx context.Context, req ownFolderRequest) ([]ownFolderStep, error) {
	if len(req.Domains) == 0 {
		return nil, errOwnFolderNoSection
	}
	place, err := s.ownFolderPlace(req)
	if err != nil {
		return nil, err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, err
	}
	var here []string
	if place.pathID != "" {
		loc, err := s.StorageLocation(place.pathID)
		if err != nil {
			return nil, err
		}
		here = sectionDomains(loc)
	}
	steps := make([]ownFolderStep, 0, len(req.Domains))
	for _, domain := range req.Domains {
		if !validOffsiteDomain(domain) {
			return nil, fmt.Errorf("unknown domain %q", domain)
		}
		step, err := s.planOwnFolder(ctx, settings, domain, place, slices.Contains(here, domain), req.CopyOver)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", domain, err)
		}
		steps = append(steps, step)
	}
	return steps, nil
}

func sectionDomains(loc storageLocation) []string {
	var out []string
	for _, sec := range loc.Sections {
		if sec.Use == useHome {
			out = append(out, sec.Domain)
		}
	}
	return out
}

func (s *Service) planOwnFolder(ctx context.Context, settings store.Settings, domain string, place ownFolderPlace, here, copyOver bool) (ownFolderStep, error) {
	from := strings.TrimSpace(domainPathRaw(domain, settings))
	to := destinationLocation(place.base, domain)
	step := ownFolderStep{Domain: domain, From: scrubRepoLocation(from), To: scrubRepoLocation(to), Key: "own:" + domain, Copies: []string{}}
	if place.dest != nil {
		row, _, err := s.store.PrimaryRemoteTarget(domain)
		if err != nil {
			return ownFolderStep{}, err
		}
		_, linked := s.ownFolderLocation(row)
		step.Same = linked && row.DestinationID == place.dest.ID || sameRepoLocation(from, to)
	} else {
		step.Same = here || sameRepoLocation(from, to)
	}
	if step.Same {
		step.To = step.From
		return step, nil
	}
	step.path = to
	step.folder = store.OwnFolder{Domain: domain, Row: s.ownFolderRow(domain, to, place)}

	targets, err := s.store.OffsiteTargetsForDomain(domain)
	if err != nil {
		return ownFolderStep{}, err
	}
	var copyAt store.OffsiteTarget
	for _, t := range targets {
		if sameRepoLocation(t.Repo, to) {
			copyAt = t
			continue
		}
		if t.Enabled {
			step.Copies = append(step.Copies, t.Name)
		}
	}
	if copyAt.ID != "" {
		if _, direct, err := s.store.CompanionFor(copyAt.ID); err != nil {
			return ownFolderStep{}, err
		} else if direct {
			return ownFolderStep{}, store.ErrTargetHasDirectRepo
		}
		step.CopyAt, step.folder.EndCopy = copyAt.ID, copyAt.ID
		step.primaryCopy = copyAt.SortOrder == 0 && sameRepoLocation(offsiteRepoFromSettings(domain, settings), copyAt.Repo)
	}

	plan := movePlan{sectionMove: sectionMove{Domain: domain, Key: step.Key, From: step.From, To: step.To}}
	if plan.src, err = s.resolveRepo(from); err != nil {
		return ownFolderStep{}, err
	}
	if plan.dst, err = s.resolveRepo(to); err != nil {
		return ownFolderStep{}, err
	}
	live, err := s.liveRepos(settings)
	if err != nil {
		return ownFolderStep{}, err
	}
	self := []string{homeID(domain), copyAt.ID}
	if place.dest != nil {
		self = append(self, place.dest.ID)
	}
	if overlapsLive(s.placesNow(), plan.dst, live, self...) {
		return ownFolderStep{}, errNestedLocation
	}
	plan.srcMode = s.primaryModeFor(settings, domain, plan.src)
	plan.mode = s.ModeFor(settings)
	if row := step.folder.Row; row != nil && restic.IsRemoteRepo(to) {
		plan.mode = s.applyTargetCreds(plan.mode, settings, *row)
		plan.lim = restic.Limits{UploadKBps: row.LimitUpload, DownloadKBps: row.LimitDownload}
		plan.mode.Limits = plan.lim
	}
	plan.mode.Compression = storedCompression(settings.CompressionFor(domain))
	step.plan = plan
	if !s.engine.RepoOpens(ctx, plan.src, plan.srcMode) {
		return step, nil
	}
	pending, all, err := s.pendingMove(ctx, plan)
	if err != nil {
		return ownFolderStep{}, err
	}
	step.Backups, step.Pending = all, len(pending)
	if copyOver && step.Pending > 0 {
		env, ok := bothSignIns(plan.dst, plan.mode.Env, plan.src, plan.srcMode.Env)
		if !ok {
			return ownFolderStep{}, errMoveSignIn
		}
		step.plan.copyMode = plan.mode
		step.plan.copyMode.Env = env
	}
	return step, nil
}

// ownFolderRow is the remote-primary row the own folder keeps at to. One in a
// storage location follows it; a typed remote address keeps the safety
// settings the section had for a typed address before.
func (s *Service) ownFolderRow(domain, to string, place ownFolderPlace) *store.OffsiteTarget {
	switch {
	case place.dest != nil:
		row := followLocation(store.OffsiteTarget{Repo: to, DestinationID: place.dest.ID}, *place.dest)
		return &row
	case !restic.IsRemoteRepo(to):
		return nil
	}
	row := store.OffsiteTarget{Repo: to, CredsRef: place.credsRef, Enabled: true}
	if old, ok, err := s.store.PrimaryRemoteTarget(domain); err == nil && ok && old.DestinationID == "" {
		row.Immutable, row.LimitUpload, row.LimitDownload, row.GrowthBudgetGB = old.Immutable, old.LimitUpload, old.LimitDownload, old.GrowthBudgetGB
	}
	return &row
}

// switchOwnFolder points the section at its new own folder: the path, the
// row and the end of the copy there, in one transaction.
func (s *Service) switchOwnFolder(step ownFolderStep) (store.Settings, error) {
	err := s.store.MoveOwnFolder(step.folder, func(st *store.Settings) {
		setDomainPath(st, step.Domain, step.path)
		if step.primaryCopy {
			setOffsiteRepoInSettings(st, step.Domain, "")
		}
	})
	if err != nil {
		return store.Settings{}, err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return store.Settings{}, err
	}
	if step.primaryCopy {
		if err := s.syncPrimaryOffsiteTarget(step.Domain, settings); err != nil {
			return store.Settings{}, err
		}
	}
	return settings, nil
}

func setDomainPath(st *store.Settings, domain, path string) {
	switch domain {
	case "containers":
		st.ContainersPath = path
	case "vms":
		st.VMsPath = path
	case "flash":
		st.FlashPath = path
	case "config":
		st.ConfigPath = path
	case "files":
		st.FilesPath = path
	case "zfs":
		st.ZFSPath = path
	}
}

// copyThenSwitch copies what the new own folder lacks, the bulk while the
// section goes on being backed up and the rest under its lock, checks the new
// place and switches. The old place is only read.
func (s *Service) copyThenSwitch(ctx context.Context, step ownFolderStep, startedAt int64) (store.Settings, error) {
	plan := step.plan
	if err := s.EnsureRepo(ctx, plan.dst, plan.mode); err != nil {
		return store.Settings{}, fmt.Errorf("open %s: %w", plan.To, err)
	}
	if err := s.copyPending(ctx, plan, startedAt); err != nil {
		return store.Settings{}, err
	}
	unlock, err := s.lockDomainWhile(ctx, plan.Domain, "replicate")
	if err != nil {
		return store.Settings{}, err
	}
	defer unlock()
	if err := s.copyPending(ctx, plan, startedAt); err != nil {
		return store.Settings{}, err
	}
	left, _, err := s.pendingMove(ctx, plan)
	if err != nil {
		return store.Settings{}, err
	}
	if len(left) > 0 {
		return store.Settings{}, errMoveMissing
	}
	if err := s.alignCopies(ctx, plan); err != nil {
		return store.Settings{}, err
	}
	if err := s.engine.Check(ctx, plan.dst, plan.mode); err != nil {
		return store.Settings{}, fmt.Errorf("check %s: %w", plan.To, err)
	}
	return s.switchOwnFolder(step)
}

// MoveOwnFolders switches every section of the request that needs nothing
// copied at once, and copies the backups of the others in the background
// first. moved runs after each switch.
func (s *Service) MoveOwnFolders(ctx context.Context, req ownFolderRequest, moved func(store.Settings)) ([]ownFolderStep, error) {
	steps, err := s.planOwnFolders(ctx, req)
	if err != nil {
		return nil, err
	}
	var later []ownFolderStep
	for _, step := range steps {
		if step.Same {
			continue
		}
		if req.CopyOver && step.Pending > 0 {
			later = append(later, step)
			continue
		}
		settings, err := s.switchOwnFolder(step)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", step.Domain, err)
		}
		moved(settings)
	}
	for _, step := range later {
		if err := s.startOwnFolderCopy(ctx, step, moved); err != nil {
			return nil, fmt.Errorf("%s: %w", step.Domain, err)
		}
	}
	return steps, nil
}

func (s *Service) startOwnFolderCopy(ctx context.Context, step ownFolderStep, moved func(store.Settings)) error {
	bctx := context.WithoutCancel(ctx)
	rctx, cancel := context.WithTimeout(bctx, 7*24*time.Hour)
	s.cancelMu.Lock()
	if _, running := s.runCancels[step.Key]; running {
		s.cancelMu.Unlock()
		cancel()
		return errMoveRunning
	}
	s.runCancels[step.Key] = cancel
	s.cancelMu.Unlock()
	go func() {
		var runID string
		defer s.recoverOperation("own folder "+step.Domain, nil, func(msg string) { s.finishMoveRun(runID, errors.New(msg)) })
		defer cancel()
		defer func() {
			s.cancelMu.Lock()
			delete(s.runCancels, step.Key)
			s.cancelMu.Unlock()
		}()
		var rErr error
		if runID, rErr = s.startRun(bctx, domainRunTargetID(step.Domain), "offsite"); rErr != nil {
			log.Printf("api: own folder %s: the run is not recorded: %v", step.Domain, rErr) //nolint:gosec // G706: domain is a fixed literal
		}
		_, startedAt := s.progBegin(rctx, step.Key, "replicate")
		settings, err := s.copyThenSwitch(rctx, step, startedAt)
		if err == nil {
			moved(settings)
		}
		s.progEnd(step.Key, "replicate", err == nil, startedAt)
		s.finishMoveRun(runID, err)
		if err != nil {
			log.Printf("api: own folder %s: %v", step.Domain, scrubError(err)) //nolint:gosec // G706: domain is a fixed literal and the error scrubbed
		}
	}()
	return nil
}

// handleOwnFolderPreview says what moving own folders would do.
// POST /api/storage/own-folders/preview
func (h *Handler) handleOwnFolderPreview(w http.ResponseWriter, r *http.Request) {
	var req ownFolderRequest
	if !decodeBody(w, r, &req) {
		return
	}
	steps, err := h.svc.planOwnFolders(r.Context(), req)
	if err != nil {
		moveFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"steps": steps}))
}

// handleMoveOwnFolders moves the own folders of the sections the body names.
// POST /api/storage/own-folders
func (h *Handler) handleMoveOwnFolders(w http.ResponseWriter, r *http.Request) {
	var req ownFolderRequest
	if !decodeBody(w, r, &req) {
		return
	}
	// The scheduler reads the paths and the off-site fields a switch writes.
	moved := func(settings store.Settings) {
		if err := h.scheduler.ReloadWithGates(settings, h.dueGates()); err != nil {
			log.Printf("api: own folder: the scheduler did not reload: %v", err)
		}
	}
	steps, err := h.svc.MoveOwnFolders(r.Context(), req, moved)
	if err != nil {
		moveFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"steps": steps}))
}
