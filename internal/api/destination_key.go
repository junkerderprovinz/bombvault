package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/remotes"
	"github.com/junkerderprovinz/bombvault/internal/sshconn"
)

// keyInstallRequest is the server of a destination that signs in with
// BombVault's SSH key, and the account's password for putting the key there.
type keyInstallRequest struct {
	Provider string            `json:"provider"`
	Settings map[string]string `json:"settings"`
	Password string            `json:"password"`
}

// keyInstallTimeout bounds the whole sign-in and install. A Storage Box
// answers within a second or two.
const keyInstallTimeout = 30 * time.Second

// installSSHKey is a var so a test can stand in for the server.
var installSSHKey = sshconn.InstallKey

// InstallDraftKey signs in to the server of a destination that is not saved
// yet with the account's password and adds BombVault's public key there. The
// password is used for that one sign-in and kept nowhere.
func (s *Service) InstallDraftKey(ctx context.Context, req keyInstallRequest) error {
	p, err := findProvider(req.Provider)
	if err != nil {
		return err
	}
	if p.Auth != remotes.AuthSSHKey {
		return errors.New("this kind of destination does not sign in with BombVault's SSH key")
	}
	d, err := s.draftFor(p, req.Settings)
	if err != nil {
		return err
	}
	host, user, port := strings.TrimSpace(d.Settings["host"]), strings.TrimSpace(d.Settings["user"]), strings.TrimSpace(d.Settings["port"])
	if port == "" {
		port = "22"
	}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return errors.New("the port has to be a number between 1 and 65535")
	}
	if host == "" || user == "" {
		return errors.New("fill in the host and the user name first")
	}
	if req.Password == "" {
		return errors.New("type the account's password first")
	}
	pub, err := os.ReadFile(s.sshKeyPath() + ".pub")
	if err != nil {
		return errors.New("BombVault has no SSH key yet; it makes one when it starts")
	}
	ctx, cancel := context.WithTimeout(ctx, keyInstallTimeout)
	defer cancel()
	target := sshconn.KeyTarget{
		Host: host, User: user, Port: port,
		StorageBox: p.ID == "storagebox",
		KnownHosts: filepath.Join(s.cfg.DataDir, "ssh", "known_hosts"),
	}
	if err := installSSHKey(ctx, target, req.Password, strings.TrimSpace(string(pub))); err != nil {
		return err
	}
	log.Printf("api: put the ssh key on %s@%s:%s", user, host, port) //nolint:gosec // G706: user and host are the form's own fields, logged without the password
	return nil
}

// keyInstallCode names a failure the wizard explains in the reader's language.
func keyInstallCode(err error) string {
	switch {
	case errors.Is(err, sshconn.ErrPasswordRefused):
		return "password-refused"
	case errors.Is(err, sshconn.ErrNoPasswordSignIn):
		return "no-password-sign-in"
	case errors.Is(err, sshconn.ErrSessionRefused):
		return "session-refused"
	case errors.Is(err, sshconn.ErrUnreachable):
		return "unreachable"
	case errors.Is(err, sshconn.ErrHostKeyChanged):
		return "host-key-changed"
	case errors.Is(err, sshconn.ErrNoWayIn):
		return "no-way-in"
	}
	return ""
}

// handleInstallDraftKey puts BombVault's public key on a destination's server.
// POST /api/offsite/drafts/install-key
func (h *Handler) handleInstallDraftKey(w http.ResponseWriter, r *http.Request) {
	var req keyInstallRequest
	if !decodeBody(w, r, &req) {
		return
	}
	err := h.svc.InstallDraftKey(r.Context(), req)
	if err == nil {
		writeJSON(w, http.StatusOK, okEnvelope(nil))
		return
	}
	code := keyInstallCode(err)
	// The error carries what the server printed, and nothing keeps a server
	// from printing the password it was given.
	if req.Password != "" {
		err = errors.New(strings.ReplaceAll(err.Error(), req.Password, "[password]"))
	}
	if code != "" {
		writeJSON(w, http.StatusOK, codedFailEnvelope(err, code))
		return
	}
	writeJSON(w, http.StatusOK, failEnvelope(err))
}
