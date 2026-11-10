package api

import (
	"errors"
	"net/http"
)

// mountSharedRepo registers the routes of the shared repository: the owner's
// card with its server, the asks to join and the members, and on a member the
// repositories of the group it can join or copies into.
func (h *Handler) mountSharedRepo(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/shared-repo", h.handleGetSharedRepo)
	mux.HandleFunc("POST /api/shared-repo", h.handleSetUpSharedRepo)
	mux.HandleFunc("POST /api/shared-repo/check", h.handleCheckSharedRepo)
	mux.HandleFunc("POST /api/shared-repo/joins/{id}/allow", h.handleAllowSharedJoin)
	mux.HandleFunc("POST /api/shared-repo/joins/{id}/refuse", h.handleRefuseSharedJoin)
	mux.HandleFunc("DELETE /api/shared-repo/members/{id}", h.handleRemoveSharedMember)
	mux.HandleFunc("GET /api/shared-repo/offers", h.handleListSharedOffers)
	mux.HandleFunc("POST /api/shared-repo/offers/{id}/join", h.handleJoinSharedRepo)
	mux.HandleFunc("POST /api/shared-repo/offers/{id}/leave", h.handleLeaveSharedRepo)
}

// handleGetSharedRepo answers both cards: the shared repository this instance
// holds, when it holds one, and the ones it asked to join or copies into. An
// allowed join that broke off is picked up again on the way.
// GET /api/shared-repo
func (h *Handler) handleGetSharedRepo(w http.ResponseWriter, r *http.Request) {
	answer := map[string]any{"server": nil, "defaultPort": sharedServerPort}
	v, err := h.svc.sharedServerView(r.Context())
	switch {
	case err == nil:
		answer["server"] = v
	case !errors.Is(err, errNoSharedRepo):
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	memberships, err := h.svc.sharedMemberships()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	answer["memberships"] = memberships
	holdings, err := h.svc.sharedHoldings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	answer["holdings"] = holdings
	h.svc.finishSharedJoinsSoon()
	h.svc.tellSharedHoldersSoon()
	writeJSON(w, http.StatusOK, okEnvelope(answer))
}

// handleSetUpSharedRepo creates the shared repository and starts its server.
// POST /api/shared-repo
func (h *Handler) handleSetUpSharedRepo(w http.ResponseWriter, r *http.Request) {
	var in receiverServerInput
	if !decodeBody(w, r, &in) {
		return
	}
	v, err := h.svc.SetUpSharedRepo(r.Context(), in)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"server": v}))
}

// handleCheckSharedRepo runs the append-only check of the shared server again.
// POST /api/shared-repo/check
func (h *Handler) handleCheckSharedRepo(w http.ResponseWriter, r *http.Request) {
	v, err := h.svc.CheckSharedServer(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"server": v}))
}

// answerShared answers a change on the owner's card with the card as it is
// now and whether the member heard.
func (h *Handler) answerShared(w http.ResponseWriter, r *http.Request, told bool, err error) {
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	v, err := h.svc.sharedServerView(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"server": v, "told": told}))
}

// handleAllowSharedJoin lets a member into the shared repository.
// POST /api/shared-repo/joins/{id}/allow
func (h *Handler) handleAllowSharedJoin(w http.ResponseWriter, r *http.Request) {
	told, err := h.svc.AllowSharedJoin(r.Context(), r.PathValue("id"))
	h.answerShared(w, r, told, err)
}

// handleRefuseSharedJoin turns a member's ask down.
// POST /api/shared-repo/joins/{id}/refuse
func (h *Handler) handleRefuseSharedJoin(w http.ResponseWriter, r *http.Request) {
	told, err := h.svc.RefuseSharedJoin(r.Context(), r.PathValue("id"))
	h.answerShared(w, r, told, err)
}

// handleRemoveSharedMember takes a member out of the shared repository.
// DELETE /api/shared-repo/members/{id}
func (h *Handler) handleRemoveSharedMember(w http.ResponseWriter, r *http.Request) {
	told, err := h.svc.RemoveSharedMember(r.Context(), r.PathValue("id"))
	h.answerShared(w, r, told, err)
}

// handleListSharedOffers lists the shared repositories the group's members
// hold, for the destination wizard.
// GET /api/shared-repo/offers
func (h *Handler) handleListSharedOffers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"offers": h.svc.SharedRepoOffers(r.Context())}))
}

// handleJoinSharedRepo asks a member to let this instance into its shared
// repository.
// POST /api/shared-repo/offers/{id}/join
func (h *Handler) handleJoinSharedRepo(w http.ResponseWriter, r *http.Request) {
	state, err := h.svc.JoinSharedRepo(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"state": state}))
}

// handleLeaveSharedRepo takes this instance out of a member's shared
// repository.
// POST /api/shared-repo/offers/{id}/leave
func (h *Handler) handleLeaveSharedRepo(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.LeaveSharedRepo(r.Context(), r.PathValue("id")); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}
