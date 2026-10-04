package api

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The public API is a small versioned set of routes for scripts and home
// automation. Each route calls the MCP tool that answers the same question, so
// the two share one implementation, one set of start limits and one scrubbing
// of host details, and a token's calls land in its log the way a key's do.
// Everything under /api/v1 needs an API token, whether or not a login password
// is set; only the OpenAPI description is open.

const (
	apiV1Prefix      = "/api/v1/"
	apiV1OpenAPIPath = "/api/v1/openapi.json"
	apiV1AuthRealm   = `Bearer realm="bombvault-api"`
	apiV1MaxBody     = 64 << 10
)

// apiV1OpenAPI is the description of the routes below. TestAPIV1RoutesMatchOpenAPIAndDocs
// keeps the two in step.
//
//go:embed openapi.json
var apiV1OpenAPI []byte

// apiV1Route is one route of the public API. start marks a route that sets
// work going, which a read-only token is refused.
type apiV1Route struct {
	method string
	path   string
	start  bool
	call   func(h *Handler, ctx context.Context, r *http.Request) (*mcp.CallToolResult, error)
}

// errAPIV1Argument is a request the route could not turn into tool arguments.
type errAPIV1Argument struct{ msg string }

func (e errAPIV1Argument) Error() string { return e.msg }

func apiV1Routes() []apiV1Route {
	return []apiV1Route{
		{method: http.MethodGet, path: "/api/v1/health", call: apiV1Tool((*Handler).toolGetHealth, nil)},
		{method: http.MethodGet, path: "/api/v1/status", call: apiV1Tool((*Handler).toolGetStatus, nil)},
		{method: http.MethodGet, path: "/api/v1/activity", call: apiV1Tool((*Handler).toolGetActivity, nil)},
		{method: http.MethodGet, path: "/api/v1/items", call: apiV1Tool((*Handler).toolListItems, apiV1Query(
			queryArg{name: "domain"},
		))},
		{method: http.MethodGet, path: "/api/v1/runs", call: apiV1Tool((*Handler).toolListRuns, apiV1Query(
			queryArg{name: "limit", number: true},
			queryArg{name: "domain"},
			queryArg{name: "item"},
			queryArg{name: "status"},
			queryArg{name: "kind"},
			queryArg{name: "since", number: true},
		))},
		{method: http.MethodGet, path: "/api/v1/anomalies", call: apiV1Tool((*Handler).toolListAnomalies, apiV1Query(
			queryArg{name: "state"},
			queryArg{name: "severity", list: true},
			queryArg{name: "domain"},
			queryArg{name: "limit", number: true},
		))},
		{method: http.MethodGet, path: "/api/v1/anomalies/{id}", call: apiV1Tool((*Handler).toolGetAnomaly, func(r *http.Request) (map[string]any, error) {
			return map[string]any{"id": r.PathValue("id")}, nil
		})},
		{method: http.MethodGet, path: "/api/v1/storage/{domain}", call: apiV1Tool((*Handler).toolGetStorageStats, func(r *http.Request) (map[string]any, error) {
			args, err := apiV1Query(queryArg{name: "limit", number: true})(r)
			if err != nil {
				return nil, err
			}
			args["domain"] = r.PathValue("domain")
			return args, nil
		})},
		{method: http.MethodPost, path: "/api/v1/backups", start: true, call: (*Handler).apiV1StartBackup},
		{method: http.MethodPost, path: "/api/v1/backups/everything", start: true, call: apiV1Tool((*Handler).toolStartBackupEverything, nil)},
		{method: http.MethodPost, path: "/api/v1/runs/{id}/cancel", start: true, call: apiV1Tool((*Handler).toolCancelBackup, func(r *http.Request) (map[string]any, error) {
			return map[string]any{"runId": r.PathValue("id")}, nil
		})},
	}
}

type mcpToolFunc func(h *Handler, ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error)

// apiV1Tool adapts a tool to a route: args reads the tool's arguments off the
// request, and nil means the tool takes none.
func apiV1Tool(tool mcpToolFunc, args func(*http.Request) (map[string]any, error)) func(*Handler, context.Context, *http.Request) (*mcp.CallToolResult, error) {
	return func(h *Handler, ctx context.Context, r *http.Request) (*mcp.CallToolResult, error) {
		in := map[string]any{}
		if args != nil {
			var err error
			if in, err = args(r); err != nil {
				return nil, err
			}
		}
		return callMCPTool(h, ctx, tool, in)
	}
}

func callMCPTool(h *Handler, ctx context.Context, tool mcpToolFunc, args map[string]any) (*mcp.CallToolResult, error) {
	raw, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	return tool(h, ctx, &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: raw}})
}

// queryArg is one query parameter a route passes on to its tool. number turns
// it into an integer, list collects every value and every comma-separated part.
type queryArg struct {
	name   string
	number bool
	list   bool
}

func apiV1Query(params ...queryArg) func(*http.Request) (map[string]any, error) {
	return func(r *http.Request) (map[string]any, error) {
		q := r.URL.Query()
		out := map[string]any{}
		for name := range q {
			known := false
			for _, p := range params {
				known = known || p.name == name
			}
			if !known {
				return nil, errAPIV1Argument{fmt.Sprintf("unknown query parameter %q", name)}
			}
		}
		for _, p := range params {
			values := q[p.name]
			if len(values) == 0 {
				continue
			}
			switch {
			case p.list:
				var all []string
				for _, v := range values {
					for part := range strings.SplitSeq(v, ",") {
						if part = strings.TrimSpace(part); part != "" {
							all = append(all, part)
						}
					}
				}
				out[p.name] = all
			case p.number:
				n, err := strconv.ParseInt(values[0], 10, 64)
				if err != nil {
					return nil, errAPIV1Argument{p.name + " must be a whole number"}
				}
				out[p.name] = n
			default:
				out[p.name] = values[0]
			}
		}
		return out, nil
	}
}

// apiV1StartBackup starts one item when the body names one and the whole
// domain otherwise, the two MCP tools behind a single route.
func (h *Handler) apiV1StartBackup(ctx context.Context, r *http.Request) (*mcp.CallToolResult, error) {
	if mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mediaType != "application/json" {
		return nil, errAPIV1Argument{"send the body as application/json"}
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, apiV1MaxBody+1))
	if err != nil {
		return nil, errAPIV1Argument{"the body could not be read"}
	}
	if len(body) > apiV1MaxBody {
		return nil, errAPIV1Argument{"the body is too large"}
	}
	var in map[string]any
	if err := json.Unmarshal(body, &in); err != nil || in == nil {
		return nil, errAPIV1Argument{`the body must be a JSON object such as {"domain":"containers"}`}
	}
	if item, ok := in["item"]; ok && item != "" {
		return callMCPTool(h, ctx, (*Handler).toolStartBackup, in)
	}
	delete(in, "item")
	return callMCPTool(h, ctx, (*Handler).toolStartDomainBackup, in)
}

// mountAPIV1 registers the public routes and their description.
func (h *Handler) mountAPIV1(mux *http.ServeMux) {
	if h.mcp == nil {
		h.mcp = newMCPState()
	}
	mux.HandleFunc("GET "+apiV1OpenAPIPath, handleAPIV1OpenAPI)
	for _, route := range apiV1Routes() {
		mux.HandleFunc(route.method+" "+route.path, h.serveAPIV1(route))
	}
}

func handleAPIV1OpenAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	if _, err := w.Write(apiV1OpenAPI); err != nil {
		log.Printf("api: v1: send the OpenAPI description: %v", err)
	}
}

// serveAPIV1 is the gate in front of one route: origin, address throttle,
// token, the token's request budget, then the tool. It mirrors serveMCP, and a
// token's failures count against the same per-address throttle as a login.
func (h *Handler) serveAPIV1(route apiV1Route) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokens, err := h.store.ActiveAPITokens()
		if err != nil {
			log.Printf("api: v1: could not read the tokens: %v", err)
			writeAPIV1Error(w, http.StatusServiceUnavailable, "unavailable", "the API is unavailable")
			return
		}
		if foreignOrigin(r) {
			writeAPIV1Error(w, http.StatusForbidden, "forbidden_origin", "cross-origin requests are not accepted")
			return
		}
		addr := h.loginClientKey(r)
		bucket := "api|" + addr
		if h.loginThrottled(bucket) {
			w.Header().Set("Retry-After", strconv.Itoa(int(loginWindow.Seconds())))
			writeAPIV1Error(w, http.StatusTooManyRequests, "throttled", "too many failed attempts, wait a minute")
			return
		}
		presented, present, conflict := presentedMCPKey(r)
		if !present {
			w.Header().Set("WWW-Authenticate", apiV1AuthRealm)
			writeAPIV1Error(w, http.StatusUnauthorized, "no_token", "an API token is required")
			return
		}
		k, match := matchMCPKey(secret.HashMCPKey(h.cfg.AppKey, presented), tokens)
		if conflict || !match {
			h.recordLoginFail(bucket)
			if h.mcpAuthLogDue(addr) {
				log.Printf("api: v1: refused a request from %s: the token does not match any active one", addr)
			}
			w.Header().Set("WWW-Authenticate", apiV1AuthRealm+`, error="invalid_token"`)
			writeAPIV1Error(w, http.StatusUnauthorized, "invalid_token", "the API token is not valid")
			return
		}

		now := h.mcp.now()
		if allowed, retry := h.mcp.calls.allow(k.ID, now); !allowed {
			h.recordMCPRefusal(k.ID, "rate_limited", now)
			w.Header().Set("Retry-After", retryAfterSeconds(retry))
			writeAPIV1Error(w, http.StatusTooManyRequests, "rate_limited", "too many requests for this token")
			return
		}
		h.touchMCPKey(k, addr, now)

		ctx := withMCPCaller(r.Context(), mcpCaller{
			Via:             viaAPI,
			KeyID:           k.ID,
			Label:           k.Label,
			Hint:            k.Hint,
			CanStartBackups: k.CanStartBackups,
			ClientAddr:      addr,
		})
		res, err := route.call(h, ctx, r)
		var bad errAPIV1Argument
		switch {
		case errors.As(err, &bad):
			h.recordMCPEvent(k.ID, store.MCPKeyEvent{At: now.Unix(), Outcome: "invalid_argument", Routine: true})
			writeAPIV1Error(w, http.StatusBadRequest, "invalid_argument", bad.msg)
			return
		case err != nil:
			log.Printf("api: v1: %s %s: %v", route.method, route.path, err)
			writeAPIV1Error(w, http.StatusInternalServerError, "failed", "the request failed")
			return
		}
		writeAPIV1Result(w, res)
	}
}

// writeAPIV1Result answers with the tool's structured content. A tool error
// keeps its {"error": {...}} body and gets the status its code stands for.
func writeAPIV1Result(w http.ResponseWriter, res *mcp.CallToolResult) {
	w.Header().Set("Cache-Control", "no-store")
	if !res.IsError {
		body := res.StructuredContent
		// The follow-up sentence tells an assistant which tool to call next,
		// which means nothing to an HTTP client.
		if m, ok := body.(map[string]any); ok {
			delete(m, "followUp")
		}
		writeJSON(w, http.StatusOK, body)
		return
	}
	content, _ := res.StructuredContent.(map[string]any)
	detail, _ := content["error"].(map[string]any)
	code, _ := detail["code"].(string)
	if seconds, ok := detail["retryAfterSeconds"].(int); ok {
		w.Header().Set("Retry-After", strconv.Itoa(seconds))
	}
	writeJSON(w, apiV1Status(code), content)
}

// apiV1Status is the HTTP status a tool error code stands for.
func apiV1Status(code string) int {
	switch code {
	case "invalid_argument", "ambiguous":
		return http.StatusBadRequest
	case "not_permitted":
		return http.StatusForbidden
	case "not_found":
		return http.StatusNotFound
	case "busy", "domain_off", "not_running", "nothing_to_back_up":
		return http.StatusConflict
	case "cooldown", "rate_limited", "retention_guard":
		return http.StatusTooManyRequests
	case "unavailable":
		return http.StatusServiceUnavailable
	case "timeout":
		return http.StatusGatewayTimeout
	}
	return http.StatusInternalServerError
}

func writeAPIV1Error(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": msg}})
}
