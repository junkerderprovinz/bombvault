package api_test

import (
	"io"
	"net/http"
	"net/http/httptest"
)

// jsonReq builds a test request with the JSON Content-Type header that
// decodeBody requires.
func jsonReq(method, target string, body io.Reader) *http.Request {
	r := httptest.NewRequest(method, target, body)
	r.Header.Set("Content-Type", "application/json")
	return r
}
