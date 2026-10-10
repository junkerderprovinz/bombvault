package api

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/group"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// receiverDocker answers the calls setting up a receiver makes. Anything
// else would panic on the nil interface it embeds.
type receiverDocker struct {
	dockercli.Docker
	live    map[string]bool
	allocs  []model.Allocation
	imageID string

	pulled  []string
	created []model.Inspect
	signals []string
}

func (d *receiverDocker) Signal(_ context.Context, name, signal string) error {
	d.signals = append(d.signals, name+" "+signal)
	return nil
}

func (d *receiverDocker) InspectName(_ context.Context, name string) (string, error) {
	if d.live[name] {
		return name, nil
	}
	return "", nil
}

func (d *receiverDocker) Allocations(context.Context) ([]model.Allocation, error) {
	return d.allocs, nil
}

func (d *receiverDocker) ImageID(context.Context, string) (string, error) { return d.imageID, nil }

func (d *receiverDocker) Pull(_ context.Context, img string) error {
	d.pulled = append(d.pulled, img)
	return nil
}

func (d *receiverDocker) CreateAndStart(_ context.Context, in model.Inspect, _ bool) error {
	d.created = append(d.created, in)
	if d.live == nil {
		d.live = map[string]bool{}
	}
	d.live[in.Name] = true
	return nil
}

// appendOnlyServer stands in for a rest-server run with --append-only and
// --private-repos: it refuses every delete from user and lets nobody else in.
func appendOnlyServer(t *testing.T, user, password string, refuse bool) (host string, port int) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != user || p != password || !strings.HasPrefix(r.URL.Path, "/"+user+"/") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Method == http.MethodDelete && refuse {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	h, ps, err := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	port, _ = strconv.Atoi(ps)
	return h, port
}

type receiverFixture struct {
	svc       *Service
	st        *store.Repo
	docker    *receiverDocker
	mount     string
	templates string
}

func newReceiverFixture(t *testing.T) *receiverFixture {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	if _, err := st.MutateSettings(func(s *store.Settings) error {
		s.InstanceName = "Vault Box"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	mount := t.TempDir()
	flash := t.TempDir()
	if err := os.MkdirAll(filepath.Join(flash, "config", "plugins", "dockerMan"), 0o750); err != nil {
		t.Fatal(err)
	}
	templates := filepath.Join(flash, "config", "plugins", "dockerMan", "templates-user")
	cfg := config.Config{
		AppKey:            strings.Repeat("cd", 32),
		HostMountRoot:     mount,
		HostSourceRoot:    "/mnt",
		FlashTemplatesDir: templates,
	}
	d := &receiverDocker{}
	return &receiverFixture{
		svc:       &Service{cfg: cfg, store: st, docker: d},
		st:        st,
		docker:    d,
		mount:     mount,
		templates: templates,
	}
}

func TestReceiverContainerKeepsTheRecipesAppendOnlyFlags(t *testing.T) {
	in := receiverContainerSpec("/mnt/user/restic", 8123, true)

	if in.Name != "rest-server" || in.Config.Image != "restic/rest-server:0.14.0" {
		t.Fatalf("name %q image %q, want rest-server on the pinned recipe image", in.Name, in.Config.Image)
	}
	if len(in.Config.Env) != 1 || in.Config.Env[0] != "OPTIONS="+restServerOptions {
		t.Fatalf("env = %v, want the recipe's OPTIONS alone", in.Config.Env)
	}
	for _, flag := range []string{"--append-only", "--private-repos", "--htpasswd-file /data/.htpasswd"} {
		if !strings.Contains(in.Config.Env[0], flag) {
			t.Errorf("OPTIONS lacks %s", flag)
		}
	}
	hc := in.HostConfig
	if len(hc.Binds) != 1 || hc.Binds[0] != "/mnt/user/restic:/data" {
		t.Errorf("binds = %v, want the folder at /data", hc.Binds)
	}
	if got := hc.PortBindings["8000/tcp"]; len(got) != 1 || got[0].HostPort != "8123" || len(hc.PortBindings) != 1 {
		t.Errorf("port bindings = %v, want 8000/tcp published on 8123", hc.PortBindings)
	}
	if hc.RestartPolicy.Name != "unless-stopped" || hc.NetworkMode != "bridge" {
		t.Errorf("restart %q network %q", hc.RestartPolicy.Name, hc.NetworkMode)
	}
	if hc.Privileged || len(hc.CapAdd) > 0 || len(hc.Devices) > 0 {
		t.Error("the receiver must not get any privilege")
	}
	if hc.NanoCPUs != startTestCPUs || hc.Memory != startTestMemory || hc.MemorySwap != startTestMemory ||
		hc.PidsLimit == nil || *hc.PidsLimit != startTestPids {
		t.Errorf("limits cpu %d mem %d swap %d pids %v, want the start test's", hc.NanoCPUs, hc.Memory, hc.MemorySwap, hc.PidsLimit)
	}
	if !in.Running {
		t.Error("the receiver is created running")
	}
	if in.Config.Labels["net.unraid.docker.managed"] != "dockerman" {
		t.Errorf("on Unraid the Docker tab has to own the container, labels %v", in.Config.Labels)
	}
	if off := receiverContainerSpec("/srv/restic", 8000, false); len(off.Config.Labels) != 0 {
		t.Errorf("off Unraid no Unraid labels, got %v", off.Config.Labels)
	}
}

func TestSettingUpAReceiverStartsItAndFindsItAppendOnly(t *testing.T) {
	f := newReceiverFixture(t)
	restore := receiverStartWait
	receiverStartWait = 0
	t.Cleanup(func() { receiverStartWait = restore })

	// The password is generated inside, so the stand-in server checks it
	// against the hash written into the folder, as rest-server does.
	var hash string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != "vault-box" || bcrypt.CompareHashAndPassword([]byte(hash), []byte(p)) != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)
	_, ps, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	port, _ := strconv.Atoi(ps)
	f.docker.allocs = []model.Allocation{{Name: "plex", HostPorts: []string{"32400/tcp"}}}

	hook := f.docker
	f.svc.docker = &htpasswdPeek{receiverDocker: hook, mount: f.mount, folder: "user/restic", hash: &hash}

	res, err := f.svc.SetUpReceiver(context.Background(), receiverServerInput{
		Folder: "/user/restic/", Port: port, Host: "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("SetUpReceiver: %v", err)
	}
	if len(hook.pulled) != 1 || hook.pulled[0] != restServerImage {
		t.Errorf("pulled %v, want the missing image once", hook.pulled)
	}
	if len(hook.created) != 1 || hook.created[0].HostConfig.Binds[0] != "/mnt/user/restic:/data" {
		t.Fatalf("created %+v, want one container on the host path of the folder", hook.created)
	}

	raw, err := os.ReadFile(filepath.Join(f.mount, "user", "restic", ".htpasswd"))
	if err != nil {
		t.Fatal(err)
	}
	user, written, _ := strings.Cut(strings.TrimSpace(string(raw)), ":")
	if user != "vault-box" || bcrypt.CompareHashAndPassword([]byte(written), []byte(res.Password)) != nil {
		t.Fatalf("htpasswd %q does not let vault-box in with the answered password", raw)
	}

	if res.Template != "written" {
		t.Fatalf("template = %q, want written", res.Template)
	}
	tmpl, err := os.ReadFile(filepath.Join(f.templates, "my-rest-server.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(tmpl), res.Password) || strings.Contains(string(tmpl), written) {
		t.Error("the template on the flash drive must not carry the login")
	}
	var parsed struct {
		Name    string `xml:"Name"`
		Configs []struct {
			Target string `xml:"Target,attr"`
			Value  string `xml:",chardata"`
		} `xml:"Config"`
	}
	if err := xml.Unmarshal(tmpl, &parsed); err != nil {
		t.Fatalf("the template does not parse: %v", err)
	}
	values := map[string]string{}
	for _, c := range parsed.Configs {
		values[c.Target] = c.Value
	}
	if parsed.Name != "rest-server" || values["8000"] != ps || values["/data"] != "/mnt/user/restic" || values["OPTIONS"] != restServerOptions {
		t.Fatalf("template %s %v does not describe the container", parsed.Name, values)
	}

	if res.Server.Check != receiverProtected {
		t.Fatalf("check = %q (%s), want protected", res.Server.Check, res.Server.CheckDetail)
	}
	if res.Server.URL != "http://127.0.0.1:"+ps || res.Server.User != "vault-box" || !res.Server.Present {
		t.Fatalf("server view = %+v", res.Server)
	}
	rs, ok, _ := f.st.GetReceiverServer()
	if !ok || strings.Contains(string(rs.PasswordEnc), res.Password) {
		t.Fatal("the password has to be stored sealed")
	}
	if plain, err := secret.Decrypt(f.svc.cfg.AppKey, rs.PasswordEnc); err != nil || string(plain) != res.Password {
		t.Fatalf("the sealed password does not open to the answered one: %v", err)
	}
}

// htpasswdPeek hands the stand-in server the bcrypt hash BombVault wrote when
// the container is created, the moment a real rest-server reads it.
type htpasswdPeek struct {
	*receiverDocker
	mount, folder string
	hash          *string
}

func (p *htpasswdPeek) CreateAndStart(ctx context.Context, in model.Inspect, start bool) error {
	raw, err := os.ReadFile(filepath.Join(p.mount, filepath.FromSlash(p.folder), ".htpasswd"))
	if err != nil {
		return err
	}
	_, hash, _ := strings.Cut(strings.TrimSpace(string(raw)), ":")
	*p.hash = hash
	return p.receiverDocker.CreateAndStart(ctx, in, start)
}

func TestAReceiverThatAcceptsDeletesIsReportedUnprotected(t *testing.T) {
	f := newReceiverFixture(t)
	host, port := appendOnlyServer(t, "vault", "pw", false)
	rs := store.ReceiverServer{ContainerName: "rest-server", Port: port, User: "vault", Host: host}
	if err := f.st.SaveReceiverServer(rs); err != nil {
		t.Fatal(err)
	}
	f.svc.checkReceiver(context.Background(), rs, "pw")
	got, _, _ := f.st.GetReceiverServer()
	if got.Check != receiverUnprotected || got.CheckDetail == "" {
		t.Fatalf("check = %q %q, want unprotected with what it accepted", got.Check, got.CheckDetail)
	}
}

func TestSettingUpAReceiverRefusesAContainerOfTheSameName(t *testing.T) {
	f := newReceiverFixture(t)
	f.docker.live = map[string]bool{"rest-server": true}

	_, err := f.svc.SetUpReceiver(context.Background(), receiverServerInput{Folder: "user/restic", Port: 8000})
	if err == nil || !strings.Contains(err.Error(), "exists already") {
		t.Fatalf("err = %v, want a refusal that names the existing container", err)
	}
	if len(f.docker.created) != 0 {
		t.Fatal("an existing container must never be replaced")
	}
	if _, err := os.Stat(filepath.Join(f.mount, "user", "restic")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a refused setup must not touch the folder: %v", err)
	}
	if _, ok, _ := f.st.GetReceiverServer(); ok {
		t.Fatal("a refused setup must store nothing")
	}
}

func TestSettingUpAReceiverRefusesATakenPort(t *testing.T) {
	f := newReceiverFixture(t)
	f.docker.allocs = []model.Allocation{{Name: "minio", HostPorts: []string{"8000/tcp"}}}

	_, err := f.svc.SetUpReceiver(context.Background(), receiverServerInput{Folder: "user/restic", Port: 8000})
	if err == nil || !strings.Contains(err.Error(), "minio") {
		t.Fatalf("err = %v, want the container holding the port named", err)
	}
	if len(f.docker.created) != 0 {
		t.Fatal("nothing may be created on a taken port")
	}
}

func TestReceiverFormIsChecked(t *testing.T) {
	f := newReceiverFixture(t)
	for name, in := range map[string]receiverServerInput{
		"no folder":         {},
		"folder outside":    {Folder: "../etc"},
		"folder with colon": {Folder: "user/a:b"},
		"port too high":     {Folder: "user/restic", Port: 70000},
		"host with path":    {Folder: "user/restic", Host: "http://box/x"},
	} {
		if _, err := f.svc.SetUpReceiver(context.Background(), in); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if len(f.docker.created) != 0 {
		t.Fatal("a refused form must create nothing")
	}
}

func TestTheTemplateIsOfferedAsADownloadWithoutTheFlashDrive(t *testing.T) {
	f := newReceiverFixture(t)
	f.svc.cfg.FlashTemplatesDir = filepath.Join(t.TempDir(), "missing", "templates-user")
	rs := store.ReceiverServer{ContainerName: "rest-server", Port: 8000, HostPath: "/mnt/user/restic"}

	if got := f.svc.writeReceiverTemplate(rs, true); got != "download" {
		t.Fatalf("template = %q, want download", got)
	}
	if _, err := os.Stat(f.svc.cfg.FlashTemplatesDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("without the flash drive nothing may be written inside the container")
	}
	if got := f.svc.writeReceiverTemplate(rs, false); got != "none" {
		t.Fatalf("off Unraid template = %q, want none", got)
	}
}

func TestHtpasswdKeepsOtherUsersAndReplacesTheOwnLine(t *testing.T) {
	existing := "alice:$2y$05$old\nvault:$2y$05$stale\r\n\n"
	got := htpasswdWith(existing, "vault", "vault:$2a$12$new")
	if got != "alice:$2y$05$old\nvault:$2a$12$new\n" {
		t.Fatalf("htpasswd = %q", got)
	}
	if got := htpasswdWith("", "vault", "vault:x"); got != "vault:x\n" {
		t.Fatalf("a new file = %q", got)
	}
	if got := htpasswdWith(existing, "vault", ""); got != "alice:$2y$05$old\n" {
		t.Fatalf("a removed line = %q", got)
	}
}

func TestReceiverUserIsDerivedFromTheInstanceName(t *testing.T) {
	for in, want := range map[string]string{
		"Vault Box":             "vault-box",
		"tower":                 "tower",
		"  ":                    "bombvault",
		"Ünraid #2!":            "nraid-2",
		strings.Repeat("a", 40): strings.Repeat("a", 32),
	} {
		if got := defaultReceiverUser(in); got != want {
			t.Errorf("defaultReceiverUser(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestATakenLoginGetsTheFirstFreeNumber(t *testing.T) {
	taken := map[string]bool{"attic": true, "attic-2": true}
	if got := uniqueReceiverUser("attic", taken); got != "attic-3" {
		t.Errorf("got %q, want attic-3", got)
	}
	long := strings.Repeat("a", 32)
	if got := uniqueReceiverUser(long, map[string]bool{long: true}); got != strings.Repeat("a", 30)+"-2" {
		t.Errorf("got %q, want the name cut to make room for the number", got)
	}
}

// receiverWithOutsider sets up a receiver record in the fixture's mount with
// a running container and an htpasswd file holding the outsider's login.
func receiverWithOutsider(t *testing.T, f *receiverFixture) store.ReceiverServer {
	t.Helper()
	pw, enc, err := f.svc.addHtpasswdUser("user/restic", "vault-box")
	if err != nil {
		t.Fatal(err)
	}
	rs := store.ReceiverServer{ContainerName: "rest-server", Folder: "user/restic", Port: 8001, User: "vault-box", PasswordEnc: enc}
	if err := f.st.SaveReceiverServer(rs); err != nil {
		t.Fatal(err)
	}
	f.docker.live = map[string]bool{"rest-server": true}
	t.Cleanup(func() {
		if line := htpasswdLineOf(t, f, "vault-box"); bcrypt.CompareHashAndPassword([]byte(line), []byte(pw)) != nil {
			t.Error("the login for someone outside the group must keep working")
		}
	})
	return rs
}

// htpasswdLineOf is user's hash in the fixture receiver's htpasswd file, or
// empty when user has no line.
func htpasswdLineOf(t *testing.T, f *receiverFixture, user string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(f.mount, "user", "restic", ".htpasswd"))
	if err != nil {
		t.Fatal(err)
	}
	for l := range strings.SplitSeq(string(raw), "\n") {
		if hash, ok := strings.CutPrefix(l, user+":"); ok {
			return hash
		}
	}
	return ""
}

func askPeerReceiver(t *testing.T, f *receiverFixture, req peerReceiverRequest) peerReceiverAnswer {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/group/peer/receiver", strings.NewReader(string(body)))
	f.svc.handlePeerReceiver(w, r.WithContext(context.WithValue(r.Context(), peerSenderKey{}, req.InstanceID)))
	var ans peerReceiverAnswer
	if err := json.Unmarshal(w.Body.Bytes(), &ans); err != nil || !ans.OK {
		t.Fatalf("answer %s: %v", w.Body.String(), err)
	}
	return ans
}

func TestAMemberOffersItsReceiverOnlyWhileTheContainerIsThere(t *testing.T) {
	f := newReceiverFixture(t)
	ask := peerReceiverRequest{InstanceID: "m-attic", Name: "attic"}
	if ans := askPeerReceiver(t, f, ask); ans.Receiver != nil {
		t.Fatalf("an instance without a receiver offers %+v", ans.Receiver)
	}
	receiverWithOutsider(t, f)
	f.docker.live = nil
	if ans := askPeerReceiver(t, f, ask); ans.Receiver != nil {
		t.Fatal("a receiver whose container is gone must not be offered")
	}

	f.docker.live = map[string]bool{"rest-server": true}
	ans := askPeerReceiver(t, f, ask)
	if ans.Receiver == nil || ans.Receiver.InstanceName != "Vault Box" || ans.Receiver.Port != 8001 ||
		ans.Receiver.User != "" || ans.Receiver.Password != "" {
		t.Fatalf("an offer without a login asked for = %+v", ans.Receiver)
	}
	if logins, _ := f.st.ListReceiverLogins(); len(logins) != 0 || len(f.docker.signals) != 0 {
		t.Fatalf("listing the receiver must not hand out a login: %+v", logins)
	}
}

func TestEachPartnerGetsALoginOfItsOwn(t *testing.T) {
	f := newReceiverFixture(t)
	receiverWithOutsider(t, f)
	for _, member := range []string{"m-attic", "m-barn"} {
		grant := store.RoleRequest{Direction: store.RoleRequestIn, MemberID: member, Role: store.RoleReceiver, Sections: allRoleSections}
		if err := f.st.GrantRoleRequest(grant); err != nil {
			t.Fatal(err)
		}
	}

	attic := askPeerReceiver(t, f, peerReceiverRequest{InstanceID: "m-attic", Name: "attic", Login: true}).Receiver
	barn := askPeerReceiver(t, f, peerReceiverRequest{InstanceID: "m-barn", Name: "Attic!", Login: true}).Receiver
	if attic == nil || barn == nil || attic.User != "attic" || barn.User != "attic-2" || attic.Password == barn.Password {
		t.Fatalf("logins %+v and %+v, want two different ones", attic, barn)
	}
	for _, r := range []*peerReceiver{attic, barn} {
		if bcrypt.CompareHashAndPassword([]byte(htpasswdLineOf(t, f, r.User)), []byte(r.Password)) != nil {
			t.Errorf("the htpasswd file does not let %s in", r.User)
		}
	}
	if len(f.docker.signals) != 2 || f.docker.signals[0] != "rest-server SIGHUP" {
		t.Fatalf("signals = %v, want one SIGHUP per new login", f.docker.signals)
	}

	again := askPeerReceiver(t, f, peerReceiverRequest{InstanceID: "m-attic", Name: "attic loft", Login: true}).Receiver
	if again.User != attic.User || again.Password != attic.Password {
		t.Fatalf("a repeat request got %+v, want the same login back", again)
	}
	if len(f.docker.signals) != 2 {
		t.Fatal("a repeat request changes nothing, so it reloads nothing")
	}
	if l, _, _ := f.st.GetReceiverLogin("m-attic"); l.MemberName != "attic loft" {
		t.Fatalf("a renamed member keeps its login under its new name, got %q", l.MemberName)
	}

	view, err := f.svc.receiverView(context.Background())
	if err != nil || len(view.Logins) != 2 {
		t.Fatalf("the tab lists %+v, %v; want both partners", view.Logins, err)
	}

	if err := f.svc.RevokeReceiverLogin(context.Background(), "m-attic"); err != nil {
		t.Fatal(err)
	}
	if htpasswdLineOf(t, f, "attic") != "" || htpasswdLineOf(t, f, "attic-2") == "" {
		t.Fatal("revoking takes that partner's line out and leaves the others")
	}
	if _, ok, _ := f.st.GetReceiverLogin("m-attic"); ok {
		t.Fatal("a revoked login must be forgotten")
	}
	if len(f.docker.signals) != 3 {
		t.Fatalf("signals = %v, want a reload after the revoke", f.docker.signals)
	}
}

func TestAMembersReceiverIsPlacedWhereThisInstanceReachesIt(t *testing.T) {
	offer := peerReceiver{InstanceName: "attic", Port: 8001}

	direct := group.Member{ID: "m1", Name: "attic", Direct: true, Address: "https://192.168.1.20:3443"}
	if v := groupReceiverFor(direct, offer); v.NeedsAddress || v.URL != "http://192.168.1.20:8001" {
		t.Errorf("direct member: %+v", v)
	}

	relayed := group.Member{ID: "m1", Name: "attic", Relay: true, Address: "https://192.168.1.20:3443"}
	if v := groupReceiverFor(relayed, offer); !v.NeedsAddress || v.URL != "" {
		t.Errorf("a member reached only through the relay needs a direct address: %+v", v)
	}

	set := offer
	set.Host = "vault.lan"
	if v := groupReceiverFor(relayed, set); v.NeedsAddress || v.URL != "http://vault.lan:8001" {
		t.Errorf("an address the member set wins: %+v", v)
	}
}

func TestAPairedMembersReceiverReachesTheDestinationWizard(t *testing.T) {
	a := newInstance(t, "cellar", strings.Repeat("a1", 32))
	b := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, a, b)

	if code, out := a.do(t, http.MethodGet, "/api/offsite/group-receivers", nil); code != http.StatusOK || len(out["receivers"].([]any)) != 0 {
		t.Fatalf("a member without a receiver is listed: %d %v", code, out)
	}

	b.svc.cfg.HostMountRoot = t.TempDir()
	rs := store.ReceiverServer{ContainerName: "rest-server", Folder: "restic", Port: 8001, User: "attic"}
	if err := b.st.SaveReceiverServer(rs); err != nil {
		t.Fatal(err)
	}
	b.svc.docker = &receiverDocker{live: map[string]bool{"rest-server": true}}

	_, out := a.do(t, http.MethodGet, "/api/offsite/group-receivers", nil)
	list, _ := out["receivers"].([]any)
	if len(list) != 1 {
		t.Fatalf("receivers = %v, want attic's", out)
	}
	entry := list[0].(map[string]any)
	if entry["name"] != "attic" || entry["memberId"] != b.id(t) || entry["needsAddress"] != true || entry["password"] != nil {
		t.Fatalf("entry = %v, want attic without a direct route and without a login", entry)
	}
	if _, out := a.do(t, http.MethodPost, "/api/offsite/group-receivers/"+b.id(t)+"/login", nil); out["ok"] != false {
		t.Fatalf("a login over the relay alone must be refused: %v", out)
	}
	if logins, _ := b.st.ListReceiverLogins(); len(logins) != 0 {
		t.Fatal("a member out of reach must not get a login it cannot use")
	}

	rs.Host = "192.168.1.20"
	if err := b.st.SaveReceiverServer(rs); err != nil {
		t.Fatal(err)
	}
	if _, out := a.do(t, http.MethodPost, "/api/offsite/group-receivers/"+b.id(t)+"/login", nil); out["ok"] != false {
		t.Fatalf("a login before attic allowed cellar as a sender: %v", out)
	}
	asked, ok := roleWith(t, b, store.RoleRequestIn, a, store.RoleReceiver)
	if !ok || asked.State != store.RoleAsked {
		t.Fatalf("asking for a login left the request %+v, %v on attic; want it waiting", asked, ok)
	}
	if logins, _ := b.st.ListReceiverLogins(); len(logins) != 0 {
		t.Fatal("a member got a login before a person allowed it")
	}
	answerRole(t, b, asked.ID, store.RoleAllowed)
	_, out = a.do(t, http.MethodPost, "/api/offsite/group-receivers/"+b.id(t)+"/login", nil)
	login, _ := out["login"].(map[string]any)
	if out["ok"] != true || login["url"] != "http://192.168.1.20:8001/cellar" || login["user"] != "cellar" || login["password"] == "" {
		t.Fatalf("login = %v, want cellar's own login under its own path", out)
	}
	stored, ok, _ := b.st.GetReceiverLogin(a.id(t))
	if !ok || stored.User != "cellar" || stored.MemberName != "cellar" {
		t.Fatalf("the receiver keeps the login under the asking member's id, got %+v", stored)
	}
}
