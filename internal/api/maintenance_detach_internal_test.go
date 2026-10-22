package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// ctxEngine records whether the context a check or prune ran under was still
// live when it reached the engine.
type ctxEngine struct {
	ResticEngine
	ran map[string]error
}

func (e *ctxEngine) Check(ctx context.Context, repo string, mode restic.Mode) error {
	e.ran["check"] = ctx.Err()
	return e.ResticEngine.Check(ctx, repo, mode)
}

func (e *ctxEngine) Prune(ctx context.Context, repo string, mode restic.Mode) error {
	e.ran["prune"] = ctx.Err()
	return e.ResticEngine.Prune(ctx, repo, mode)
}

func TestACheckOrPruneOutlivesTheRequestThatStartedIt(t *testing.T) {
	f, _, _, _ := flashScene(t)
	e := &ctxEngine{ResticEngine: f.svc.engine, ran: map[string]error{}}
	f.svc.engine = e
	f.signIn()

	for _, c := range []struct{ path, op string }{
		{"/api/check/flash?source=local", "check"},
		{"/api/prune/flash?source=local", "prune"},
	} {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		r := jsonReq(http.MethodPost, c.path, nil).WithContext(ctx)
		r.AddCookie(&http.Cookie{Name: f.h.sessionCookieNameFor(), Value: f.session}) //nolint:gosec // G124: a cookie on a test request
		w := httptest.NewRecorder()
		f.h.Router().ServeHTTP(w, r)
		err, ran := e.ran[c.op]
		if !ran || err != nil {
			t.Errorf("%s after its request ended: ran %v, context %v, want it run to the end (%s)", c.op, ran, err, w.Body.String())
		}
	}
}
