package api

import (
	"encoding/json"

	kitnotify "github.com/junkerderprovinz/parleyport/kit/notify"

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

// channelOf is the channel with id, or an empty one.
func channelOf(c notify.Config, id string) kitnotify.Channel {
	for _, ch := range c.Channels {
		if ch.ID == id {
			return ch
		}
	}
	return kitnotify.Channel{}
}

// matrixToken is the token of the Matrix channel a flat config became.
func matrixToken(c notify.Config) string {
	if ch := channelOf(c, "matrix"); ch.Matrix != nil {
		return ch.Matrix.Token
	}
	return ""
}

// unraidChannel is an Unraid channel as the card sends it to be tested.
var unraidChannel = kitnotify.Channel{ID: "unraid", Kind: kitnotify.KindUnraid, Enabled: true}
