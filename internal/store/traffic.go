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
	// IdleCPUPct, IdleNetMbit and IdleQuietMin say when a container that is
	// no media server counts as idle: CPU below IdleCPUPct percent of a core
	// and traffic both ways below IdleNetMbit, for IdleQuietMin minutes.
	IdleCPUPct   int
	IdleNetMbit  int
	IdleQuietMin int
}

// DefaultTrafficSettings is what an install that never saved them uses.
func DefaultTrafficSettings() TrafficSettings {
	return TrafficSettings{StreamMbit: 2, StreamLimitKiB: 512, StreamHoldMin: 5, IdleCPUPct: 10, IdleNetMbit: 1, IdleQuietMin: 3}
}

// TrafficSettings returns the stored settings, or the defaults.
func (r *Repo) TrafficSettings() (TrafficSettings, error) {
	s := DefaultTrafficSettings()
	var throttle int
	var servers sql.NullString
	err := r.db.QueryRow(`SELECT stream_throttle, media_servers, stream_mbit, stream_limit_kib, stream_hold_min,
		idle_cpu_pct, idle_net_mbit, idle_quiet_min
		FROM traffic_settings WHERE id = 1`).
		Scan(&throttle, &servers, &s.StreamMbit, &s.StreamLimitKiB, &s.StreamHoldMin,
			&s.IdleCPUPct, &s.IdleNetMbit, &s.IdleQuietMin)
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
	_, err := r.db.Exec(`INSERT INTO traffic_settings (id, stream_throttle, media_servers, stream_mbit, stream_limit_kib, stream_hold_min,
		  idle_cpu_pct, idle_net_mbit, idle_quiet_min)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET stream_throttle = excluded.stream_throttle, media_servers = excluded.media_servers,
		  stream_mbit = excluded.stream_mbit, stream_limit_kib = excluded.stream_limit_kib, stream_hold_min = excluded.stream_hold_min,
		  idle_cpu_pct = excluded.idle_cpu_pct, idle_net_mbit = excluded.idle_net_mbit, idle_quiet_min = excluded.idle_quiet_min`,
		boolInt(s.StreamThrottle), servers, s.StreamMbit, s.StreamLimitKiB, s.StreamHoldMin,
		s.IdleCPUPct, s.IdleNetMbit, s.IdleQuietMin)
	if err != nil {
		return fmt.Errorf("SetTrafficSettings: %w", err)
	}
	return nil
}

// IdleWaitHours maps each target that waits for its app to be idle before a
// scheduled backup to the most hours it waits.
func (r *Repo) IdleWaitHours() (map[string]int, error) {
	rows, err := r.db.Query(`SELECT target_id, hours FROM item_idle_wait WHERE hours > 0`)
	if err != nil {
		return nil, fmt.Errorf("IdleWaitHours: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	out := map[string]int{}
	for rows.Next() {
		var id string
		var h int
		if err := rows.Scan(&id, &h); err != nil {
			return nil, fmt.Errorf("IdleWaitHours: %w", err)
		}
		out[id] = h
	}
	return out, rows.Err()
}

// SetIdleWaitHours sets how many hours a scheduled backup of the target waits
// at most for its app to be idle. Zero switches the wait off.
func (r *Repo) SetIdleWaitHours(targetID string, hours int) error {
	var err error
	if hours <= 0 {
		_, err = r.db.Exec(`DELETE FROM item_idle_wait WHERE target_id = ?`, targetID)
	} else {
		_, err = r.db.Exec(`INSERT INTO item_idle_wait (target_id, hours) VALUES (?, ?)
			ON CONFLICT(target_id) DO UPDATE SET hours = excluded.hours`, targetID, hours)
	}
	if err != nil {
		return fmt.Errorf("SetIdleWaitHours: %w", err)
	}
	return nil
}

// SetContainerIdleWait is SetIdleWaitHours for a container by name, creating
// its target row if it has none yet, so the wait can be set before the first
// backup.
func (r *Repo) SetContainerIdleWait(name string, hours int) error {
	t, err := r.GetTargetByContainer(name)
	if errors.Is(err, sql.ErrNoRows) {
		t, err = r.UpsertTarget(Target{ContainerName: name})
	}
	if err != nil {
		return fmt.Errorf("SetContainerIdleWait: %w", err)
	}
	return r.SetIdleWaitHours(t.ID, hours)
}

// IdleWaitGroup is a scheduled backup held back until its app is idle, kept so
// the wait outlives a restart. Members back up together: a single container,
// or the members of one compose stack that were due in the same run.
type IdleWaitGroup struct {
	Key     string
	Stack   string
	Members []string
	// Trigger names the run that held the backup back: domain, item or
	// everything.
	Trigger  string
	Since    int64
	Deadline int64
}

// SaveIdleWaitGroup stores g, replacing a group under the same key.
func (r *Repo) SaveIdleWaitGroup(g IdleWaitGroup) error {
	members, err := json.Marshal(g.Members)
	if err != nil {
		return fmt.Errorf("SaveIdleWaitGroup: %w", err)
	}
	_, err = r.db.Exec(`INSERT INTO idle_wait_groups (key, stack, members, trigger, since, deadline) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET stack = excluded.stack, members = excluded.members, trigger = excluded.trigger,
		  since = excluded.since, deadline = excluded.deadline`,
		g.Key, g.Stack, string(members), g.Trigger, g.Since, g.Deadline)
	if err != nil {
		return fmt.Errorf("SaveIdleWaitGroup: %w", err)
	}
	return nil
}

// DeleteIdleWaitGroup forgets a group once its backup ran or was dropped.
func (r *Repo) DeleteIdleWaitGroup(key string) error {
	if _, err := r.db.Exec(`DELETE FROM idle_wait_groups WHERE key = ?`, key); err != nil {
		return fmt.Errorf("DeleteIdleWaitGroup: %w", err)
	}
	return nil
}

// ListIdleWaitGroups returns the stored groups, the longest waiting first.
func (r *Repo) ListIdleWaitGroups() ([]IdleWaitGroup, error) {
	rows, err := r.db.Query(`SELECT key, stack, members, trigger, since, deadline FROM idle_wait_groups ORDER BY since, key`)
	if err != nil {
		return nil, fmt.Errorf("ListIdleWaitGroups: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	var out []IdleWaitGroup
	for rows.Next() {
		var g IdleWaitGroup
		var members string
		if err := rows.Scan(&g.Key, &g.Stack, &members, &g.Trigger, &g.Since, &g.Deadline); err != nil {
			return nil, fmt.Errorf("ListIdleWaitGroups: %w", err)
		}
		if err := json.Unmarshal([]byte(members), &g.Members); err != nil {
			return nil, fmt.Errorf("ListIdleWaitGroups: members of %s: %w", g.Key, err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
