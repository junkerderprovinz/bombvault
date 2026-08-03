package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrEmptyOffsiteRepo is returned by UpsertOffsiteTarget when the target has no
// repo location. An off-site DESTINATION with an empty repo is meaningless (it
// addresses nowhere) and would make offsiteRepoFor return "" for a domain that
// still has a target row, so it is rejected at the store boundary. The backfill
// migration only inserts rows for a non-empty off-site column, so it is unaffected.
var ErrEmptyOffsiteRepo = errors.New("off-site target repo must not be empty")

// OffsiteTarget is one off-site DESTINATION for a domain: a restic repo the
// domain's local backups are replicated to, plus that destination's own
// schedule/retention/limits/append-only settings. It is the plural successor to
// the single-repo-per-domain off-site columns on Settings.
//
// NOTE the naming trap: store.Target is a backup SOURCE (a container). This type
// (and its table offsite_targets / history column offsite_target_id) is the
// off-site DESTINATION and is unrelated to Target.
//
// Stage 1 is data-model-only: rows exist and are backfilled one-per-domain from
// the old Settings columns, but nothing in the live replication path consumes
// them yet (see primaryOffsiteTarget in internal/api). Do not rewire callers.
type OffsiteTarget struct {
	ID     string
	Domain string
	Name   string
	Repo   string
	// CredsRef selects which credential set this destination uses. Empty means
	// the shared/global cloud creds (today's single-repo behavior). Reserved for
	// stage 2; backfill leaves it empty.
	CredsRef string
	// StorageClass is the S3 storage class for this destination. It lives in the
	// encrypted cloud_conf blob, which the pure-SQL backfill migration cannot
	// decode (it needs the app secret key), so the backfill leaves this empty and
	// stage 2 copies it once ModeFor is target-aware.
	StorageClass         string
	Immutable            bool
	Schedule             string
	RetentionKeepLast    int
	RetentionKeepDaily   int
	RetentionKeepWeekly  int
	RetentionKeepMonthly int
	LimitUpload          int
	LimitDownload        int
	GrowthBudgetGB       int
	Enabled              bool
	CreatedAt            int64
	SortOrder            int
}

// UpsertOffsiteTarget inserts or updates an off-site target by id. An empty ID
// is assigned via newID(); CreatedAt is stamped now when 0. Returns the stored
// OffsiteTarget (with the assigned id/timestamp).
func (r *Repo) UpsertOffsiteTarget(t OffsiteTarget) (OffsiteTarget, error) {
	if strings.TrimSpace(t.Repo) == "" {
		return OffsiteTarget{}, ErrEmptyOffsiteRepo
	}
	if t.ID == "" {
		t.ID = newID()
	}
	if t.CreatedAt == 0 {
		t.CreatedAt = time.Now().Unix()
	}

	_, err := r.db.Exec(`
		INSERT INTO offsite_targets (id, domain, name, repo, creds_ref, storage_class, immutable, schedule,
		  retention_keep_last, retention_keep_daily, retention_keep_weekly, retention_keep_monthly,
		  limit_upload, limit_download, growth_budget_gb, enabled, created_at, sort_order)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
		  domain                 = excluded.domain,
		  name                   = excluded.name,
		  repo                   = excluded.repo,
		  creds_ref              = excluded.creds_ref,
		  storage_class          = excluded.storage_class,
		  immutable              = excluded.immutable,
		  schedule               = excluded.schedule,
		  retention_keep_last    = excluded.retention_keep_last,
		  retention_keep_daily   = excluded.retention_keep_daily,
		  retention_keep_weekly  = excluded.retention_keep_weekly,
		  retention_keep_monthly = excluded.retention_keep_monthly,
		  limit_upload           = excluded.limit_upload,
		  limit_download         = excluded.limit_download,
		  growth_budget_gb       = excluded.growth_budget_gb,
		  enabled                = excluded.enabled,
		  sort_order             = excluded.sort_order`,
		t.ID, t.Domain, t.Name, t.Repo, t.CredsRef, t.StorageClass, boolInt(t.Immutable), t.Schedule,
		t.RetentionKeepLast, t.RetentionKeepDaily, t.RetentionKeepWeekly, t.RetentionKeepMonthly,
		t.LimitUpload, t.LimitDownload, t.GrowthBudgetGB, boolInt(t.Enabled), t.CreatedAt, t.SortOrder,
	)
	if err != nil {
		return OffsiteTarget{}, fmt.Errorf("UpsertOffsiteTarget: %w", err)
	}
	return t, nil
}

const offsiteTargetCols = `id, domain, name, repo, creds_ref, storage_class, immutable, schedule,
	retention_keep_last, retention_keep_daily, retention_keep_weekly, retention_keep_monthly,
	limit_upload, limit_download, growth_budget_gb, enabled, created_at, sort_order`

// ListOffsiteTargets returns all off-site targets ordered by domain, then
// sort_order, then created_at (a stable per-domain display order).
func (r *Repo) ListOffsiteTargets() ([]OffsiteTarget, error) {
	rows, err := r.db.Query(`
		SELECT ` + offsiteTargetCols + `
		FROM offsite_targets ORDER BY domain, sort_order, created_at`)
	if err != nil {
		return nil, fmt.Errorf("ListOffsiteTargets: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []OffsiteTarget
	for rows.Next() {
		t, err := scanOffsiteTarget(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// OffsiteTargetsForDomain returns the off-site targets for a single domain,
// ordered by sort_order then created_at.
func (r *Repo) OffsiteTargetsForDomain(domain string) ([]OffsiteTarget, error) {
	rows, err := r.db.Query(`
		SELECT `+offsiteTargetCols+`
		FROM offsite_targets WHERE domain = ? ORDER BY sort_order, created_at`, domain)
	if err != nil {
		return nil, fmt.Errorf("OffsiteTargetsForDomain: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []OffsiteTarget
	for rows.Next() {
		t, err := scanOffsiteTarget(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetOffsiteTarget returns the off-site target with the given id. The bool is
// false (with a zero OffsiteTarget) when no such row exists.
func (r *Repo) GetOffsiteTarget(id string) (OffsiteTarget, bool, error) {
	row := r.db.QueryRow(`
		SELECT `+offsiteTargetCols+`
		FROM offsite_targets WHERE id = ?`, id)
	t, err := scanOffsiteTarget(row)
	if errors.Is(err, sql.ErrNoRows) {
		return OffsiteTarget{}, false, nil
	}
	if err != nil {
		return OffsiteTarget{}, false, err
	}
	return t, true, nil
}

// DeleteOffsiteTarget removes the off-site target with the given id. It is a
// no-op (no error) if the row does not exist.
func (r *Repo) DeleteOffsiteTarget(id string) error {
	if _, err := r.db.Exec(`DELETE FROM offsite_targets WHERE id = ?`, id); err != nil {
		return fmt.Errorf("DeleteOffsiteTarget: %w", err)
	}
	return nil
}

func scanOffsiteTarget(s scanner) (OffsiteTarget, error) {
	var t OffsiteTarget
	var immutable, enabled int
	err := s.Scan(
		&t.ID, &t.Domain, &t.Name, &t.Repo, &t.CredsRef, &t.StorageClass, &immutable, &t.Schedule,
		&t.RetentionKeepLast, &t.RetentionKeepDaily, &t.RetentionKeepWeekly, &t.RetentionKeepMonthly,
		&t.LimitUpload, &t.LimitDownload, &t.GrowthBudgetGB, &enabled, &t.CreatedAt, &t.SortOrder,
	)
	if err != nil {
		return OffsiteTarget{}, fmt.Errorf("scanOffsiteTarget: %w", err)
	}
	t.Immutable = immutable != 0
	t.Enabled = enabled != 0
	return t, nil
}
