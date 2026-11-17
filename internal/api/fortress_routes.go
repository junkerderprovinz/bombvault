package api

import (
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/group"
	"github.com/junkerderprovinz/parleyport/kit/phrase"
)

func (h *Handler) registerFortressMembership(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/fortress", h.handleFortress)
	mux.HandleFunc("POST /api/fortress/join", h.handleFortressJoin)
	mux.HandleFunc("POST /api/fortress/phrase/show", h.handleFortressShow)
	mux.HandleFunc("POST /api/fortress/phrase", h.handleFortressChange)
	mux.HandleFunc("PUT /api/fortress/permissions", h.handleFortressPermission)
	mux.HandleFunc("DELETE /api/fortress/members/{id}", h.handleFortressRemove)
	mux.HandleFunc("GET /api/fortress/members/{id}/status", h.handleFortressMemberStatus)
	mux.HandleFunc("POST /api/fortress/open/{id}", h.handleFortressOpen)
	mux.HandleFunc("DELETE /api/fortress", h.handleFortressLeave)
}

// fortressView is what GET /api/fortress answers with. It never carries the
// phrase, which has its own route behind the password.
type fortressView struct {
	OK     bool `json:"ok"`
	Joined bool `json:"joined"`
	// Phrase says whether BombVault keeps the phrase to show and change it,
	// which a membership from the console does not.
	Phrase bool `json:"phrase"`
	// DefaultDevice is the name this instance goes by without one of its own.
	DefaultDevice string                   `json:"defaultDevice"`
	PasswordSet   bool                     `json:"passwordSet"`
	Members       []fortressMemberView     `json:"members"`
	Permissions   []fortressPermissionView `json:"permissions"`
	// PairedWith names the devices of the group this instance is paired
	// with, which move into the fortress on joining and leave it with this
	// instance.
	PairedWith []string `json:"pairedWith"`
	// Removed are the members taken out here since the phrase last changed,
	// while the fortress takes no new member.
	Removed []fortressRemoved `json:"removed"`
	// RemovedHere says another member took this instance out.
	RemovedHere bool `json:"removedHere"`
	// PhraseChanged says a member told this instance that the fortress left
	// its phrase without handing it the new one, so the page asks for it.
	PhraseChanged bool `json:"phraseChanged"`
}

// fortressMemberView is another instance of the fortress. App is empty for
// one BombVault knows only from the other members.
type fortressMemberView struct {
	ID         string `json:"id"`
	App        string `json:"app"`
	Name       string `json:"name"`
	Version    string `json:"version"`
	Device     string `json:"device"`
	Deployment string `json:"deployment"`
	// Host is the server it says it runs on, which two members of one app
	// should not share.
	Host   string `json:"host,omitempty"`
	Kind   string `json:"kind"`
	Online bool   `json:"online"`
	// Opens says it offers its pages to open through the fortress.
	Opens bool `json:"opens"`
	// Relay means it is reached through the relay alone.
	Relay    bool  `json:"relay"`
	LastSeen int64 `json:"lastSeen,omitempty"`
	// OldPhrase says it was last seen before the phrase changed, so it still
	// holds the old one.
	OldPhrase bool `json:"oldPhrase,omitempty"`
	// Logo is the app's mark as its status publishes it, a data URI that the
	// page shows only as an image.
	Logo string `json:"logo,omitempty"`
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
		DefaultDevice: h.svc.defaultPairName(),
		PasswordSet:   settings.AuthPasswordHash != "",
		Members:       []fortressMemberView{},
		Permissions:   []fortressPermissionView{},
		PairedWith:    []string{},
	}
	v.RemovedHere = m.RemovedHere
	if v.Removed, err = h.svc.fortressRemovedList(); err != nil {
		return fortressView{}, err
	}
	for _, pm := range h.svc.pairing().Members() {
		v.PairedWith = append(v.PairedWith, pm.Name)
	}
	slices.Sort(v.PairedWith)
	f := h.svc.fortressNow()
	v.Joined = f != nil || m.Joined()
	if f == nil {
		return v, nil
	}
	v.PhraseChanged = f.PhraseChanged()
	// The pairing's instances are BombVault's own, which the page shows apart
	// even before the fortress heard their app.
	paired := map[string]bool{}
	if p := f.Pairing(); p != nil {
		for _, pm := range p.Members() {
			paired[pm.ID] = true
		}
	}
	for _, mb := range f.Members() {
		if mb.State != group.StateApproved || mb.App.ID == "" && paired[mb.ID] {
			continue
		}
		mv := fortressMemberView{
			ID: mb.ID, App: mb.App.ID, Name: appName(mb.App), Version: mb.App.Version,
			Device: mb.Device, Deployment: mb.Deployment, Host: mb.Host, Kind: string(mb.Kind),
			Online: mb.Online(), Opens: slices.Contains(mb.Actions, fortress.OpenAction),
			Relay: mb.Relay && !mb.Direct, Logo: h.svc.appLogo(f, mb),
		}
		if !mb.LastSeen.IsZero() {
			mv.LastSeen = mb.LastSeen.Unix()
			mv.OldPhrase = !mv.Online && m.SeenBeforeChange(mb.LastSeen)
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
	case errors.Is(err, errShutOut):
		// The page has words of its own for the kit's code, which an error
		// would take the place of.
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "code": group.ClosedUntilNewPhrase})
	case errors.Is(err, errNoMember):
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "code": "noMember", "error": err.Error()})
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
// and the devices that still hold the old one. The answer shows the phrase,
// so it asks for the password as showing it does. POST /api/fortress/phrase
func (h *Handler) handleFortressChange(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if !decodeBody(w, r, &body) || !h.passwordAgain(w, r, body.Password) {
		return
	}
	words, left, err := h.svc.ChangeFortressPhrase(r.Context())
	if err != nil {
		writeFortressError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"phrase": words, "left": left}))
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

// handleFortressRemove takes another member out of the fortress, which then
// takes no new member until the phrase changes, as the view says meanwhile.
// DELETE /api/fortress/members/{id}
func (h *Handler) handleFortressRemove(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RemoveFortressMember(r.PathValue("id")); err != nil {
		writeFortressError(w, err)
		return
	}
	h.writeFortressView(w)
}

// handleFortressLeave takes BombVault out of its fortress, and out of the
// group on the same phrase. DELETE /api/fortress
func (h *Handler) handleFortressLeave(w http.ResponseWriter, r *http.Request) {
	if m, err := h.store.FortressMembership(); err == nil && !m.Joined() && h.svc.fortressNow() == nil && !h.svc.pairing().Active() {
		writeFortressError(w, errNoFortress)
		return
	}
	if err := h.leavePairing(r.Context()); err != nil {
		writeFortressError(w, err)
		return
	}
	h.writeFortressView(w)
}
