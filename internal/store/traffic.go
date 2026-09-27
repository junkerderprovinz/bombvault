package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// TrafficSettings decide when BombVault sees a media server streaming and how
// far it slows its off-site uploads meanwhile.
type TrafficSettings struct {
	// StreamThrottle lowers the off-site upload limit while a stream runs.
	StreamThrottle bool
	// MediaServers are the containers watched for streams. Nil means nobody
	// chose yet, and the image names decide; an empty list watches none.
	MediaServers []string
	// StreamMbit is the send rate of a media server, in Mbit/s, from which it
	// counts as streaming.
	StreamMbit int
	// StreamLimitKiB is the upload limit while a stream runs.
	StreamLimitKiB int
	// StreamHoldMin is how long the send rate has to stay below StreamMbit
	// before the normal limit comes back.
	StreamHoldMin int
}

// DefaultTrafficSettings is what an install that never saved them uses.
func DefaultTrafficSettings() TrafficSettings {
	return TrafficSettings{StreamMbit: 2, StreamLimitKiB: 512, StreamHoldMin: 5}
}

// TrafficSettings returns the stored settings, or the defaults.
func (r *Repo) TrafficSettings() (TrafficSettings, error) {
	s := DefaultTrafficSettings()
	var throttle int
	var servers sql.NullString
	err := r.db.QueryRow(`SELECT stream_throttle, media_servers, stream_mbit, stream_limit_kib, stream_hold_min
		FROM traffic_settings WHERE id = 1`).
		Scan(&throttle, &servers, &s.StreamMbit, &s.StreamLimitKiB, &s.StreamHoldMin)
	if errors.Is(err, sql.ErrNoRows) {
		return s, nil
	}
	if err != nil {
		return TrafficSettings{}, fmt.Errorf("TrafficSettings: %w", err)
	}
	s.StreamThrottle = throttle != 0
	if servers.Valid {
		s.MediaServers = []string{}
		if err := json.Unmarshal([]byte(servers.String), &s.MediaServers); err != nil {
			return TrafficSettings{}, fmt.Errorf("TrafficSettings: media servers: %w", err)
		}
	}
	return s, nil
}

// SetTrafficSettings stores s whole.
func (r *Repo) SetTrafficSettings(s TrafficSettings) error {
	var servers sql.NullString
	if s.MediaServers != nil {
		b, err := json.Marshal(s.MediaServers)
		if err != nil {
			return fmt.Errorf("SetTrafficSettings: %w", err)
		}
		servers = sql.NullString{String: string(b), Valid: true}
	}
	_, err := r.db.Exec(`INSERT INTO traffic_settings (id, stream_throttle, media_servers, stream_mbit, stream_limit_kib, stream_hold_min)
		VALUES (1, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET stream_throttle = excluded.stream_throttle, media_servers = excluded.media_servers,
		  stream_mbit = excluded.stream_mbit, stream_limit_kib = excluded.stream_limit_kib, stream_hold_min = excluded.stream_hold_min`,
		boolInt(s.StreamThrottle), servers, s.StreamMbit, s.StreamLimitKiB, s.StreamHoldMin)
	if err != nil {
		return fmt.Errorf("SetTrafficSettings: %w", err)
	}
	return nil
}
