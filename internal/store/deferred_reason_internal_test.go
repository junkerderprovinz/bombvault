package store

import "testing"

// The frontend reads the server, the count and the rate back out of these
// sentences, so their shape is pinned here as well as in deferral.test.ts.
func TestDeferredReasonsKeepTheirShape(t *testing.T) {
	for got, want := range map[string]string{
		DeferredSessions("Plex", 1):  "Plex has 1 active session",
		DeferredSessions("Emby", 3):  "Emby has 3 active sessions",
		DeferredNoAnswer("Jellyfin"): "Jellyfin did not answer",
		DeferredNoKey("Jellyfin"):    "Jellyfin has no API key set",
		DeferredSending(100, 1):      "the container sent 100 KiB/s or more within the last 1 minute",
		DeferredSending(100, 5):      "the container sent 100 KiB/s or more within the last 5 minutes",
		DeferredMeasuring(1):         "BombVault has watched the container's traffic for less than 1 minute",
		DeferredMeasuring(10):        "BombVault has watched the container's traffic for less than 10 minutes",
	} {
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}
