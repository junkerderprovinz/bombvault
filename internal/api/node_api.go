package api

import (
	"bytes"
	"cmp"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/store"
	kitgroup "github.com/junkerderprovinz/parleyport/kit/group"
)

// nodeAPIPath is where home passes a request to a node's own routes.
const nodeAPIPath = "/node/api"

// nodeRoutes are the routes of a node home may reach, as the router
// registers them: reading the containers, folder sets, runs and anomalies,
// browsing the folders a set can point at, and backing up and restoring the
// entries. Nothing that deletes, makes folders, or reaches the settings, the
// storage, the keys, the group or the tokens, is among them.
var nodeRoutes = []string{
	"GET /api/status",
	"GET /api/runs",
	"GET /api/anomalies/summary",
	"GET /api/containers",
	"GET /api/containers/{name}",
	"GET /api/containers/{name}/mounts",
	"GET /api/containers/{name}/snapshots",
	"PATCH /api/containers/{name}",
	"POST /api/containers/{name}/backup",
	"POST /api/containers/{name}/restore",
	"GET /api/files",
	"GET /api/files/sets/{id}/snapshots",
	"POST /api/files/sets",
	"PATCH /api/files/sets/{id}",
	"POST /api/files/sets/{id}/backup",
	"POST /api/files/sets/{id}/restore",
	"POST /api/backup/cancel",
	"GET /api/browse",
}

// nodeRouteShadows are routes of the router that a wildcard of nodeRoutes
// would match too. The router takes them by their fixed name, so they are no
// node route.
var nodeRouteShadows = []string{
	"GET /api/containers/backup-order",
}

// nodeRouteMux matches a request against nodeRoutes alone.
var nodeRouteMux = func() *http.ServeMux {
	mux := http.NewServeMux()
	for _, r := range nodeRoutes {
		mux.Handle(r, http.NotFoundHandler())
	}
	for _, r := range nodeRouteShadows {
		mux.Handle(r, http.NotFoundHandler())
	}
	return mux
}()

// nodeRoute returns the pattern of nodeRoutes r matches, "" for none. The mux
// answers an unclean path with a redirect and no pattern, so a path that
// climbs out of a listed one matches none.
func nodeRoute(r *http.Request) string {
	_, pattern := nodeRouteMux.Handler(r)
	if slices.Contains(nodeRouteShadows, pattern) {
		return ""
	}
	return pattern
}

// nodeAPIRequest is a request home passes to a node.
type nodeAPIRequest struct {
	Method string          `json:"method"`
	Path   string          `json:"path"`
	Query  string          `json:"query,omitempty"`
	Body   json.RawMessage `json:"body,omitempty"`
}

// request is req as the node's router takes it, under ctx. It refuses a path
// that names a host or carries a query of its own.
func (req nodeAPIRequest) request(ctx context.Context) (*http.Request, error) {
	if !strings.HasPrefix(req.Path, "/") || strings.HasPrefix(req.Path, "//") || strings.ContainsAny(req.Path, "?#") {
		return nil, errNodeRoute
	}
	u := &url.URL{Path: req.Path, RawQuery: req.Query}
	r, err := http.NewRequestWithContext(ctx, req.Method, u.RequestURI(), bytes.NewReader(req.Body))
	if err != nil {
		return nil, errNodeRoute
	}
	if len(req.Body) > 0 {
		r.Header.Set("Content-Type", "application/json")
	}
	return r, nil
}

// nodeAPIAnswer is a node's answer to a request home passed on. Body is the
// answer's JSON, compressed with gzip.
type nodeAPIAnswer struct {
	OK     bool   `json:"ok"`
	Status int    `json:"status"`
	Body   []byte `json:"body"`
}

// body is the answer's JSON.
func (a nodeAPIAnswer) body() ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(a.Body))
	if err != nil {
		return nil, err
	}
	defer zr.Close() //nolint:errcheck // reading from memory
	return io.ReadAll(io.LimitReader(zr, pageLimit))
}

var (
	errNodeRoute   = errors.New("that request is not passed to a node")
	errNodeNotJSON = errors.New("the node answered with something other than JSON")
)

// ServeNodeAPI sets the router the requests home passes on run through.
func (s *Service) ServeNodeAPI(h http.Handler) { s.nodeAPI.Store(&h) }

// api runs a request home passed on through this node's own router, as a
// request home started, and answers with its JSON compressed.
func (n *nodeSide) api(w http.ResponseWriter, r *http.Request) {
	call, ok := n.signed(w, r, keydeposit.ProofNodeAPI)
	if !ok || !n.adopted(w) {
		return
	}
	router := n.s.nodeAPI.Load()
	if router == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": "this node does not take requests yet"})
		return
	}
	var req nodeAPIRequest
	if json.Unmarshal(call.Body, &req) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "the request could not be read"})
		return
	}
	inner, err := req.request(WithRunOrigin(r.Context(), RunOrigin{Via: "home", KeyID: n.home.ID}))
	if err != nil || nodeRoute(inner) == "" {
		writeJSON(w, http.StatusForbidden, map[string]any{"ok": false, "error": errNodeRoute.Error()})
		return
	}
	rec := &pageRecorder{header: http.Header{}}
	(*router).ServeHTTP(rec, inner)
	if media, _, _ := mime.ParseMediaType(rec.header.Get("Content-Type")); rec.over || media != "application/json" {
		writeJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": errNodeNotJSON.Error()})
		return
	}
	var zipped bytes.Buffer
	zw := gzip.NewWriter(&zipped)
	if _, err := zw.Write(rec.body.Bytes()); err != nil || zw.Close() != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "the answer could not be compressed"})
		return
	}
	writeJSON(w, http.StatusOK, nodeAPIAnswer{OK: true, Status: cmp.Or(rec.status, http.StatusOK), Body: zipped.Bytes()})
}

// CallNodeAPI passes req to the node with id and returns its answer's status
// and JSON.
func (s *Service) CallNodeAPI(ctx context.Context, id string, req nodeAPIRequest) (int, []byte, error) {
	inner, err := req.request(ctx)
	if err != nil || nodeRoute(inner) == "" {
		return 0, nil, errNodeRoute
	}
	n, err := s.store.GetNode(id)
	if err != nil {
		return 0, nil, err
	}
	link, ok := s.nodeLinkOf(id)
	if !ok || n.AdoptedAt == 0 || n.AdoptionDue || n.State != "" {
		return 0, nil, errNodeNotAdopted
	}
	var ans nodeAPIAnswer
	if err := s.callNode(ctx, link, keydeposit.ProofNodeAPI, id, nodeAPIPath, req, &ans); err != nil {
		return 0, nil, err
	}
	body, err := ans.body()
	if err != nil {
		return 0, nil, errNodeNotJSON
	}
	return ans.Status, body, nil
}

// handleNodeAPI passes a request to a node's own routes. It reaches a server
// that holds Docker as root, so where a password is set it takes a person
// signed in for it, never a paired phone or a token.
// GET, POST and PATCH /api/nodes/{id}/api/{path...}
func (h *Handler) handleNodeAPI(w http.ResponseWriter, r *http.Request) {
	if !h.requireSignIn(w, r, "reaching a node") {
		return
	}
	if r.Method != http.MethodGet && !crossOriginGuard(w, r) {
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxNodeCall/2))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "the request could not be read"})
		return
	}
	req := nodeAPIRequest{Method: r.Method, Path: "/api/" + r.PathValue("path"), Query: r.URL.RawQuery}
	if len(bytes.TrimSpace(body)) > 0 {
		req.Body = body
	}
	status, answer, err := h.svc.CallNodeAPI(r.Context(), r.PathValue("id"), req)
	switch {
	case errors.Is(err, errNodeRoute):
		writeJSON(w, http.StatusForbidden, map[string]any{"ok": false, "error": err.Error()})
	case errors.Is(err, store.ErrNodeNotFound):
		writeJSON(w, http.StatusNotFound, codedFailEnvelope(err, "not-found"))
	case errors.Is(err, errNodeNotAdopted):
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": err.Error(), "code": codeNodeNotAdopted})
	case errors.Is(err, kitgroup.ErrUnreachable):
		writeJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": "the node is not reachable right now", "code": "node-away"})
	case err != nil:
		writeJSON(w, http.StatusBadGateway, failEnvelope(err))
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(answer)
	}
}

// handleNodeRoutes lists the routes of a node home may reach, for the pages
// that show a node.
// GET /api/nodes/routes
func (h *Handler) handleNodeRoutes(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"routes": nodeRoutes}))
}
