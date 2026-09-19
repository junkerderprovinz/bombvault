package store

import "testing"

// SeedOffsiteTarget adds an enabled replication destination behind the domain's last one.
func SeedOffsiteTarget(t *testing.T, r *Repo, domain, location string) OffsiteTarget {
	t.Helper()
	tg, err := r.CreateOffsiteTarget(OffsiteTarget{Domain: domain, Name: location, Repo: location, Enabled: true})
	if err != nil {
		t.Fatalf("seed offsite target %s: %v", location, err)
	}
	return tg
}

// SeedFieldTarget adds the enabled row a domain's off-site field edits.
func SeedFieldTarget(t *testing.T, r *Repo, domain, location string) OffsiteTarget {
	t.Helper()
	tg, err := r.UpsertOffsiteTarget(OffsiteTarget{Domain: domain, Name: "Primary", Repo: location, Enabled: true})
	if err != nil {
		t.Fatalf("seed field target %s: %v", location, err)
	}
	return tg
}

// SeedNamedRepo adds an enabled named repository.
func SeedNamedRepo(t *testing.T, r *Repo, name, location string) OffsiteTarget {
	t.Helper()
	tg, err := r.UpsertOffsiteTarget(OffsiteTarget{Role: RoleRepo, Name: name, Repo: location, Enabled: true})
	if err != nil {
		t.Fatalf("seed named repo %s: %v", name, err)
	}
	return tg
}

// SeedCopyRule stores a copy rule; no skip means every target.
func SeedCopyRule(t *testing.T, r *Repo, domain, identity string, skip ...string) {
	t.Helper()
	if err := r.SetCopyRule(domain, identity, skip); err != nil {
		t.Fatalf("seed copy rule %s: %v", identity, err)
	}
}

// RuleSkip returns the stored skip list of a name and whether it has a rule.
func RuleSkip(t *testing.T, r *Repo, domain, identity string) ([]string, bool) {
	t.Helper()
	rule, found, err := r.CopyRuleFor(domain, identity)
	if err != nil {
		t.Fatalf("read copy rule %s: %v", identity, err)
	}
	return rule.Skip, found
}

// SeedDefault stores a domain's placement default; no skip means every target.
func SeedDefault(t *testing.T, r *Repo, domain, home string, skip ...string) PlacementDefault {
	t.Helper()
	d, err := r.PutPlacementDefault(domain, home, skip)
	if err != nil {
		t.Fatalf("seed default %s: %v", domain, err)
	}
	return d
}

// SeedListing records a listing of a target for a domain.
func SeedListing(t *testing.T, r *Repo, domain, targetID string, listedAt int64, rows ...ItemCopies) {
	t.Helper()
	if err := r.RecordTargetListing(domain, targetID, listedAt, rows); err != nil {
		t.Fatalf("seed listing of %s: %v", targetID, err)
	}
}

// CopiesOf is one row of a listing.
func CopiesOf(identity string, count int, latest int64) ItemCopies {
	return ItemCopies{Identity: identity, SnapshotCount: count, LatestSnapshotAt: latest}
}
