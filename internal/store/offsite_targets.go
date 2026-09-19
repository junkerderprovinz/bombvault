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
	// RoleRepo is a NAMED REPOSITORY (#204): a location written down once and
	// then PICKED by individual containers, VMs and folder sets, instead of
	// being typed into each of them.
	//
	// Same table for the same reason "primary" is here: the shape a named
	// repository needs is exactly what this struct already carries - a location,
	// a credential set, an S3 storage class, bandwidth limits, an enabled flag -
	// and every existing consumer of those fields works on such a row unchanged.
	// Every query in this file filters on an explicit role, so these rows are
	// invisible to the replication loop and the off-site CRUD by construction.
	//
	// Domain is deliberately EMPTY on a repo row. A location is a place; which
	// items send their backups there is the items' business, and scoping a
	// repository to one domain would mean writing the same B2 bucket down three
	// times to use it from a container, a VM and a folder set.
	RoleRepo = "repo"
)

// UpsertOffsiteTarget inserts t or updates the row with its id, keeping that
// row's sort_order, and returns the row as stored. An empty ID gets a fresh one
// and an empty Role means RoleOffsite.
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
	tx, err := r.db.Begin()
	if err != nil {
		return OffsiteTarget{}, fmt.Errorf("UpsertOffsiteTarget: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after a successful commit is a no-op
	_, err = tx.Exec(`
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
		  enabled                = excluded.enabled`,
		t.ID, t.Domain, t.Name, t.Repo, t.Role, t.CredsRef, t.StorageClass, boolInt(t.Immutable), t.Schedule,
		t.RetentionKeepLast, t.RetentionKeepDaily, t.RetentionKeepWeekly, t.RetentionKeepMonthly,
		t.LimitUpload, t.LimitDownload, t.GrowthBudgetGB, boolInt(t.Enabled), t.CreatedAt, t.SortOrder,
	)
	if err != nil {
		return OffsiteTarget{}, fmt.Errorf("UpsertOffsiteTarget: %w", err)
	}
	return commitStoredTargetTx(tx, t.ID)
}

// CreateOffsiteTarget inserts a replication destination behind the domain's last one.
func (r *Repo) CreateOffsiteTarget(t OffsiteTarget) (OffsiteTarget, error) {
	if strings.TrimSpace(t.Repo) == "" {
		return OffsiteTarget{}, ErrEmptyOffsiteRepo
	}
	if t.ID == "" {
		t.ID = newID()
	}
	if t.CreatedAt == 0 {
		t.CreatedAt = time.Now().Unix()
	}
	tx, err := r.db.Begin()
	if err != nil {
		return OffsiteTarget{}, fmt.Errorf("CreateOffsiteTarget: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after a successful commit is a no-op
	_, err = tx.Exec(`
		INSERT INTO offsite_targets (id, domain, name, repo, role, creds_ref, storage_class, immutable, schedule,
		  retention_keep_last, retention_keep_daily, retention_keep_weekly, retention_keep_monthly,
		  limit_upload, limit_download, growth_budget_gb, enabled, created_at, sort_order)
		SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, COALESCE(MAX(sort_order), 0) + 1
		  FROM offsite_targets WHERE role = ? AND domain = ?`,
		t.ID, t.Domain, t.Name, t.Repo, RoleOffsite, t.CredsRef, t.StorageClass, boolInt(t.Immutable), t.Schedule,
		t.RetentionKeepLast, t.RetentionKeepDaily, t.RetentionKeepWeekly, t.RetentionKeepMonthly,
		t.LimitUpload, t.LimitDownload, t.GrowthBudgetGB, boolInt(t.Enabled), t.CreatedAt,
		RoleOffsite, t.Domain,
	)
	if err != nil {
		return OffsiteTarget{}, fmt.Errorf("CreateOffsiteTarget: %w", err)
	}
	return commitStoredTargetTx(tx, t.ID)
}

// commitStoredTargetTx reads back the row a write in tx just made, then commits.
func commitStoredTargetTx(tx *sql.Tx, id string) (OffsiteTarget, error) {
	t, err := scanOffsiteTarget(tx.QueryRow(`SELECT `+offsiteTargetCols+` FROM offsite_targets WHERE id = ?`, id))
	if err != nil {
		return OffsiteTarget{}, err
	}
	if err := tx.Commit(); err != nil {
		return OffsiteTarget{}, fmt.Errorf("commit offsite target: %w", err)
	}
	return t, nil
}

// FieldOffsiteTarget returns the row the domain's off-site settings field edits:
// role offsite, sort_order 0, enabled or not.
func (r *Repo) FieldOffsiteTarget(domain string) (OffsiteTarget, bool, error) {
	row := r.db.QueryRow(`SELECT `+offsiteTargetCols+`
		FROM offsite_targets WHERE domain = ? AND role = ? AND sort_order = 0
		ORDER BY created_at, id LIMIT 1`, domain, RoleOffsite)
	t, err := scanOffsiteTarget(row)
	if errors.Is(err, sql.ErrNoRows) {
		return OffsiteTarget{}, false, nil
	}
	if err != nil {
		return OffsiteTarget{}, false, err
	}
	return t, true, nil
}

// NormalizeOffsiteSortOrder applies the rule of the offsite_targets_primary_slot
// migration to one domain: the oldest target whose location is field takes
// sort_order 0, and every other target on 0 moves behind the domain's last one,
// in the order they were created. With field empty, the oldest switched-off
// target already on sort_order 0 keeps it instead: that is the row a cleared
// field leaves behind, still holding its id and credentials for the next fill.
func (r *Repo) NormalizeOffsiteSortOrder(domain, field string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("NormalizeOffsiteSortOrder: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after a successful commit is a no-op
	slots, err := targetSlotsTx(tx, domain)
	if err != nil {
		return err
	}
	primary, last := "", 0
	for _, s := range slots {
		switch {
		case primary != "":
		case field != "" && s.repo == field:
			primary = s.id
		case field == "" && s.order == 0 && !s.enabled:
			primary = s.id
		}
		last = max(last, s.order)
	}
	next := last + 1
	for _, s := range slots {
		order := s.order
		switch {
		case s.id == primary:
			order = 0
		case s.order == 0:
			order = next
			next++
		}
		if order == s.order {
			continue
		}
		if _, err := tx.Exec(`UPDATE offsite_targets SET sort_order = ? WHERE id = ?`, order, s.id); err != nil {
			return fmt.Errorf("NormalizeOffsiteSortOrder: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("NormalizeOffsiteSortOrder commit: %w", err)
	}
	return nil
}

type targetSlot struct {
	id, repo string
	order    int
	enabled  bool
}

// targetSlotsTx lists a domain's replication destinations in creation order.
func targetSlotsTx(tx *sql.Tx, domain string) ([]targetSlot, error) {
	rows, err := tx.Query(`SELECT id, repo, sort_order, enabled FROM offsite_targets
		WHERE domain = ? AND role = ? ORDER BY created_at, id`, domain, RoleOffsite)
	if err != nil {
		return nil, fmt.Errorf("list target slots: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	var out []targetSlot
	for rows.Next() {
		var s targetSlot
		var enabled int
		if err := rows.Scan(&s.id, &s.repo, &s.order, &enabled); err != nil {
			return nil, fmt.Errorf("scan target slot: %w", err)
		}
		s.enabled = enabled != 0
		out = append(out, s)
	}
	return out, rows.Err()
}

const offsiteTargetCols = `id, domain, name, repo, role, creds_ref, storage_class, immutable, schedule,
	retention_keep_last, retention_keep_daily, retention_keep_weekly, retention_keep_monthly,
	limit_upload, limit_download, growth_budget_gb, enabled, created_at, sort_order`

// ListOffsiteTargets returns all off-site REPLICATION DESTINATIONS (role =
// 'offsite'; a domain's "primary" safety-config row, if any, is never among
// them — see PrimaryRemoteTarget) ordered by domain, then sort_order, then
// created_at, then id (a stable per-domain display order, tie-broken all the
// way down so equal sort_order and created_at never leave the order to chance).
func (r *Repo) ListOffsiteTargets() ([]OffsiteTarget, error) {
	rows, err := r.db.Query(`
		SELECT `+offsiteTargetCols+`
		FROM offsite_targets WHERE role = ? ORDER BY domain, sort_order, created_at, id`, RoleOffsite)
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
// 'offsite') for a single domain, ordered by sort_order, then created_at, then
// id. A domain's "primary" row (issue #152 remote-primary safety settings, if any)
// is deliberately excluded — see PrimaryRemoteTarget — so it can never be
// picked up by the replication loop, the multi-target CRUD UI, or anything
// else that iterates a domain's off-site destinations.
func (r *Repo) OffsiteTargetsForDomain(domain string) ([]OffsiteTarget, error) {
	rows, err := r.db.Query(`
		SELECT `+offsiteTargetCols+`
		FROM offsite_targets WHERE domain = ? AND role = ? ORDER BY sort_order, created_at, id`, domain, RoleOffsite)
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

// ListNamedRepos returns the NAMED REPOSITORIES (role = RoleRepo), ordered the
// way the picker shows them. Never nil.
func (r *Repo) ListNamedRepos() ([]OffsiteTarget, error) {
	rows, err := r.db.Query(`SELECT `+offsiteTargetCols+`
		FROM offsite_targets WHERE role = ? ORDER BY sort_order, name, created_at`, RoleRepo)
	if err != nil {
		return nil, fmt.Errorf("ListNamedRepos: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	out := make([]OffsiteTarget, 0)
	for rows.Next() {
		t, err := scanOffsiteTarget(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetNamedRepo returns one named repository by id, or sql.ErrNoRows. Scoped to
// the role, so an off-site target's id can never be resolved through here.
func (r *Repo) GetNamedRepo(id string) (OffsiteTarget, error) {
	row := r.db.QueryRow(`SELECT `+offsiteTargetCols+`
		FROM offsite_targets WHERE id = ? AND role = ?`, id, RoleRepo)
	return scanOffsiteTarget(row)
}

// itemsUsingNamedRepoQ is the in-use count, written once and run against either
// the database or an open transaction, so the guarded writes below cannot drift
// from the count the interface shows.
const itemsUsingNamedRepoQ = `
		SELECT (SELECT COUNT(*) FROM targets   WHERE repo = ?)
		     + (SELECT COUNT(*) FROM vms       WHERE repo = ?)
		     + (SELECT COUNT(*) FROM file_sets WHERE repo = ?)`

// DeleteNamedRepoIfUnused deletes a named repository ONLY while nothing points
// at it, counting and deleting in ONE transaction. It returns the count it saw:
// 0 means the row is gone, anything else means nothing was written.
//
// The count and the delete were two separate statements, which left a window:
// an item pointed at the repository between them was silently put back on its
// domain repository, and its next backup landed there looking exactly like a
// working backup. The window is small and a single operator will rarely hit it -
// but the whole point of the refusal is that this particular mistake is
// invisible afterwards, so it must not have a race that reproduces it.
func (r *Repo) DeleteNamedRepoIfUnused(id string) (int, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("DeleteNamedRepoIfUnused: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after a successful commit is a no-op
	var n int
	if err := tx.QueryRow(itemsUsingNamedRepoQ, id, id, id).Scan(&n); err != nil {
		return 0, fmt.Errorf("DeleteNamedRepoIfUnused count: %w", err)
	}
	if n > 0 {
		return n, nil
	}
	if _, err := tx.Exec(`DELETE FROM offsite_targets WHERE id = ? AND role = ?`, id, RoleRepo); err != nil {
		return 0, fmt.Errorf("DeleteNamedRepoIfUnused: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("DeleteNamedRepoIfUnused commit: %w", err)
	}
	return 0, nil
}

// SetNamedRepoLocationIfUnused moves a named repository's location ONLY while
// nothing points at it, in one transaction, for the same reason
// DeleteNamedRepoIfUnused does it that way: everything already written stays
// where it is, so a move under a live item makes its next backup succeed into
// an empty repository. Returns the in-use count it saw; 0 means the move was
// written.
func (r *Repo) SetNamedRepoLocationIfUnused(id, location string) (int, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("SetNamedRepoLocationIfUnused: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after a successful commit is a no-op
	var n int
	if err := tx.QueryRow(itemsUsingNamedRepoQ, id, id, id).Scan(&n); err != nil {
		return 0, fmt.Errorf("SetNamedRepoLocationIfUnused count: %w", err)
	}
	if n > 0 {
		return n, nil
	}
	if _, err := tx.Exec(`UPDATE offsite_targets SET repo = ? WHERE id = ? AND role = ?`, location, id, RoleRepo); err != nil {
		return 0, fmt.Errorf("SetNamedRepoLocationIfUnused: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("SetNamedRepoLocationIfUnused commit: %w", err)
	}
	return 0, nil
}

// ItemsUsingNamedRepo counts the containers, VMs and file sets that currently
// point at this named repository. Used for DISPLAY (the "n in use" badge) and to
// explain a refusal before it happens; the refusals themselves are enforced by
// DeleteNamedRepoIfUnused / SetNamedRepoLocationIfUnused, which re-count inside
// their own transaction so the answer cannot go stale between the two calls.
func (r *Repo) ItemsUsingNamedRepo(id string) (int, error) {
	var n int
	err := r.db.QueryRow(itemsUsingNamedRepoQ, id, id, id).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("ItemsUsingNamedRepo: %w", err)
	}
	return n, nil
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
// What the observation tables recorded for the target goes with it.
func (r *Repo) DeleteOffsiteTarget(id string) error {
	err := r.inTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM offsite_targets WHERE id = ? AND role = ?`, id, RoleOffsite); err != nil {
			return err
		}
		return deleteTargetObservationsTx(tx, id)
	})
	if err != nil {
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
