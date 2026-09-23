package api

import (
	"io"
	"net/http"
	"net/http/httptest"
)

// jsonReq builds a test request with the JSON Content-Type header every real
// client sends. decodeBody rejects a body without it, which blocks cross-site
// form posts (see crossOriginGuard), so the guard's own tests build their
// requests without this helper.
func jsonReq(method, target string, body io.Reader) *http.Request {
	r := httptest.NewRequest(method, target, body)
	r.Header.Set("Content-Type", "application/json")
	return r
}
