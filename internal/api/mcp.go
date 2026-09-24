package api

import (
	"bytes"
	"context"
	"crypto/subtle"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The endpoint an assistant talks to, and the budget one key gets there. The
// settings card reads all three numbers off the API rather than repeating them.
const (
	mcpEndpointPath = "/mcp"

	mcpStartsPerHour    = 12
	mcpStartCooldown    = 15 * time.Minute
	mcpItemStartsPerDay = 4
)

// mcpSDKVersion is the protocol library the interop checks ran against. A bump
// moves the protocol surface and the SDK's own security defaults, so it goes in
// on its own and with the interop checks re-run.
const mcpSDKVersion = "v1.8.0"

const (
	mcpMaxBody        = 1 << 20
	mcpCallsPerMinute = 120
	mcpTouchEvery     = time.Minute
	mcpAuthLogEvery   = 10 * time.Second

	// How many addresses the auth-failure throttle remembers. Everything older
	// than mcpAuthLogEvery is dead weight, so the sweep at this mark almost
	// always empties the map; the cap is what holds a caller rotating through
	// an address range to a fixed amount of memory.
	mcpAuthLogMax = 1024

	// How long a tool that reads the database and Docker may take. Shutdown
	// ends it too, so nothing outlives the process.
	mcpReadTimeout = 15 * time.Second

	mcpAuthRealm = `Bearer realm="bombvault-mcp"`

	// The JSON-RPC 2.0 code for a request the server refuses to look at.
	mcpInvalidRequest = -32600
)

// mcpResticTimeout is how long a tool that spawns restic may take; listing a
// remote repository is the slow case it is cut for. A variable, so a test can
// shorten it.
var mcpResticTimeout = time.Minute

// mcpCaller is the key a request came in with. The gate puts it into the
// request context, which is how a tool learns who is asking.
type mcpCaller struct {
	KeyID string
	// Label is the operator's name for the key. get_health shows it to the
	// assistant; it never reaches a log line.
	Label           string
	Hint            string
	CanStartBackups bool
	ClientAddr      string
}

type mcpCallerKey struct{}

func withMCPCaller(ctx context.Context, c mcpCaller) context.Context {
	return context.WithValue(ctx, mcpCallerKey{}, c)
}

func mcpCallerFrom(ctx context.Context) (mcpCaller, bool) {
	c, ok := ctx.Value(mcpCallerKey{}).(mcpCaller)
	return c, ok
}

// mcpState is what the endpoint keeps between requests. NewHandler creates it,
// and Router() creates one for the zero-value Handlers the route-registration
// tests build. It is never shared between Handlers.
type mcpState struct {
	http  http.Handler
	calls *slidingWindow

	// starts is the hourly budget one key has for launching backups. A slot is
	// reserved before the service is asked and given back unless a backup
	// began.
	starts *slidingWindow

	// listSem is the single slot the restic-spawning tools share, so a key
	// cannot put a dozen listings on one repository at once.
	listSem chan struct{}

	touchMu sync.Mutex
	touched map[string]int64

	authLogMu sync.Mutex
	authLog   map[string]int64

	countMu   sync.Mutex
	requests  map[string]uint64
	toolCalls map[mcpToolOutcome]uint64

	// now is time.Now; internal tests replace it with a fixed clock.
	now func() time.Time
}

// mcpToolOutcome is one cell of the tool-call counter: which tool answered
// what.
type mcpToolOutcome struct {
	tool    string
	outcome string
}

func newMCPState() *mcpState {
	return &mcpState{
		calls:     newSlidingWindow(time.Minute, mcpCallsPerMinute),
		starts:    newSlidingWindow(time.Hour, mcpStartsPerHour),
		listSem:   make(chan struct{}, 1),
		touched:   map[string]int64{},
		authLog:   map[string]int64{},
		requests:  map[string]uint64{},
		toolCalls: map[mcpToolOutcome]uint64{},
		now:       time.Now,
	}
}

// buildMCPHTTP constructs the server and the transport in front of it. Router()
// calls it, so a bad tool registration fails every router test in CI instead of
// panicking at boot.
func (h *Handler) buildMCPHTTP() http.Handler {
	srv := h.newMCPServer()

	getServer := func(r *http.Request) *mcp.Server {
		if _, ok := mcpCallerFrom(r.Context()); !ok {
			// The SDK answers 400. Only a request that skipped the gate can
			// arrive without a caller.
			return nil
		}
		return srv
	}

	return mcp.NewStreamableHTTPHandler(getServer, &mcp.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
		// The SDK's rule refuses a request that reached a loopback address
		// under any other name, before any key is read, which is what
		// tailscale serve and a reverse proxy in the same network namespace
		// both look like. It is meant for unauthenticated localhost servers.
		// Here the key is the defence: a DNS-rebinding page cannot know one,
		// and it cannot mint one either, because creating a key while no login
		// password is set only works from a host a public DNS name cannot
		// rebind (mcpKeyHostAllowed).
		DisableLocalhostProtection:   true,
		MaxRequestBodyBytes:          mcpMaxBody,
		PropagateRequestCancellation: true,
	})
}

// serveMCP is the gate in front of the transport: feature switch, origin,
// address throttle, key, per-key budget and batch shape, in that order. Every
// outcome is counted for /metrics.
func (h *Handler) serveMCP(w http.ResponseWriter, r *http.Request) {
	keys, err := h.store.ActiveMCPKeys()
	if err != nil {
		log.Printf("api: mcp: could not read the keys: %v", err)
		h.countMCPRequest("unavailable")
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "mcp unavailable"})
		return
	}
	if len(keys) == 0 {
		h.countMCPRequest("not_found")
		http.NotFound(w, r)
		return
	}
	if foreignOrigin(r) {
		h.countMCPRequest("forbidden_origin")
		writeMCPJSONRPCError(w, http.StatusForbidden, mcpInvalidRequest, "cross-origin requests are not accepted")
		return
	}

	addr := h.loginClientKey(r)
	bucket := "mcp|" + addr
	if h.loginThrottled(bucket) {
		h.countMCPRequest("throttled")
		w.Header().Set("Retry-After", strconv.Itoa(int(loginWindow.Seconds())))
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "too many failed attempts, wait a minute"})
		return
	}

	presented, present, conflict := presentedMCPKey(r)
	if !present {
		h.countMCPRequest("no_key")
		w.Header().Set("WWW-Authenticate", mcpAuthRealm)
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "an MCP key is required"})
		return
	}
	k, match := matchMCPKey(secret.HashMCPKey(h.cfg.AppKey, presented), keys)
	if conflict || !match {
		h.recordLoginFail(bucket)
		h.logMCPAuthFailure(addr, conflict)
		h.countMCPRequest("invalid_key")
		w.Header().Set("WWW-Authenticate", mcpAuthRealm+`, error="invalid_token"`)
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "the MCP key is not valid"})
		return
	}

	// A success leaves the failure bucket where it is. Behind a shared proxy
	// address an assistant polling legitimately would otherwise reset a
	// guesser's count with every call, and the throttle would never engage.
	now := h.mcp.now()
	if allowed, retry := h.mcp.calls.allow(k.ID, now); !allowed {
		h.countMCPRequest("rate_limited")
		w.Header().Set("Retry-After", retryAfterSeconds(retry))
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "too many requests for this key"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, mcpMaxBody)
	// A body that already declares more than the limit is refused for its size
	// by the transport below, whatever its first byte says.
	if r.ContentLength <= mcpMaxBody {
		if batch, _ := isBatchBody(r); batch {
			h.countMCPRequest("batch_refused")
			writeMCPJSONRPCError(w, http.StatusBadRequest, mcpInvalidRequest, "batch requests are not accepted")
			return
		}
	}

	h.touchMCPKey(k, addr, now)
	h.countMCPRequest("ok")
	ctx := withMCPCaller(r.Context(), mcpCaller{
		KeyID:           k.ID,
		Label:           k.Label,
		Hint:            k.Hint,
		CanStartBackups: k.CanStartBackups,
		ClientAddr:      addr,
	})
	h.mcp.http.ServeHTTP(w, r.WithContext(ctx))
}

// presentedMCPKey reads the key from Authorization: Bearer or from X-API-Key,
// which mcp-remote uses because its argument handling breaks a header value
// containing a space on some platforms. conflict means both arrived and they
// disagree.
func presentedMCPKey(r *http.Request) (key string, present, conflict bool) {
	var bearer string
	if v := strings.TrimSpace(r.Header.Get("Authorization")); v != "" {
		if scheme, rest, found := strings.Cut(v, " "); found && strings.EqualFold(scheme, "bearer") {
			bearer = strings.TrimSpace(rest)
		}
	}
	apiKey := strings.TrimSpace(r.Header.Get("X-API-Key"))
	switch {
	case bearer != "" && apiKey != "":
		return bearer, true, bearer != apiKey
	case bearer != "":
		return bearer, true, false
	case apiKey != "":
		return apiKey, true, false
	}
	return "", false, false
}

// foreignOrigin reports an Origin header that is present and names something
// other than the host the request claims to have reached. It catches a browser
// page on another origin that csrfGate lets through because it is same-site or
// sends no Sec-Fetch-Site at all. It does not catch DNS rebinding, whose page
// carries the attacker's name in both headers; the key does that.
func foreignOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	if origin == "null" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return true
	}
	return !strings.EqualFold(u.Host, r.Host)
}

// matchMCPKey compares digest against every active key and never stops at the
// first hit, so how long it takes says nothing about which key was presented.
func matchMCPKey(digest string, keys []store.MCPKey) (store.MCPKey, bool) {
	var found store.MCPKey
	var ok bool
	for _, k := range keys {
		if subtle.ConstantTimeCompare([]byte(digest), []byte(k.Digest)) == 1 {
			found, ok = k, true
		}
	}
	return found, ok
}

// touchMCPKey records when and from where a key was last used, at most once per
// mcpTouchEvery, so an assistant at work does not add a write per request to
// the single SQLite connection. A failed write is logged and nothing more.
func (h *Handler) touchMCPKey(k store.MCPKey, addr string, now time.Time) {
	h.mcp.touchMu.Lock()
	recent := now.Unix()-h.mcp.touched[k.ID] < int64(mcpTouchEvery.Seconds())
	if !recent {
		h.mcp.touched[k.ID] = now.Unix()
	}
	h.mcp.touchMu.Unlock()
	if recent {
		return
	}
	if err := h.store.TouchMCPKey(k.ID, now.Unix(), addr); err != nil {
		log.Printf("api: mcp: key %s ...%s: could not record its last use: %v", k.ID, k.Hint, err)
	}
}

// logMCPAuthFailure names the address and the reason at most once per
// mcpAuthLogEvery per address, so a looping client cannot crowd everything else
// out of the log ring the diagnostics bundle ships.
func (h *Handler) logMCPAuthFailure(addr string, conflict bool) {
	if !h.mcpAuthLogDue(addr) {
		return
	}
	reason := "the key does not match any active one"
	if conflict {
		reason = "Authorization and X-API-Key carry different keys"
	}
	log.Printf("api: mcp: refused a request from %s: %s", addr, reason)
}

// mcpAuthLogDue reports whether this address's refusal is the one to write, and
// records it when it is. At mcpAuthLogMax addresses the entries whose quiet
// window has run out go first; while a flood keeps the map full even after
// that, the endpoint stays silent rather than remembering every address it was
// ever reached from.
func (h *Handler) mcpAuthLogDue(addr string) bool {
	now := h.mcp.now().Unix()
	cutoff := now - int64(mcpAuthLogEvery.Seconds())
	h.mcp.authLogMu.Lock()
	defer h.mcp.authLogMu.Unlock()
	if h.mcp.authLog[addr] > cutoff {
		return false
	}
	if len(h.mcp.authLog) >= mcpAuthLogMax {
		for seen, at := range h.mcp.authLog {
			if at <= cutoff {
				delete(h.mcp.authLog, seen)
			}
		}
		if len(h.mcp.authLog) >= mcpAuthLogMax {
			return false
		}
	}
	h.mcp.authLog[addr] = now
	return true
}

// writeMCPJSONRPCError answers with a JSON-RPC error envelope. The id is null
// because the gate decides before the request has been parsed, and JSON-RPC 2.0
// wants the member either way; clients that validate against the schema reject
// a response without it.
func writeMCPJSONRPCError(w http.ResponseWriter, status, code int, msg string) {
	writeJSON(w, status, map[string]any{
		"jsonrpc": "2.0",
		"id":      nil,
		"error":   map[string]any{"code": code, "message": msg},
	})
}

// rewoundBody hands the bytes isBatchBody read back to the next reader ahead of
// the rest of the request body.
type rewoundBody struct {
	io.Reader
	io.Closer
}

// isBatchBody reports whether the body's first non-whitespace byte is '[', the
// shape of a JSON-RPC batch. Whatever it read is put back in front of the rest,
// so the transport below reads the body unchanged, and a read error travels
// with it rather than being answered here: the transport turns an oversized
// body into 413 exactly as it would without the peek.
func isBatchBody(r *http.Request) (bool, error) {
	var peeked []byte
	rest := r.Body
	defer func() {
		r.Body = rewoundBody{Reader: io.MultiReader(bytes.NewReader(peeked), rest), Closer: rest}
	}()

	buf := make([]byte, 512)
	for {
		n, err := rest.Read(buf)
		peeked = append(peeked, buf[:n]...)
		if i := firstJSONToken(peeked); i >= 0 {
			return peeked[i] == '[', nil
		}
		if err != nil {
			return false, err
		}
	}
}

// firstJSONToken returns the index of the first byte that is not JSON
// whitespace, or -1 while there is none yet.
func firstJSONToken(b []byte) int {
	for i, c := range b {
		switch c {
		case ' ', '\t', '\n', '\r':
		default:
			return i
		}
	}
	return -1
}

// retryAfterSeconds is a wait in the form a Retry-After header carries.
func retryAfterSeconds(d time.Duration) string {
	return strconv.Itoa(secondsUntil(d))
}

// mcpToolContext gives a tool its own deadline and ends it when the service
// starts shutting down, so a listing that outlives its request cannot outlive
// the process.
func (h *Handler) mcpToolContext(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(ctx, d)
	stop := context.AfterFunc(h.svc.StopContext(), cancel)
	return ctx, func() {
		stop()
		cancel()
	}
}

// acquireList takes the listing slot without waiting for it. A call that finds
// it taken answers busy rather than queueing behind a listing that may hold it
// for a minute.
func (s *mcpState) acquireList() (release func(), ok bool) {
	select {
	case s.listSem <- struct{}{}:
		return func() { <-s.listSem }, true
	default:
		return nil, false
	}
}

func (h *Handler) countMCPRequest(outcome string) {
	h.mcp.countMu.Lock()
	h.mcp.requests[outcome]++
	h.mcp.countMu.Unlock()
}

func (h *Handler) countMCPToolCall(tool, outcome string) {
	h.mcp.countMu.Lock()
	h.mcp.toolCalls[mcpToolOutcome{tool: tool, outcome: outcome}]++
	h.mcp.countMu.Unlock()
}
