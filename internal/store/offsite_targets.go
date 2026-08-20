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
	// Role distinguishes what this row configures: "offsite" (the default — a
	// replication DESTINATION the domain's local backup is copied to) or
	// "primary" (issue #152: the domain's own primary backup path, resolved from
	// Settings.<Domain>Path, IS a remote restic repo — this row carries ONLY the
	// safety settings for it: LimitUpload/LimitDownload/Immutable/GrowthBudgetGB.
	// See internal/api's primaryRemoteTarget doc comment for the full contract).
	//
	// A "primary" row is NEVER a replication destination and is excluded from
	// every off-site-target query (OffsiteTargetsForDomain, ListOffsiteTargets)
	// so it can never be picked up by copyToOffsite / the multi-target replication
	// loop / the off-site CRUD UI — those all still see exactly the rows they saw
	// before this field existed. At most one "primary" row exists per domain.
	//
	// Reusing this struct/table (rather than a parallel schema) for the primary
	// case is deliberate: the shape needed — a repo location, bandwidth limits,
	// an append-only flag, a growth budget, a credential-set selector, an S3
	// storage class — is EXACTLY what OffsiteTarget already carries, and every
	// existing consumer of one of those fields (limitFlags, the tamper-test probe
	// in runTamperTestForTarget, offsiteModeForTarget's CredsRef/StorageClass
	// resolution) works on a "primary" row unmodified, for free. A "primary"
	// row's Repo field is a best-effort snapshot of the domain's path AT SAVE
	// TIME (for the tamper-test/deploy-snippet flows, which need SOME repo
	// string) — it is NEVER authoritative for backup path resolution, which
	// always reads Settings.<Domain>Path directly (unchanged).
	//
	// Normalized at the store boundary: an empty Role (every row inserted before
	// this field existed, and any caller that does not set it) is treated as
	// "offsite" on write, so no backfill/migration of existing rows is needed.
	Role string
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

// Off-site target roles (see OffsiteTarget.Role's doc comment).
const (
	RoleOffsite = "offsite" // a replication destination (the default)
	RolePrimary = "primary" // safety settings for a domain's own remote primary
)

// UpsertOffsiteTarget inserts or updates an off-site target by id. An empty ID
// is assigned via newID(); CreatedAt is stamped now when 0. An empty Role
// normalizes to RoleOffsite, so every row written before this field existed (and
// every caller that does not set it) keeps behaving as a replication
// destination. Returns the stored OffsiteTarget (with the assigned
// id/timestamp/role).
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
	if t.Role == "" {
		t.Role = RoleOffsite
	}

	_, err := r.db.Exec(`
		INSERT INTO offsite_targets (id, domain, name, repo, role, creds_ref, storage_class, immutable, schedule,
		  retention_keep_last, retention_keep_daily, retention_keep_weekly, retention_keep_monthly,
		  limit_upload, limit_download, growth_budget_gb, enabled, created_at, sort_order)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
		  domain                 = excluded.domain,
		  name                   = excluded.name,
		  repo                   = excluded.repo,
		  role                   = excluded.role,
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
		t.ID, t.Domain, t.Name, t.Repo, t.Role, t.CredsRef, t.StorageClass, boolInt(t.Immutable), t.Schedule,
		t.RetentionKeepLast, t.RetentionKeepDaily, t.RetentionKeepWeekly, t.RetentionKeepMonthly,
		t.LimitUpload, t.LimitDownload, t.GrowthBudgetGB, boolInt(t.Enabled), t.CreatedAt, t.SortOrder,
	)
	if err != nil {
		return OffsiteTarget{}, fmt.Errorf("UpsertOffsiteTarget: %w", err)
	}
	return t, nil
}

const offsiteTargetCols = `id, domain, name, repo, role, creds_ref, storage_class, immutable, schedule,
	retention_keep_last, retention_keep_daily, retention_keep_weekly, retention_keep_monthly,
	limit_upload, limit_download, growth_budget_gb, enabled, created_at, sort_order`

// ListOffsiteTargets returns all off-site REPLICATION DESTINATIONS (role =
// 'offsite'; a domain's "primary" safety-config row, if any, is never among
// them — see PrimaryRemoteTarget) ordered by domain, then sort_order, then
// created_at (a stable per-domain display order).
func (r *Repo) ListOffsiteTargets() ([]OffsiteTarget, error) {
	rows, err := r.db.Query(`
		SELECT `+offsiteTargetCols+`
		FROM offsite_targets WHERE role = ? ORDER BY domain, sort_order, created_at`, RoleOffsite)
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

// OffsiteTargetsForDomain returns the off-site REPLICATION DESTINATIONS (role =
// 'offsite') for a single domain, ordered by sort_order then created_at. A
// domain's "primary" row (issue #152 remote-primary safety settings, if any)
// is deliberately excluded — see PrimaryRemoteTarget — so it can never be
// picked up by the replication loop, the multi-target CRUD UI, or anything
// else that iterates a domain's off-site destinations.
func (r *Repo) OffsiteTargetsForDomain(domain string) ([]OffsiteTarget, error) {
	rows, err := r.db.Query(`
		SELECT `+offsiteTargetCols+`
		FROM offsite_targets WHERE domain = ? AND role = ? ORDER BY sort_order, created_at`, domain, RoleOffsite)
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

// GetOffsiteTarget returns the off-site REPLICATION DESTINATION (role =
// 'offsite') with the given id. The bool is false (with a zero OffsiteTarget)
// when no such row exists — including when id names a "primary" row: the
// off-site CRUD/test/delete handlers that call this must never read, edit,
// probe or delete a domain's remote-primary safety-config row through the
// off-site-target id surface (that row is reached only via
// PrimaryRemoteTarget/UpsertPrimaryRemoteTarget, keyed by domain, never id).
func (r *Repo) GetOffsiteTarget(id string) (OffsiteTarget, bool, error) {
	row := r.db.QueryRow(`
		SELECT `+offsiteTargetCols+`
		FROM offsite_targets WHERE id = ? AND role = ?`, id, RoleOffsite)
	t, err := scanOffsiteTarget(row)
	if errors.Is(err, sql.ErrNoRows) {
		return OffsiteTarget{}, false, nil
	}
	if err != nil {
		return OffsiteTarget{}, false, err
	}
	return t, true, nil
}

// DeleteOffsiteTarget removes the off-site REPLICATION DESTINATION (role =
// 'offsite') with the given id. It is a no-op (no error) if the row does not
// exist, or if id names a "primary" row — the off-site delete handler must
// never be able to remove a domain's remote-primary safety-config row (that
// row is removed only via DeletePrimaryRemoteTarget, keyed by domain).
func (r *Repo) DeleteOffsiteTarget(id string) error {
	if _, err := r.db.Exec(`DELETE FROM offsite_targets WHERE id = ? AND role = ?`, id, RoleOffsite); err != nil {
		return fmt.Errorf("DeleteOffsiteTarget: %w", err)
	}
	return nil
}

// PrimaryRemoteTarget returns the domain's "primary" row (issue #152: the
// remote-primary safety settings — bandwidth limits, append-only, growth
// budget — for when Settings.<Domain>Path is itself a restic remote), if one
// has been saved. The bool is false (with a zero OffsiteTarget) when the
// domain has never had its remote-primary safety settings configured — that is
// the common case (a local primary, or a remote primary nobody has opened the
// safety dialog for yet), not an error. At most one such row exists per
// domain (UpsertPrimaryRemoteTarget enforces it); this returns the first if
// more than one somehow exists (defensive — should be unreachable).
func (r *Repo) PrimaryRemoteTarget(domain string) (OffsiteTarget, bool, error) {
	row := r.db.QueryRow(`
		SELECT `+offsiteTargetCols+`
		FROM offsite_targets WHERE domain = ? AND role = ? ORDER BY created_at LIMIT 1`, domain, RolePrimary)
	t, err := scanOffsiteTarget(row)
	if errors.Is(err, sql.ErrNoRows) {
		return OffsiteTarget{}, false, nil
	}
	if err != nil {
		return OffsiteTarget{}, false, err
	}
	return t, true, nil
}

// UpsertPrimaryRemoteTarget creates or updates the domain's "primary" row (see
// PrimaryRemoteTarget). t.Domain and t.Role are stamped by this method (a
// caller-supplied value in either field is ignored), so callers only need to
// fill in Repo/CredsRef/StorageClass/Immutable/LimitUpload/LimitDownload/
// GrowthBudgetGB/Enabled. When a row already exists for the domain, its
// id/created_at are preserved (an update in place, exactly like
// UpsertOffsiteTarget's id-keyed upsert) rather than creating a second row.
func (r *Repo) UpsertPrimaryRemoteTarget(domain string, t OffsiteTarget) (OffsiteTarget, error) {
	existing, ok, err := r.PrimaryRemoteTarget(domain)
	if err != nil {
		return OffsiteTarget{}, fmt.Errorf("UpsertPrimaryRemoteTarget: read existing: %w", err)
	}
	t.Domain = domain
	t.Role = RolePrimary
	if ok {
		t.ID = existing.ID
		t.CreatedAt = existing.CreatedAt
	} else {
		t.ID = ""
		t.CreatedAt = 0
	}
	if t.Name == "" {
		t.Name = "Primary (remote)"
	}
	return r.UpsertOffsiteTarget(t)
}

// DeletePrimaryRemoteTarget removes the domain's "primary" row, if any (a
// no-op, no error, when none exists) — used when the operator clears a
// domain's remote-primary safety settings (e.g. switching the path back to a
// local folder).
func (r *Repo) DeletePrimaryRemoteTarget(domain string) error {
	if _, err := r.db.Exec(`DELETE FROM offsite_targets WHERE domain = ? AND role = ?`, domain, RolePrimary); err != nil {
		return fmt.Errorf("DeletePrimaryRemoteTarget: %w", err)
	}
	return nil
}

func scanOffsiteTarget(s scanner) (OffsiteTarget, error) {
	var t OffsiteTarget
	var immutable, enabled int
	err := s.Scan(
		&t.ID, &t.Domain, &t.Name, &t.Repo, &t.Role, &t.CredsRef, &t.StorageClass, &immutable, &t.Schedule,
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
