package api_test

// #166 — the API contract half of "every N days" for the drills / tamper-test /
// digest schedules. The reporter's complaint was that the option could not be
// used: the save came back ok:false with "this schedule does not support
// 'everyN'" and the UI snapped the value back. Now that the scheduler genuinely
// enforces the interval for those three (each records its own last-run pass,
// see internal/store schedule_job_runs.go), the rejection is gone — but it must
// still stand for the five OFF-SITE replication schedules, which still have no
// last-run fact to gate on.

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

// TestSettingsPutAcceptsEveryNOnDrillsTamperDigest walks the three schedules the
// issue is about: each must SAVE with an everyN cadence and read back unchanged
// on the next GET (the "survives a reload" half — a value that saved but did not
// persist would look identical in the UI to the original bug).
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

			// Read it back: the stored cadence must be exactly what was saved.
			if s := getSettingsField(t, h, tc.field); s != tc.cadence {
				t.Fatalf("%s read back as %q, want %q", tc.field, s, tc.cadence)
			}
		})
	}
}

// TestSettingsPutStillRejectsEveryNOnOffsite pins the half of the old rule that
// deliberately stays: an off-site replication job has no last-run fact, so its
// interval could not be enforced and the cadence is still refused at save time.
func TestSettingsPutStillRejectsEveryNOnOffsite(t *testing.T) {
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

// TestSettingsPutAcceptsEveryNOnAllThreeTogether saves all three at once, the
// way the Settings page does when a user edits more than one card before the
// auto-save fires — a per-field allow-list that only worked one at a time would
// still leave the reporter stuck.
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

// getSettingsField GETs /api/settings and returns one string field out of the
// nested "settings" object (the GET envelope is shape-symmetric with the PUT
// body, so the settings live one level down from "ok").
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
