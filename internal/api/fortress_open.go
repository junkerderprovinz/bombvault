package api

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"path"
	"slices"
	"strings"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/group"
	"github.com/junkerderprovinz/parleyport/kit/status"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// A signed-in user of another app in the fortress opens BombVault's pages
// through it, and a user signed in here opens theirs. Each request of such a
// page reaches serveOpened as one from far away, with the cookies the kit
// keeps for that user and BombVault's own sign-in in front.

// runKindOpened is the kind of the run on store.FortressTargetID that says
// another member opened this instance's pages.
const runKindOpened = "opened"

// codeOpenedRefused marks what a page opened through the fortress cannot
// reach.
const codeOpenedRefused = "opened-refused"

// openedClosed are the doors an opened page finds closed: setting and
// resetting the password and the other ways in, the keys of assistants and
// scripts, the pages and feeds that gate themselves on a token, and opening
// other members from here. An entry ending in a slash closes what lies below
// it, any other the path itself and what lies below.
var openedClosed = []string{
	"/api/auth/password", "/api/auth/reset", "/api/auth/totp", "/api/auth/passkey/register", "/api/auth/passkeys/",
	mcpEndpointPath, "/api/mcp", "/api/oauth", "/oauth", "/.well-known",
	"/api/tokens", "/api/v1",
	"/metrics", "/widget", "/api/widget",
	"/api/fortress/open", strings.TrimSuffix(fortress.OpenPath, "/"),
	"/api/group/call", relayConnectPath,
}

func closedToOpened(p string) bool {
	p = path.Clean("/" + p)
	for _, door := range openedClosed {
		if strings.HasSuffix(door, "/") {
			if strings.HasPrefix(p, door) {
				return true
			}
		} else if p == door || strings.HasPrefix(p, door+"/") {
			return true
		}
	}
	return false
}

type openedKey struct{}

// openedPage reports whether r is a request of a page another member opened
// here through the fortress.
func openedPage(r *http.Request) bool {
	if on, _ := r.Context().Value(openedKey{}).(bool); on {
		return true
	}
	_, ok := fortress.Opened(r)
	return ok
}

// serveOpened answers one request of a page another member opened here: what
// a browser reaches, with the doors of openedClosed shut and no download, as
// for a phone over the relay.
func (s *Service) serveOpened(w http.ResponseWriter, r *http.Request) {
	pages := s.pages.Load()
	if pages == nil {
		http.NotFound(w, r)
		return
	}
	if closedToOpened(r.URL.Path) {
		refuseOpened(w, http.StatusForbidden, "this works at BombVault's own address, not through the fortress")
		return
	}
	nd := &noDownload{ResponseWriter: w}
	(*pages).ServeHTTP(nd, r.WithContext(context.WithValue(r.Context(), openedKey{}, true)))
	nd.finish()
}

func refuseOpened(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"ok": false, "error": msg, "code": codeOpenedRefused})
}

// noDownload refuses an answer that the browser would save as a file: files
// are what a backup is made of, and an opened page is no way to carry them off.
type noDownload struct {
	http.ResponseWriter
	wrote, refused bool
}

func (n *noDownload) WriteHeader(code int) {
	if n.wrote {
		return
	}
	n.wrote = true
	h := n.Header()
	if !strings.Contains(strings.ToLower(h.Get("Content-Disposition")), "attachment") {
		n.ResponseWriter.WriteHeader(code)
		return
	}
	n.refused = true
	for _, name := range []string{"Content-Disposition", "Content-Encoding", "Content-Length", "Etag", "Last-Modified"} {
		h.Del(name)
	}
	refuseOpened(n.ResponseWriter, http.StatusForbidden, "files are saved at BombVault's own address, not through the fortress")
}

func (n *noDownload) Write(b []byte) (int, error) {
	if !n.wrote {
		n.WriteHeader(http.StatusOK)
	}
	if n.refused {
		return len(b), nil
	}
	return n.ResponseWriter.Write(b)
}

func (n *noDownload) Flush() {
	if f, ok := n.ResponseWriter.(http.Flusher); ok && !n.refused {
		f.Flush()
	}
}

func (n *noDownload) Unwrap() http.ResponseWriter { return n.ResponseWriter }

// finish answers a handler that wrote nothing, so that what it set in the
// header is checked as well.
func (n *noDownload) finish() {
	if !n.wrote {
		n.WriteHeader(http.StatusOK)
	}
}

// asksSignIn reports whether BombVault asks for its own sign-in: only then do
// its pages open through the fortress, since without a password every holder
// of the phrase could use them.
func (s *Service) asksSignIn() bool {
	settings, err := s.store.GetSettings()
	return err == nil && settings.AuthPasswordHash != ""
}

// openedFrom writes into the activity log that another member opened this
// instance's pages, naming its app, its device and the end of its id, which
// the bar above the page shows as well.
func (s *Service) openedFrom(c fortress.Caller) {
	subject := openedSubject(c, s.memberAppName(c.ID))
	log.Printf("fortress: %s opened the pages of this instance", subject) //nolint:gosec // G706: names from the fortress, %s on a log line
	id, err := s.startRun(withRunSubject(context.Background(), subject), store.FortressTargetID, runKindOpened)
	if err == nil {
		err = s.store.FinishRun(id, "success", "", 0, "")
	}
	if err != nil {
		log.Printf("fortress: the opening is missing from the activity log: %v", err)
	}
}

// openedSubject names who opened the pages: "KnightLoader · Bottich · 3f2a91c0".
func openedSubject(c fortress.Caller, app string) string {
	return strings.Join([]string{cmp.Or(app, c.App), cmp.Or(c.Device, "?"), memberTail(c.ID)}, " · ")
}

// memberTail is the end of a member's id, as the bar of an opened page shows
// it.
func memberTail(id string) string {
	if len(id) > 8 {
		return id[len(id)-8:]
	}
	return id
}

// memberAppName is the name of the app member id runs, empty while it said
// none.
func (s *Service) memberAppName(id string) string {
	f := s.fortressNow()
	if f == nil {
		return ""
	}
	for _, m := range f.Members() {
		if m.ID == id {
			return appName(m.App)
		}
	}
	return ""
}

// passwordSession is the session a request carries that somebody signed in
// with the password, and false for none.
func (h *Handler) passwordSession(r *http.Request) (string, bool) {
	hash, epoch, on := h.authEnabled()
	if !on || openedPage(r) || relayedPage(r) {
		return "", false
	}
	c, err := h.sessionCookie(r)
	if err != nil || secret.PairedSessionToken(c.Value) || !secret.ValidSessionToken(h.cfg.AppKey, hash, epoch, c.Value) {
		return "", false
	}
	return c.Value, true
}

// sessionName names a session to the fortress without giving it away.
func sessionName(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// FortressTabs serves the tabs in which a user signed in here opens other
// members of the fortress, outside the auth gate and the security headers:
// the kit checks the session itself, and the bar it draws frames the opened
// page. A phone's page over the relay and a page opened here through the
// fortress do not reach them.
func (h *Handler) FortressTabs() http.Handler {
	session := func(r *http.Request) (string, bool) {
		token, ok := h.passwordSession(r)
		if !ok {
			return "", false
		}
		return sessionName(token), true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f := h.svc.fortressNow()
		if f == nil || relayedPage(r) || openedPage(r) {
			http.NotFound(w, r)
			return
		}
		f.OpenHandler(session, nil).ServeHTTP(w, r)
	})
}

// handleFortressOpen opens another member's pages for the browser that asks,
// in a tab of their own, and answers with its path. Only a session somebody
// signed in for with the password may open one: the other app keeps its
// sign-in apart by it, and the tab closes once it ends.
// POST /api/fortress/open/{id}
func (h *Handler) handleFortressOpen(w http.ResponseWriter, r *http.Request) {
	if openedPage(r) {
		refuseOpened(w, http.StatusForbidden, "this works at BombVault's own address, not through the fortress")
		return
	}
	if !h.requireAuthForSecrets(w, r, "opening another app") {
		return
	}
	token, ok := h.passwordSession(r)
	if !ok {
		writeJSON(w, http.StatusForbidden, map[string]any{"ok": false, "error": "sign in with the password to open another app", "code": codeOwnLoginNeeded})
		return
	}
	f := h.svc.fortressNow()
	if f == nil {
		writeFortressError(w, errNoFortress)
		return
	}
	valid := func() bool {
		hash, epoch, on := h.authEnabled()
		return on && secret.ValidSessionToken(h.cfg.AppKey, hash, epoch, token)
	}
	ctx, cancel := context.WithTimeout(r.Context(), moveTimeout)
	defer cancel()
	tab, err := f.Open(ctx, fortress.Visit{
		Member:  r.PathValue("id"),
		Session: sessionName(token),
		Valid:   valid,
		Back:    forwardedPrefix(r) + "settings/pairing",
	})
	if err != nil {
		code := openRefusal(err)
		log.Printf("fortress: opening member %s refused: %s (%v)", r.PathValue("id"), code, err) //nolint:gosec // G706: an id from the path, %s on a log line
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "code": code, "error": scrubError(err)})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"path": tab}))
}

// openRefusal is the code the page words a refused opening by.
func openRefusal(err error) string {
	var no *fortress.Refusal
	switch {
	case errors.Is(err, group.ErrUnreachable), errors.Is(err, group.ErrNoSuchMember), errors.Is(err, group.ErrRemoved):
		return "unreachable"
	case errors.Is(err, fortress.ErrNoSuchAction):
		return "no_ui"
	case errors.Is(err, fortress.ErrPermissionOff), errors.Is(err, fortress.ErrNotUsed):
		return "permission_off"
	case errors.As(err, &no) && no.Code == "ui_no_login":
		return "ui_no_login"
	case errors.As(err, &no) && (no.Code == "busy" || no.Code == "jars_full"):
		return "busy"
	case errors.Is(err, context.DeadlineExceeded):
		return "unreachable"
	}
	return "failed"
}

// memberStatusLog is how many lines of a member's log its status window
// shows, and memberLogPages how many pages of its log are read for them.
const (
	memberStatusLog = 10
	memberLogPages  = 5
)

type memberStatusView struct {
	OK     bool             `json:"ok"`
	Report memberReportView `json:"report"`
	// Log is the end of the member's log, the newest line first.
	Log []memberLogLine `json:"log"`
}

type memberReportView struct {
	State    string           `json:"state"`
	NeedsYou []memberNeedView `json:"needsYou"`
	Work     []memberWorkView `json:"work"`
	Next     []memberNextView `json:"next"`
}

type memberNeedView struct {
	Severity string `json:"severity"`
	Text     string `json:"text"`
}

type memberWorkView struct {
	Title    string   `json:"title"`
	Progress *float64 `json:"progress,omitempty"`
}

type memberNextView struct {
	Title string `json:"title"`
	At    int64  `json:"at"`
}

type memberLogLine struct {
	At    int64  `json:"at"`
	Level string `json:"level"`
	Text  string `json:"text"`
}

// handleFortressMemberStatus reads what a member without pages of its own,
// such as a plugin, shows in place of them: its status and the end of its
// log, in the language the page asks for.
// GET /api/fortress/members/{id}/status
func (h *Handler) handleFortressMemberStatus(w http.ResponseWriter, r *http.Request) {
	f := h.svc.fortressNow()
	if f == nil {
		writeFortressError(w, errNoFortress)
		return
	}
	id, lang := r.PathValue("id"), r.URL.Query().Get("lang")
	ctx, cancel := context.WithTimeout(r.Context(), moveTimeout)
	defer cancel()
	rep, err := f.Status(ctx, id, lang)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "code": openRefusal(err), "error": scrubError(err)})
		return
	}
	v := memberStatusView{OK: true, Report: memberReport(rep), Log: memberLogTail(ctx, f, id, lang)}
	writeJSON(w, http.StatusOK, v)
}

func memberReport(rep status.Report) memberReportView {
	v := memberReportView{State: string(rep.State), NeedsYou: []memberNeedView{}, Work: []memberWorkView{}, Next: []memberNextView{}}
	for _, n := range rep.NeedsYou {
		v.NeedsYou = append(v.NeedsYou, memberNeedView{Severity: string(n.Severity), Text: n.Text})
	}
	for _, t := range rep.Work {
		v.Work = append(v.Work, memberWorkView{Title: t.Title, Progress: t.Progress})
	}
	for _, s := range rep.Next {
		v.Next = append(v.Next, memberNextView{Title: s.Title, At: s.At.Unix()})
	}
	return v
}

// memberLogTail reads the member's log from its start, which is the only way
// the status contract pages, and keeps the newest lines. A log that does not
// answer leaves the window without lines.
func memberLogTail(ctx context.Context, f *fortress.Fortress, id, lang string) []memberLogLine {
	var tail []status.LogEntry
	after := ""
	for range memberLogPages {
		page, err := f.Log(ctx, id, status.LogQuery{After: after, Limit: status.MaxLogLimit, Lang: lang})
		if err != nil {
			break
		}
		tail = append(tail, page.Entries...)
		if len(tail) > memberStatusLog {
			tail = tail[len(tail)-memberStatusLog:]
		}
		if len(page.Entries) < status.MaxLogLimit || page.Next == after {
			break
		}
		after = page.Next
	}
	out := make([]memberLogLine, 0, len(tail))
	for _, e := range slices.Backward(tail) {
		out = append(out, memberLogLine{At: e.At.Unix(), Level: string(e.Level), Text: e.Text})
	}
	return out
}
