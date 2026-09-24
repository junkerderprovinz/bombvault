package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Alias records OldName as a former name of the target, so a rename keeps one
// history without rewriting the repository: readers use the current name plus
// every alias. An alias claims only the OldName snapshots taken before
// LinkedAt; a machine that reuses the name later is not this target.
type Alias struct {
	ID       string
	Domain   string // "container" or "vm"
	OldName  string
	TargetID string
	LinkedAt int64 // unix seconds
	// PrevDefinition is the VM definition the entry had under OldName, which
	// an unlink or a rename back restores. Only AliasByOldName and
	// TargetAliasesWithDefinitions read it.
	PrevDefinition string
}

func (r *Repo) AddAlias(domain, oldName, targetID string) (Alias, error) {
	return r.AddAliasAt(domain, oldName, targetID, time.Now().Unix())
}

// AddAliasAt is AddAlias with an explicit linked_at (unix seconds). An alias
// claims only its old name's snapshots from before linked_at, so Discover
// passes the time the rename happened rather than the time it runs.
func (r *Repo) AddAliasAt(domain, oldName, targetID string, linkedAt int64) (Alias, error) {
	a := Alias{ID: newID(), Domain: domain, OldName: oldName, TargetID: targetID, LinkedAt: linkedAt}
	_, err := r.db.Exec(`INSERT INTO target_aliases (id, domain, old_name, target_id, linked_at) VALUES (?, ?, ?, ?, ?)`,
		a.ID, a.Domain, a.OldName, a.TargetID, a.LinkedAt)
	if err != nil {
		return Alias{}, fmt.Errorf("AddAlias: %w", err)
	}
	return a, nil
}

// AddVMAliasAt is AddAliasAt for a VM entry, keeping prevDefinition, the
// definition the entry had under oldName, for an unlink to restore.
func (r *Repo) AddVMAliasAt(oldName, targetID string, linkedAt int64, prevDefinition string) (Alias, error) {
	a := Alias{ID: newID(), Domain: "vm", OldName: oldName, TargetID: targetID, LinkedAt: linkedAt, PrevDefinition: prevDefinition}
	_, err := r.db.Exec(`INSERT INTO target_aliases (id, domain, old_name, target_id, linked_at, prev_definition) VALUES (?, ?, ?, ?, ?, ?)`,
		a.ID, a.Domain, a.OldName, a.TargetID, a.LinkedAt, a.PrevDefinition)
	if err != nil {
		return Alias{}, fmt.Errorf("AddVMAliasAt: %w", err)
	}
	return a, nil
}

func (r *Repo) AliasNames(domain, targetID string) ([]string, error) {
	rows, err := r.db.Query(`SELECT old_name FROM target_aliases WHERE domain = ? AND target_id = ? ORDER BY linked_at`, domain, targetID)
	if err != nil {
		return nil, fmt.Errorf("AliasNames: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, fmt.Errorf("AliasNames scan: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *Repo) AliasByOldName(domain, oldName string) (Alias, error) {
	var a Alias
	err := r.db.QueryRow(`SELECT id, domain, old_name, target_id, linked_at, prev_definition FROM target_aliases WHERE domain = ? AND old_name = ?`, domain, oldName).
		Scan(&a.ID, &a.Domain, &a.OldName, &a.TargetID, &a.LinkedAt, &a.PrevDefinition)
	return a, err
}

func (r *Repo) ListAliases(domain string) ([]Alias, error) {
	return r.queryAliases("ListAliases", false, `SELECT id, domain, old_name, target_id, linked_at FROM target_aliases WHERE domain = ? ORDER BY old_name`, domain)
}

// TargetAliases returns one target's aliases with their linked_at, oldest
// link first: what a reader needs to decide which old-name snapshots the
// target may claim.
func (r *Repo) TargetAliases(domain, targetID string) ([]Alias, error) {
	return r.queryAliases("TargetAliases", false, `SELECT id, domain, old_name, target_id, linked_at FROM target_aliases WHERE domain = ? AND target_id = ? ORDER BY linked_at`, domain, targetID)
}

// TargetAliasesWithDefinitions is TargetAliases with each alias's
// PrevDefinition, for the link records a definition mirror carries. Readers
// use TargetAliases, which leaves the definitions in the database.
func (r *Repo) TargetAliasesWithDefinitions(domain, targetID string) ([]Alias, error) {
	return r.queryAliases("TargetAliasesWithDefinitions", true, `SELECT id, domain, old_name, target_id, linked_at, prev_definition FROM target_aliases WHERE domain = ? AND target_id = ? ORDER BY linked_at`, domain, targetID)
}

// entryTable is where one domain keeps its entries: the domain its aliases
// carry, the table and the column that holds the name.
type entryTable struct{ domain, table, nameCol string }

var (
	containerEntries = entryTable{"container", "targets", "container_name"}
	vmEntries        = entryTable{"vm", "vms", "name"}
)

// entriesOf returns the entry table whose aliases carry aliasDomain.
func entriesOf(aliasDomain string) (entryTable, error) {
	for _, e := range []entryTable{containerEntries, vmEntries} {
		if e.domain == aliasDomain {
			return e, nil
		}
	}
	return entryTable{}, fmt.Errorf("no entries have aliases of %q", aliasDomain)
}

// carries reports whether a row of e has name.
func (e entryTable) carries(tx *sql.Tx, name string) (bool, error) {
	var n int
	if err := tx.QueryRow(`SELECT count(*) FROM `+e.table+` WHERE `+e.nameCol+` = ?`, name).Scan(&n); err != nil {
		return false, fmt.Errorf("check the entry of %s: %w", name, err)
	}
	return n > 0, nil
}

// renameWithAlias moves the entry of e on oldName to newName and links
// oldName to it, in one transaction; see RenameTargetWithAlias. A VM alias
// keeps the definition the entry had under oldName, and a VM row takes uuid.
func (r *Repo) renameWithAlias(e entryTable, oldName, newName, newDefinition, uuid string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("rename %q: %w", oldName, err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit
	var id, prevDefinition string
	if err := tx.QueryRow(`SELECT id, definition FROM `+e.table+` WHERE `+e.nameCol+` = ?`, oldName).Scan(&id, &prevDefinition); err != nil {
		return fmt.Errorf("rename %q: read it: %w", oldName, err)
	}
	var taken int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM `+e.table+` WHERE `+e.nameCol+` = ?`, newName).Scan(&taken); err != nil {
		return fmt.Errorf("rename %q: check %q: %w", oldName, newName, err)
	}
	if taken > 0 {
		return fmt.Errorf("%q already has its own entry", newName)
	}
	var aliasOwner string
	err = tx.QueryRow(`SELECT target_id FROM target_aliases WHERE domain = ? AND old_name = ?`, e.domain, newName).Scan(&aliasOwner)
	switch {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return fmt.Errorf("rename %q: read alias %q: %w", oldName, newName, err)
	case aliasOwner != id:
		return fmt.Errorf("%q is another entry's former name", newName)
	default:
		if _, err := tx.Exec(`DELETE FROM target_aliases WHERE domain = ? AND old_name = ?`, e.domain, newName); err != nil {
			return fmt.Errorf("rename %q: drop alias %q: %w", oldName, newName, err)
		}
	}
	placement, prefix, err := placementDomainForAlias(e.domain)
	if err != nil {
		return err
	}
	if err := renameCopyRuleTx(tx, placement, prefix+oldName, prefix+newName); err != nil {
		return fmt.Errorf("rename %q: %w", oldName, err)
	}
	if err := e.moveRow(tx, id, newName, newDefinition, uuid); err != nil {
		return fmt.Errorf("rename %q: %w", oldName, err)
	}
	if e.domain != "vm" {
		prevDefinition = ""
	}
	if _, err := tx.Exec(`INSERT INTO target_aliases (id, domain, old_name, target_id, linked_at, prev_definition) VALUES (?, ?, ?, ?, ?, ?)`,
		newID(), e.domain, oldName, id, time.Now().Unix(), prevDefinition); err != nil {
		return fmt.Errorf("rename %q: alias: %w", oldName, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("rename %q: commit: %w", oldName, err)
	}
	return nil
}

// unlinkAlias moves the entry of e that oldName is linked to back onto
// oldName and removes the alias, in one transaction; see UnlinkAlias. A VM
// row takes uuid.
func (r *Repo) unlinkAlias(e entryTable, oldName, newDefinition, uuid string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("unlink %q: %w", oldName, err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit
	var targetID string
	if err := tx.QueryRow(`SELECT target_id FROM target_aliases WHERE domain = ? AND old_name = ?`, e.domain, oldName).Scan(&targetID); err != nil {
		return fmt.Errorf("unlink %q: read alias: %w", oldName, err)
	}
	var taken int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM `+e.table+` WHERE `+e.nameCol+` = ? AND id != ?`, oldName, targetID).Scan(&taken); err != nil {
		return fmt.Errorf("unlink %q: check it: %w", oldName, err)
	}
	if taken > 0 {
		return fmt.Errorf("%q already has its own entry", oldName)
	}
	var current string
	if err := tx.QueryRow(`SELECT `+e.nameCol+` FROM `+e.table+` WHERE id = ?`, targetID).Scan(&current); err != nil {
		return fmt.Errorf("unlink %q: read the linked entry: %w", oldName, err)
	}
	placement, prefix, err := placementDomainForAlias(e.domain)
	if err != nil {
		return err
	}
	if err := renameCopyRuleTx(tx, placement, prefix+current, prefix+oldName); err != nil {
		return fmt.Errorf("unlink %q: %w", oldName, err)
	}
	if err := e.moveRow(tx, targetID, oldName, newDefinition, uuid); err != nil {
		return fmt.Errorf("unlink %q: %w", oldName, err)
	}
	// The snapshots taken under current while linked stay the entry's, and
	// nothing links current to it any more.
	if err := keepRuleOnNameTx(tx, e, oldName, current); err != nil {
		return fmt.Errorf("unlink %q: %w", oldName, err)
	}
	if _, err := tx.Exec(`DELETE FROM target_aliases WHERE domain = ? AND old_name = ?`, e.domain, oldName); err != nil {
		return fmt.Errorf("unlink %q: delete alias: %w", oldName, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("unlink %q: commit: %w", oldName, err)
	}
	return nil
}

// moveRow gives row id its name and definition, and a VM row its uuid. A row
// that is not there fails, so no caller drops an alias after moving nothing.
func (e entryTable) moveRow(tx *sql.Tx, id, name, definition, uuid string) error {
	set, args := e.nameCol+` = ?, definition = ?`, []any{name, definition}
	if e.domain == "vm" {
		set, args = set+`, uuid = ?`, append(args, uuid)
	}
	res, err := tx.Exec(`UPDATE `+e.table+` SET `+set+` WHERE id = ?`, append(args, id)...) //nolint:gosec // G202: table and column names are fixed in this file; every value is a bound parameter
	if err != nil {
		return fmt.Errorf("move the entry: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("no %s entry has id %s", e.domain, id)
	}
	return nil
}

// queryAliases runs query, which selects an alias's columns in struct order,
// with prev_definition last when withDefinition is set.
func (r *Repo) queryAliases(op string, withDefinition bool, query string, args ...any) ([]Alias, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close() //nolint:errcheck // read-only
	var out []Alias
	for rows.Next() {
		var a Alias
		dest := []any{&a.ID, &a.Domain, &a.OldName, &a.TargetID, &a.LinkedAt}
		if withDefinition {
			dest = append(dest, &a.PrevDefinition)
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("%s scan: %w", op, err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
