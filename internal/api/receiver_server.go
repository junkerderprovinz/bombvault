package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/group"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/platform"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/template"
)

// The receiver runs under the limits of a start test's copy, which rest-server
// stays far below.
const (
	receiverCPUs   = startTestCPUs
	receiverMemory = startTestMemory
	receiverPids   = startTestPids
)

// htpasswdName is the login file rest-server reads from its data folder.
const htpasswdName = ".htpasswd"

// The outcomes of a receiver's append-only check.
const (
	receiverProtected    = "protected"
	receiverUnprotected  = "unprotected"
	receiverInconclusive = "inconclusive"
)

// receiverStartWait is how long the first check waits for a new server to
// answer, and receiverRetry how often it asks meanwhile. Variables so a test
// does not wait them out.
var (
	receiverStartWait = 20 * time.Second
	receiverRetry     = time.Second
)

// groupReceiverWait bounds the question to each member whether it runs a
// receiver, so one slow member does not hold up the wizard.
const groupReceiverWait = 8 * time.Second

var receiverHostRe = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?$`)

var errReceiverExists = errors.New("this instance runs a receiver already")

var errNoReceiver = errors.New("no receiver is set up on this instance")

var errNoMemberReceiver = errors.New("that instance runs no receiver")

var errReceiverNeedsAddress = errors.New("this instance reaches that one only through the relay, which carries no backups. Add that instance's address under Settings, Pairing, or enter the server's address by hand")

// receiverServerInput is the "Set up receiver" form.
type receiverServerInput struct {
	Folder string `json:"folder"`
	Port   int    `json:"port"`
	// Host is the address members reach the server at, empty to use the one
	// they reach this instance at.
	Host string `json:"host"`
}

// receiverServerView is the receiving server as the Receiver tab shows it,
// without its password.
type receiverServerView struct {
	ContainerName string `json:"containerName"`
	Folder        string `json:"folder"`
	HostPath      string `json:"hostPath"`
	Port          int    `json:"port"`
	// User is the login for someone outside the group.
	User string `json:"user"`
	Host string `json:"host"`
	// URL is the server's address without a login, empty when this instance
	// does not know its own address.
	URL       string `json:"url"`
	CreatedAt int64  `json:"createdAt"`
	// Present is false once the container is gone from Docker.
	Present     bool   `json:"present"`
	Check       string `json:"check"`
	CheckDetail string `json:"checkDetail"`
	CheckedAt   int64  `json:"checkedAt"`
	// Logins are the group members that fetched a login of their own.
	Logins []receiverLoginView `json:"logins"`
}

// receiverLoginView is a member's login as the Receiver tab lists it.
type receiverLoginView struct {
	MemberID  string `json:"memberId"`
	Name      string `json:"name"`
	User      string `json:"user"`
	CreatedAt int64  `json:"createdAt"`
}

// receiverSetupResult is what setting up a receiver answers. Password is the
// login for someone outside the group, shown this once; members of the group
// each get a login of their own over the group.
type receiverSetupResult struct {
	Server   receiverServerView `json:"server"`
	Password string             `json:"password"`
	// Template is "written" when the Unraid template went onto the flash
	// drive, "download" when it could not and the page offers it instead,
	// and "none" off Unraid.
	Template string `json:"template"`
}

// defaultReceiverUser derives a login from the instance name: lower case, with
// each run of other characters turned into one dash.
func defaultReceiverUser(instanceName string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(instanceName) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	user := strings.TrimRight(b.String(), "-")
	if len(user) > 32 {
		user = strings.TrimRight(user[:32], "-")
	}
	if user == "" {
		return "bombvault"
	}
	return user
}

// htpasswdLine is user's line in an htpasswd file, holding a bcrypt hash of
// password.
func htpasswdLine(user, password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptDeployCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return user + ":" + string(hash), nil
}

// htpasswdWith returns the htpasswd file existing with line as user's only
// line, or without any line for user when line is empty. Every other user
// keeps theirs.
func htpasswdWith(existing, user, line string) string {
	var out []string
	for l := range strings.SplitSeq(existing, "\n") {
		l = strings.TrimRight(l, "\r")
		if strings.TrimSpace(l) == "" || strings.HasPrefix(l, user+":") {
			continue
		}
		out = append(out, l)
	}
	if line != "" {
		out = append(out, line)
	}
	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, "\n") + "\n"
}

// uniqueReceiverUser is base, or base with the first free number appended,
// kept within the 32 characters a login may have.
func uniqueReceiverUser(base string, taken map[string]bool) string {
	if !taken[base] {
		return base
	}
	for i := 2; ; i++ {
		suffix := "-" + strconv.Itoa(i)
		user := strings.TrimRight(base[:min(len(base), 32-len(suffix))], "-") + suffix
		if !taken[user] {
			return user
		}
	}
}

// receiverContainerSpec is the recipe the receiver's container is created
// from: the recipe's image and options, the data folder at /data, the port
// published to the host and the limits of a start test's copy. On Unraid it
// carries the labels that make the Docker tab treat it as one of its own.
func receiverContainerSpec(hostPath string, port int, unraid bool) model.Inspect {
	pids := int64(receiverPids)
	labels := map[string]string{}
	if unraid {
		labels["net.unraid.docker.managed"] = "dockerman"
		labels["net.unraid.docker.icon"] = restServerIcon
	}
	return model.Inspect{
		Name:    restServerName,
		Running: true,
		Config: model.Config{
			Image:  restServerImage,
			Env:    []string{"OPTIONS=" + restServerOptions},
			Labels: labels,
		},
		HostConfig: model.HostConfig{
			Binds: []string{hostPath + ":/data"},
			PortBindings: map[string][]model.PortBinding{
				strconv.Itoa(restServerContainerPort) + "/tcp": {{HostPort: strconv.Itoa(port)}},
			},
			RestartPolicy: model.RestartPolicy{Name: "unless-stopped"},
			NetworkMode:   "bridge",
			NanoCPUs:      receiverCPUs,
			Memory:        receiverMemory,
			MemorySwap:    receiverMemory,
			PidsLimit:     &pids,
		},
		Network: model.NetworkEndpoint{Name: "bridge"},
	}
}

// receiverTemplate is the Unraid template of a receiver BombVault set up, so
// its port, folder and options stay editable in the Docker tab.
func receiverTemplate(rs store.ReceiverServer) string {
	note := "  BombVault set up this append-only rest-server and keeps its login in\n  " +
		xmlCommentSafe(rs.HostPath+"/"+htpasswdName) + ".\n  Every field below is editable in the Docker tab."
	return restServerTemplate(rs.ContainerName, rs.Port, rs.HostPath, note)
}

// checkReceiverInput normalises the form and returns the data folder as the
// host sees it.
func (s *Service) checkReceiverInput(in receiverServerInput) (receiverServerInput, string, error) {
	in.Folder = strings.Trim(strings.TrimSpace(in.Folder), "/")
	in.Host = strings.TrimSpace(in.Host)
	if in.Port == 0 {
		in.Port = restServerPort
	}
	if in.Folder == "" {
		return in, "", errors.New("choose the folder the repositories go into")
	}
	if _, err := paths.Resolve(s.cfg.HostMountRoot, in.Folder); err != nil {
		return in, "", s.repoPathError(in.Folder, err)
	}
	// Docker splits a bind at its colons.
	if strings.Contains(in.Folder, ":") {
		return in, "", errors.New("the folder name may not contain a colon")
	}
	if in.Port < 1 || in.Port > 65535 {
		return in, "", errors.New("the port has to be between 1 and 65535")
	}
	if in.Host != "" && !receiverHostRe.MatchString(in.Host) {
		return in, "", errors.New("enter the address as a host name or an IP address, without http:// or a port")
	}
	return in, path.Join(s.cfg.HostSourceRoot, in.Folder), nil
}

// writeHtpasswd puts line into the htpasswd file of the data folder rel under
// the host data mount as user's line, or drops user's line when line is
// empty, creating the folder when it is missing. It goes
// through os.Root so a link inside the folder cannot send the write anywhere
// else.
func (s *Service) writeHtpasswd(rel, user, line string) error {
	root, err := os.OpenRoot(s.cfg.HostMountRoot)
	if err != nil {
		return err
	}
	defer root.Close() //nolint:errcheck // a failed close changes nothing written
	if err := root.MkdirAll(rel, 0o700); err != nil {
		return fmt.Errorf("create the folder: %w", err)
	}
	name := path.Join(rel, htpasswdName)
	existing, err := root.ReadFile(name)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("read %s: %w", htpasswdName, err)
	}
	if err := root.WriteFile(name, []byte(htpasswdWith(string(existing), user, line)), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", htpasswdName, err)
	}
	return nil
}

// addHtpasswdUser gives user a new password in the htpasswd file of the data
// folder rel and returns it in the clear and sealed with this instance's key.
func (s *Service) addHtpasswdUser(rel, user string) (string, []byte, error) {
	password, err := randomDeployPassword()
	if err != nil {
		return "", nil, err
	}
	enc, err := secret.Encrypt(s.cfg.AppKey, []byte(password))
	if err != nil {
		return "", nil, err
	}
	line, err := htpasswdLine(user, password)
	if err != nil {
		return "", nil, err
	}
	if err := s.writeHtpasswd(rel, user, line); err != nil {
		return "", nil, err
	}
	return password, enc, nil
}

// SetUpReceiver creates and starts an append-only rest-server for the group
// to copy to, writes its Unraid template where the Docker tab finds it, and
// checks that it refuses deletes. A container already called rest-server is
// never replaced.
func (s *Service) SetUpReceiver(ctx context.Context, in receiverServerInput) (receiverSetupResult, error) {
	in, hostPath, err := s.checkReceiverInput(in)
	if err != nil {
		return receiverSetupResult{}, err
	}
	if old, ok, err := s.store.GetReceiverServer(); err != nil {
		return receiverSetupResult{}, err
	} else if ok {
		live, err := s.docker.InspectName(ctx, old.ContainerName)
		if err != nil {
			return receiverSetupResult{}, err
		}
		if live != "" {
			return receiverSetupResult{}, errReceiverExists
		}
		if err := s.store.DeleteReceiverServer(); err != nil {
			return receiverSetupResult{}, err
		}
	}
	if live, err := s.docker.InspectName(ctx, restServerName); err != nil {
		return receiverSetupResult{}, err
	} else if live != "" {
		return receiverSetupResult{}, fmt.Errorf("a container called %s exists already, and BombVault leaves it alone. Remove or rename it in the Docker tab first, or keep it and add it as a destination by hand", restServerName)
	}
	allocs, err := s.docker.Allocations(ctx)
	if err != nil {
		return receiverSetupResult{}, err
	}
	want := strconv.Itoa(in.Port) + "/tcp"
	for _, a := range allocs {
		for _, p := range a.HostPorts {
			if p == want {
				return receiverSetupResult{}, fmt.Errorf("port %d is taken by %s; choose another one", in.Port, a.Name)
			}
		}
	}

	settings, err := s.store.GetSettings()
	if err != nil {
		return receiverSetupResult{}, err
	}
	user := defaultReceiverUser(instanceDisplayName(settings))
	password, enc, err := s.addHtpasswdUser(in.Folder, user)
	if err != nil {
		return receiverSetupResult{}, err
	}

	if id, err := s.docker.ImageID(ctx, restServerImage); err != nil {
		return receiverSetupResult{}, err
	} else if id == "" {
		if err := s.docker.Pull(ctx, restServerImage); err != nil {
			return receiverSetupResult{}, fmt.Errorf("download %s: %w", restServerImage, err)
		}
	}
	unraid := s.platformFn().Kind() == platform.KindUnraid
	if err := s.docker.CreateAndStart(ctx, receiverContainerSpec(hostPath, in.Port, unraid), true); err != nil {
		return receiverSetupResult{}, fmt.Errorf("create the %s container: %w", restServerName, err)
	}

	rs := store.ReceiverServer{
		ContainerName: restServerName,
		Folder:        in.Folder,
		HostPath:      hostPath,
		Port:          in.Port,
		User:          user,
		PasswordEnc:   enc,
		Host:          in.Host,
	}
	if err := s.store.SaveReceiverServer(rs); err != nil {
		return receiverSetupResult{}, fmt.Errorf("the container runs, but BombVault could not note it down: %w", err)
	}
	result := receiverSetupResult{Password: password, Template: s.writeReceiverTemplate(rs, unraid)}
	s.checkReceiver(ctx, rs, password)
	result.Server, err = s.receiverView(ctx)
	return result, err
}

// writeReceiverTemplate writes the receiver's Unraid template into the flash
// drive's user templates and reports how it went for receiverSetupResult. The
// templates folder's parent has to exist: without it the flash is not mounted,
// and the write would land inside this container.
func (s *Service) writeReceiverTemplate(rs store.ReceiverServer, unraid bool) string {
	if !unraid {
		return "none"
	}
	if _, err := os.Stat(filepath.Dir(s.cfg.FlashTemplatesDir)); err != nil {
		log.Printf("receiver: the Unraid templates folder is not reachable: %v", err)
		return "download"
	}
	if err := template.Write(s.cfg.FlashTemplatesDir, rs.ContainerName, receiverTemplate(rs)); err != nil {
		log.Printf("receiver: write the Unraid template: %v", err)
		return "download"
	}
	return "written"
}

// receiverHost is where members reach the receiver: the address set for it,
// or the host this instance answers the group on.
func (s *Service) receiverHost(rs store.ReceiverServer) string {
	if rs.Host != "" {
		return rs.Host
	}
	return urlHost(s.selfDirectURL())
}

// urlHost is the host of an address, empty when it has none.
func urlHost(addr string) string {
	u, err := url.Parse(strings.TrimSpace(addr))
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// restBase is a rest-server's address with its port.
func restBase(host string, port int) string {
	return "http://" + net.JoinHostPort(host, strconv.Itoa(port))
}

// receiverProbeBases are the addresses this instance can try the receiver at:
// the container's own on Docker's network, which a BombVault on the bridge
// network reaches, and the one members use.
func (s *Service) receiverProbeBases(ctx context.Context, rs store.ReceiverServer) []string {
	var out []string
	if allocs, err := s.docker.Allocations(ctx); err == nil {
		for _, a := range allocs {
			if a.Name == rs.ContainerName && a.IPv4 != "" {
				out = append(out, restBase(a.IPv4, restServerContainerPort))
			}
		}
	}
	if host := s.receiverHost(rs); host != "" {
		out = append(out, restBase(host, rs.Port))
	}
	return out
}

// checkReceiver runs the tamper test's probe against the receiver and records
// the outcome. A server that has just started gets receiverStartWait to
// answer.
func (s *Service) checkReceiver(ctx context.Context, rs store.ReceiverServer, password string) {
	bases := s.receiverProbeBases(ctx, rs)
	verdict, detail := receiverInconclusive, "BombVault cannot reach the server from here"
	deadline := time.Now().Add(receiverStartWait)
probing:
	for len(bases) > 0 {
		for _, b := range bases {
			v, err := probeAppendOnly(ctx, b+"/"+rs.User, rs.User, password)
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
	if err := s.store.RecordReceiverServerCheck(verdict, detail); err != nil {
		log.Printf("receiver: record the append-only check: %v", err)
	}
}

// CheckReceiver runs the append-only check of the receiver again.
func (s *Service) CheckReceiver(ctx context.Context) (receiverServerView, error) {
	rs, ok, err := s.store.GetReceiverServer()
	if err != nil {
		return receiverServerView{}, err
	}
	if !ok {
		return receiverServerView{}, errNoReceiver
	}
	password, err := secret.Decrypt(s.cfg.AppKey, rs.PasswordEnc)
	if err != nil {
		return receiverServerView{}, errors.New("the receiver's stored password could not be opened")
	}
	s.checkReceiver(ctx, rs, string(password))
	return s.receiverView(ctx)
}

// receiverView reads the receiver back as the Receiver tab shows it.
func (s *Service) receiverView(ctx context.Context) (receiverServerView, error) {
	rs, ok, err := s.store.GetReceiverServer()
	if err != nil {
		return receiverServerView{}, err
	}
	if !ok {
		return receiverServerView{}, errNoReceiver
	}
	logins, err := s.store.ListReceiverLogins()
	if err != nil {
		return receiverServerView{}, err
	}
	live, err := s.docker.InspectName(ctx, rs.ContainerName)
	if err != nil {
		log.Printf("receiver: look up container %q: %v", rs.ContainerName, err)
	}
	v := receiverServerView{
		ContainerName: rs.ContainerName, Folder: rs.Folder, HostPath: rs.HostPath, Port: rs.Port, User: rs.User,
		Host: rs.Host, CreatedAt: rs.CreatedAt, Present: live != "" || err != nil,
		Check: rs.Check, CheckDetail: rs.CheckDetail, CheckedAt: rs.CheckedAt,
		Logins: make([]receiverLoginView, 0, len(logins)),
	}
	for _, l := range logins {
		v.Logins = append(v.Logins, receiverLoginView{MemberID: l.MemberID, Name: l.MemberName, User: l.User, CreatedAt: l.CreatedAt})
	}
	if host := s.receiverHost(rs); host != "" {
		v.URL = restBase(host, rs.Port)
	}
	return v, nil
}

// reloadReceiver tells rest-server to read its htpasswd file again. Should the
// signal not arrive, rest-server notices the changed file by itself within 30
// seconds.
func (s *Service) reloadReceiver(ctx context.Context, rs store.ReceiverServer) {
	sig, ok := s.docker.(dockercli.Signaler)
	if !ok {
		return
	}
	if err := sig.Signal(ctx, rs.ContainerName, "SIGHUP"); err != nil {
		log.Printf("receiver: ask %s to reload its logins: %v", rs.ContainerName, err)
	}
}

// receiverLoginFor returns the login of the group member memberID, creating it
// on the first request. The login is tied to the member's id, so a member that
// is renamed keeps it; name only labels it on the Receiver tab. Only a member
// allowed as a sender gets one, and the check runs under the lock a revoke
// takes, so a login cannot appear after its role was taken away.
func (s *Service) receiverLoginFor(ctx context.Context, rs store.ReceiverServer, memberID, name string) (user, password string, err error) {
	s.receiverMu.Lock()
	defer s.receiverMu.Unlock()
	role, ok, err := s.store.FindRoleRequest(store.RoleRequestIn, memberID, store.RoleReceiver)
	if err != nil {
		return "", "", err
	}
	if !ok || role.State != store.RoleAllowed {
		return "", "", errPeerNotAllowed
	}
	l, ok, err := s.store.GetReceiverLogin(memberID)
	if err != nil {
		return "", "", err
	}
	if ok {
		if name != "" && name != l.MemberName {
			if err := s.store.RenameReceiverLogin(memberID, name); err != nil {
				log.Printf("receiver: rename the login of %q: %v", memberID, err)
			}
		}
		plain, err := secret.Decrypt(s.cfg.AppKey, l.PasswordEnc)
		if err != nil {
			return "", "", errors.New("a stored receiver password could not be opened")
		}
		return l.User, string(plain), nil
	}

	logins, err := s.store.ListReceiverLogins()
	if err != nil {
		return "", "", err
	}
	taken := map[string]bool{rs.User: true}
	for _, l := range logins {
		taken[l.User] = true
	}
	user = uniqueReceiverUser(defaultReceiverUser(name), taken)
	password, enc, err := s.addHtpasswdUser(rs.Folder, user)
	if err != nil {
		return "", "", err
	}
	if err := s.store.CreateReceiverLogin(store.ReceiverLogin{MemberID: memberID, MemberName: name, User: user, PasswordEnc: enc}); err != nil {
		return "", "", err
	}
	s.reloadReceiver(ctx, rs)
	return user, password, nil
}

// RevokeReceiverLogin takes a member's login off the receiver. What the member
// copied stays in the folder.
func (s *Service) RevokeReceiverLogin(ctx context.Context, memberID string) error {
	rs, ok, err := s.store.GetReceiverServer()
	if err != nil {
		return err
	}
	if !ok {
		return errNoReceiver
	}
	s.receiverMu.Lock()
	defer s.receiverMu.Unlock()
	l, ok, err := s.store.GetReceiverLogin(memberID)
	if err != nil || !ok {
		return err
	}
	if err := s.writeHtpasswd(rs.Folder, l.User, ""); err != nil {
		return err
	}
	if err := s.store.DeleteReceiverLogin(memberID); err != nil {
		return err
	}
	s.reloadReceiver(ctx, rs)
	return nil
}

// peerReceiverRequest is what a member sends when it asks about the receiver:
// who it is, and whether it wants its login or only to know the receiver is
// there, as the destination wizard's list does.
type peerReceiverRequest struct {
	InstanceID string `json:"instanceId"`
	Name       string `json:"name"`
	Login      bool   `json:"login"`
}

// peerReceiverBodyMax caps the request, which is a few dozen bytes.
const peerReceiverBodyMax = 4 << 10

// peerReceiver is what a member hands out about the receiver it set up. User
// and Password are the asking member's own login, and empty unless it asked
// for one.
type peerReceiver struct {
	InstanceName string `json:"instanceName"`
	// Host is empty when the receiver answers where the member does.
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port"`
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
}

// peerReceiverAnswer is the answer to POST /api/group/peer/receiver. Receiver
// is nil on an instance without one.
type peerReceiverAnswer struct {
	OK        bool          `json:"ok"`
	Receiver  *peerReceiver `json:"receiver"`
	DirectURL string        `json:"directUrl,omitempty"`
}

// handlePeerReceiver tells a member about the receiver this instance runs for
// the group, and hands it a login of its own when it asks for one and is
// allowed as a sender. A receiver whose container is gone is not offered.
// POST /api/group/peer/receiver
func (s *Service) handlePeerReceiver(w http.ResponseWriter, r *http.Request) {
	var in peerReceiverRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, peerReceiverBodyMax)).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "malformed receiver request"})
		return
	}
	in.InstanceID, in.Name = strings.TrimSpace(in.InstanceID), strings.TrimSpace(in.Name)
	if in.InstanceID == "" || len(in.InstanceID) > 64 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "the asking instance did not say who it is"})
		return
	}
	ans := peerReceiverAnswer{OK: true, DirectURL: s.selfDirectURL()}
	rs, ok, err := s.store.GetReceiverServer()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if !ok {
		writeJSON(w, http.StatusOK, ans)
		return
	}
	live, err := s.docker.InspectName(r.Context(), rs.ContainerName)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if live == "" {
		writeJSON(w, http.StatusOK, ans)
		return
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	offer := &peerReceiver{InstanceName: instanceDisplayName(settings), Host: rs.Host, Port: rs.Port}
	if in.Login {
		caller, ok := s.peerCaller(r)
		if !ok || caller.ID != in.InstanceID {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "the asking instance is not the one the group names for this call"})
			return
		}
		offer.User, offer.Password, err = s.receiverLoginFor(r.Context(), rs, caller.ID, in.Name)
		if errors.Is(err, errPeerNotAllowed) {
			s.noteOldReceiverAsk(caller, in.Name)
		}
		if err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
	}
	ans.Receiver = offer
	writeJSON(w, http.StatusOK, ans)
}

// groupReceiverView is a member's receiver as the destination wizard offers
// it, without a login.
type groupReceiverView struct {
	MemberID string `json:"memberId"`
	Name     string `json:"name"`
	// URL is the server's address without a login.
	URL string `json:"url,omitempty"`
	// NeedsAddress is true when this instance reaches the member only through
	// the relay, which carries no backups to a rest-server.
	NeedsAddress bool `json:"needsAddress"`
}

// groupReceiverFor places a member's receiver: at the address the member set
// for it, or at the host this instance reaches the member at directly.
func groupReceiverFor(m group.Member, r peerReceiver) groupReceiverView {
	v := groupReceiverView{MemberID: m.ID, Name: r.InstanceName}
	if v.Name == "" {
		v.Name = m.Name
	}
	host := r.Host
	if host == "" && m.Direct {
		host = urlHost(m.Address)
	}
	if host == "" {
		v.NeedsAddress = true
		return v
	}
	v.URL = restBase(host, r.Port)
	return v
}

// askReceiver asks one member about its receiver, and for this instance's
// login on it when login is true; nil means it runs none.
func (s *Service) askReceiver(ctx context.Context, memberID string, login bool) (*peerReceiver, error) {
	g, err := s.store.GetGroupState()
	if err != nil {
		return nil, err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, err
	}
	req := peerReceiverRequest{InstanceID: g.InstanceID, Name: instanceDisplayName(settings), Login: login}
	var ans peerReceiverAnswer
	if err := s.callMember(ctx, memberID, http.MethodPost, "/api/group/peer/receiver", req, &ans); err != nil {
		return nil, err
	}
	return ans.Receiver, nil
}

// GroupReceivers asks every instance in the group whether it runs a receiver.
// A member that does not answer is left out.
func (s *Service) GroupReceivers(ctx context.Context) []groupReceiverView {
	members := s.pairing().Members()
	found := make([]*groupReceiverView, len(members))
	var wg sync.WaitGroup
	for i, m := range members {
		if m.Kind != "" {
			continue
		}
		wg.Go(func() {
			cctx, cancel := context.WithTimeout(ctx, groupReceiverWait)
			defer cancel()
			r, err := s.askReceiver(cctx, m.ID, false)
			if err != nil || r == nil {
				return
			}
			v := groupReceiverFor(m, *r)
			found[i] = &v
		})
	}
	wg.Wait()
	out := []groupReceiverView{}
	for _, v := range found {
		if v != nil {
			out = append(out, *v)
		}
	}
	return out
}

// groupReceiverLogin is what the destination wizard fills in for a member's
// receiver.
type groupReceiverLogin struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	User     string `json:"user"`
	Password string `json:"password"`
}

// GroupReceiverLogin fetches this instance's own login on a member's
// receiver, which the member creates the first time it is asked after it
// allowed this instance as a sender. Its URL ends in the login, the one place
// rest-server lets that login write.
func (s *Service) GroupReceiverLogin(ctx context.Context, memberID string) (groupReceiverLogin, error) {
	var member *group.Member
	for _, m := range s.pairing().Members() {
		if m.ID == memberID && m.Kind == "" {
			member = &m
			break
		}
	}
	if member == nil {
		return groupReceiverLogin{}, errMemberGone
	}
	// Asking without a login first keeps a member out of reach from handing
	// out a login nobody can use.
	r, err := s.askReceiver(ctx, memberID, false)
	if err != nil {
		return groupReceiverLogin{}, err
	}
	if r == nil {
		return groupReceiverLogin{}, errNoMemberReceiver
	}
	if groupReceiverFor(*member, *r).NeedsAddress {
		return groupReceiverLogin{}, errReceiverNeedsAddress
	}
	if err := s.ensureReceiverRole(ctx, *member); err != nil {
		return groupReceiverLogin{}, err
	}
	r, err = s.askReceiver(ctx, memberID, true)
	if err != nil {
		return groupReceiverLogin{}, err
	}
	if r == nil {
		return groupReceiverLogin{}, errNoMemberReceiver
	}
	v := groupReceiverFor(*member, *r)
	return groupReceiverLogin{Name: v.Name, URL: v.URL + "/" + r.User, User: r.User, Password: r.Password}, nil
}

// handleGetReceiver answers the Receiver tab: the receiver, when one is set
// up, and what the form starts from.
// GET /api/receiver/server
func (h *Handler) handleGetReceiver(w http.ResponseWriter, r *http.Request) {
	answer := map[string]any{
		"server":        nil,
		"defaultPort":   restServerPort,
		"hostMountRoot": h.cfg.HostMountRoot,
	}
	v, err := h.svc.receiverView(r.Context())
	switch {
	case err == nil:
		answer["server"] = v
	case !errors.Is(err, errNoReceiver):
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(answer))
}

// handleSetUpReceiver creates the receiver. It starts a container on the host
// and answers with the outsider's password.
// POST /api/receiver/server
func (h *Handler) handleSetUpReceiver(w http.ResponseWriter, r *http.Request) {
	var in receiverServerInput
	if !decodeBody(w, r, &in) {
		return
	}
	res, err := h.svc.SetUpReceiver(r.Context(), in)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"server": res.Server, "password": res.Password, "template": res.Template}))
}

// handleCheckReceiver runs the receiver's append-only check again.
// POST /api/receiver/server/check
func (h *Handler) handleCheckReceiver(w http.ResponseWriter, r *http.Request) {
	v, err := h.svc.CheckReceiver(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"server": v}))
}

// handleForgetReceiver stops offering the receiver to the group. The container
// and its data stay.
// DELETE /api/receiver/server
func (h *Handler) handleForgetReceiver(w http.ResponseWriter, _ *http.Request) {
	if err := h.store.DeleteReceiverServer(); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleRevokeReceiverLogin takes a group member's login off the receiver.
// DELETE /api/receiver/server/logins/{id}
func (h *Handler) handleRevokeReceiverLogin(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.revokeReceiverOf(r.Context(), r.PathValue("id")); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	v, err := h.svc.receiverView(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"server": v}))
}

// handleReceiverTemplate downloads the receiver's Unraid template, for a host
// where BombVault could not write it onto the flash drive itself.
// GET /api/receiver/server/template
func (h *Handler) handleReceiverTemplate(w http.ResponseWriter, _ *http.Request) {
	rs, ok, err := h.store.GetReceiverServer()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": errNoReceiver.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+template.FileName(rs.ContainerName)+`"`)
	_, _ = w.Write([]byte(receiverTemplate(rs)))
}

// handleListGroupReceivers lists the receivers the group's members run, for
// the destination wizard.
// GET /api/offsite/group-receivers
func (h *Handler) handleListGroupReceivers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"receivers": h.svc.GroupReceivers(r.Context())}))
}

// handleGroupReceiverLogin hands the wizard the address and login of a
// member's receiver.
// POST /api/offsite/group-receivers/{id}/login
func (h *Handler) handleGroupReceiverLogin(w http.ResponseWriter, r *http.Request) {
	login, err := h.svc.GroupReceiverLogin(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"login": login}))
}
