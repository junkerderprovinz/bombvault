package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// An emergency brings the backups of an install that is gone to this one. The
// person shows the key of those backups, with the recovery kit, through the
// group or typed. It is held in memory while they look at what it opens, and
// kept, sealed, only once they adopt it.

// emergencyTTL is how long the keys of an emergency stay in memory.
const emergencyTTL = 30 * time.Minute

// The kinds the runs of store.KeyTargetID have on the server that takes a key
// over.
const (
	keyEventRead    = "keyread"
	keyEventAdopt   = "keyadopt"
	keyEventUndo    = "keyundo"
	keyEventSignIns = "keysignins"
)

// What a place an emergency looked into answered.
const (
	placeReadable    = "readable"
	placeMissing     = "missing"
	placeWrongKey    = "wrong-key"
	placeUnreachable = "unreachable"
	// placeNeedsSignIn is a remote place the emergency has no sign-in for. It
	// comes back with the settings backup.
	placeNeedsSignIn = "sign-in"
)

var (
	errNoEmergency      = errors.New("the emergency has ended or was never started. Show the key again")
	errAdoptOwnBackups  = errors.New("this server has backups of its own, and taking over another key would leave them unreadable. To read the old backups without taking them over, open their folder under Settings, Restore")
	errAdoptSameKey     = errors.New("this server already backs up under that key, so there is nothing to take over")
	errAdoptNothingRead = errors.New("the key opens none of the places it was tried on, so it was not taken over")
	errAdoptInUse       = errors.New("backups were written under the key that was taken over, so it stays")
	errEmergencyKey     = errors.New("the key has to be the 64 characters of an APP_KEY")
)

var emergencyCodes = []struct {
	err  error
	code string
}{
	{errNoEmergency, "emergency-ended"},
	{errAdoptOwnBackups, "adopt-own-backups"},
	{errAdoptSameKey, "adopt-same-key"},
	{errAdoptNothingRead, "adopt-nothing-readable"},
	{errAdoptInUse, "adopt-in-use"},
	{errEmergencyKey, "emergency-key"},
	{errBackupKeyLocked, "backup-key-locked"},
}

// emergencyPlace is one repository an emergency looked into.
type emergencyPlace struct {
	kitLocation
	State string `json:"state"`
}

// emergencySession is the emergency in progress. It lives in memory and ends
// with the process, on finish, on cancel and when its time is up.
type emergencySession struct {
	source                string
	backupKey, sealingKey string
	origin                kitOrigin
	encrypted             bool
	creds                 CloudCreds
	rcloneConf            string
	places                []emergencyPlace
	expires               time.Time
	timer                 *time.Timer
}

// emergencyView is what the routes of an emergency answer with. It names no
// key.
type emergencyView struct {
	// BackupKey is the state GET /api/status has under the same name.
	BackupKey string `json:"backupKey"`
	// Source and AdoptedAt describe the key that was taken over.
	Source    string `json:"source,omitempty"`
	AdoptedAt int64  `json:"adoptedAt,omitempty"`
	// Undoable says DELETE /api/emergency would put the server back under its
	// own key: one was taken over and no backup was written under it.
	Undoable bool `json:"undoable"`
	// Lost names what the old install had stored sealed that the settings
	// restore could not bring over, until POST /api/emergency/finish.
	Lost    *sealedLoss           `json:"lost,omitempty"`
	Session *emergencySessionView `json:"session,omitempty"`
}

// kitOrigin says whose kit an emergency reads and how old it is, as far as
// the kit or the deposit it came out of says. A typed key has no origin.
type kitOrigin struct {
	// OwnerID is the install a deposit was sealed by, empty for a kit that
	// was uploaded.
	OwnerID string `json:"ownerId,omitempty"`
	// Server is the name the install gave itself.
	Server string `json:"server,omitempty"`
	// WrittenAt is when the kit was written or the deposit sealed, in Unix
	// seconds, zero when the kit does not say.
	WrittenAt int64 `json:"writtenAt,omitempty"`
	// NewerWith names a member that lists a newer deposit of the same
	// install than the one that was opened.
	NewerWith string `json:"newerWith,omitempty"`
}

type emergencySessionView struct {
	Source string `json:"source"`
	// Kit is shown to the person before anything is taken over.
	Kit       kitOrigin        `json:"kit"`
	Encrypted bool             `json:"encrypted"`
	Places    []emergencyPlace `json:"places"`
	ExpiresAt int64            `json:"expiresAt"`
	// CloudSignIn and RcloneConfig say the kit carries the shared sign-in of
	// its remote repositories, which POST /api/emergency/sign-ins stores.
	CloudSignIn  bool `json:"cloudSignIn"`
	RcloneConfig bool `json:"rcloneConfig"`
	// Refusal is the code adopting would be refused with, empty when it would
	// go through.
	Refusal string `json:"refusal,omitempty"`
}

func emergencyCode(err error) string {
	for _, c := range emergencyCodes {
		if errors.Is(err, c.err) {
			return c.code
		}
	}
	return ""
}

// keyEvent writes one line about a key into the activity log, as a run of its
// own kind on a target of its own, and into the server log. It carries what
// happened and why it failed, never a key.
func (s *Service) keyEvent(ctx context.Context, kind string, err error) {
	status, why := "success", ""
	if err != nil {
		status, why = "failed", truncateRunErr(err)
		log.Printf("api: %s refused: %s", kind, scrubError(err)) //nolint:gosec // G706: kind is a fixed literal and the error scrubbed
	} else {
		log.Printf("api: %s", kind) //nolint:gosec // G706: kind is a fixed literal
	}
	if s.store == nil {
		return
	}
	id, rErr := s.startRun(ctx, store.KeyTargetID, kind)
	if rErr == nil {
		rErr = s.store.FinishRun(id, status, "", 0, why)
	}
	if rErr != nil {
		log.Printf("api: %s is missing from the activity log: %v", kind, rErr) //nolint:gosec // G706: kind is a fixed literal
	}
}

// probePlaces tries the key on every place and says what each answered. A
// local place is looked for below this server's mount under the folder it had
// on the old one.
func (s *Service) probePlaces(ctx context.Context, sess *emergencySession, typed bool) {
	for i, p := range sess.places {
		sess.places[i].State = s.probePlace(ctx, sess, p.kitLocation, typed)
	}
}

func (s *Service) probePlace(ctx context.Context, sess *emergencySession, loc kitLocation, typed bool) string {
	repo := loc.Location
	if !restic.IsRemoteRepo(repo) {
		if loc.Folder == "" {
			return placeMissing
		}
		resolved, err := s.resolveRepo(loc.Folder)
		if err != nil || localRepoMissing(resolved) {
			return placeMissing
		}
		repo = resolved
	} else if isRcloneLocation(repo) || (len(cloudEnv(sess.creds)) == 0 && !strings.Contains(repo, "@")) {
		return placeNeedsSignIn
	}
	mode := restic.Mode{
		Encrypted: sess.encrypted, Password: restickey.Derive(sess.backupKey),
		Env: cloudEnv(sess.creds), NoLock: true, NoAmbientCreds: true,
	}
	modes := []restic.Mode{mode}
	if typed {
		// A typed key says nothing about encryption, so both kinds are tried.
		plain := mode
		plain.Encrypted, plain.Password = false, ""
		mode.Encrypted = true
		modes = []restic.Mode{mode, plain}
	}
	var last error
	for _, m := range modes {
		pctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		last = s.engine.RepoOpensErr(pctx, repo, m)
		cancel()
		if last == nil {
			if typed {
				sess.encrypted = sess.encrypted || m.Encrypted
			}
			return placeReadable
		}
	}
	if isWrongKey(last) {
		return placeWrongKey
	}
	return placeUnreachable
}

// beginEmergency replaces the emergency in progress with sess.
func (s *Service) beginEmergency(sess *emergencySession) {
	s.emergencyMu.Lock()
	defer s.emergencyMu.Unlock()
	if s.emergency != nil {
		s.emergency.timer.Stop()
	}
	sess.expires = time.Now().Add(emergencyTTL)
	sess.timer = time.AfterFunc(emergencyTTL, func() { s.endEmergency(sess) })
	s.emergency = sess
}

// endEmergency drops the keys of sess, or of whatever emergency is in progress
// when sess is nil.
func (s *Service) endEmergency(sess *emergencySession) {
	s.emergencyMu.Lock()
	defer s.emergencyMu.Unlock()
	if s.emergency == nil || (sess != nil && s.emergency != sess) {
		return
	}
	s.emergency.timer.Stop()
	s.emergency = nil
}

func (s *Service) currentEmergency() (*emergencySession, error) {
	s.emergencyMu.Lock()
	defer s.emergencyMu.Unlock()
	if s.emergency == nil || time.Now().After(s.emergency.expires) {
		return nil, errNoEmergency
	}
	return s.emergency, nil
}

// adoptionRefusal says why the key of sess may not be taken over, nil when it
// may.
func (s *Service) adoptionRefusal(sess *emergencySession) error {
	now := s.currentBackupKey()
	if now.state != backupKeyLocked && sess.backupKey == now.key {
		return errAdoptSameKey
	}
	if !slices.ContainsFunc(sess.places, func(p emergencyPlace) bool { return p.State == placeReadable }) {
		return errAdoptNothingRead
	}
	// A locked key is the one case where the backups this server has are the
	// ones the new key is meant to open again.
	if now.state == backupKeyLocked {
		return nil
	}
	written, err := s.store.BackupsWrittenSince(now.adoptedAt)
	if err != nil {
		return err
	}
	if written {
		return errAdoptOwnBackups
	}
	return nil
}

// EmergencyState describes the backup key and the emergency in progress.
func (s *Service) EmergencyState() (emergencyView, error) {
	now := s.currentBackupKey()
	view := emergencyView{BackupKey: now.state}
	if now.state != backupKeyOwn {
		view.Source, view.AdoptedAt = now.source, now.adoptedAt
		written, err := s.store.BackupsWrittenSince(now.adoptedAt)
		if err != nil {
			return emergencyView{}, err
		}
		view.Undoable = !written
		adopted, _, err := s.store.AdoptedKey()
		if err != nil {
			return emergencyView{}, err
		}
		view.Lost = readSealedLoss(adopted.Lost)
	}
	sess, err := s.currentEmergency()
	if err != nil {
		return view, nil //nolint:nilerr // no emergency in progress is an answer
	}
	view.Session = &emergencySessionView{
		Source: sess.source, Kit: sess.origin, Encrypted: sess.encrypted, Places: sess.places,
		ExpiresAt: sess.expires.Unix(), Refusal: emergencyCode(s.adoptionRefusal(sess)),
		CloudSignIn: sess.creds != (CloudCreds{}), RcloneConfig: strings.TrimSpace(sess.rcloneConf) != "",
	}
	return view, nil
}

// emergencyRead is what the person shows to start an emergency: a recovery
// kit, with the age identity that opens a sealed one, or a typed key with the
// places to try it on and their sign-in.
type emergencyRead struct {
	Kit      string      `json:"kit"`
	Identity string      `json:"identity"`
	Key      string      `json:"key"`
	Places   []string    `json:"places"`
	Creds    *CloudCreds `json:"creds"`
}

// ReadEmergency starts an emergency from what the person showed and tries the
// key on the places it names. Nothing is stored.
func (s *Service) ReadEmergency(ctx context.Context, in emergencyRead) error {
	sess := &emergencySession{}
	typed := in.Kit == ""
	if typed {
		if !foreignKeyRe.MatchString(in.Key) {
			return errEmergencyKey
		}
		sess.source, sess.backupKey, sess.sealingKey = store.KeyFromTyped, in.Key, in.Key
		if in.Creds != nil {
			sess.creds = *in.Creds
		}
		for _, place := range in.Places {
			if place = strings.TrimSpace(place); place != "" {
				loc := kitLocation{Kind: kitLocal, Location: place, Folder: place}
				if restic.IsRemoteRepo(place) {
					loc.Kind, loc.Folder = kitOffsite, ""
				}
				sess.places = append(sess.places, emergencyPlace{kitLocation: loc})
			}
		}
	} else {
		kit, err := uploadedKit(in.Kit, in.Identity)
		if err != nil {
			return err
		}
		return s.beginEmergencyFromKit(ctx, kit, store.KeyFromKit, kitOrigin{})
	}
	if sess.places == nil {
		sess.places = []emergencyPlace{}
	}
	s.probePlaces(ctx, sess, typed)
	s.beginEmergency(sess)
	return nil
}

// beginEmergencyFromKit starts an emergency from a recovery kit, whether it
// was uploaded or came out of a deposit. origin carries what only a deposit
// says; the name and the time come from the kit.
func (s *Service) beginEmergencyFromKit(ctx context.Context, kit recoveryKit, source string, origin kitOrigin) error {
	if !kit.HasKey {
		return errKitHasNoKey
	}
	origin.Server, origin.WrittenAt = kit.Server, kit.WrittenAt
	sess := &emergencySession{
		source: source, backupKey: kit.appKey, sealingKey: kit.sealingKey, origin: origin,
		encrypted: kit.Encrypted, creds: kit.creds, rcloneConf: kit.rcloneConf,
		places: []emergencyPlace{},
	}
	for _, loc := range kit.Locations {
		sess.places = append(sess.places, emergencyPlace{kitLocation: loc})
	}
	s.probePlaces(ctx, sess, false)
	s.beginEmergency(sess)
	return nil
}

// AdoptEmergencyKey makes the key of the emergency in progress the key of
// this server's backups. It is the first thing an emergency stores: the key,
// sealed under APP_KEY, and whether the repositories carry a password.
func (s *Service) AdoptEmergencyKey() error {
	sess, err := s.currentEmergency()
	if err != nil {
		return err
	}
	if err := s.adoptionRefusal(sess); err != nil {
		return err
	}
	enc, err := secret.Encrypt(s.cfg.AppKey, []byte(sess.backupKey))
	if err != nil {
		return err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return err
	}
	if err := s.store.SetAdoptedKey(enc, sess.source, settings.EncryptionEnabled); err != nil {
		return err
	}
	s.forgetBackupKey()
	_, err = s.store.MutateSettings(func(settings *store.Settings) error {
		settings.EncryptionEnabled = sess.encrypted
		return nil
	})
	return err
}

// UndoAdoption puts the server back under its own key and ends the emergency.
// A key that backups were written under stays, since those would be lost.
func (s *Service) UndoAdoption() error {
	now := s.currentBackupKey()
	if now.state != backupKeyOwn {
		written, err := s.store.BackupsWrittenSince(now.adoptedAt)
		if err != nil {
			return err
		}
		if written {
			return errAdoptInUse
		}
		adopted, _, err := s.store.AdoptedKey()
		if err != nil {
			return err
		}
		if err := s.store.ClearAdoptedKey(); err != nil {
			return err
		}
		s.forgetBackupKey()
		if before := adopted.EncryptionBefore; before != nil {
			if _, err := s.store.MutateSettings(func(settings *store.Settings) error {
				settings.EncryptionEnabled = *before
				return nil
			}); err != nil {
				return err
			}
		}
	}
	s.endEmergency(nil)
	return nil
}

// StoreEmergencySignIns saves the shared sign-ins the kit of the emergency
// carries, so the places it names can be reached before any settings come
// back. They are sealed under APP_KEY like every sign-in typed in by hand.
func (s *Service) StoreEmergencySignIns() error {
	sess, err := s.currentEmergency()
	if err != nil {
		return err
	}
	if sess.creds != (CloudCreds{}) {
		if err := s.SetCloudCreds(sess.creds); err != nil {
			return err
		}
	}
	if strings.TrimSpace(sess.rcloneConf) != "" {
		return s.SetRcloneConf(sess.rcloneConf)
	}
	return nil
}

func (h *Handler) answerEmergency(w http.ResponseWriter, ctx context.Context, kind string, err error) {
	if kind != "" {
		h.svc.keyEvent(ctx, kind, err)
	}
	if err != nil {
		if code := emergencyCode(err); code != "" {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error(), "code": code})
			return
		}
		kitFail(w, err)
		return
	}
	view, err := h.svc.EmergencyState()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"emergency": view}))
}

// handleEmergencyState serves GET /api/emergency.
func (h *Handler) handleEmergencyState(w http.ResponseWriter, r *http.Request) {
	h.answerEmergency(w, r.Context(), "", nil)
}

// handleEmergencyRead serves POST /api/emergency/read with the body
// {kit, identity} or {key, places, creds}.
func (h *Handler) handleEmergencyRead(w http.ResponseWriter, r *http.Request) {
	var body emergencyRead
	if !decodeBody(w, r, &body) {
		return
	}
	h.answerEmergency(w, r.Context(), keyEventRead, h.svc.ReadEmergency(r.Context(), body))
}

// handleEmergencyAdopt serves POST /api/emergency/adopt.
func (h *Handler) handleEmergencyAdopt(w http.ResponseWriter, r *http.Request) {
	if !crossOriginGuard(w, r) || !h.requireSignIn(w, r, "taking over a backup key") {
		return
	}
	h.answerEmergency(w, r.Context(), keyEventAdopt, h.svc.AdoptEmergencyKey())
}

// handleEmergencyFinish serves POST /api/emergency/finish: the keys of the
// emergency leave memory and what was taken over stays.
func (h *Handler) handleEmergencyFinish(w http.ResponseWriter, r *http.Request) {
	if !crossOriginGuard(w, r) {
		return
	}
	h.svc.endEmergency(nil)
	h.answerEmergency(w, r.Context(), "", h.store.ClearAdoptedLoss())
}

// handleEmergencySignIns serves POST /api/emergency/sign-ins.
func (h *Handler) handleEmergencySignIns(w http.ResponseWriter, r *http.Request) {
	if !crossOriginGuard(w, r) || !h.requireSignIn(w, r, "storing the sign-ins of a recovery kit") {
		return
	}
	h.answerEmergency(w, r.Context(), keyEventSignIns, h.svc.StoreEmergencySignIns())
}

// handleEmergencyUndo serves DELETE /api/emergency.
func (h *Handler) handleEmergencyUndo(w http.ResponseWriter, r *http.Request) {
	if !crossOriginGuard(w, r) || !h.requireSignIn(w, r, "putting the backup key back") {
		return
	}
	h.answerEmergency(w, r.Context(), keyEventUndo, h.svc.UndoAdoption())
}
