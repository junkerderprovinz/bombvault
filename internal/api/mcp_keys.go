package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// mcpKeyView is one key as the settings card sees it. The key itself is handed
// out once at creation and is not in here; the hint is what tells two apart.
type mcpKeyView struct {
	ID              string `json:"id"`
	Label           string `json:"label"`
	Hint            string `json:"hint"`
	CanStartBackups bool   `json:"canStartBackups"`
	CreatedAt       int64  `json:"createdAt"`
	RotatedAt       int64  `json:"rotatedAt"`
	LastUsedAt      int64  `json:"lastUsedAt"`
	LastUsedFrom    string `json:"lastUsedFrom"`
	RevokedAt       int64  `json:"revokedAt"`
	RevokedReason   string `json:"revokedReason"`
	InUse           bool   `json:"inUse"`
	Unusable        string `json:"unusable"`
}

var (
	errMCPKeyLabel         = errors.New("a key needs a name of 1 to 64 characters")
	errMCPKeyNeedsPassword = errors.New("set a login password before creating a key or adding a certificate name from this address")
)

// mcpErrorCodes translates the refusals of the card's two subjects, the keys
// and the certificate, into the codes it renders as its own sentences.
var mcpErrorCodes = []struct {
	err  error
	code string
}{
	{store.ErrMCPKeyNotFound, "mcp-key-not-found"},
	{store.ErrMCPKeyLimit, "mcp-key-limit"},
	{store.ErrMCPKeyLabelTaken, "mcp-key-label-taken"},
	{store.ErrMCPKeyInUse, "mcp-key-in-use"},
	{store.ErrMCPKeyActive, "mcp-key-active"},
	{errCertNameInvalid, "cert-name-invalid"},
	{errCertNameLimit, "cert-name-limit"},
	{errCertNotOwn, "cert-not-own"},
	{errCertWriteFailed, "cert-write-failed"},
}

func writeMCPError(w http.ResponseWriter, err error) {
	for _, c := range mcpErrorCodes {
		if errors.Is(err, c.err) {
			writeJSON(w, http.StatusOK, codedFailEnvelope(err, c.code))
			return
		}
	}
	writeJSON(w, http.StatusOK, failEnvelope(err))
}

// normalizeMCPKeyLabel trims a label and reports whether it is a usable name:
// 1 to 64 runes, no control characters.
func normalizeMCPKeyLabel(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if n := utf8.RuneCountInString(s); n == 0 || n > 64 {
		return "", false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return "", false
		}
	}
	return s, true
}

// mcpKeyIDParam reads the {id} path value and answers 400 itself when it is not
// the 32-hex shape every row in the table carries.
func mcpKeyIDParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !validRunID(id) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid key id"})
		return "", false
	}
	return id, true
}

// mcpKeyPrivateSuffixes only ever resolve inside a network, so no answer from
// public DNS can point one of them at a LAN address.
var mcpKeyPrivateSuffixes = []string{".local", ".lan", ".home", ".home.arpa", ".internal", ".localdomain", ".ts.net"}

// mcpKeyHostAllowed reports whether a key may be created or replaced from the
// given request host while no login password is set: an IP literal, localhost,
// a single-label name or a private-use suffix, none of which a public DNS name
// can rebind.
func mcpKeyHostAllowed(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.ToLower(strings.Trim(strings.TrimSpace(host), "[]"))
	if host == "" {
		return false
	}
	if net.ParseIP(host) != nil || !strings.Contains(host, ".") {
		return true
	}
	for _, suffix := range mcpKeyPrivateSuffixes {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}

// mcpKeysAllowedFrom reports whether this request may mint key material. A
// login password settles it: the session cookie is bound to the real host name,
// so a page that rebound its DNS to this address carries no session. Without
// one, only a host that cannot be rebound at all counts.
func (h *Handler) mcpKeysAllowedFrom(r *http.Request) bool {
	if _, _, on := h.authEnabled(); on {
		return true
	}
	return mcpKeyHostAllowed(r.Host)
}

func (h *Handler) mcpKeyViewOf(k store.MCPKey, inUse bool) mcpKeyView {
	return mcpKeyView{
		ID:              k.ID,
		Label:           k.Label,
		Hint:            k.Hint,
		CanStartBackups: k.CanStartBackups,
		CreatedAt:       k.CreatedAt,
		RotatedAt:       k.RotatedAt,
		LastUsedAt:      k.LastUsedAt,
		LastUsedFrom:    k.LastUsedFrom,
		RevokedAt:       k.RevokedAt,
		RevokedReason:   k.RevokedReason,
		InUse:           inUse,
		Unusable:        h.mcpKeyUnusable(k),
	}
}

// mcpKeyUnusable names why a key can no longer authenticate, or "" while it
// still can. A reinstall or a restore onto another container brings a new
// APP_KEY and every digest stops matching at once; the check value is derived
// from the id, so it tells that apart from a client sending the wrong key.
func (h *Handler) mcpKeyUnusable(k store.MCPKey) string {
	if k.RevokedAt == 0 && k.Check != secret.MCPKeyCheck(h.cfg.AppKey, k.ID) {
		return "app-key-changed"
	}
	return ""
}

// mcpKeyItem builds the view of a single key for a mutation's response.
func (h *Handler) mcpKeyItem(k store.MCPKey) mcpKeyView {
	inUse, err := h.store.MCPKeyIDsInUse()
	if err != nil {
		log.Printf("api: mcp: could not read which keys the run history names: %v", err)
	}
	return h.mcpKeyViewOf(k, inUse[k.ID])
}

func (h *Handler) handleListMCPKeys(w http.ResponseWriter, r *http.Request) {
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
	// Empty rather than nil, so the card always gets two arrays to iterate.
	active, revoked := []mcpKeyView{}, []mcpKeyView{}
	for _, k := range rows {
		v := h.mcpKeyViewOf(k, inUse[k.ID])
		if k.RevokedAt != 0 {
			revoked = append(revoked, v)
			continue
		}
		active = append(active, v)
	}
	_, _, authOn := h.authEnabled()
	var certificate any
	if info, ok := h.svc.CertificateInfo(); ok {
		certificate = info
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"endpointPath":     mcpEndpointPath,
		"certificate":      certificate,
		"limit":            store.MCPKeyLimit,
		"authEnabled":      authOn,
		"hostAllowsKeys":   h.mcpKeysAllowedFrom(r),
		"startsPerHour":    mcpStartsPerHour,
		"cooldownMinutes":  int(mcpStartCooldown / time.Minute),
		"itemStartsPerDay": mcpItemStartsPerDay,
		"keys":             active,
		"revoked":          revoked,
	}))
}

func (h *Handler) handleCreateMCPKey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Label           string `json:"label"`
		CanStartBackups *bool  `json:"canStartBackups"`
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
	canStart := body.CanStartBackups == nil || *body.CanStartBackups

	key, err := secret.NewMCPKey()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	id := newMCPKeyID()
	row, err := h.store.CreateMCPKey(id, label,
		secret.HashMCPKey(h.cfg.AppKey, key), secret.MCPKeyHint(key), secret.MCPKeyCheck(h.cfg.AppKey, id),
		canStart, time.Now().Unix())
	if err != nil {
		writeMCPError(w, err)
		return
	}
	h.recordMCPKeyChange(r, row, "created", "created")
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"key": key, "item": h.mcpKeyItem(row)}))
}

func (h *Handler) handleUpdateMCPKey(w http.ResponseWriter, r *http.Request) {
	id, ok := mcpKeyIDParam(w, r)
	if !ok {
		return
	}
	var body struct {
		Label           *string `json:"label"`
		CanStartBackups *bool   `json:"canStartBackups"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	var label *string
	if body.Label != nil {
		clean, valid := normalizeMCPKeyLabel(*body.Label)
		if !valid {
			writeJSON(w, http.StatusOK, codedFailEnvelope(errMCPKeyLabel, "mcp-key-label-invalid"))
			return
		}
		label = &clean
	}
	row, err := h.store.UpdateMCPKey(id, label, body.CanStartBackups)
	if err != nil {
		writeMCPError(w, err)
		return
	}
	event := ""
	if body.CanStartBackups != nil {
		event = "set to read only"
		if *body.CanStartBackups {
			event = "allowed to start backups"
		}
	}
	h.recordMCPKeyChange(r, row, "updated", event)
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"item": h.mcpKeyItem(row)}))
}

func (h *Handler) handleRotateMCPKey(w http.ResponseWriter, r *http.Request) {
	id, ok := mcpKeyIDParam(w, r)
	if !ok {
		return
	}
	if !h.mcpKeysAllowedFrom(r) {
		writeJSON(w, http.StatusOK, codedFailEnvelope(errMCPKeyNeedsPassword, "mcp-key-needs-password"))
		return
	}
	key, err := secret.NewMCPKey()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	row, err := h.store.RotateMCPKey(id,
		secret.HashMCPKey(h.cfg.AppKey, key), secret.MCPKeyHint(key), secret.MCPKeyCheck(h.cfg.AppKey, id),
		time.Now().Unix())
	if err != nil {
		writeMCPError(w, err)
		return
	}
	h.recordMCPKeyChange(r, row, "rotated", "replaced")
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"key": key, "item": h.mcpKeyItem(row)}))
}

func (h *Handler) handleRevokeMCPKey(w http.ResponseWriter, r *http.Request) {
	id, ok := mcpKeyIDParam(w, r)
	if !ok {
		return
	}
	row, err := h.store.GetMCPKey(id)
	if err != nil {
		writeMCPError(w, err)
		return
	}
	if err := h.store.RevokeMCPKey(id, "user", time.Now().Unix()); err != nil {
		writeMCPError(w, err)
		return
	}
	h.recordMCPKeyChange(r, row, "revoked", "revoked")
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

func (h *Handler) handlePurgeMCPKey(w http.ResponseWriter, r *http.Request) {
	id, ok := mcpKeyIDParam(w, r)
	if !ok {
		return
	}
	row, err := h.store.GetMCPKey(id)
	if err != nil {
		writeMCPError(w, err)
		return
	}
	if err := h.store.PurgeMCPKey(id); err != nil {
		writeMCPError(w, err)
		return
	}
	h.recordMCPKeyChange(r, row, "purged", "")
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleMCPCertificate hands out the certificate the web interface serves, so
// an operator can make their client trust it.
func (h *Handler) handleMCPCertificate(w http.ResponseWriter, r *http.Request) {
	pemBytes, ok := h.svc.CertificatePEM()
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Content-Disposition", `attachment; filename="bombvault-cert.pem"`)
	if _, err := w.Write(pemBytes); err != nil {
		log.Printf("api: mcp: send certificate: %v", err)
	}
}

// handleAddMCPCertificateName adds the address the operator reached BombVault
// on to its certificate, which is what a Node client needs before it connects.
// It follows the same host rule as creating a key: the names are material the
// server then presents, there are only sixteen of them, and nothing in the
// product takes one back.
func (h *Handler) handleAddMCPCertificateName(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Host string `json:"host"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if h.cfg.HTTPOnly {
		http.NotFound(w, r)
		return
	}
	if !h.mcpKeysAllowedFrom(r) {
		writeJSON(w, http.StatusOK, codedFailEnvelope(errMCPKeyNeedsPassword, "mcp-key-needs-password"))
		return
	}
	info, err := h.svc.AddCertificateName(body.Host)
	if err != nil {
		writeMCPError(w, err)
		return
	}
	log.Printf("api: mcp: certificate reissued for %d address(es)", len(info.Names))
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"certificate": info}))
}

// recordMCPKeyChange logs the mutation and, for a change worth waking the
// operator for, notifies as well. The log line carries the id and the hint and
// never the label, because it ends up in the diagnostics bundle.
//
// The notification is sent beside the response, not in front of it: a create
// and a rotate carry the one-time key, and an endpoint that swallows the
// connection would otherwise hold that key back for the notifier's whole
// budget and leave an unusable row behind if the client gave up first.
func (h *Handler) recordMCPKeyChange(r *http.Request, k store.MCPKey, logged, event string) {
	log.Printf("api: mcp: key %s ...%s %s", k.ID, k.Hint, logged)
	if event == "" {
		return
	}
	ctx, addr := context.WithoutCancel(r.Context()), loginClientKey(r)
	h.svc.notifyMCPKeyChange(ctx, event, k.Label, k.Hint, addr)
}

// newMCPKeyID returns a 32 hex character id. The caller mints it because the
// key_check value is derived from the id before the row exists.
func newMCPKeyID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Sprintf("newMCPKeyID: %v", err))
	}
	return hex.EncodeToString(buf)
}

// RevokeMCPKeysAfterConfigRestore revokes every active MCP key when a staged
// configuration restore was applied at this boot: the restored database may
// hold keys that were revoked after it was saved.
func RevokeMCPKeysAfterConfigRestore(st *store.Repo, applied bool, now time.Time) {
	if !applied {
		return
	}
	n, err := st.RevokeAllMCPKeys("config-restore", now.Unix())
	if err != nil {
		log.Printf("selfrestore: could not revoke MCP keys after the configuration restore: %v", err)
		return
	}
	if n > 0 {
		log.Printf("selfrestore: revoked %d MCP key(s): a restored configuration may contain keys that were revoked after it was saved; create new keys under Settings > System > MCP server", n)
	}
}
