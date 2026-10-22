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
	// errOwnFolderUnreadable is an old own folder that is there but does not
	// answer: an unmounted disk, a server that is down, a sign-in refused.
	errOwnFolderUnreadable = errors.New("the backups in the old own folder could not be read, so they cannot be copied")
	errOwnFolderChanged    = fmt.Errorf("%w: the own folder was set to another place in the meantime and stays there", errPlacementStale)
	errOwnFolderNewBackup  = fmt.Errorf("%w: a backup reached the old own folder while the move was being planned, so it needs planning again", errPlacementStale)
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
	// WiderKeep says CopyAt keeps backups the section's own keep rule would
	// forget. From the switch on the section keeps what either rule keeps.
	WiderKeep bool `json:"widerKeep,omitempty"`
	// SwitchesOn says the section's own folder is switched off and its
	// backups go straight to CopyAt, so the switch turns the own folder on.
	SwitchesOn bool `json:"switchesOn,omitempty"`

	plan   movePlan
	folder store.OwnFolder
	// from is the path the settings held when the step was planned, path the
	// one the switch writes.
	from, path string
	// copyKeep is the keep rule of CopyAt, nil without one.
	copyKeep *store.RetentionKeep
	// places are the storage locations the move reads and writes, by the id
	// an erasure there runs under.
	places [2]string
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
	row, _, err := s.store.PrimaryRemoteTarget(domain)
	if err != nil {
		return ownFolderStep{}, err
	}
	fromDest, linked := s.ownFolderLocation(row)
	if place.dest != nil {
		step.Same = linked && row.DestinationID == place.dest.ID || sameRepoLocation(from, to)
	} else {
		step.Same = here || sameRepoLocation(from, to)
	}
	if step.Same {
		step.To = step.From
		return step, nil
	}
	step.from, step.path = from, to
	step.places = [2]string{locationErasureID(from, fromDest.ID), locationErasureID(to, "")}
	if place.dest != nil {
		step.places[1] = place.dest.ID
	}
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
		// After the switch the copy's history ages by the section's own rule,
		// which must not forget what the copy kept.
		keep := targetKeep(copyAt)
		step.copyKeep = &keep
		step.WiderKeep = widerKeep(localKeep(settings, domain), keep) != localKeep(settings, domain)
		if r := step.folder.Row; r != nil && place.dest == nil {
			r.Immutable = r.Immutable || copyAt.Immutable
		}
		if copyAt.Enabled {
			off, err := s.localOff(domain)
			if err != nil {
				return ownFolderStep{}, err
			}
			step.SwitchesOn, step.folder.SwitchOn = off, off
		}
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
		plan.mode = withRowStorageClass(s.applyTargetCreds(plan.mode, settings, *row), *row)
		plan.lim = restic.Limits{UploadKBps: row.LimitUpload, DownloadKBps: row.LimitDownload}
		plan.mode.Limits = plan.lim
	}
	plan.mode.Compression = storedCompression(settings.CompressionFor(domain))
	step.plan = plan
	opens, err := s.oldOwnFolderOpens(ctx, plan)
	switch {
	case err != nil && copyOver:
		return ownFolderStep{}, fmt.Errorf("%w: %w", errOwnFolderUnreadable, err)
	case err != nil || !opens:
		return step, nil
	}
	pending, all, err := s.pendingMove(ctx, plan)
	if err != nil {
		// Without the copy nothing is read from the old place, so a share that
		// is not mounted or a bucket that does not answer is no reason to stay.
		if !copyOver {
			return step, nil
		}
		return ownFolderStep{}, fmt.Errorf("%w: %w", errOwnFolderUnreadable, err)
	}
	step.Backups, step.Pending = all, len(pending)
	if copyOver && step.Pending > 0 {
		if err := step.plan.joinSignIns(); err != nil {
			return ownFolderStep{}, err
		}
	}
	return step, nil
}

// oldOwnFolderOpens reports whether the old own folder holds a repository.
// One that was never created holds nothing; one that is there and does not
// answer, or a remote one that held backups and holds no repository now, is an
// error, so its backups are not taken for none.
func (s *Service) oldOwnFolderOpens(ctx context.Context, plan movePlan) (bool, error) {
	err := s.engine.RepoOpensErr(ctx, plan.src, plan.srcMode)
	switch {
	case err == nil:
		return true, nil
	case restic.IsRemoteRepo(plan.src) && isRepoDefinitelyAbsent(err):
		if s.recordedEstablishment(plan.src) != repoNeverEstablished {
			return false, errors.New("it held backups before and holds no repository now")
		}
		return false, nil
	case restic.IsRemoteRepo(plan.src):
		return false, err
	case s.repoEstablished(plan.src) && !s.destinationMounted(plan.src):
		return false, ErrBackupPathNotMounted
	case localRepoMissing(plan.src):
		return false, nil
	}
	return false, err
}

// widerKeep keeps every backup that a or b keeps. All zero keeps them all.
func widerKeep(a, b store.RetentionKeep) store.RetentionKeep {
	if a == (store.RetentionKeep{}) || b == (store.RetentionKeep{}) {
		return store.RetentionKeep{}
	}
	return store.RetentionKeep{
		KeepLast: max(a.KeepLast, b.KeepLast), KeepDaily: max(a.KeepDaily, b.KeepDaily), KeepWeekly: max(a.KeepWeekly, b.KeepWeekly),
		KeepMonthly: max(a.KeepMonthly, b.KeepMonthly), KeepYearly: max(a.KeepYearly, b.KeepYearly),
	}
}

// locationErasureID is the id the deletion of the backups at the storage
// location around a section's folder runs under: the storage location the
// folder was put into, or the folder it lies in.
func locationErasureID(raw, destID string) string {
	if destID != "" {
		return destID
	}
	return repoLocationKey(scrubRepoLocation(pathRoot(raw)))
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
// row and the end of the copy there, in one transaction. A path set in the
// meantime stays, since the step was planned for the one before it.
func (s *Service) switchOwnFolder(step ownFolderStep) (store.Settings, error) {
	err := s.store.MoveOwnFolder(step.folder, func(st *store.Settings) error {
		if strings.TrimSpace(domainPathRaw(step.Domain, *st)) != step.from {
			return errOwnFolderChanged
		}
		setDomainPath(st, step.Domain, step.path)
		if step.primaryCopy {
			setOffsiteRepoInSettings(st, step.Domain, "")
		}
		if step.copyKeep != nil {
			if keep := widerKeep(localKeep(*st, step.Domain), *step.copyKeep); keep != localKeep(*st, step.Domain) {
				own := st.OwnRetention()
				own[step.Domain] = keep
				st.SetOwnRetention(own)
			}
		}
		return nil
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
// first. moved runs after each switch. Nothing is switched unless every
// section of the request can go: none is moving already, no backup holds one
// that switches at once, and no place is being emptied.
func (s *Service) MoveOwnFolders(ctx context.Context, req ownFolderRequest, moved func(store.Settings)) ([]ownFolderStep, error) {
	steps, err := s.planOwnFolders(ctx, req)
	if err != nil {
		return nil, err
	}
	var now, later []ownFolderStep
	for _, step := range steps {
		switch {
		case step.Same:
		case req.CopyOver && step.Pending > 0:
			later = append(later, step)
		default:
			now = append(now, step)
		}
	}
	claims, err := s.claimOwnFolders(ctx, append(slices.Clone(now), later...))
	if err != nil {
		return nil, err
	}
	var unlocks []func()
	defer func() {
		for _, unlock := range unlocks {
			unlock()
		}
	}()
	for _, step := range now {
		unlock, ok := s.tryLockDomainFor(step.Domain, "replicate")
		if !ok {
			for _, c := range claims {
				s.releaseOwnFolder(c)
			}
			return nil, fmt.Errorf("%s: %w", step.Domain, errDomainBusy)
		}
		unlocks = append(unlocks, unlock)
	}
	if req.CopyOver {
		for _, step := range now {
			if err := s.nothingNewToCopy(ctx, step); err != nil {
				for _, c := range claims {
					s.releaseOwnFolder(c)
				}
				return nil, fmt.Errorf("%s: %w", step.Domain, err)
			}
		}
	}
	for i, step := range now {
		settings, err := s.switchOwnFolder(step)
		if err != nil {
			for _, c := range claims[i:] {
				s.releaseOwnFolder(c)
			}
			return nil, fmt.Errorf("%s: %w", step.Domain, err)
		}
		s.releaseOwnFolder(claims[i])
		moved(settings)
	}
	for _, c := range claims[len(now):] {
		s.startOwnFolderCopy(c, moved)
	}
	return steps, nil
}

// nothingNewToCopy reads the old own folder of a step planned with nothing to
// copy once more, under the section's lock: a backup that ran while the
// request was planned wrote there, and a switch would leave it behind.
func (s *Service) nothingNewToCopy(ctx context.Context, step ownFolderStep) error {
	if step.Same {
		return nil
	}
	opens, err := s.oldOwnFolderOpens(ctx, step.plan)
	if err != nil {
		return fmt.Errorf("%w: %w", errOwnFolderUnreadable, err)
	}
	if !opens {
		return nil
	}
	left, _, err := s.pendingMove(ctx, step.plan)
	if err != nil {
		return fmt.Errorf("%w: %w", errOwnFolderUnreadable, err)
	}
	if len(left) > 0 {
		return errOwnFolderNewBackup
	}
	return nil
}

// ownFolderClaim holds a section's own folder for one move until it is done.
type ownFolderClaim struct {
	step   ownFolderStep
	ctx    context.Context
	cancel context.CancelFunc
}

// claimOwnFolders registers a move for each step under its key and the
// places it reads and writes, or none when one of them cannot start.
func (s *Service) claimOwnFolders(ctx context.Context, steps []ownFolderStep) ([]ownFolderClaim, error) {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	for _, step := range steps {
		if _, running := s.runCancels[step.Key]; running {
			return nil, fmt.Errorf("%s: %w", step.Domain, errMoveRunning)
		}
		for _, id := range step.places {
			if _, erasing := s.runCancels["erase:"+id]; erasing && id != "" {
				return nil, fmt.Errorf("%s: %w", step.Domain, errMoveErasing)
			}
		}
	}
	if s.moving == nil {
		s.moving = map[string][2]string{}
	}
	bctx := context.WithoutCancel(ctx)
	claims := make([]ownFolderClaim, 0, len(steps))
	for _, step := range steps {
		rctx, cancel := context.WithTimeout(bctx, 7*24*time.Hour)
		s.runCancels[step.Key] = cancel
		s.moving[step.Key] = step.places
		claims = append(claims, ownFolderClaim{step: step, ctx: rctx, cancel: cancel})
	}
	return claims, nil
}

func (s *Service) releaseOwnFolder(c ownFolderClaim) {
	c.cancel()
	s.cancelMu.Lock()
	delete(s.runCancels, c.step.Key)
	delete(s.moving, c.step.Key)
	s.cancelMu.Unlock()
}

func (s *Service) startOwnFolderCopy(c ownFolderClaim, moved func(store.Settings)) {
	step, rctx := c.step, c.ctx
	bctx := context.WithoutCancel(rctx)
	go func() {
		var runID string
		defer s.recoverOperation("own folder "+step.Domain, nil, func(msg string) { s.finishMoveRun(runID, errors.New(msg)) })
		defer s.releaseOwnFolder(c)
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
