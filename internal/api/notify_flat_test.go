package api_test

import (
	"encoding/json"

	"github.com/junkerderprovinz/bombvault/internal/notify"
)

// oldNotify builds a config from the flat settings BombVault stored before
// channels, so a test reads as the settings a user had. Each channel tries
// once, so a test of a failing endpoint does not wait out the retries.
func oldNotify(flat map[string]any) notify.Config {
	data, err := json.Marshal(flat)
	if err != nil {
		panic(err)
	}
	var c notify.Config
	if err := json.Unmarshal(data, &c); err != nil {
		panic(err)
	}
	for i := range c.Channels {
		c.Channels[i].Attempts = 1
	}
	return c
}
