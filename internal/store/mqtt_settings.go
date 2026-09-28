package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// MQTTSettings is the connection to the broker Home Assistant listens on. The
// password is sealed with the APP_KEY like every other stored credential.
// NodeID names this instance in the topics and in Home Assistant; it is made
// once, so renaming the instance does not create a second device.
type MQTTSettings struct {
	Enabled     bool
	Host        string
	Port        int
	Username    string
	PasswordEnc []byte
	TLS         bool
	Prefix      string
	Buttons     bool
	NodeID      string
}

// DefaultMQTTSettings is what a fresh install starts from. The buttons start
// off, so switching the link on does not let every client of the broker start
// backups.
func DefaultMQTTSettings() MQTTSettings {
	return MQTTSettings{Port: 1883, Prefix: "bombvault"}
}

// GetMQTTSettings returns the stored settings, or the defaults before the
// first save.
func (r *Repo) GetMQTTSettings() (MQTTSettings, error) {
	s := DefaultMQTTSettings()
	err := r.db.QueryRow(`SELECT enabled, host, port, username, password_enc, tls, topic_prefix, buttons, node_id
		FROM mqtt_settings WHERE id = 1`).
		Scan(&s.Enabled, &s.Host, &s.Port, &s.Username, &s.PasswordEnc, &s.TLS, &s.Prefix, &s.Buttons, &s.NodeID)
	if errors.Is(err, sql.ErrNoRows) {
		return DefaultMQTTSettings(), nil
	}
	if err != nil {
		return MQTTSettings{}, fmt.Errorf("GetMQTTSettings: %w", err)
	}
	return s, nil
}

// SaveMQTTSettings replaces the stored settings.
func (r *Repo) SaveMQTTSettings(s MQTTSettings) error {
	_, err := r.db.Exec(`INSERT INTO mqtt_settings (id, enabled, host, port, username, password_enc, tls, topic_prefix, buttons, node_id)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET enabled = excluded.enabled, host = excluded.host, port = excluded.port,
		  username = excluded.username, password_enc = excluded.password_enc, tls = excluded.tls,
		  topic_prefix = excluded.topic_prefix, buttons = excluded.buttons, node_id = excluded.node_id`,
		s.Enabled, s.Host, s.Port, s.Username, s.PasswordEnc, s.TLS, s.Prefix, s.Buttons, s.NodeID)
	if err != nil {
		return fmt.Errorf("SaveMQTTSettings: %w", err)
	}
	return nil
}
