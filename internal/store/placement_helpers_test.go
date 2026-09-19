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
