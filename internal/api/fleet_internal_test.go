package api

import (
	"net/http/httptest"
	"testing"
)

// Only the header is accepted. A correct token in the query string must fail,
// because the request line ends up in reverse proxy logs.
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
		{"a right token in the query is refused", "secret", "", "secret", false},
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
