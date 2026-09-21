package api

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The guards in this file all pin the SAME shape of defect, the one the third
// review round found six times over: an operation that says "this domain" and
// reaches only the domain's own repository, while an item's data sits in a named
// one (#204).
//
// They are source scans on purpose. What has to hold is which RESOLUTION a call
// site uses - domainReposForOp (every repository) versus domainRepoSource (one)
// - and that is a property of the line, not of an outcome a fake engine can be
// driven to produce. A behavioural test here would assert that a check ran twice
// against a stub, which is satisfied by a loop over one repository listed twice.

// mustReadService returns internal/api/service.go as a string.
func mustReadService(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	return string(raw)
}

// funcBody returns the source of one top-level method, from its "func (s
// *Service) <name>(" line to that function's own closing brace - the first line
// that is exactly "}" at column zero, which gofmt guarantees for a top-level
// declaration and for nothing inside it.
//
// It used to run to the next "^func " instead, which silently swallowed
// everything between the end of the function and the start of the next one:
// package-level vars, consts, types, and any doc COMMENT of the following
// function. A guard looking for a call string could therefore be satisfied by
// that call appearing in the next function's comment, which is exactly the
// false green a source scan must not have.
func funcBody(t *testing.T, src, name string) string {
	t.Helper()
	return receiverFuncBody(t, src, `\(s \*Service\)`, name, "service.go")
}

// receiverFuncBody is funcBody for any receiver and any file. A guard that reads
// the WHOLE file cannot tell a call from a mention of one in a comment, which is
// exactly the failure the funcBody helper was written for - and then two guards
// below kept scanning whole files anyway.
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

// TestDomainWideOpsReachEveryRepository pins the four operations that have to
// see ALL of a domain's data.
//
// Each of them resolved the domain's own repository alone, which turned each
// into its own kind of lie: a green integrity check over a repository the data
// is not in, an unlock that leaves the stuck lock where it is, a prune that
// never reclaims what retention freed, and a restorability drill that reads back
// somebody else's bytes.
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

// TestDomainWideOpsBuildTheModePerRepository pins the other half of the same
// change. A named repository carries its own credentials, storage class and
// bandwidth caps, and an off-site copy carries its target's; running the loop
// with one mode would address somebody else's bucket with the wrong keys.
//
// It matches on the LOOP VARIABLE, not just the helper name, because the shape
// this is meant to catch keeps the call and hoists it out of the loop. A source
// scan can only go that far; the assertion that the engine really saw two
// different modes lives in TestCheckDomainChecksBothRepositoriesWithTheirOwnModes
// (named_repos_multi_test.go), which is the instrument this one used to be
// mistaken for.
// modePerRepoRe matches the mode being built from the loop variable, whatever
// the element's type happens to be today.
var modePerRepoRe = regexp.MustCompile(`s\.repoModeFor\(settings, domain, source, r\b`)

func TestDomainWideOpsBuildTheModePerRepository(t *testing.T) {
	src := mustReadService(t)
	for _, fn := range []string{"CheckDomain", "UnlockDomain", "pruneDomain", "runSubsetDrill"} {
		body := funcBody(t, src, fn)
		// Matched WITHOUT the argument's exact spelling. Pinning "…, source, r)"
		// made this guard fail the day the loop variable became a struct and the
		// call became "…, source, r.Loc)" - a failure about a rename, in a test
		// whose whole job is to be believed when it speaks. What has to hold is
		// that the mode is built INSIDE the loop from the loop's own element.
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

// TestDeleteSnapshotFindsTheRepositoryHoldingIt pins the one operation that
// cannot simply loop: a snapshot id names ONE snapshot, and forgetting it has to
// happen in the repository it is in. Resolving the domain repository answered
// "no matching ID" for a snapshot the list beside the button was showing.
func TestDeleteSnapshotFindsTheRepositoryHoldingIt(t *testing.T) {
	body := funcBody(t, mustReadService(t), "DeleteSnapshot")
	if !strings.Contains(body, "s.repoHoldingSnapshot(") {
		t.Error("DeleteSnapshot no longer searches for the repository the snapshot is in.\n" +
			"It then deletes from the domain repository, which for an item on its own repository\n" +
			"means the delete button reports 'no matching ID' for a snapshot that is right there.")
	}
}

// TestReplicationAgreesWithItself pins the resolution of a contradiction the
// review found: the per-item hook replicated whatever repository the backup had
// just written (a named one included), while the manual and scheduled passes
// replicated the domain repository only. The same snapshot was therefore copied
// off-site or not depending on WHICH trigger fired.
//
// The answer is that the two cover different ground on purpose - the hook copies
// the repository that just changed, the whole-domain passes cover every source -
// so both must go through offsiteReplicationSources' rule about which
// repositories qualify.
func TestReplicationAgreesWithItself(t *testing.T) {
	src := mustReadService(t)
	for _, fn := range []string{"ReplicateOffsite", "StartReplicateOffsite"} {
		if !strings.Contains(funcBody(t, src, fn), "s.offsiteReplicationSources(") {
			t.Errorf("%s no longer replicates every repository this domain's items write to.\n"+
				"An item on a local named repository is then quietly left out of the off-site copy,\n"+
				"and the operation still reports success.", fn)
		}
	}
	// The hook's own half: it must refuse a REMOTE named repository rather than
	// trying to copy one cloud into another, which one restic process cannot do
	// (it carries a single set of backend credentials) and which nobody asked
	// for - a remote named repository IS the off-site copy.
	//
	// Asserted as the SHARED predicate, not as the condition spelled out twice.
	// The two spellings drifted: the hook additionally required a non-empty
	// Named.ID, so when namedRepoForLocation's store read failed - refFor then
	// yields Own=false with no named row - the hook attempted a copy the
	// whole-domain pass had excluded. One helper is the only way a source scan
	// can see that they still agree.
	for _, fn := range []string{"replicateOffsite", "offsiteReplicationSources"} {
		if !strings.Contains(funcBody(t, src, fn), "alreadyOffSite(") {
			t.Errorf("%s no longer asks the shared predicate about a REMOTE named repository.\n"+
				"restic copy carries one set of backend credentials, so a copy from one cloud into\n"+
				"another authenticates the wrong one - and the source is already off site. Spelling\n"+
				"the condition out in each place is how the two halves came to answer differently.", fn)
		}
	}
}

// TestDiscoverLooksInEveryRepository pins the data-loss half of the same family.
//
// Discover is the path back from a lost /config: it rebuilds items out of the
// snapshots that still exist. Reading only the domain repository left an item
// pointed at a named one unrecoverable, with its backups sitting there intact.
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

// TestTheRecoveryKitNamesTheNamedRepositories pins the last resort.
//
// Every other location in the kit is one the user configured and could re-derive
// from their own settings. A named repository's location existed ONLY in the
// database, so losing /config without this section leaves the data in a place
// nothing left on the box can name.
func TestTheRecoveryKitNamesTheNamedRepositories(t *testing.T) {
	body := funcBody(t, mustReadService(t), "RecoveryKit")
	for _, want := range []string{"s.store.ListNamedRepos()", "Named repositories"} {
		if !strings.Contains(body, want) {
			t.Errorf("the recovery kit no longer lists the per-item repositories (%s).\n"+
				"They are the one location in BombVault that a lost /config makes unfindable.", want)
		}
	}
}

// TestTheUnprefixedRemoteAdviceSurvivesTheScrubber pins a message against the
// machinery that carries it.
//
// Every error leaving the API goes through scrubError, and its absPathRe
// (`(/[^\s:"']+)+`) redacts any slash-led token - not only a real filesystem
// path. A refusal that told the user "write it as rclone:myremote:bucket/path"
// therefore arrived on screen as "...bucket[path]": advice that cannot be
// typed, in the one message whose whole job is to say what to type. It happened
// twice in a row, first by echoing the caller's location back and then in a
// fixed example, which is why the invariant is pinned rather than the wording.
func TestTheUnprefixedRemoteAdviceSurvivesTheScrubber(t *testing.T) {
	h := &Handler{}
	err := h.validateNamedRepo(store.OffsiteTarget{Name: "Kalte Ablage", Repo: "BackBlaze:bucket/cold"}, true)
	if err == nil {
		t.Fatal("an rclone remote name without a restic prefix is no longer refused")
	}
	if got := scrubError(err); got != err.Error() {
		t.Errorf("the refusal is mangled on its way to the screen.\n  written: %s\n  arrives: %s\n"+
			"Remove the slash from the message: the scrubber redacts any slash-led token, so an\n"+
			"example path in this particular message reaches the user unusable.", err.Error(), got)
	}
}

// TestTheInUseRefusalsAreOneTransaction pins that the two refusals which protect
// a live repository re-count INSIDE their own write.
//
// Counting first and writing second leaves a window: an item that starts
// pointing at the repository in between is silently put back on its domain
// repository, and its next backup lands there looking exactly like a working
// backup. That is precisely the outcome the refusal exists to prevent, so it may
// not have a race that reproduces it.
func TestTheInUseRefusalsAreOneTransaction(t *testing.T) {
	raw, err := os.ReadFile("named_repos_crud.go")
	if err != nil {
		t.Fatalf("read named_repos_crud.go: %v", err)
	}
	src := string(raw)
	// Scoped to the two handlers' own bodies. A whole-file Contains passes on a
	// mention in a comment - including the comment that explains why the guarded
	// transaction exists - so the guard would keep reporting green over a handler
	// rewritten back to a separate count and write.
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
