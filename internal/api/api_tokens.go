package api

import (
	"log"
	"net/http"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// API tokens are rows of mcp_keys with kind "api". They are made, shown once,
// renamed, allowed to start backups, replaced, revoked and deleted exactly like
// MCP keys, and the routes for everything after the first step are the MCP
// key handlers under /api/tokens. A token opens /api/v1 and nothing else.

func (h *Handler) handleListAPITokens(w http.ResponseWriter, r *http.Request) {
	rows, err := h.store.ListMCPKeys()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	inUse, err := h.store.MCPKeyIDsInUse()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	calls, err := h.store.MCPKeyCallsSince(mcpCallsSince(r, h.mcp.now()))
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	active, revoked := []mcpKeyView{}, []mcpKeyView{}
	for _, k := range rows {
		if k.Kind != store.MCPKindAPI {
			continue
		}
		v := h.mcpKeyViewOf(k, inUse[k.ID], calls[k.ID])
		if k.RevokedAt != 0 {
			revoked = append(revoked, v)
			continue
		}
		active = append(active, v)
	}
	_, _, authOn := h.authEnabled()
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"basePath":         apiV1Prefix,
		"openapiPath":      apiV1OpenAPIPath,
		"limit":            store.APITokenLimit,
		"authEnabled":      authOn,
		"hostAllowsKeys":   h.mcpKeysAllowedFrom(r),
		"startsPerHour":    mcpStartsPerHour,
		"cooldownMinutes":  int(mcpStartCooldown / time.Minute),
		"itemStartsPerDay": mcpItemStartsPerDay,
		"tokens":           active,
		"revoked":          revoked,
	}))
}

// handleCreateAPIToken mints a token under the same host rule as an MCP key. A
// token starts read-only unless the request asks for more.
func (h *Handler) handleCreateAPIToken(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Label           string `json:"label"`
		CanStartBackups bool   `json:"canStartBackups"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if !h.mcpKeysAllowedFrom(r) {
		writeJSON(w, http.StatusOK, codedFailEnvelope(errMCPKeyNeedsPassword, "mcp-key-needs-password"))
		return
	}
	label, ok := normalizeMCPKeyLabel(body.Label)
	if !ok {
		writeJSON(w, http.StatusOK, codedFailEnvelope(errMCPKeyLabel, "mcp-key-label-invalid"))
		return
	}
	token, err := secret.NewAPIToken()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	id := newMCPKeyID()
	row, err := h.store.CreateAPIToken(id, label,
		secret.HashMCPKey(h.cfg.AppKey, token), secret.MCPKeyHint(token), secret.MCPKeyCheck(h.cfg.AppKey, id),
		body.CanStartBackups, time.Now().Unix())
	if err != nil {
		writeMCPError(w, err)
		return
	}
	h.recordMCPKeyChange(r, row, "created", "created")
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"key": token, "item": h.mcpKeyItem(r, row)}))
}

// newKeyMaterial is the generator for a replacement of k: a token keeps its
// token prefix, so a secret scanner still tells it from an MCP key.
func newKeyMaterial(k store.MCPKey) (string, error) {
	if k.Kind == store.MCPKindAPI {
		return secret.NewAPIToken()
	}
	return secret.NewMCPKey()
}

// logKeyChange writes the line a key or token change leaves in the log, by id
// and hint, never by label.
func logKeyChange(k store.MCPKey, logged string) {
	switch k.Kind {
	case store.MCPKindOAuth:
		log.Printf("api: mcp: grant %s of client %s %s", k.ID, k.OAuthClient, logged)
	case store.MCPKindAPI:
		log.Printf("api: token %s ...%s %s", k.ID, k.Hint, logged)
	default:
		log.Printf("api: mcp: key %s ...%s %s", k.ID, k.Hint, logged)
	}
}
