package api

import (
	"net/http/httptest"
	"testing"
)

// TestFleetTokenOK pins fleetTokenOK's fail-closed contract: an empty stored
// token always fails (feature off), the HEADER is the only accepted carrier,
// and a mismatch fails even with a non-empty stored token.
//
// The query cases are the point of this test rather than an afterthought. The
// ?token= form used to be accepted here and nothing ever sent it: this
// instance's own peer poll and its mesh-offer sender both set the header. What
// it did do is put a credential in a request line, where the reverse proxy in
// front writes it to its access log. So a right token in the query must FAIL,
// and asserting that is the only way this stays true.
func TestFleetTokenOK(t *testing.T) {
	cases := []struct {
		name        string
		stored      string
		headerToken string
		queryToken  string
		want        bool
	}{
		{"empty stored always fails", "", "anything", "", false},
		{"empty stored fails even with empty presented", "", "", "", false},
		{"header match", "secret", "secret", "", true},
		{"a right token in the QUERY is refused", "secret", "", "secret", false},
		{"the header still decides when both are present", "secret", "secret", "wrong", true},
		{"a query token cannot rescue a wrong header", "secret", "wrong", "secret", false},
		{"mismatch fails", "secret", "wrong", "", false},
		{"no token presented fails", "secret", "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			url := "/api/fleet/status"
			if c.queryToken != "" {
				url += "?token=" + c.queryToken
			}
			r := httptest.NewRequest("GET", url, nil)
			if c.headerToken != "" {
				r.Header.Set("X-Fleet-Token", c.headerToken)
			}
			if got := fleetTokenOK(r, c.stored); got != c.want {
				t.Fatalf("fleetTokenOK(stored=%q, header=%q, query=%q) = %v, want %v",
					c.stored, c.headerToken, c.queryToken, got, c.want)
			}
		})
	}
}
