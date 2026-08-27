package api_test

import (
	"io"
	"net/http"
	"net/http/httptest"
)

// jsonReq is the api_test-package twin of the internal jsonReq helper: a
// request carrying the Content-Type header every real client sends, which
// decodeBody now requires (see crossOriginGuard).
func jsonReq(method, target string, body io.Reader) *http.Request {
	r := httptest.NewRequest(method, target, body)
	r.Header.Set("Content-Type", "application/json")
	return r
}
