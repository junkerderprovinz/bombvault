package api

import (
	"net/http/httptest"
	"testing"
)

// TestFleetTokenOK pins fleetTokenOK's fail-closed contract: an empty stored
// token always fails (feature off), a header match wins over a query-param
// match when both are present, and a mismatch fails even with a non-empty
// stored token.
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
		{"query match", "secret", "", "secret", true},
		{"header wins over query when both present", "secret", "secret", "wrong", true},
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
