package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// ErrNotifyConfChanged refuses a conversion of a notification config that was
// replaced since it was read.
var ErrNotifyConfChanged = errors.New("the notification config changed while it was converted")

// ConvertNotifyConf replaces the sealed notification config old with next and
// keeps old aside, so a later version can still read what was converted.
func (r *Repo) ConvertNotifyConf(old, next string, at int64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("ConvertNotifyConf: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // a rollback after commit is a no-op
	res, err := tx.Exec(`UPDATE settings SET notify_conf = ? WHERE id = 1 AND notify_conf = ?`, next, old)
	if err != nil {
		return fmt.Errorf("ConvertNotifyConf: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		return ErrNotifyConfChanged
	}
	if _, err := tx.Exec(`INSERT OR REPLACE INTO notify_before_channels (id, conf, converted_at) VALUES (1, ?, ?)`, old, at); err != nil {
		return fmt.Errorf("ConvertNotifyConf: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ConvertNotifyConf: %w", err)
	}
	return nil
}

// NotifyConfBeforeChannels returns the sealed config ConvertNotifyConf set
// aside and when, or "" when nothing was converted.
func (r *Repo) NotifyConfBeforeChannels() (string, int64, error) {
	var conf string
	var at int64
	err := r.db.QueryRow(`SELECT conf, converted_at FROM notify_before_channels WHERE id = 1`).Scan(&conf, &at)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, nil
	}
	if err != nil {
		return "", 0, fmt.Errorf("NotifyConfBeforeChannels: %w", err)
	}
	return conf, at, nil
}
