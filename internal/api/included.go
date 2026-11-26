package api

import (
	"context"
	"errors"
	"net/http"
)

var errIncludedMissing = errors.New("name whether the entry is in the schedule")

// handleSetIncluded serves PUT /api/items/{kind}/{key}/included with the body
// {included}, the one field of an entry's PATCH route a paired phone may write.
// The answer is the entry's row.
func (h *Handler) handleSetIncluded(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Included *bool `json:"included"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if body.Included == nil {
		writeJSON(w, http.StatusOK, codedFailEnvelope(errIncludedMissing, "bad-request"))
		return
	}
	settings, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	item, err := h.backupItem(r.Context(), settings, r.PathValue("kind"), r.PathValue("key"))
	switch {
	case errors.Is(err, errNoSuchItem), errors.Is(err, errUnknownItemKind):
		writeJSON(w, http.StatusNotFound, codedFailEnvelope(err, "not-found"))
		return
	case err != nil:
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if err := h.setIncluded(r.Context(), item.Kind, item.Key, *body.Included); err != nil {
		if errors.Is(err, errUnknownItemKind) {
			writeJSON(w, http.StatusOK, codedFailEnvelope(errors.New("this entry has no schedule of its own"), "bad-request"))
			return
		}
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if err := h.reloadScheduler(); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if item, err = h.backupItem(r.Context(), settings, item.Kind, item.Key); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"item": item}))
}

// setIncluded is the schedule switch of each kind's PATCH route, with the
// checks that route makes for it.
func (h *Handler) setIncluded(ctx context.Context, kind, key string, included bool) error {
	switch kind {
	case "container":
		return h.svc.SetInclude(ctx, key, included)
	case "vm":
		return h.svc.SetVMInclude(ctx, key, included)
	case zfsDomain:
		return h.svc.PatchZFSDataset(ctx, key, ZFSDatasetPatch{Enabled: &included})
	case "files":
		fs, err := h.store.GetFileSet(key)
		if err != nil {
			return err
		}
		enabling := included && !fs.Enabled
		fs.Enabled = included
		if err := h.svc.validateFileSet(fs, enabling); err != nil {
			return err
		}
		return h.store.UpdateFileSet(fs)
	}
	return errUnknownItemKind
}
