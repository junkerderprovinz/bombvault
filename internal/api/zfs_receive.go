package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// zfsReceiveBodyMax caps a request over the group. A member list of a few
// thousand datasets stays far below it.
const zfsReceiveBodyMax = 512 << 10

const zfsReceiveMaxMembers = 2000

// zfsReceiveSlotRe matches the slot routes, which carry no session and gate
// themselves on the slot's token. The request routes beside them do not
// match, since a slot id is 32 hex characters.
var zfsReceiveSlotRe = regexp.MustCompile(`^/api/zfs/receive/[0-9a-f]{32}/`)

func zfsReceiveSlotPath(path string) bool { return zfsReceiveSlotRe.MatchString(path) }

// registerZFSReceiveRoutes adds the receiving side of the ZFS replica: the requests a
// person answers, behind the session, and the slot routes a source streams
// through.
func (h *Handler) registerZFSReceiveRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/zfs/receive/requests", h.handleListZFSReceive)
	mux.HandleFunc("POST /api/zfs/receive/requests/{id}", h.handleDecideZFSReceive)
	mux.HandleFunc("PATCH /api/zfs/receive/requests/{id}", h.handlePatchZFSReceive)
	mux.HandleFunc("GET /api/zfs/receive/{slot}/points", h.svc.handleZFSSlotPoints)
	mux.HandleFunc("PUT /api/zfs/receive/{slot}/members/{member}", h.svc.handleZFSSlotReceive)
	mux.HandleFunc("POST /api/zfs/receive/{slot}/members/{member}/abort", h.svc.handleZFSSlotAbort)
	mux.HandleFunc("GET /api/zfs/receive/{slot}/members/{member}/send", h.svc.handleZFSSlotSend)
}

// peerZFSReceiveRequest is what a source sends when it asks to replicate one
// of its ZFS items here. Renew is set when a person on the source asked, which
// is the only request that reopens a revoked slot.
type peerZFSReceiveRequest struct {
	InstanceID string               `json:"instanceId"`
	Name       string               `json:"name"`
	Item       string               `json:"item"`
	Dataset    string               `json:"dataset"`
	Members    []string             `json:"members"`
	Keep       store.ZFSReplicaKeep `json:"keep"`
	Renew      bool                 `json:"renew"`
}

// peerZFSReceiveAnswer is the slot's state, and for an allowed slot what the
// source streams through: the slot, its token, where the members land, and
// where and under which key this instance answers.
type peerZFSReceiveAnswer struct {
	OK        bool   `json:"ok"`
	State     string `json:"state"`
	Slot      string `json:"slot,omitempty"`
	Token     string `json:"token,omitempty"`
	Base      string `json:"base,omitempty"`
	Pin       string `json:"pin,omitempty"`
	DirectURL string `json:"directUrl,omitempty"`
}

// handlePeerZFSReceive records a source's request and answers with the
// slot's state at once. A person answers later; the source learns the answer
// by asking again, which it does before every run.
// POST /api/group/peer/zfs-receive
func (s *Service) handlePeerZFSReceive(w http.ResponseWriter, r *http.Request) {
	var in peerZFSReceiveRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, zfsReceiveBodyMax)).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "malformed receive request"})
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	slices.Sort(in.Members)
	in.Members = slices.Compact(in.Members)
	if msg := peerZFSReceiveRefusal(in); msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}
	slot, err := s.store.AskZFSReceive(store.ZFSReceiveSlot{
		PeerID: in.InstanceID, PeerName: in.Name, ItemID: in.Item, Dataset: in.Dataset,
		SourceServer: zfsReplicaFolderName(in.Name), Members: in.Members, ProposedKeep: in.Keep,
	}, in.Renew)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	ans := peerZFSReceiveAnswer{OK: true, State: slot.State, DirectURL: s.selfDirectURL()}
	if slot.State == store.ZFSReceiveAllowed {
		token, err := secret.Decrypt(s.cfg.AppKey, slot.TokenEnc)
		if err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(errors.New("a stored slot token could not be opened")))
			return
		}
		ans.Slot, ans.Token, ans.Base, ans.Pin = slot.ID, string(token), slot.Base(), s.tlsPin()
	}
	writeJSON(w, http.StatusOK, ans)
}

func peerZFSReceiveRefusal(in peerZFSReceiveRequest) string {
	switch {
	case !zfs.ValidSourceID(in.InstanceID):
		return "the asking instance did not say who it is"
	case !validRunID(in.Item):
		return "the request names no item"
	case zfs.ValidateDatasetName(in.Dataset) != nil:
		return "the request names no dataset"
	case !slices.Contains(in.Members, in.Dataset) || len(in.Members) > zfsReceiveMaxMembers:
		return fmt.Sprintf("the request has to list the item's root and at most %d members", zfsReceiveMaxMembers)
	}
	for _, m := range in.Members {
		if zfs.ValidateMemberName(m) != nil || (m != in.Dataset && !zfs.DescendantOf(m, in.Dataset)) {
			return fmt.Sprintf("%q is not a dataset of %s", m, in.Dataset)
		}
	}
	return zfsKeepRefusal(in.Keep)
}

func zfsKeepRefusal(keep store.ZFSReplicaKeep) string {
	if _, ok := keep.Counts(); !ok {
		return fmt.Sprintf("unknown keep preset %q", keep.Preset)
	}
	if slices.ContainsFunc(keep.Own[:], func(n int) bool { return n < 0 }) {
		return "a keep count is negative"
	}
	return ""
}

// tlsPin is the SHA-256 of the public key this instance serves TLS with,
// which a source checks the slot routes against. It survives a reissued
// certificate, which keeps the key.
func (s *Service) tlsPin() string {
	if s.cfg.HTTPOnly {
		return ""
	}
	pair := servedCertificate.Load()
	if pair == nil || len(pair.Certificate) == 0 {
		return ""
	}
	leaf := pair.Leaf
	if leaf == nil {
		var err error
		if leaf, err = x509.ParseCertificate(pair.Certificate[0]); err != nil {
			return ""
		}
	}
	return spkiPin(leaf)
}

func spkiPin(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	return hex.EncodeToString(sum[:])
}

// zfsReceiveView is a slot as the receive page lists it. Item is the source's
// root dataset.
type zfsReceiveView struct {
	ID           string               `json:"id"`
	Peer         string               `json:"peer"`
	PeerName     string               `json:"peerName"`
	SourceServer string               `json:"sourceServer"`
	Item         string               `json:"item"`
	Members      []string             `json:"members"`
	ProposedKeep store.ZFSReplicaKeep `json:"proposedKeep"`
	State        string               `json:"state"`
	Pool         string               `json:"pool"`
	Root         string               `json:"root"`
	Keep         store.ZFSReplicaKeep `json:"keep"`
	AskedAt      string               `json:"askedAt"`
	DecidedAt    string               `json:"decidedAt"`
	LastReceived string               `json:"lastReceived"`
	Bytes        int64                `json:"bytes"`
}

func zfsReceiveViewOf(s store.ZFSReceiveSlot) zfsReceiveView {
	members := s.Members
	if members == nil {
		members = []string{}
	}
	return zfsReceiveView{
		ID: s.ID, Peer: s.PeerID, PeerName: s.PeerName, SourceServer: s.SourceServer, Item: s.Dataset,
		Members: members, ProposedKeep: s.ProposedKeep, State: s.State, Pool: s.Pool, Root: s.Root, Keep: s.Keep,
		AskedAt: unixRFC3339(s.AskedAt), DecidedAt: unixRFC3339(s.DecidedAt), LastReceived: unixRFC3339(s.LastReceived),
		Bytes: s.Bytes,
	}
}

func unixRFC3339(sec int64) string {
	if sec == 0 {
		return ""
	}
	return time.Unix(sec, 0).UTC().Format(time.RFC3339)
}

// handleListZFSReceive lists what other instances asked to replicate here. A
// refused request is not listed: it stays refused until the source asks for
// more.
// GET /api/zfs/receive/requests
func (h *Handler) handleListZFSReceive(w http.ResponseWriter, _ *http.Request) {
	slots, err := h.store.ListZFSReceiveSlots()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, failEnvelope(err))
		return
	}
	out := make([]zfsReceiveView, 0, len(slots))
	for _, s := range slots {
		if s.State != store.ZFSReceiveRefused {
			out = append(out, zfsReceiveViewOf(s))
		}
	}
	writeJSON(w, http.StatusOK, out)
}

type zfsReceiveAnswer struct {
	OK bool `json:"ok"`
	zfsReceiveView
}

// handleDecideZFSReceive allows a request into a pool and root of this host,
// refuses it, or revokes an allowed one. A revoke keeps what arrived.
// POST /api/zfs/receive/requests/{id}
func (h *Handler) handleDecideZFSReceive(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validRunID(id) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid request id"})
		return
	}
	var body struct {
		Decision string                `json:"decision"`
		Pool     string                `json:"pool"`
		Root     string                `json:"root"`
		Keep     *store.ZFSReplicaKeep `json:"keep"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	slot, ok, err := h.store.GetZFSReceiveSlot(id)
	if err != nil || !ok {
		zfsReceiveMissing(w, err)
		return
	}
	d := store.ZFSReceiveDecision{}
	switch body.Decision {
	case "allow":
		d, err = h.svc.zfsReceiveAllow(r.Context(), slot, body.Pool, body.Root, body.Keep)
		if err != nil {
			zfsReplicaFail(w, err)
			return
		}
	case "refuse":
		d.State = store.ZFSReceiveRefused
	case "revoke":
		d.State = store.ZFSReceiveRevoked
	default:
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "the decision must be allow, refuse or revoke"})
		return
	}
	slot, err = h.store.DecideZFSReceive(id, d)
	if errors.Is(err, store.ErrZFSReceiveMove) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "the request cannot take that answer from where it stands"})
		return
	}
	if err != nil {
		zfsReplicaFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, zfsReceiveAnswer{OK: true, zfsReceiveView: zfsReceiveViewOf(slot)})
}

// zfsReceiveAllow checks where a person lets a slot land and mints its
// token. The pool has to exist on this host, since the first stream would
// otherwise fail long after the dialog closed.
func (s *Service) zfsReceiveAllow(ctx context.Context, slot store.ZFSReceiveSlot, pool, root string, keep *store.ZFSReplicaKeep) (store.ZFSReceiveDecision, error) {
	d := store.ZFSReceiveDecision{State: store.ZFSReceiveAllowed, Pool: pool, Root: root, Keep: slot.Keep}
	if keep != nil {
		d.Keep = *keep
	}
	slot.Root = root
	switch {
	case zfs.ValidateDatasetName(pool) != nil || strings.Contains(pool, "/"):
		return d, fmt.Errorf("%q is not a pool name", pool)
	case root != pool && !strings.HasPrefix(root, pool+"/"):
		return d, fmt.Errorf("root %q is not on pool %q", root, pool)
	case zfs.ValidateDatasetName(slot.Base()) != nil:
		return d, fmt.Errorf("%q is not a dataset name that fits", slot.Base())
	}
	if msg := zfsKeepRefusal(d.Keep); msg != "" {
		return d, errors.New(msg)
	}
	host, err := s.zfsReceiveHost()
	if err != nil {
		return d, err
	}
	if _, err := zfsEndState(ctx, host, pool); err != nil {
		return d, fmt.Errorf("pool %s: %w", pool, err)
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return d, err
	}
	d.TokenEnc, err = secret.Encrypt(s.cfg.AppKey, []byte(base64.RawURLEncoding.EncodeToString(raw)))
	return d, err
}

// handlePatchZFSReceive changes what a slot's members keep here.
// PATCH /api/zfs/receive/requests/{id}
func (h *Handler) handlePatchZFSReceive(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validRunID(id) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid request id"})
		return
	}
	var body struct {
		Keep *store.ZFSReplicaKeep `json:"keep"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if body.Keep == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "nothing to change"})
		return
	}
	if msg := zfsKeepRefusal(*body.Keep); msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}
	if err := h.store.SetZFSReceiveKeep(id, *body.Keep); err != nil {
		zfsReceiveMissing(w, err)
		return
	}
	slot, ok, err := h.store.GetZFSReceiveSlot(id)
	if err != nil || !ok {
		zfsReceiveMissing(w, err)
		return
	}
	writeJSON(w, http.StatusOK, zfsReceiveAnswer{OK: true, zfsReceiveView: zfsReceiveViewOf(slot)})
}

func zfsReceiveMissing(w http.ResponseWriter, err error) {
	if err == nil || errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "no such receive request"})
		return
	}
	writeJSON(w, http.StatusOK, failEnvelope(err))
}
