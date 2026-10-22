package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
)

// A paired phone away from home opens this instance's interface through the
// group. Each request of the page travels as one sealed call to
// POST /api/group/peer/page, runs through the same handler a browser reaches,
// and comes back with its status, a few headers and the body.
const (
	// pagePart is the most of an answer one call carries. A part is sealed
	// and encoded twice on its way, and the relay gives each frame a few
	// seconds to reach a phone on a slow mobile connection.
	pagePart = 256 << 10
	// pageLimit caps one answer as the handler wrote it, compressed, and with
	// it what one call can make this instance hold.
	pageLimit = 16 << 20
	// pageAskLimit caps a request: the 1 MiB a handler reads at most, encoded.
	pageAskLimit = 2 << 20
	// pagePartsTTL is how long the rest of a large answer waits for the
	// phone, which fetches it right away.
	pagePartsTTL = 20 * time.Second
	// pagePartsBudget bounds what waiting answers may hold in memory, and
	// pagePartsPerSender what one phone's may, so one cannot hold it all.
	pagePartsBudget    = 64 << 20
	pagePartsPerSender = 24 << 20
)

// pageHeadersIn are the request headers a page may send. The session is the
// one the phone names in its call, never a cookie from the page.
var pageHeadersIn = []string{"Content-Type", "Accept", "Sec-Fetch-Site", "Sec-Fetch-Mode", "Sec-Fetch-Dest"}

// pageHeadersOut are the response headers the phone gets back. Set-Cookie is
// not among them: the phone keeps its session itself.
var pageHeadersOut = []string{
	"Content-Type", "Content-Encoding", "Cache-Control", "Location",
	"Content-Security-Policy", "X-Content-Type-Options", "Referrer-Policy", "Permissions-Policy",
}

var pageMethods = map[string]bool{
	http.MethodGet: true, http.MethodHead: true, http.MethodPost: true,
	http.MethodPut: true, http.MethodPatch: true, http.MethodDelete: true,
}

// relayedPublic are the routes outside pairedRoutes a page over the relay
// reaches: the two the shell asks before anything else. Signing in, the
// password reset and the doors that gate themselves on a token stay at the
// instance's own address.
var relayedPublic = map[string]bool{
	"GET /api/auth":   true,
	"GET /api/health": true,
}

// relayedRefused are routes of a phone's session a page over the relay does
// not reach. Signing out would clear a cookie the phone does not keep there.
var relayedRefused = map[string]bool{
	"POST /api/logout": true,
}

type relayedKey struct{}

// relayedPage reports whether a request is a page a paired phone asked for
// through the group. authGate holds it to the routes of a phone's session
// whether or not a login password is set.
func relayedPage(r *http.Request) bool {
	on, _ := r.Context().Value(relayedKey{}).(bool)
	return on
}

// peerPageAsk is one request of a page, or a request for the next part of an
// answer too large for one call.
type peerPageAsk struct {
	Method string            `json:"method,omitempty"`
	Path   string            `json:"path,omitempty"`
	Header map[string]string `json:"header,omitempty"`
	Body   []byte            `json:"body,omitempty"`
	Part   string            `json:"part,omitempty"`
	Offset int               `json:"offset,omitempty"`
}

// peerPageCall is what a phone sends: the ask as it signed it, the session
// it holds on this instance, and its signature over the ask.
type peerPageCall struct {
	Ask     []byte `json:"ask"`
	Session string `json:"session,omitempty"`
	peerProof
}

// peerPageAnswer is what the page's handler answered. Size is the length of
// the whole body; when it is longer than Body, Part names the rest. Proof is
// this instance's signature over the whole answer, so a member that takes
// over this instance's place on the relay cannot answer in its name.
type peerPageAnswer struct {
	OK     bool              `json:"ok"`
	Status int               `json:"status,omitempty"`
	Header map[string]string `json:"header,omitempty"`
	Body   []byte            `json:"body,omitempty"`
	Size   int               `json:"size"`
	Part   string            `json:"part,omitempty"`
	Proof  []byte            `json:"proof,omitempty"`
}

// codeSessionEnded tells a phone that the session it named has ended, through
// a sign-out everywhere or a reset, and that it has to ask for a new one.
const codeSessionEnded = "session-ended"

// ServePhonePages sets the handler a page over the relay runs through: the
// whole of what a browser reaches, security headers included.
func (s *Service) ServePhonePages(h http.Handler) { s.pages.Store(&h) }

// handlePeerPage answers one request of a page a paired phone opens through
// the group.
// POST /api/group/peer/page
func (s *Service) handlePeerPage(w http.ResponseWriter, r *http.Request) {
	var call peerPageCall
	var ask peerPageAsk
	if json.NewDecoder(io.LimitReader(r.Body, pageAskLimit)).Decode(&call) != nil || json.Unmarshal(call.Ask, &ask) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "the request could not be read"})
		return
	}
	sender := peerSender(r)
	if !s.admitPhone(w, sender) {
		return
	}
	if err := s.verifyPhone(r.Context(), sender, keydeposit.ProofPage, call.Ask, call.peerProof); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]any{"ok": false, "error": errUnsignedCall.Error()})
		return
	}
	if ask.Part != "" {
		body, size, ok := s.pageParts.take(ask.Part, sender, ask.Offset)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "that answer is gone; load the page again"})
			return
		}
		writeJSON(w, http.StatusOK, peerPageAnswer{OK: true, Body: body, Size: size})
		return
	}
	pages := s.pages.Load()
	if pages == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "no pages here"})
		return
	}
	req, err := pageRequest(r.Context(), ask)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if s.pageSession == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "no pages here"})
		return
	}
	cookie, ok := s.pageSession(sender, call.Session)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "the session has ended", "code": codeSessionEnded})
		return
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := &pageRecorder{header: http.Header{}}
	(*pages).ServeHTTP(rec, req)
	answer := rec.answer()
	if answer.Proof, err = s.signPageAnswer(sender, call, answer); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if len(answer.Body) > pagePart {
		token, ok := s.pageParts.keep(sender, answer.Body)
		if !ok {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": "too much is waiting to go through the relay; try again in a minute"})
			return
		}
		answer.Body, answer.Part = answer.Body[:pagePart], token
	}
	writeJSON(w, http.StatusOK, answer)
}

// signPageAnswer signs what goes back to the phone, bound to the call it
// answers by the call's time and the digest of its ask.
func (s *Service) signPageAnswer(phoneID string, call peerPageCall, a peerPageAnswer) ([]byte, error) {
	seed, err := s.depositSeed()
	if err != nil {
		return nil, err
	}
	g, err := s.store.GetGroupState()
	if err != nil {
		return nil, err
	}
	claim := keydeposit.Claim{Action: keydeposit.ProofPageAnswer, OwnerID: g.InstanceID, KeeperID: phoneID, Counter: call.Counter, Deposit: pageAnswerDigest(call.Ask, a)}
	return keydeposit.Prove(seed, claim), nil
}

// pageAnswerDigest is what the signature over an answer covers: the ask it
// answers, the status, every header and the whole body, each with its
// length. The app spells it the same way.
func pageAnswerDigest(ask []byte, a peerPageAnswer) []byte {
	h := sha256.New()
	h.Write([]byte("bombvault page answer 1\x00"))
	field := func(b []byte) {
		h.Write(binary.BigEndian.AppendUint64(nil, uint64(len(b))))
		h.Write(b)
	}
	sum := sha256.Sum256(ask)
	field(sum[:])
	field([]byte(strconv.Itoa(a.Status)))
	names := make([]string, 0, len(a.Header))
	for k := range a.Header {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		field([]byte(k))
		field([]byte(a.Header[k]))
	}
	sum = sha256.Sum256(a.Body)
	field(sum[:])
	return h.Sum(nil)
}

// pageRequest turns a page's request into one for this instance's handler,
// marked as relayed so authGate holds it to the routes of a phone.
func pageRequest(ctx context.Context, ask peerPageAsk) (*http.Request, error) {
	method := strings.ToUpper(ask.Method)
	if !pageMethods[method] {
		return nil, errors.New("that method does not go through the relay")
	}
	if !strings.HasPrefix(ask.Path, "/") || strings.HasPrefix(ask.Path, "//") {
		return nil, errors.New("the path has to start with one slash")
	}
	u, err := url.ParseRequestURI(ask.Path)
	if err != nil || u.Host != "" {
		return nil, errors.New("the path could not be read")
	}
	// The live stream never ends, and a call has to. The phone asks again
	// for what runs now instead.
	if u.Path == "/api/progress" {
		q := u.Query()
		q.Set("snapshot", "1")
		u.RawQuery = q.Encode()
	}
	req, err := http.NewRequestWithContext(context.WithValue(ctx, relayedKey{}, true), method, u.RequestURI(), bytes.NewReader(ask.Body))
	if err != nil {
		return nil, err
	}
	for _, name := range pageHeadersIn {
		for k, v := range ask.Header {
			if strings.EqualFold(k, name) {
				req.Header.Set(name, v)
			}
		}
	}
	req.Header.Set("Accept-Encoding", "gzip")
	return req, nil
}

var errPageTooLarge = errors.New("the answer is too large for the relay")

// pageRecorder keeps what a handler answers, up to pageLimit. Its Flush does
// nothing; the progress handler only has to find one.
type pageRecorder struct {
	header http.Header
	status int
	body   bytes.Buffer
	over   bool
}

func (p *pageRecorder) Header() http.Header { return p.header }

func (p *pageRecorder) WriteHeader(status int) {
	if p.status == 0 {
		p.status = status
	}
}

func (p *pageRecorder) Write(b []byte) (int, error) {
	p.WriteHeader(http.StatusOK)
	if p.body.Len()+len(b) > pageLimit {
		p.over = true
		return 0, errPageTooLarge
	}
	return p.body.Write(b)
}

func (p *pageRecorder) Flush() {}

// answer is what goes back to the phone. A download is refused: files are
// what a backup is made of, and the relay is no way to carry them off.
func (p *pageRecorder) answer() peerPageAnswer {
	if p.over {
		return pageRefusal(http.StatusRequestEntityTooLarge, "this is too large to go through the relay; open the instance at its own address")
	}
	if strings.Contains(strings.ToLower(p.header.Get("Content-Disposition")), "attachment") {
		return pageRefusal(http.StatusForbidden, "files are saved at the instance's own address, not through the relay")
	}
	status := p.status
	if status == 0 {
		status = http.StatusOK
	}
	header := map[string]string{}
	for _, name := range pageHeadersOut {
		if v := p.header.Get(name); v != "" {
			header[name] = v
		}
	}
	return peerPageAnswer{OK: true, Status: status, Header: header, Body: p.body.Bytes(), Size: p.body.Len()}
}

func pageRefusal(status int, msg string) peerPageAnswer {
	body, _ := json.Marshal(map[string]any{"ok": false, "error": msg, "code": codeRelayRefused})
	return peerPageAnswer{
		OK:     true,
		Status: status,
		Header: map[string]string{"Content-Type": "application/json"},
		Body:   body,
		Size:   len(body),
	}
}

// codeRelayRefused marks what a page over the relay cannot have.
const codeRelayRefused = "relay-refused"

// pageParts holds the rest of answers too large for one call until the phone
// that asked has fetched them.
type pageParts struct {
	mu      sync.Mutex
	entries map[string]*pageEntry
	held    int
	by      map[string]int
}

// pageEntry is one waiting answer. left counts the parts not fetched yet,
// whose offsets are in taken as false; the phone fetches them in parallel.
type pageEntry struct {
	sender  string
	body    []byte
	taken   []bool
	left    int
	expires time.Time
}

// keep stores body for sender, its first part counted as handed out with the
// first answer, and returns the token the rest is fetched by. It refuses
// while the waiting answers hold the whole budget.
func (p *pageParts) keep(sender string, body []byte) (string, bool) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", false
	}
	token := hex.EncodeToString(raw)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sweepLocked(time.Now())
	if p.held+len(body) > pagePartsBudget || p.by[sender]+len(body) > pagePartsPerSender {
		return "", false
	}
	if p.entries == nil {
		p.entries, p.by = map[string]*pageEntry{}, map[string]int{}
	}
	parts := (len(body) + pagePart - 1) / pagePart
	e := &pageEntry{sender: sender, body: body, taken: make([]bool, parts), left: parts - 1, expires: time.Now().Add(pagePartsTTL)}
	e.taken[0] = true
	p.entries[token] = e
	p.held += len(body)
	p.by[sender] += len(body)
	return token, true
}

// take returns the part of token's answer at offset and the answer's whole
// size, for the sender that asked for it only. Once every part has been
// fetched the entry is gone.
func (p *pageParts) take(token, sender string, offset int) ([]byte, int, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sweepLocked(time.Now())
	e := p.entries[token]
	if e == nil || e.sender != sender || offset < 0 || offset >= len(e.body) || offset%pagePart != 0 {
		return nil, 0, false
	}
	if i := offset / pagePart; !e.taken[i] {
		e.taken[i] = true
		e.left--
	}
	part := e.body[offset:min(offset+pagePart, len(e.body))]
	if e.left == 0 {
		p.dropLocked(token, e)
	}
	return part, len(e.body), true
}

func (p *pageParts) sweepLocked(now time.Time) {
	for token, e := range p.entries {
		if now.After(e.expires) {
			p.dropLocked(token, e)
		}
	}
}

func (p *pageParts) dropLocked(token string, e *pageEntry) {
	delete(p.entries, token)
	p.held -= len(e.body)
	if p.by[e.sender] -= len(e.body); p.by[e.sender] <= 0 {
		delete(p.by, e.sender)
	}
}
