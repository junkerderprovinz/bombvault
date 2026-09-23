package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// SkipAll alone in a skip list leaves every target out.
const SkipAll = "*"

var (
	ErrBadSkip       = errors.New(`skip must be a JSON list of target ids, or ["*"] alone`)
	ErrStackCopyRule = errors.New("stack folders follow the containers default and take no copy rule")
	ErrRuleDomain    = errors.New("identity does not belong to the domain")
	ErrCopyRuleTaken = errors.New("that name already has a copy rule")
)

// CopyRule is what one item does not get copied: [] means every enabled target,
// ["*"] none, anything else the listed target ids stay out.
type CopyRule struct {
	Domain    string
	Identity  string
	Skip      []string
	UpdatedAt int64
}

// rulePrefix is the tag prefix of each domain that takes copy rules.
var rulePrefix = map[string]string{"containers": "container:", "vms": "vm:", "files": "fileset:"}

// queryer is what a read needs from a *sql.DB or a *sql.Tx.
type queryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// inTx runs fn in one transaction and commits when it returns nil.
func (r *Repo) inTx(fn func(*sql.Tx) error) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // rollback after a successful commit is a no-op
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// CopyRuleFor returns the rule of one snapshot name.
func (r *Repo) CopyRuleFor(domain, identity string) (CopyRule, bool, error) {
	rule := CopyRule{Domain: domain, Identity: identity}
	var raw string
	err := r.db.QueryRow(`SELECT skip, updated_at FROM offsite_copy_rules WHERE domain = ? AND identity = ?`,
		domain, identity).Scan(&raw, &rule.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return CopyRule{}, false, nil
	}
	if err != nil {
		return CopyRule{}, false, fmt.Errorf("CopyRuleFor: %w", err)
	}
	if rule.Skip, err = decodeSkip(raw); err != nil {
		return CopyRule{}, false, fmt.Errorf("copy rule %s: %w", identity, err)
	}
	return rule, true, nil
}

// CopyRulesForDomain returns a domain's rules by snapshot name. One rule that
// cannot be read fails the whole answer.
func (r *Repo) CopyRulesForDomain(domain string) (map[string]CopyRule, error) {
	return copyRulesQ(r.db, domain)
}

// ListCopyRules returns every rule, by domain and name.
func (r *Repo) ListCopyRules() ([]CopyRule, error) {
	rows, err := r.db.Query(`SELECT domain, identity, skip, updated_at FROM offsite_copy_rules ORDER BY domain, identity`)
	if err != nil {
		return nil, fmt.Errorf("ListCopyRules: %w", err)
	}
	return scanCopyRules(rows)
}

// SetCopyRule writes or replaces the rule of one snapshot name.
func (r *Repo) SetCopyRule(domain, identity string, skip []string) error {
	err := r.inTx(func(tx *sql.Tx) error {
		return setCopyRuleTx(tx, domain, identity, skip, time.Now().Unix())
	})
	if err != nil {
		return fmt.Errorf("SetCopyRule: %w", err)
	}
	return nil
}

// DeleteCopyRule removes the rule of one snapshot name, so it follows the default.
func (r *Repo) DeleteCopyRule(domain, identity string) error {
	if err := r.inTx(func(tx *sql.Tx) error { return deleteCopyRuleTx(tx, domain, identity) }); err != nil {
		return fmt.Errorf("DeleteCopyRule: %w", err)
	}
	return nil
}

// MoveCopyRule carries the rule of one name to another. Without a rule at from
// it does nothing; a rule already at to is ErrCopyRuleTaken.
func (r *Repo) MoveCopyRule(domain, from, to string) error {
	for _, identity := range []string{from, to} {
		if err := CheckRuleIdentity(domain, identity); err != nil {
			return fmt.Errorf("MoveCopyRule: %w", err)
		}
	}
	if err := r.inTx(func(tx *sql.Tx) error { return moveCopyRuleTx(tx, domain, from, to) }); err != nil {
		return fmt.Errorf("MoveCopyRule: %w", err)
	}
	return nil
}

func copyRulesQ(q queryer, domain string) (map[string]CopyRule, error) {
	rows, err := q.Query(`SELECT domain, identity, skip, updated_at FROM offsite_copy_rules WHERE domain = ?`, domain)
	if err != nil {
		return nil, fmt.Errorf("copy rules of %s: %w", domain, err)
	}
	list, err := scanCopyRules(rows)
	if err != nil {
		return nil, err
	}
	out := make(map[string]CopyRule, len(list))
	for _, rule := range list {
		out[rule.Identity] = rule
	}
	return out, nil
}

func scanCopyRules(rows *sql.Rows) ([]CopyRule, error) {
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	out := []CopyRule{}
	for rows.Next() {
		var rule CopyRule
		var raw string
		if err := rows.Scan(&rule.Domain, &rule.Identity, &raw, &rule.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan copy rule: %w", err)
		}
		skip, err := decodeSkip(raw)
		if err != nil {
			return nil, fmt.Errorf("copy rule %s: %w", rule.Identity, err)
		}
		rule.Skip = skip
		out = append(out, rule)
	}
	return out, rows.Err()
}

func setCopyRuleTx(tx *sql.Tx, domain, identity string, skip []string, now int64) error {
	if err := CheckRuleIdentity(domain, identity); err != nil {
		return err
	}
	raw, err := encodeSkip(skip)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO offsite_copy_rules (domain, identity, skip, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(domain, identity) DO UPDATE SET skip = excluded.skip, updated_at = excluded.updated_at`,
		domain, identity, raw, now)
	return err
}

func deleteCopyRuleTx(tx *sql.Tx, domain, identity string) error {
	_, err := tx.Exec(`DELETE FROM offsite_copy_rules WHERE domain = ? AND identity = ?`, domain, identity)
	return err
}

func moveCopyRuleTx(tx *sql.Tx, domain, from, to string) error {
	var held, taken int
	if err := tx.QueryRow(`SELECT count(*) FROM offsite_copy_rules WHERE domain = ? AND identity = ?`, domain, from).Scan(&held); err != nil {
		return err
	}
	if held == 0 {
		return nil
	}
	if err := tx.QueryRow(`SELECT count(*) FROM offsite_copy_rules WHERE domain = ? AND identity = ?`, domain, to).Scan(&taken); err != nil {
		return err
	}
	if taken > 0 {
		return ErrCopyRuleTaken
	}
	_, err := tx.Exec(`UPDATE offsite_copy_rules SET identity = ?, updated_at = ? WHERE domain = ? AND identity = ?`,
		to, time.Now().Unix(), domain, from)
	return err
}

// encodeSkip stores a skip list sorted and without repeats; nil is every target.
func encodeSkip(skip []string) (string, error) {
	if err := ValidSkipList(skip); err != nil {
		return "", err
	}
	list := slices.Compact(slices.Sorted(slices.Values(skip)))
	if list == nil {
		list = []string{}
	}
	b, err := json.Marshal(list)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// decodeSkip reads a stored skip list and refuses anything encodeSkip would not
// have written, null included.
func decodeSkip(raw string) ([]string, error) {
	var list []string
	if err := json.Unmarshal([]byte(raw), &list); err != nil || list == nil {
		return nil, ErrBadSkip
	}
	if err := ValidSkipList(list); err != nil {
		return nil, err
	}
	return list, nil
}

// ValidSkipList refuses an empty id and "*" next to anything.
func ValidSkipList(skip []string) error {
	for _, id := range skip {
		if strings.TrimSpace(id) == "" || (id == SkipAll && len(skip) > 1) {
			return ErrBadSkip
		}
	}
	return nil
}

// CheckRuleIdentity refuses a name a domain's rules cannot hold. Project folders
// follow the containers default and never take a rule of their own.
func CheckRuleIdentity(domain, identity string) error {
	if strings.HasPrefix(identity, "stack:") {
		return ErrStackCopyRule
	}
	prefix, ok := rulePrefix[domain]
	if !ok || !strings.HasPrefix(identity, prefix) || len(identity) == len(prefix) {
		return ErrRuleDomain
	}
	return nil
}

// placementDomainForAlias maps the domain an alias row carries to the placement
// domain and identity prefix the entry's copy rules use.
func placementDomainForAlias(aliasDomain string) (domain, prefix string, err error) {
	switch aliasDomain {
	case "container":
		return "containers", rulePrefix["containers"], nil
	case "vm":
		return "vms", rulePrefix["vms"], nil
	}
	return "", "", fmt.Errorf("no placement domain for aliases of %q", aliasDomain)
}

// renameCopyRuleTx carries an entry's rule to the name it moves to. A rule
// already on that name refuses the move, even when the entry has none, so the
// entry cannot inherit a choice made for something else.
func renameCopyRuleTx(tx *sql.Tx, domain, from, to string) error {
	var taken int
	if err := tx.QueryRow(`SELECT count(*) FROM offsite_copy_rules WHERE domain = ? AND identity = ?`, domain, to).Scan(&taken); err != nil {
		return fmt.Errorf("read the copy rule of %s: %w", to, err)
	}
	if taken > 0 {
		return fmt.Errorf("%s: %w", to, ErrCopyRuleTaken)
	}
	return moveCopyRuleTx(tx, domain, from, to)
}

// cloneCopyRuleTx writes the rule of from onto to as well. A rule already on to
// stays, since it belongs to whatever carries that name today.
func cloneCopyRuleTx(tx *sql.Tx, domain, from, to string) error {
	_, err := tx.Exec(`INSERT OR IGNORE INTO offsite_copy_rules (domain, identity, skip, updated_at)
		SELECT domain, ?, skip, ? FROM offsite_copy_rules WHERE domain = ? AND identity = ?`,
		to, time.Now().Unix(), domain, from)
	if err != nil {
		return fmt.Errorf("copy the rule of %s to %s: %w", from, to, err)
	}
	return nil
}

// keepRuleOnAliasesTx leaves the rule of an entry that is about to be deleted
// on each of its former names, because the snapshots taken under them outlive
// the row.
func keepRuleOnAliasesTx(tx *sql.Tx, aliasDomain, targetID, name string) error {
	domain, prefix, err := placementDomainForAlias(aliasDomain)
	if err != nil {
		return err
	}
	rows, err := tx.Query(`SELECT old_name FROM target_aliases WHERE domain = ? AND target_id = ?`, aliasDomain, targetID)
	if err != nil {
		return fmt.Errorf("read the former names of %s: %w", name, err)
	}
	defer rows.Close() //nolint:errcheck // read-only
	var olds []string
	for rows.Next() {
		var old string
		if err := rows.Scan(&old); err != nil {
			return fmt.Errorf("read the former names of %s: %w", name, err)
		}
		olds = append(olds, old)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read the former names of %s: %w", name, err)
	}
	for _, old := range olds {
		if err := cloneCopyRuleTx(tx, domain, prefix+name, prefix+old); err != nil {
			return err
		}
	}
	return nil
}
