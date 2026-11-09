package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Covers the boundaries between ok, warn and overdue, plus never and off.
func TestRPOStatus(t *testing.T) {
	const day = int64(86400)
	const now = int64(1_000_000_000)

	cases := []struct {
		name      string
		last      int64
		period    int64
		scheduled bool
		want      string
	}{
		{"not scheduled", now - day, day, false, "off"},
		{"zero period", now - day, 0, true, "off"},
		{"scheduled but never run", 0, day, true, "never"},
		{"fresh backup is ok", now, day, true, "ok"},
		{"exactly one period is ok", now - day, day, true, "ok"},
		{"just past one period is warn", now - day - 1, day, true, "warn"},
		{"between one and two periods is warn", now - day*2, day, true, "warn"},
		{"exactly two periods is warn", now - day*2, day, true, "warn"},
		{"just past two periods is overdue", now - day*2 - 1, day, true, "overdue"},
		{"far overdue", now - day*30, day, true, "overdue"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := rpoStatus(now, c.last, c.period, c.scheduled); got != c.want {
				t.Fatalf("rpoStatus(now, last=%d, period=%d, scheduled=%v) = %q, want %q",
					c.last, c.period, c.scheduled, got, c.want)
			}
		})
	}
}

// A monitor that checks only the status line must see the failure, as it does
// on /api/v1/status. The body keeps the envelope the pages read.
func TestStatusThatCannotBeReadAnswersAServerError(t *testing.T) {
	h, _, _, db := newMCPGateHandler(t)
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	w := httptest.NewRecorder()
	h.handleStatus(w, httptest.NewRequest(http.MethodGet, "/api/status", nil))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	var body struct {
		OK    *bool  `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q: %v", w.Body, err)
	}
	if body.OK == nil || *body.OK || body.Error == "" {
		t.Errorf("body = %s, want ok false with the error", w.Body)
	}
}
