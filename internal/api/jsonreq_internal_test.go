package api

import (
	"io"
	"net/http"
	"net/http/httptest"
)

// jsonReq builds a test request the way every real client builds one: with the
// Content-Type header that says the body is JSON.
//
// decodeBody refuses a body-carrying request that does not declare JSON, which
// is what closes the cross-site form-post hole (see crossOriginGuard). A test
// that builds its request without the header is therefore testing a request no
// client sends, and would fail for a reason that has nothing to do with what it
// is about. Deliberately NOT applied to the guard's own tests, which are the
// ones that need to send a request the wrong way on purpose.
func jsonReq(method, target string, body io.Reader) *http.Request {
	r := httptest.NewRequest(method, target, body)
	r.Header.Set("Content-Type", "application/json")
	return r
}
