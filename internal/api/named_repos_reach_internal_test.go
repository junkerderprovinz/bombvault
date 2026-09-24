package api

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// These guards check that operations on a whole domain also reach the named
// repositories its items use, not only the domain's own. They scan the source
// because what matters is which resolution a call site uses: domainReposForOp
// returns every repository, domainRepoSource only one.

// mustReadService returns the service layer's source: every service*.go file of
// the package, one after the other.
func mustReadService(t *testing.T) string {
	t.Helper()
	files, err := filepath.Glob("service*.go")
	if err != nil {
		t.Fatal(err)
	}
	var src strings.Builder
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(f) //nolint:gosec // G304: a file name from this package's own directory
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		src.Write(raw)
	}
	return src.String()
}

// funcBody returns the source of a *Service method in the service files, up to
// its closing brace at column zero. Stopping there keeps the next function's doc
// comment from satisfying a guard.
func funcBody(t *testing.T, src, name string) string {
	t.Helper()
	return receiverFuncBody(t, src, `\(s \*Service\)`, name, "service*.go")
}

// receiverFuncBody is funcBody for any receiver and file.
func receiverFuncBody(t *testing.T, src, receiver, name, file string) string {
	t.Helper()
	start := regexp.MustCompile(`(?m)^func ` + receiver + ` ` + regexp.QuoteMeta(name) + `\(`).FindStringIndex(src)
	if start == nil {
		t.Fatalf("no method %s in %s - it was renamed or removed, and this guard no longer watches anything", name, file)
	}
	rest := src[start[1]:]
	if end := regexp.MustCompile(`(?m)^\}`).FindStringIndex(rest); end != nil {
		return rest[:end[0]]
	}
	t.Fatalf("no closing brace found for %s; %s is not gofmt-formatted", name, file)
	return ""
}

// Check, unlock, prune and the restore drill only mean something over all of a
// domain's data.
func TestDomainWideOpsReachEveryRepository(t *testing.T) {
	src := mustReadService(t)
	for _, fn := range []string{"CheckDomain", "UnlockDomain", "pruneDomain", "runSubsetDrill"} {
		body := funcBody(t, src, fn)
		if !strings.Contains(body, "s.domainReposForOp(") {
			t.Errorf("%s no longer resolves EVERY repository of its domain (domainReposForOp).\n"+
				"With a per-item repository (#204) in play, the domain repository is not all of the\n"+
				"domain's data, and this operation is only meaningful over all of it.", fn)
		}
		if strings.Contains(body, "s.domainRepoSource(") {
			t.Errorf("%s is back on domainRepoSource, which answers with ONE repository.", fn)
		}
	}
}

// modePerRepoRe matches the mode being built from the loop variable r or one
// of its fields.
var modePerRepoRe = regexp.MustCompile(`s\.repoModeFor\(settings, domain, source, r\b`)

// A named repository has its own credentials, storage class and bandwidth
// caps, and an off-site copy has its target's, so the mode must be built
// inside the loop. TestCheckDomainChecksBothRepositoriesWithTheirOwnModes
// checks what the engine actually receives.
func TestDomainWideOpsBuildTheModePerRepository(t *testing.T) {
	src := mustReadService(t)
	for _, fn := range []string{"CheckDomain", "UnlockDomain", "pruneDomain", "runSubsetDrill"} {
		body := funcBody(t, src, fn)
		if !modePerRepoRe.MatchString(body) {
			t.Errorf("%s no longer builds the restic mode per repository from the loop variable.\n"+
				"A named repository's credentials, storage class and limits are its own, and an\n"+
				"off-site copy's belong to its target row rather than to the domain's primary.", fn)
		}
		if strings.Contains(body, "s.primaryModeFor(settings, domain, r)") {
			t.Errorf("%s is back on primaryModeFor, which describes the domain's PRIMARY.\n"+
				"On the off-site source that applies the primary's credentials to somebody else's bucket.", fn)
		}
	}
}

// A snapshot id names one snapshot, so the delete has to find the repository
// holding it instead of looping.
func TestDeleteSnapshotFindsTheRepositoryHoldingIt(t *testing.T) {
	body := funcBody(t, mustReadService(t), "DeleteSnapshot")
	if !strings.Contains(body, "s.repoHoldingSnapshot(") {
		t.Error("DeleteSnapshot no longer searches for the repository the snapshot is in.\n" +
			"It then deletes from the domain repository, which for an item on its own repository\n" +
			"means the delete button reports 'no matching ID' for a snapshot that is right there.")
	}
}

// The per-item hook copies the repository a backup just wrote, and the manual
// and scheduled passes cover every source. Both must apply the same rule about
// which repositories qualify, or a snapshot's off-site copy would depend on the
// trigger.
func TestReplicationAgreesWithItself(t *testing.T) {
	src := mustReadService(t)
	for _, fn := range []string{"ReplicateOffsite", "StartReplicateOffsite"} {
		if !strings.Contains(funcBody(t, src, fn), "s.offsiteReplicationSources(") {
			t.Errorf("%s no longer replicates every repository this domain's items write to.\n"+
				"An item on a local named repository is then quietly left out of the off-site copy,\n"+
				"and the operation still reports success.", fn)
		}
	}
	// A remote named repository is already the off-site copy, and one restic
	// process cannot copy between two clouds with a single set of backend
	// credentials. Both paths must skip it through the same predicate so they
	// cannot drift apart.
	for _, fn := range []string{"replicateOffsite", "offsiteReplicationSources"} {
		if !strings.Contains(funcBody(t, src, fn), "alreadyOffSite(") {
			t.Errorf("%s no longer asks the shared predicate about a REMOTE named repository.\n"+
				"restic copy carries one set of backend credentials, so a copy from one cloud into\n"+
				"another authenticates the wrong one - and the source is already off site. Spelling\n"+
				"the condition out in each place is how the two halves came to answer differently.", fn)
		}
	}
}

// Discover rebuilds items from their snapshots after /config is lost, so it has
// to look in the named repositories too.
func TestDiscoverLooksInEveryRepository(t *testing.T) {
	src := mustReadService(t)
	for _, fn := range []string{"Discover", "DiscoverVMs", "DiscoverFileSets"} {
		body := funcBody(t, src, fn)
		if !strings.Contains(body, "s.discoverNamesAcrossRepos(") {
			t.Errorf("%s no longer looks in every repository of its domain.\n"+
				"An item on its own repository is then never rebuilt after a /config loss, and\n"+
				"nothing on screen says its backups exist.", fn)
		}
	}
}

// A named repository's location lives only in the database, so after losing
// /config the recovery kit is the one place that still names it.
func TestTheRecoveryKitNamesTheNamedRepositories(t *testing.T) {
	body := funcBody(t, mustReadService(t), "RecoveryKit")
	for _, want := range []string{"s.store.ListNamedRepos()", "Named repositories"} {
		if !strings.Contains(body, want) {
			t.Errorf("the recovery kit no longer lists the per-item repositories (%s).\n"+
				"They are the one location in BombVault that a lost /config makes unfindable.", want)
		}
	}
}

// scrubError redacts any token starting with a slash, not only real paths. An
// example such as rclone:myremote:bucket/path in this refusal would reach the
// user as "bucket[path]", advice that cannot be typed.
func TestTheUnprefixedRemoteAdviceSurvivesTheScrubber(t *testing.T) {
	h := &Handler{}
	err := h.validateNamedRepo(store.OffsiteTarget{Name: "Kalte Ablage", Repo: "BackBlaze:bucket/cold"}, true, true)
	if err == nil {
		t.Fatal("an rclone remote name without a restic prefix is no longer refused")
	}
	if got := scrubError(err); got != err.Error() {
		t.Errorf("the refusal is mangled on its way to the screen.\n  written: %s\n  arrives: %s\n"+
			"Remove the slash from the message: the scrubber redacts any slash-led token, so an\n"+
			"example path in this particular message reaches the user unusable.", err.Error(), got)
	}
}

// Deleting or moving a repository that is in use must count its users inside
// the same write. Counting first leaves a window in which an item starts using
// it and is then silently put back on its domain repository.
func TestTheInUseRefusalsAreOneTransaction(t *testing.T) {
	raw, err := os.ReadFile("named_repos_crud.go")
	if err != nil {
		t.Fatalf("read named_repos_crud.go: %v", err)
	}
	src := string(raw)
	for _, c := range []struct{ fn, want string }{
		{"handleDeleteNamedRepo", "h.store.DeleteNamedRepoIfUnused("},
		{"handleUpdateNamedRepo", "h.store.SetNamedRepoLocationIfUnused("},
	} {
		body := receiverFuncBody(t, src, `\(h \*Handler\)`, c.fn, "named_repos_crud.go")
		if !strings.Contains(body, c.want) {
			t.Errorf("%s is back on a separate count and write: it does not call %s. "+
				"Counting first and writing second leaves a window in which an item starts using "+
				"the repository, and the write then goes through anyway.", c.fn, c.want)
		}
	}
}
