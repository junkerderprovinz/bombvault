package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"

	"github.com/junkerderprovinz/bombvault/internal/group"
)

// peerCall is one call a member makes, as the group hands it to this
// instance once it vouched for the sender.
type peerCall struct {
	Method string
	Path   string
	Body   []byte
	Sender string
	Kind   string
	Key    []byte
}

// callPeer answers call the way a call through the group is answered.
func (s *Service) callPeer(ctx context.Context, call peerCall) (int, []byte) {
	r := httptest.NewRequestWithContext(group.WithSender(ctx, group.Sender{ID: call.Sender, Kind: call.Kind, Key: call.Key}), call.Method, call.Path, bytes.NewReader(call.Body))
	if len(call.Body) > 0 {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	s.servePeer(w, r)
	if w.Code == 0 {
		return http.StatusOK, w.Body.Bytes()
	}
	return w.Code, w.Body.Bytes()
}
