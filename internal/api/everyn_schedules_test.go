package api_test

// The drills, tamper-test and digest schedules accept an "everyN" cadence
// because each records its own last run for the scheduler to gate on (see
// internal/store schedule_job_runs.go). The five off-site replication
// schedules keep no such record and refuse it.

import (
	"net/http"
	"strings"
	"testing"
)

// baseScheduleBody is the minimum valid PUT /api/settings payload the cases
// below extend, so a failure is never caused by an unrelated missing field.
const baseScheduleBody = `"containersPath": "backups/c",
		"vmsPath": "backups/v",
		"flashPath": "backups/f",
		"containersSchedule": "off",
		"vmsSchedule": "off",
		"flashSchedule": "off"`

// Each cadence must also read back unchanged: a value that saved but did not
// persist would look like a refusal in the UI.
func TestSettingsPutAcceptsEveryNOnDrillsTamperDigest(t *testing.T) {
	cases := []struct {
		field   string
		cadence string
	}{
		{"drillsSchedule", "everyN 14 03:00"},
		{"tamperTestSchedule", "everyN 10 04:30"},
		{"digestSchedule", "everyN 3 08:15"},
	}
	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			d := &fakeServiceDocker{}
			h, _ := newTestRouter(t, d, &fakeResticEngine{})

			body := "{" + baseScheduleBody + `,
				"` + tc.field + `": "` + tc.cadence + `"}`
			w, m := doJSON(t, h, http.MethodPut, "/api/settings", body)
			if w.Code != http.StatusOK {
				t.Fatalf("put status = %d body=%s", w.Code, w.Body.String())
			}
			if m["ok"] != true {
				t.Fatalf("%s must accept %q (#166), got %v", tc.field, tc.cadence, m)
			}

			if s := getSettingsField(t, h, tc.field); s != tc.cadence {
				t.Fatalf("%s read back as %q, want %q", tc.field, s, tc.cadence)
			}
		})
	}
}

// An off-site replication job keeps no last-run record, so its interval could
// not be enforced.
func TestSettingsPutRejectsEveryNOnOffsite(t *testing.T) {
	for _, field := range []string{
		"containersOffsiteSchedule", "vmsOffsiteSchedule", "flashOffsiteSchedule",
		"configOffsiteSchedule", "filesOffsiteSchedule",
	} {
		t.Run(field, func(t *testing.T) {
			d := &fakeServiceDocker{}
			h, _ := newTestRouter(t, d, &fakeResticEngine{})

			body := "{" + baseScheduleBody + `,
				"` + field + `": "everyN 5 02:00"}`
			w, m := doJSON(t, h, http.MethodPut, "/api/settings", body)
			if w.Code != http.StatusOK {
				t.Fatalf("put status = %d body=%s", w.Code, w.Body.String())
			}
			if m["ok"] != false {
				t.Fatalf("%s must still refuse everyN, got %v", field, m)
			}
			if s, _ := m["error"].(string); !strings.Contains(s, "everyN") {
				t.Fatalf("the refusal should name everyN, got %q", m["error"])
			}
		})
	}
}

// The Settings page sends all three in one request when the user edits several
// cards before the auto-save fires.
func TestSettingsPutAcceptsEveryNOnAllThreeTogether(t *testing.T) {
	d := &fakeServiceDocker{}
	h, _ := newTestRouter(t, d, &fakeResticEngine{})

	body := "{" + baseScheduleBody + `,
		"drillsSchedule": "everyN 14 03:00",
		"tamperTestSchedule": "everyN 7 04:00",
		"digestSchedule": "everyN 2 09:00"}`
	w, m := doJSON(t, h, http.MethodPut, "/api/settings", body)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("all three everyN cadences must save together, status=%d got %v", w.Code, m)
	}

	for field, want := range map[string]string{
		"drillsSchedule":     "everyN 14 03:00",
		"tamperTestSchedule": "everyN 7 04:00",
		"digestSchedule":     "everyN 2 09:00",
	} {
		if s := getSettingsField(t, h, field); s != want {
			t.Fatalf("%s read back as %q, want %q", field, s, want)
		}
	}
}

// getSettingsField returns one string field of the "settings" object in
// GET /api/settings.
func getSettingsField(t *testing.T, h http.Handler, field string) string {
	t.Helper()
	w, env := doJSON(t, h, http.MethodGet, "/api/settings", "")
	if w.Code != http.StatusOK {
		t.Fatalf("get status = %d body=%s", w.Code, w.Body.String())
	}
	s, ok := env["settings"].(map[string]any)
	if !ok {
		t.Fatalf("GET /api/settings has no settings object, got %v", env)
	}
	v, _ := s[field].(string)
	return v
}
