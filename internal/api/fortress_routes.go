package api

import (
	"errors"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/junkerderprovinz/parleyport/kit/group"
	"github.com/junkerderprovinz/parleyport/kit/phrase"
)

// maxDeviceName bounds the server's name in the fortress, as the kit bounds
// an app's.
const maxDeviceName = 64

func (h *Handler) registerFortressMembership(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/fortress", h.handleFortress)
	mux.HandleFunc("POST /api/fortress/join", h.handleFortressJoin)
	mux.HandleFunc("POST /api/fortress/phrase/show", h.handleFortressShow)
	mux.HandleFunc("POST /api/fortress/phrase", h.handleFortressChange)
	mux.HandleFunc("PUT /api/fortress/device", h.handleFortressDevice)
	mux.HandleFunc("PUT /api/fortress/permissions", h.handleFortressPermission)
	mux.HandleFunc("DELETE /api/fortress", h.handleFortressLeave)
}

// fortressView is what GET /api/fortress answers with. It never carries the
// phrase, which has its own route behind the password.
type fortressView struct {
	OK     bool `json:"ok"`
	Joined bool `json:"joined"`
	// Phrase says whether BombVault keeps the phrase to show and change it,
	// which a membership from the console does not.
	Phrase        bool                     `json:"phrase"`
	Device        string                   `json:"device"`
	DefaultDevice string                   `json:"defaultDevice"`
	PasswordSet   bool                     `json:"passwordSet"`
	Members       []fortressMemberView     `json:"members"`
	Permissions   []fortressPermissionView `json:"permissions"`
}

// fortressMemberView is another instance of the fortress that said which app
// it runs.
type fortressMemberView struct {
	ID         string `json:"id"`
	App        string `json:"app"`
	Name       string `json:"name"`
	Version    string `json:"version"`
	Device     string `json:"device"`
	Deployment string `json:"deployment"`
	Kind       string `json:"kind"`
	Online     bool   `json:"online"`
	// Relay means it is reached through the relay alone.
	Relay    bool  `json:"relay"`
	LastSeen int64 `json:"lastSeen,omitempty"`
}

// fortressPermissionView is whether one app may run an action at another.
// Only those where BombVault acts or is acted at are shown here, since only
// those have words in BombVault.
type fortressPermissionView struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Action string `json:"action"`
	On     bool   `json:"on"`
}

func (h *Handler) fortressViewNow() (fortressView, error) {
	m, err := h.store.FortressMembership()
	if err != nil {
		return fortressView{}, err
	}
	settings, err := h.store.GetSettings()
	if err != nil {
		return fortressView{}, err
	}
	v := fortressView{
		OK:            true,
		Phrase:        m.Joined(),
		Device:        h.svc.fortressDevice(),
		DefaultDevice: h.svc.defaultFortressDevice(),
		PasswordSet:   settings.AuthPasswordHash != "",
		Members:       []fortressMemberView{},
		Permissions:   []fortressPermissionView{},
	}
	f := h.svc.fortressNow()
	v.Joined = f != nil || m.Joined()
	if f == nil {
		return v, nil
	}
	for _, mb := range f.Members() {
		if mb.State != group.StateApproved || mb.App.ID == "" {
			continue
		}
		mv := fortressMemberView{
			ID: mb.ID, App: mb.App.ID, Name: mb.App.Name, Version: mb.App.Version,
			Device: mb.Device, Deployment: mb.Deployment, Kind: string(mb.Kind),
			Online: mb.Online(), Relay: mb.Relay && !mb.Direct,
		}
		if !mb.LastSeen.IsZero() {
			mv.LastSeen = mb.LastSeen.Unix()
		}
		v.Members = append(v.Members, mv)
	}
	for _, p := range f.Permissions() {
		if p.From == "bombvault" || p.To == "bombvault" {
			v.Permissions = append(v.Permissions, fortressPermissionView{From: p.From, To: p.To, Action: p.Action, On: p.On})
		}
	}
	return v, nil
}

func (h *Handler) writeFortressView(w http.ResponseWriter) {
	v, err := h.fortressViewNow()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// writeFortressError answers with the code the page words a refusal by.
func writeFortressError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errInFortress):
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "code": "inFortress", "error": err.Error()})
	case errors.Is(err, errNoFortress):
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "code": "noFortress", "error": err.Error()})
	default:
		writeJSON(w, http.StatusOK, failEnvelope(err))
	}
}

// handleFortress reports whether BombVault is in a fortress, under which
// name, and the other apps there. GET /api/fortress
func (h *Handler) handleFortress(w http.ResponseWriter, _ *http.Request) {
	h.writeFortressView(w)
}

// handleFortressJoin joins the fortress of a phrase. A phrase that does not
// decode is answered with the reason and the word at fault, as on joining
// the group. POST /api/fortress/join
func (h *Handler) handleFortressJoin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Phrase string `json:"phrase"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	err := h.svc.JoinFortress(body.Phrase)
	var de *phrase.DecodeError
	switch {
	case errors.As(err, &de):
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":       false,
			"error":    strings.TrimPrefix(de.Error(), "phrase: "),
			"reason":   de.Reason,
			"word":     de.Word,
			"position": de.Position,
			"count":    de.Count,
		})
	case err != nil:
		writeFortressError(w, err)
	default:
		h.writeFortressView(w)
	}
}

// handleFortressShow shows the phrase again, behind the login password as
// the group's phrase is. POST /api/fortress/phrase/show
func (h *Handler) handleFortressShow(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if !decodeBody(w, r, &body) || !h.passwordAgain(w, r, body.Password) {
		return
	}
	words, err := h.svc.fortressPhrase()
	if err != nil {
		writeFortressError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"phrase": words}))
}

// handleFortressChange gives the fortress a new phrase and answers with it
// and the devices that still hold the old one. POST /api/fortress/phrase
func (h *Handler) handleFortressChange(w http.ResponseWriter, r *http.Request) {
	words, left, err := h.svc.ChangeFortressPhrase(r.Context())
	if err != nil {
		writeFortressError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"phrase": words, "left": left}))
}

// handleFortressDevice names this server for the other members.
// PUT /api/fortress/device
func (h *Handler) handleFortressDevice(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Device string `json:"device"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.Device)
	if utf8.RuneCountInString(name) > maxDeviceName || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "code": "deviceName", "error": "the name must be one line of at most 64 characters"})
		return
	}
	if err := h.svc.SetFortressDevice(name); err != nil {
		writeFortressError(w, err)
		return
	}
	h.writeFortressView(w)
}

// handleFortressPermission switches whether one app may run an action at
// another, here and at every member in reach. PUT /api/fortress/permissions
func (h *Handler) handleFortressPermission(w http.ResponseWriter, r *http.Request) {
	var body struct {
		From   string `json:"from"`
		To     string `json:"to"`
		Action string `json:"action"`
		On     bool   `json:"on"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	f := h.svc.fortressNow()
	if f == nil {
		writeFortressError(w, errNoFortress)
		return
	}
	if err := f.SetPermission(body.From, body.To, body.Action, body.On); err != nil {
		writeFortressError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleFortressLeave takes BombVault out of its fortress.
// DELETE /api/fortress
func (h *Handler) handleFortressLeave(w http.ResponseWriter, _ *http.Request) {
	if err := h.svc.LeaveFortress(); err != nil {
		writeFortressError(w, err)
		return
	}
	h.writeFortressView(w)
}
