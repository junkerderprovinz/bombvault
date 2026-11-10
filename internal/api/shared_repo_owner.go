package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/platform"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/template"
)

// The shared repository's server is a rest-server of its own beside the
// receiver. It cannot run with --private-repos, which gives every login a
// folder only it reaches, so its data folder holds the one repository and
// nothing else.
const (
	sharedServerName    = "rest-server-shared"
	sharedServerPort    = 8001
	sharedServerOptions = "--append-only --htpasswd-file /data/.htpasswd"
	// sharedRepoFolder is the repository's folder under the data folder,
	// beside the server's login file.
	sharedRepoFolder = "shared"
	// sharedOnceUser labels the one-time key a member opens the repository
	// with until it added its own.
	sharedOnceUser = "bombvault-join"
)

// sharedHostRe is the shape restickey.Host gives a server's host.
var sharedHostRe = regexp.MustCompile(`^bv-[0-9a-f]{12}$`)

var (
	errSharedNeedsReceiver   = errors.New("set up the receiver first: members reach the shared repository at its address")
	errSharedNeedsEncryption = errors.New("a shared repository gives every server a key of its own, which needs encryption; switch it on under Settings first")
	errSharedNeedsGroup      = errors.New("a shared repository is for the servers of a group; create or join one under Settings, Instances first")
	errSharedExists          = errors.New("this instance runs a shared repository already")
	errNoSharedRepo          = errors.New("no shared repository is set up on this instance")
	errSharedNoJoin          = errors.New("that instance has not asked to join the shared repository")
	errSharedHostTaken       = errors.New("another server already writes into the shared repository under this server's host. Two servers with the same APP_KEY share a host, so give one of them its own key first")
	errSharedHostForm        = errors.New("the asking instance writes no host of its own yet; update it to the current release first")
)

// sharedEngine is what a shared repository needs of restic beyond the
// engine every repository uses.
type sharedEngine interface {
	AddKey(ctx context.Context, repo string, m restic.Mode, password, host, user string) error
	Keys(ctx context.Context, repo string, m restic.Mode) ([]restic.Key, error)
	RemoveKey(ctx context.Context, repo string, m restic.Mode, id string) error
	SnapshotsOfEveryHost(ctx context.Context, repo string, m restic.Mode) ([]restic.Snapshot, error)
}

func (s *Service) sharedEngine() (sharedEngine, error) {
	e, ok := s.engine.(sharedEngine)
	if !ok {
		return nil, errors.New("this build of restic cannot hand out keys")
	}
	return e, nil
}

// sharedOwnerMode opens the shared repository as its owner, with this
// server's own password, which the repository was created with.
func (s *Service) sharedOwnerMode() restic.Mode {
	return restic.Mode{Encrypted: true, Password: s.backupPassword(), Host: s.resticHost(), Sharing: restic.SharingOwner}
}

// sharedServerView is the shared repository as the owner's card shows it,
// without any password.
type sharedServerView struct {
	ContainerName string `json:"containerName"`
	Folder        string `json:"folder"`
	HostPath      string `json:"hostPath"`
	Port          int    `json:"port"`
	User          string `json:"user"`
	Repo          string `json:"repo"`
	// URL is where members reach the repository, empty while NeedsHost.
	URL         string `json:"url"`
	NeedsHost   bool   `json:"needsHost"`
	CreatedAt   int64  `json:"createdAt"`
	Present     bool   `json:"present"`
	Check       string `json:"check"`
	CheckDetail string `json:"checkDetail"`
	CheckedAt   int64  `json:"checkedAt"`
	// Members are the servers that write into it, Joins the asks a person
	// here has not answered yet.
	Members []sharedMemberView `json:"members"`
	Joins   []sharedJoinView   `json:"joins"`
	// CopiesIn says this server copies its own backups in as well.
	CopiesIn bool `json:"copiesIn"`
	// ID names the repository in the routes of its upkeep.
	ID string `json:"id"`
	// Former are the servers that left, whose backups are still there.
	Former []sharedFormerView `json:"former"`
}

// sharedFormerView is a server that left, as the owner's card lists it.
// Snapshots is how many it left there, as far as restic answered.
type sharedFormerView struct {
	Host        string `json:"host"`
	Name        string `json:"name"`
	LeftAt      int64  `json:"leftAt"`
	Snapshots   int    `json:"snapshots"`
	ForgetError string `json:"forgetError,omitempty"`
	Forgetting  bool   `json:"forgetting"`
}

// sharedMemberView is one server in a shared repository.
type sharedMemberView struct {
	MemberID string `json:"memberId"`
	Name     string `json:"name"`
	Host     string `json:"host"`
	// Keys are the short ids of the keys added under its host, and
	// LastSnapshotAt when its newest snapshot there was taken. The owner
	// alone fills them in.
	Keys           []string `json:"keys,omitempty"`
	LastSnapshotAt int64    `json:"lastSnapshotAt,omitempty"`
	// Self is the owner itself, which copies its own backups in.
	Self bool `json:"self,omitempty"`
}

// sharedJoinView is an ask to join as the owner's card lists it.
type sharedJoinView struct {
	MemberID string `json:"memberId"`
	Name     string `json:"name"`
	Host     string `json:"host"`
	State    string `json:"state"`
	AskedAt  int64  `json:"askedAt"`
	// ExpiresAt is when an allowed join loses its one-time key and login.
	ExpiresAt int64 `json:"expiresAt,omitempty"`
}

// sharedContainerSpec is the receiver's recipe under the shared server's name
// and options.
func sharedContainerSpec(hostPath string, port int, unraid bool) model.Inspect {
	spec := receiverContainerSpec(hostPath, port, unraid)
	spec.Name = sharedServerName
	spec.Config.Env = []string{"OPTIONS=" + sharedServerOptions}
	return spec
}

func sharedServerTemplate(ss store.SharedRepoServer) string {
	note := "  BombVault set up this append-only rest-server for the shared repository of\n  its group and keeps its logins in " +
		xmlCommentSafe(ss.HostPath+"/"+htpasswdName) + ".\n  Every login reaches the whole data folder, so keep it for this one repository."
	return restServerTemplate(ss.ContainerName, ss.Port, ss.HostPath, sharedServerOptions, note)
}

// SetUpSharedRepo creates the repository the servers of the group copy into,
// with this server's password, and starts the rest-server members reach it
// through. It sits on the host of the receiver, whose address members know.
func (s *Service) SetUpSharedRepo(ctx context.Context, in receiverServerInput) (sharedServerView, error) {
	s.sharedMu.Lock()
	defer s.sharedMu.Unlock()
	settings, err := s.store.GetSettings()
	if err != nil {
		return sharedServerView{}, err
	}
	if !settings.EncryptionEnabled {
		return sharedServerView{}, errSharedNeedsEncryption
	}
	g, err := s.store.GetGroupState()
	if err != nil {
		return sharedServerView{}, err
	}
	if g.InstanceID == "" || !s.pairing().Active() {
		return sharedServerView{}, errSharedNeedsGroup
	}
	if _, ok, err := s.store.GetReceiverServer(); err != nil {
		return sharedServerView{}, err
	} else if !ok {
		return sharedServerView{}, errSharedNeedsReceiver
	}
	if in.Port == 0 {
		in.Port = sharedServerPort
	}
	in, hostPath, err := s.checkReceiverInput(in)
	if err != nil {
		return sharedServerView{}, err
	}
	if err := s.refuseSecondSharedServer(ctx, in.Port); err != nil {
		return sharedServerView{}, err
	}

	ss := store.SharedRepoServer{
		ContainerName: sharedServerName, Folder: in.Folder, HostPath: hostPath, Port: in.Port,
		User: defaultReceiverUser(instanceDisplayName(settings)), Repo: path.Join(in.Folder, sharedRepoFolder),
	}
	password, enc, err := s.addHtpasswdUser(ss.Folder, ss.User)
	if err != nil {
		return sharedServerView{}, err
	}
	ss.PasswordEnc = enc
	repo, err := s.resolveRepo(ss.Repo)
	if err != nil {
		return sharedServerView{}, err
	}
	// A folder that held this server's shared repository before keeps it.
	if mode := s.sharedOwnerMode(); s.engine.RepoOpensErr(ctx, repo, mode) != nil {
		if err := s.engine.Init(ctx, repo, mode); err != nil {
			return sharedServerView{}, fmt.Errorf("create the repository: %w", err)
		}
	}

	if id, err := s.docker.ImageID(ctx, restServerImage); err != nil {
		return sharedServerView{}, err
	} else if id == "" {
		if err := s.docker.Pull(ctx, restServerImage); err != nil {
			return sharedServerView{}, fmt.Errorf("download %s: %w", restServerImage, err)
		}
	}
	unraid := s.platformFn().Kind() == platform.KindUnraid
	if err := s.docker.CreateAndStart(ctx, sharedContainerSpec(hostPath, in.Port, unraid), true); err != nil {
		return sharedServerView{}, fmt.Errorf("create the %s container: %w", sharedServerName, err)
	}
	if err := s.store.SaveSharedRepoServer(ss); err != nil {
		return sharedServerView{}, fmt.Errorf("the container runs, but BombVault could not note it down: %w", err)
	}
	sr, _, err := s.store.SharedRepoAt(ss.Repo)
	if err != nil {
		return sharedServerView{}, err
	}
	sr.Repo, sr.Owner = ss.Repo, g.InstanceID
	if _, err := s.store.PutSharedRepo(sr); err != nil {
		return sharedServerView{}, err
	}
	if unraid {
		if err := template.Write(s.cfg.FlashTemplatesDir, ss.ContainerName, sharedServerTemplate(ss)); err != nil {
			log.Printf("shared repository: write the Unraid template: %v", err)
		}
	}
	s.checkSharedServer(ctx, ss, password)
	return s.sharedServerView(ctx)
}

// refuseSecondSharedServer refuses a shared repository while one runs here, a
// container has its server's name or another container its port. A record
// whose container is gone makes way for the new one.
func (s *Service) refuseSecondSharedServer(ctx context.Context, port int) error {
	if old, ok, err := s.store.GetSharedRepoServer(); err != nil {
		return err
	} else if ok {
		live, err := s.docker.InspectName(ctx, old.ContainerName)
		if err != nil {
			return err
		}
		if live != "" {
			return errSharedExists
		}
	}
	if live, err := s.docker.InspectName(ctx, sharedServerName); err != nil {
		return err
	} else if live != "" {
		return fmt.Errorf("a container called %s exists already, and BombVault leaves it alone. Remove or rename it in the Docker tab first", sharedServerName)
	}
	allocs, err := s.docker.Allocations(ctx)
	if err != nil {
		return err
	}
	want := strconv.Itoa(port) + "/tcp"
	for _, a := range allocs {
		if slices.Contains(a.HostPorts, want) {
			return fmt.Errorf("port %d is taken by %s; choose another one", port, a.Name)
		}
	}
	return nil
}

// sharedAddress is where members reach the shared repository's server: the
// receiver's host with the shared server's port. NeedsHost while the
// receiver knows no address it can hand out.
func (s *Service) sharedAddress(ctx context.Context, ss store.SharedRepoServer) peerReceiver {
	rs, ok, err := s.store.GetReceiverServer()
	if err != nil || !ok {
		return peerReceiver{Port: ss.Port, NeedsHost: true}
	}
	host := s.receiverHost(ctx, rs)
	return peerReceiver{Host: host, HostFromGroup: rs.Host == "" && host != "", Port: ss.Port, NeedsHost: host == ""}
}

// checkSharedServer runs the append-only check against the shared server at
// the receiver's host and at the container's own address.
func (s *Service) checkSharedServer(ctx context.Context, ss store.SharedRepoServer, password string) {
	var bases []string
	if a := s.sharedAddress(ctx, ss); a.Host != "" {
		bases = append(bases, restBase(a.Host, ss.Port))
	}
	if allocs, err := s.docker.Allocations(ctx); err == nil {
		for _, a := range allocs {
			if a.Name == ss.ContainerName && a.IPv4 != "" {
				bases = append(bases, restBase(a.IPv4, restServerContainerPort))
			}
		}
	}
	verdict, detail := receiverInconclusive, "BombVault cannot reach the server from here"
	deadline := time.Now().Add(receiverStartWait)
probing:
	for len(bases) > 0 {
		for _, b := range bases {
			v, err := probeAppendOnly(ctx, s.restHTTPClient(), b+"/"+sharedRepoFolder, ss.User, password)
			if err != nil {
				detail = scrubError(err)
				continue
			}
			verdict, detail = receiverUnprotected, v.Detail
			if v.Protected {
				verdict = receiverProtected
			}
			break probing
		}
		if time.Now().After(deadline) {
			break
		}
		select {
		case <-ctx.Done():
			break probing
		case <-time.After(receiverRetry):
		}
	}
	if err := s.store.RecordSharedRepoServerCheck(verdict, detail); err != nil {
		log.Printf("shared repository: record the append-only check: %v", err)
	}
}

// CheckSharedServer runs the append-only check of the shared server again.
func (s *Service) CheckSharedServer(ctx context.Context) (sharedServerView, error) {
	ss, ok, err := s.store.GetSharedRepoServer()
	if err != nil {
		return sharedServerView{}, err
	}
	if !ok {
		return sharedServerView{}, errNoSharedRepo
	}
	password, err := secret.Decrypt(s.cfg.AppKey, ss.PasswordEnc)
	if err != nil {
		return sharedServerView{}, errors.New("the shared server's stored password could not be opened")
	}
	s.checkSharedServer(ctx, ss, string(password))
	return s.sharedServerView(ctx)
}

// sharedServerView reads the shared repository back as the owner's card shows
// it. The keys and the newest snapshot of each member come from restic
// without a lock, and are left out when restic does not answer in time.
func (s *Service) sharedServerView(ctx context.Context) (sharedServerView, error) {
	ss, ok, err := s.store.GetSharedRepoServer()
	if err != nil {
		return sharedServerView{}, err
	}
	if !ok {
		return sharedServerView{}, errNoSharedRepo
	}
	live, err := s.docker.InspectName(ctx, ss.ContainerName)
	if err != nil {
		log.Printf("shared repository: look up container %q: %v", ss.ContainerName, err)
	}
	v := sharedServerView{
		ContainerName: ss.ContainerName, Folder: ss.Folder, HostPath: ss.HostPath, Port: ss.Port, User: ss.User, Repo: ss.Repo,
		CreatedAt: ss.CreatedAt, Present: live != "" || err != nil, Check: ss.Check, CheckDetail: ss.CheckDetail, CheckedAt: ss.CheckedAt,
		Members: []sharedMemberView{}, Joins: []sharedJoinView{}, Former: []sharedFormerView{},
	}
	if a := s.sharedAddress(ctx, ss); a.Host != "" {
		v.URL = restBase(a.Host, ss.Port) + "/" + sharedRepoFolder
	}
	v.NeedsHost = v.URL == ""
	sr, _, err := s.store.SharedRepoAt(ss.Repo)
	if err != nil {
		return sharedServerView{}, err
	}
	v.ID = sr.ID
	keys, latest, counts := s.sharedRepoFacts(ctx, ss)
	former, err := s.store.ListSharedFormer(sr.ID)
	if err != nil {
		return sharedServerView{}, err
	}
	for _, f := range former {
		v.Former = append(v.Former, sharedFormerView{Host: f.Host, Name: f.Name, LeftAt: f.LeftAt, Snapshots: counts[f.Host],
			ForgetError: f.ForgetError, Forgetting: s.sharedUpkeep.forgets(f.Host)})
	}
	self := s.resticHost()
	for _, m := range sr.Members {
		mv := sharedMemberView{MemberID: m.Instance, Name: m.Name, Host: m.Host, LastSnapshotAt: latest[m.Host], Self: m.Host == self}
		v.CopiesIn = v.CopiesIn || mv.Self
		for _, k := range keys {
			if k.HostName == m.Host {
				mv.Keys = append(mv.Keys, shortKeyID(k.ID))
			}
		}
		v.Members = append(v.Members, mv)
	}
	joins, err := s.store.ListSharedJoins(store.SharedJoinIn)
	if err != nil {
		return sharedServerView{}, err
	}
	for _, j := range joins {
		if j.State == store.SharedJoinAsked || j.State == store.SharedJoinAllowed {
			jv := sharedJoinView{MemberID: j.MemberID, Name: j.MemberName, Host: j.Host, State: j.State, AskedAt: j.CreatedAt}
			if j.State == store.SharedJoinAllowed {
				jv.ExpiresAt = time.Unix(j.DecidedAt, 0).Add(sharedOnceFor).Unix()
			}
			v.Joins = append(v.Joins, jv)
		}
	}
	return v, nil
}

// CopyIntoOwnSharedRepo lets this server copy its own backups into the
// shared repository it holds, so it saves space there as well. It counts
// itself among the members, which its upkeep then ages like any other, and
// takes the folder up as a copy target for every section, switched off until
// a person picks the sections. It reaches the folder directly, so it needs no
// login, and nothing there keeps it from deleting: for this server it is no
// copy away from home.
func (s *Service) CopyIntoOwnSharedRepo(ctx context.Context) (sharedServerView, error) {
	s.sharedMu.Lock()
	err := s.copyIntoOwnSharedRepo()
	s.sharedMu.Unlock()
	if err != nil {
		return sharedServerView{}, err
	}
	return s.sharedServerView(ctx)
}

func (s *Service) copyIntoOwnSharedRepo() error {
	ss, ok, err := s.store.GetSharedRepoServer()
	if err != nil {
		return err
	}
	if !ok {
		return errNoSharedRepo
	}
	sr, ok, err := s.store.SharedRepoAt(ss.Repo)
	if err != nil {
		return err
	}
	if !ok || !s.ownsSharedRepo(sr) {
		return errNotSharedOwner
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return err
	}
	host := s.resticHost()
	if !slices.ContainsFunc(sr.Members, func(m store.SharedMember) bool { return m.Host == host }) {
		sr.Members = append(sr.Members, store.SharedMember{Host: host, Instance: sr.Owner, Name: instanceDisplayName(settings)})
		if _, err := s.store.PutSharedRepo(sr); err != nil {
			return err
		}
	}
	return s.takeUpSharedTargets(ss.Repo, "Shared repository", "", false)
}

// sharedFactsWait bounds what the owner's card waits for restic.
const sharedFactsWait = 15 * time.Second

// sharedRepoFacts reads the keys of the shared repository, and the newest
// snapshot and the number of snapshots under each host.
func (s *Service) sharedRepoFacts(ctx context.Context, ss store.SharedRepoServer) ([]restic.Key, map[string]int64, map[string]int) {
	latest, counts := map[string]int64{}, map[string]int{}
	eng, err := s.sharedEngine()
	if err != nil {
		return nil, latest, counts
	}
	repo, err := s.resolveRepo(ss.Repo)
	if err != nil {
		return nil, latest, counts
	}
	ctx, cancel := context.WithTimeout(ctx, sharedFactsWait)
	defer cancel()
	mode := s.sharedOwnerMode()
	keys, err := eng.Keys(ctx, repo, mode)
	if err != nil {
		log.Printf("shared repository: list its keys: %v", scrubError(err))
	}
	snaps, err := eng.SnapshotsOfEveryHost(ctx, repo, mode)
	if err != nil {
		log.Printf("shared repository: list its snapshots: %v", scrubError(err))
	}
	for _, sn := range snaps {
		counts[sn.Hostname]++
		if t := parseSnapshotTime(sn.Time); !t.IsZero() && t.Unix() > latest[sn.Hostname] {
			latest[sn.Hostname] = t.Unix()
		}
	}
	return keys, latest, counts
}

func shortKeyID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// sharedSetup is the owner's shared repository with the location and mode
// restic opens it with here.
type sharedSetup struct {
	server store.SharedRepoServer
	repo   string
	mode   restic.Mode
	eng    sharedEngine
}

func (s *Service) sharedSetupHere() (sharedSetup, error) {
	ss, ok, err := s.store.GetSharedRepoServer()
	if err != nil {
		return sharedSetup{}, err
	}
	if !ok {
		return sharedSetup{}, errNoSharedRepo
	}
	repo, err := s.resolveRepo(ss.Repo)
	if err != nil {
		return sharedSetup{}, err
	}
	eng, err := s.sharedEngine()
	if err != nil {
		return sharedSetup{}, err
	}
	return sharedSetup{server: ss, repo: repo, mode: s.sharedOwnerMode(), eng: eng}, nil
}

// sharedHostTaken reports whether a server other than memberID writes into the
// owner's shared repository under host, or this server does.
func (s *Service) sharedHostTaken(repo, memberID, host string) (bool, error) {
	if host == s.resticHost() {
		return true, nil
	}
	sr, _, err := s.store.SharedRepoAt(repo)
	if err != nil {
		return false, err
	}
	if slices.ContainsFunc(sr.Members, func(m store.SharedMember) bool { return m.Host == host && m.Instance != memberID }) {
		return true, nil
	}
	joins, err := s.store.ListSharedJoins(store.SharedJoinIn)
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(joins, func(j store.SharedJoin) bool {
		return j.Host == host && j.MemberID != memberID && j.State != store.SharedJoinRefused && j.State != store.SharedJoinExpired
	}), nil
}

// AllowSharedJoin lets a member in: a one-time key under the member's host
// opens the repository until the member added a key of its own, and a login
// of its own reaches the server. Both go to the member when it asks next,
// which it is told to do at once. The second result reports whether it heard.
func (s *Service) AllowSharedJoin(ctx context.Context, memberID string) (bool, error) {
	s.sharedMu.Lock()
	defer s.sharedMu.Unlock()
	j, ok, err := s.store.GetSharedJoin(store.SharedJoinIn, memberID)
	if err != nil {
		return false, err
	}
	if !ok || (j.State != store.SharedJoinAsked && j.State != store.SharedJoinAllowed) {
		return false, errSharedNoJoin
	}
	if j.State == store.SharedJoinAsked {
		if err := s.letIn(ctx, &j); err != nil {
			return false, err
		}
	}
	return s.tellSharedAnswer(ctx, memberID, j.State), nil
}

func (s *Service) letIn(ctx context.Context, j *store.SharedJoin) error {
	sh, err := s.sharedSetupHere()
	if err != nil {
		return err
	}
	if taken, err := s.sharedHostTaken(sh.server.Repo, j.MemberID, j.Host); err != nil {
		return err
	} else if taken {
		return errSharedHostTaken
	}
	once, err := randomHex(32)
	if err != nil {
		return err
	}
	if err := sh.eng.AddKey(ctx, sh.repo, sh.mode, once, j.Host, sharedOnceUser); err != nil {
		return fmt.Errorf("add the one-time key: %w", err)
	}
	onceMode := sh.mode
	onceMode.Password = once
	keys, err := sh.eng.Keys(ctx, sh.repo, onceMode)
	if err != nil {
		return fmt.Errorf("read the one-time key: %w", err)
	}
	i := slices.IndexFunc(keys, func(k restic.Key) bool { return k.Current })
	if i < 0 {
		return errors.New("restic did not name the one-time key")
	}
	user, err := s.freeSharedUser(sh.server, j.MemberName)
	if err != nil {
		return err
	}
	_, loginEnc, err := s.addHtpasswdUser(sh.server.Folder, user)
	if err != nil {
		return err
	}
	s.reloadContainer(ctx, sh.server.ContainerName)
	onceEnc, err := secret.Encrypt(s.cfg.AppKey, []byte(once))
	if err != nil {
		return err
	}
	j.State, j.User, j.LoginEnc, j.OnceEnc, j.OnceKey, j.DecidedAt = store.SharedJoinAllowed, user, loginEnc, onceEnc, keys[i].ID, time.Now().Unix()
	return s.store.SaveSharedJoin(*j)
}

// sharedOnceFor is how long a member may take to add its own key once let
// in. One that is up finishes within minutes of hearing the answer, and one
// that was switched off has a night and a day. The one-time key opens every
// backup in the repository, so it does not wait any longer than that.
const sharedOnceFor = 24 * time.Hour

// expireSharedJoins takes the one-time key and the login back from every
// member that was let in at least sharedOnceFor before now and never added a
// key of its own, and tells it that its join expired. It may ask again.
func (s *Service) expireSharedJoins(ctx context.Context, now time.Time) error {
	s.sharedMu.Lock()
	joins, err := s.store.ListSharedJoins(store.SharedJoinIn)
	var expired []string
	var errs []error
	for _, j := range joins {
		if err != nil || j.State != store.SharedJoinAllowed || now.Sub(time.Unix(j.DecidedAt, 0)) < sharedOnceFor {
			continue
		}
		if err := s.takeOut(ctx, j); err != nil {
			errs = append(errs, fmt.Errorf("take back what %s was let in with: %w", j.MemberName, err))
			continue
		}
		j.State, j.User, j.LoginEnc, j.OnceEnc, j.OnceKey, j.DecidedAt = store.SharedJoinExpired, "", nil, nil, "", now.Unix()
		if err := s.store.SaveSharedJoin(j); err != nil {
			errs = append(errs, err)
			continue
		}
		expired = append(expired, j.MemberID)
	}
	s.sharedMu.Unlock()
	if err != nil {
		return err
	}
	for _, id := range expired {
		s.tellSharedAnswer(ctx, id, store.SharedJoinExpired)
	}
	return errors.Join(errs...)
}

// freeSharedUser is a login for the member called name that no other login of
// the shared server has.
func (s *Service) freeSharedUser(ss store.SharedRepoServer, name string) (string, error) {
	joins, err := s.store.ListSharedJoins(store.SharedJoinIn)
	if err != nil {
		return "", err
	}
	taken := map[string]bool{ss.User: true}
	for _, j := range joins {
		taken[j.User] = true
	}
	return uniqueReceiverUser(defaultReceiverUser(name), taken), nil
}

// reloadContainer tells a rest-server to read its login file again.
func (s *Service) reloadContainer(ctx context.Context, name string) {
	if sig, ok := s.docker.(dockercli.Signaler); ok {
		if err := sig.Signal(ctx, name, "SIGHUP"); err != nil {
			log.Printf("shared repository: ask %s to reload its logins: %v", name, err)
		}
	}
}

// RefuseSharedJoin turns a member's ask down, taking back the one-time key
// and the login an earlier allow gave it. The second result reports whether
// the member heard.
func (s *Service) RefuseSharedJoin(ctx context.Context, memberID string) (bool, error) {
	s.sharedMu.Lock()
	defer s.sharedMu.Unlock()
	j, ok, err := s.store.GetSharedJoin(store.SharedJoinIn, memberID)
	if err != nil {
		return false, err
	}
	if !ok || (j.State != store.SharedJoinAsked && j.State != store.SharedJoinAllowed) {
		return false, errSharedNoJoin
	}
	if j.State == store.SharedJoinAllowed {
		if err := s.takeOut(ctx, j); err != nil {
			return false, err
		}
	}
	j.State, j.User, j.LoginEnc, j.OnceEnc, j.OnceKey, j.DecidedAt = store.SharedJoinRefused, "", nil, nil, "", time.Now().Unix()
	if err := s.store.SaveSharedJoin(j); err != nil {
		return false, err
	}
	return s.tellSharedAnswer(ctx, memberID, j.State), nil
}

// RemoveSharedMember takes a member out of the shared repository and tells it
// so. Its snapshots stay and age under this server's rules. The second result
// reports whether the member heard.
func (s *Service) RemoveSharedMember(ctx context.Context, memberID string) (bool, error) {
	s.sharedMu.Lock()
	defer s.sharedMu.Unlock()
	if err := s.removeSharedMember(ctx, memberID); err != nil {
		return false, err
	}
	return s.tellSharedAnswer(ctx, memberID, store.SharedJoinRemoved), nil
}

// removeSharedMember takes away every key under the member's host, its login
// and its place among the members. The caller holds sharedMu.
func (s *Service) removeSharedMember(ctx context.Context, memberID string) error {
	j, ok, err := s.store.GetSharedJoin(store.SharedJoinIn, memberID)
	if err != nil {
		return err
	}
	if !ok {
		return errSharedNoJoin
	}
	if err := s.takeOut(ctx, j); err != nil {
		return err
	}
	ss, ok, err := s.store.GetSharedRepoServer()
	if err != nil {
		return err
	}
	if ok {
		sr, found, err := s.store.SharedRepoAt(ss.Repo)
		if err != nil {
			return err
		}
		if found {
			for _, m := range sr.Members {
				if m.Instance != memberID {
					continue
				}
				if err := s.store.NoteSharedFormer(sr.ID, store.SharedFormer{Host: m.Host, Name: m.Name, LeftAt: time.Now().Unix()}); err != nil {
					return err
				}
			}
			sr.Members = slices.DeleteFunc(sr.Members, func(m store.SharedMember) bool { return m.Instance == memberID })
			if _, err := s.store.PutSharedRepo(sr); err != nil {
				return err
			}
		}
	}
	return s.store.DeleteSharedJoin(store.SharedJoinIn, memberID)
}

// takeOut removes the keys under the join's host and its login. A key this
// server opens the repository with stays, whatever host it names.
func (s *Service) takeOut(ctx context.Context, j store.SharedJoin) error {
	sh, err := s.sharedSetupHere()
	if err != nil {
		return err
	}
	keys, err := sh.eng.Keys(ctx, sh.repo, sh.mode)
	if err != nil {
		return fmt.Errorf("read the keys: %w", err)
	}
	for _, k := range keys {
		if k.Current || (k.HostName != j.Host && k.ID != j.OnceKey) {
			continue
		}
		if err := sh.eng.RemoveKey(ctx, sh.repo, sh.mode, k.ID); err != nil {
			return fmt.Errorf("remove the key %s: %w", shortKeyID(k.ID), err)
		}
	}
	if j.User != "" {
		if err := s.writeHtpasswd(sh.server.Folder, j.User, ""); err != nil {
			return err
		}
		s.reloadContainer(ctx, sh.server.ContainerName)
	}
	return nil
}

// peerSharedNote is the owner's answer to a member's ask, or its word that
// the member is out.
type peerSharedNote struct {
	State string `json:"state"`
	peerProof
}

// tellSharedAnswer tells a member where its join stands now and reports
// whether it heard. A member that did not hear learns it when it asks next.
func (s *Service) tellSharedAnswer(ctx context.Context, memberID, state string) bool {
	if _, ok := s.reachableMember(memberID); !ok {
		return false
	}
	proof, err := s.signCall(keydeposit.ProofSharedAnswer, memberID, callPayload(state))
	if err != nil {
		return false
	}
	return s.callMember(ctx, memberID, http.MethodPost, "/api/group/peer/shared-repo/answer", peerSharedNote{State: state, peerProof: proof}, nil) == nil
}

// peerSharedAsk is what a member sends the owner about the shared
// repository: its name and host, and whether a person there asks to join.
type peerSharedAsk struct {
	Name string `json:"name"`
	Host string `json:"host"`
	Join bool   `json:"join"`
	peerProof
}

func (a peerSharedAsk) signed() []byte {
	return callPayload(a.Name, a.Host, strconv.FormatBool(a.Join))
}

// peerSharedOffer is the owner's answer: where its shared repository is, how
// the member's join stands, and while the join is allowed, the login and the
// one-time key the member opens it with.
type peerSharedOffer struct {
	OK bool `json:"ok"`
	// Server is nil from an instance that runs no shared repository.
	Server    *peerReceiver      `json:"server"`
	Path      string             `json:"path,omitempty"`
	State     string             `json:"state,omitempty"`
	User      string             `json:"user,omitempty"`
	Login     string             `json:"login,omitempty"`
	Once      string             `json:"once,omitempty"`
	Members   []sharedMemberView `json:"members,omitempty"`
	DirectURL string             `json:"directUrl,omitempty"`
}

// handlePeerSharedJoin answers a member about the shared repository and
// records its ask to join when a person there made one. The login and the
// one-time key go only to the member the allowed join is for, in answer to a
// call it signed.
// POST /api/group/peer/shared-repo/join
func (s *Service) handlePeerSharedJoin(w http.ResponseWriter, r *http.Request) {
	var in peerSharedAsk
	if !decodePeerRole(w, r, &in) {
		return
	}
	asker, ok := s.roleAsker(r)
	if !ok {
		refuseUnnamedPeer(w)
		return
	}
	if err := s.verifyMember(r.Context(), asker, keydeposit.ProofSharedJoin, in.signed(), in.peerProof); err != nil {
		writeJSON(w, http.StatusForbidden, failEnvelope(err))
		return
	}
	ans, err := s.sharedOfferFor(r.Context(), asker, in)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, ans)
}

func (s *Service) sharedOfferFor(ctx context.Context, asker string, in peerSharedAsk) (peerSharedOffer, error) {
	s.sharedMu.Lock()
	defer s.sharedMu.Unlock()
	ans := peerSharedOffer{OK: true, DirectURL: s.selfDirectURL()}
	ss, ok, err := s.store.GetSharedRepoServer()
	if err != nil || !ok {
		return ans, err
	}
	if live, err := s.docker.InspectName(ctx, ss.ContainerName); err != nil {
		return ans, err
	} else if live == "" {
		return ans, nil
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return ans, err
	}
	addr := s.sharedAddress(ctx, ss)
	addr.InstanceName = instanceDisplayName(settings)
	ans.Server, ans.Path = &addr, sharedRepoFolder

	j, found, err := s.store.GetSharedJoin(store.SharedJoinIn, asker)
	if err != nil {
		return ans, err
	}
	if in.Join && (!found || j.State == store.SharedJoinRefused || j.State == store.SharedJoinExpired) {
		host := strings.TrimSpace(in.Host)
		if !sharedHostRe.MatchString(host) {
			return ans, errSharedHostForm
		}
		if taken, err := s.sharedHostTaken(ss.Repo, asker, host); err != nil {
			return ans, err
		} else if taken {
			return ans, errSharedHostTaken
		}
		j = store.SharedJoin{Direction: store.SharedJoinIn, MemberID: asker, MemberName: clipRoleName(in.Name), Host: host, State: store.SharedJoinAsked}
		if err := s.store.SaveSharedJoin(j); err != nil {
			return ans, err
		}
		found = true
	}
	if !found {
		return ans, nil
	}
	ans.State = j.State
	if sr, ok, err := s.store.SharedRepoAt(ss.Repo); err == nil && ok {
		for _, m := range sr.Members {
			ans.Members = append(ans.Members, sharedMemberView{MemberID: m.Instance, Name: m.Name, Host: m.Host})
		}
	}
	if j.State != store.SharedJoinAllowed {
		return ans, nil
	}
	login, err := secret.Decrypt(s.cfg.AppKey, j.LoginEnc)
	if err != nil {
		return ans, errors.New("the member's stored login could not be opened")
	}
	once, err := secret.Decrypt(s.cfg.AppKey, j.OnceEnc)
	if err != nil {
		return ans, errors.New("the stored one-time key could not be opened")
	}
	ans.User, ans.Login, ans.Once = j.User, string(login), string(once)
	return ans, nil
}

// peerSharedWord is a member's word that it added its own key, or that it
// leaves.
type peerSharedWord struct {
	Host string `json:"host"`
	peerProof
}

// handlePeerSharedJoined takes a member's word that it added its own key.
// The owner finds that key under the member's host before it removes the
// one-time key and counts the member in.
// POST /api/group/peer/shared-repo/joined
func (s *Service) handlePeerSharedJoined(w http.ResponseWriter, r *http.Request) {
	var in peerSharedWord
	if !decodePeerRole(w, r, &in) {
		return
	}
	asker, ok := s.roleAsker(r)
	if !ok {
		refuseUnnamedPeer(w)
		return
	}
	if err := s.verifyMember(r.Context(), asker, keydeposit.ProofSharedJoined, callPayload(in.Host), in.peerProof); err != nil {
		writeJSON(w, http.StatusForbidden, failEnvelope(err))
		return
	}
	if err := s.countIn(r.Context(), asker, in.Host); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

func (s *Service) countIn(ctx context.Context, memberID, host string) error {
	s.sharedMu.Lock()
	defer s.sharedMu.Unlock()
	j, ok, err := s.store.GetSharedJoin(store.SharedJoinIn, memberID)
	if err != nil {
		return err
	}
	if ok && j.State == store.SharedJoinJoined {
		return nil
	}
	if !ok || j.State != store.SharedJoinAllowed || j.Host != host {
		return errSharedNoJoin
	}
	sh, err := s.sharedSetupHere()
	if err != nil {
		return err
	}
	keys, err := sh.eng.Keys(ctx, sh.repo, sh.mode)
	if err != nil {
		return fmt.Errorf("read the keys: %w", err)
	}
	if !slices.ContainsFunc(keys, func(k restic.Key) bool { return k.HostName == j.Host && k.ID != j.OnceKey }) {
		return errors.New("the repository holds no key of that member yet")
	}
	if slices.ContainsFunc(keys, func(k restic.Key) bool { return k.ID == j.OnceKey }) {
		if err := sh.eng.RemoveKey(ctx, sh.repo, sh.mode, j.OnceKey); err != nil {
			return fmt.Errorf("remove the one-time key: %w", err)
		}
	}
	sr, _, err := s.store.SharedRepoAt(sh.server.Repo)
	if err != nil {
		return err
	}
	sr.Members = slices.DeleteFunc(sr.Members, func(m store.SharedMember) bool { return m.Instance == memberID })
	sr.Members = append(sr.Members, store.SharedMember{Host: j.Host, Instance: memberID, Name: j.MemberName})
	if _, err := s.store.PutSharedRepo(sr); err != nil {
		return err
	}
	if err := s.store.DropSharedFormer(sr.ID, j.Host); err != nil {
		return err
	}
	j.State, j.OnceEnc, j.OnceKey = store.SharedJoinJoined, nil, ""
	return s.store.SaveSharedJoin(j)
}

// handlePeerSharedLeave takes out a member that leaves the shared repository.
// POST /api/group/peer/shared-repo/leave
func (s *Service) handlePeerSharedLeave(w http.ResponseWriter, r *http.Request) {
	var in peerSharedWord
	if !decodePeerRole(w, r, &in) {
		return
	}
	asker, ok := s.roleAsker(r)
	if !ok {
		refuseUnnamedPeer(w)
		return
	}
	if err := s.verifyMember(r.Context(), asker, keydeposit.ProofSharedLeave, callPayload(in.Host), in.peerProof); err != nil {
		writeJSON(w, http.StatusForbidden, failEnvelope(err))
		return
	}
	s.sharedMu.Lock()
	err := s.removeSharedMember(r.Context(), asker)
	s.sharedMu.Unlock()
	if err != nil && !errors.Is(err, errSharedNoJoin) {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}
