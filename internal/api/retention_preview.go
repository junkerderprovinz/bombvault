package api

import (
	"context"

	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// RetentionPolicyView is the keep-policy a preview was computed with, carried
// back so the answer can say WHICH rule produced it. Three different policies
// are in play across the product — the local one, the off-site one a manual
// off-site prune uses, and the per-destination one the post-replication hook
// applies — so a removal list without its policy is a confident prediction
// about nobody knows which rule.
type RetentionPolicyView struct {
	On          bool `json:"on"`
	KeepLast    int  `json:"keepLast"`
	KeepDaily   int  `json:"keepDaily"`
	KeepWeekly  int  `json:"keepWeekly"`
	KeepMonthly int  `json:"keepMonthly"`
}

// RetentionPreviewItem is one identity's verdict: the snapshots the policy
// keeps and the ones the next retention run would delete.
type RetentionPreviewItem struct {
	Tag    string            `json:"tag"`
	Keep   []restic.Snapshot `json:"keep"`
	Remove []restic.Snapshot `json:"remove"`
}

// RetentionPreviewRepo is one repository's part of the answer. A domain is no
// longer one repository (#204): its items can point at named repositories, each
// with its own credentials, and each has to be asked separately.
type RetentionPreviewRepo struct {
	Name string `json:"name"`
	// AppendOnly marks a repository retention never touches from this box.
	// Reported rather than hidden: an empty removal list reads as "nothing to
	// do", while the truth is "this archive is immutable and is never trimmed
	// here" — and the opposite, a removal list for an append-only repository,
	// would be the most alarming false positive this feature could produce.
	AppendOnly bool                   `json:"appendOnly"`
	Items      []RetentionPreviewItem `json:"items"`
	// Error is this repository's own failure, kept per repository so one
	// unreachable destination does not blank the answer for the others.
	Error string `json:"error,omitempty"`
}

// RetentionPreview answers "what would the next retention run delete" without
// deleting anything.
type RetentionPreview struct {
	Policy RetentionPolicyView    `json:"policy"`
	Repos  []RetentionPreviewRepo `json:"repos"`
	// Skipped names repositories this preview could not cover at all, in the
	// same vocabulary the other domain-wide operations use. Without it the
	// panel would give a confident answer about a domain it only partly looked
	// at.
	Skipped []string `json:"skipped,omitempty"`
}

// previewPerRepoTimeout bounds one repository's share of a preview.
//
// The cost is real and worth stating: the preview mirrors the per-identity pass,
// so it is one restic invocation per identity per repository — a 44-container
// domain across two repositories is 88 of them, each opening the repo, and over
// a high-latency cloud backend that is minutes rather than seconds. That is why
// this is a button the operator presses rather than something the page polls.
const previewPerRepoTimeout = 10 * time.Minute

// PreviewRetention reports what the next retention run would remove for a
// domain, per repository and per item, without changing anything.
//
// It mirrors pruneDomain's resolution exactly — the same repository set, the
// same per-repository credentials, the same per-source policy — and none of its
// bookkeeping. Specifically it does NOT take the domain lock (a preview that
// refused while a backup runs would be unavailable exactly when someone wants
// to know what tonight's run will delete), does NOT clear stale locks (that
// writes to the repository), and does NOT record a run or publish progress (an
// operation that changed nothing has no business colouring the dashboard or
// appearing in the run history).
//
// Not locking has a price, and it is the right one: the answer can race a
// concurrent forget and name a snapshot that is already gone. Stale but
// harmless, the same trade every read-only restic call in this codebase makes.
func (s *Service) PreviewRetention(ctx context.Context, domain, source string) (RetentionPreview, error) {
	settings, repos, skipped, err := s.domainReposForOp(domain, source)
	if err != nil {
		return RetentionPreview{}, err
	}

	policy := s.retentionPolicyForSource(settings, source)
	out := RetentionPreview{Policy: RetentionPolicyView{
		On:          policy.Any(),
		KeepLast:    policy.KeepLast,
		KeepDaily:   policy.KeepDaily,
		KeepWeekly:  policy.KeepWeekly,
		KeepMonthly: policy.KeepMonthly,
	}}

	// Append-only repositories are classified exactly as pruneDomain classifies
	// them, and then REPORTED instead of refused. pruneDomain has to refuse —
	// it is about to write. A preview's job is to explain, so an immutable
	// archive becomes a named row saying retention never runs there, rather
	// than an absence the operator has to interpret.
	previewable := make([]domainRepoRef, 0, len(repos))
	for _, r := range repos {
		immutable := false
		if isOffsiteSource(source) {
			immutable = s.offsiteSourceImmutable(settings, domain, source)
		} else {
			immutable = s.refAppendOnly(domain, r) != appendOnlyNone
		}
		if immutable {
			out.Repos = append(out.Repos, RetentionPreviewRepo{Name: s.refName(r), AppendOnly: true})
			continue
		}
		previewable = append(previewable, r)
	}

	existing, missing, err := s.reposThatExist(previewable, "no backups yet")
	if err != nil && len(out.Repos) == 0 {
		return RetentionPreview{}, err
	}
	skipped = append(skipped, missing...)
	out.Skipped = skipNames(skipped)

	if !policy.Any() {
		// Retention is off. Every repository is still named, so the panel can
		// say "these exist, and nothing would be removed from any of them"
		// rather than showing an empty box that reads like a failure.
		for _, r := range existing {
			out.Repos = append(out.Repos, RetentionPreviewRepo{Name: s.refName(r)})
		}
		return out, nil
	}

	for _, r := range existing {
		row := RetentionPreviewRepo{Name: s.refName(r)}
		rMode := s.repoModeFor(settings, domain, source, r.Loc)

		rCtx, cancel := context.WithTimeout(ctx, previewPerRepoTimeout)
		groups, pErr := s.previewRetentionPerIdentity(rCtx, r.Loc, policy, rMode)
		cancel()

		if pErr != nil {
			// Per repository, never fatal for the whole answer: one dead
			// destination must not hide what the healthy ones would do.
			row.Error = scrubError(pErr)
		}
		for _, g := range groups {
			row.Items = append(row.Items, RetentionPreviewItem{
				Tag:    previewTagOf(g),
				Keep:   g.Keep,
				Remove: g.Remove,
			})
		}
		out.Repos = append(out.Repos, row)
	}
	return out, nil
}

// previewTagOf names a forget group for the UI. A tag-scoped group carries the
// identity BombVault selected it by; the legacy repo-wide pass has none, and is
// reported as the empty string rather than invented, so the panel can label it
// as the whole-repository fallback it is.
func previewTagOf(g restic.ForgetGroup) string {
	if len(g.Tags) > 0 {
		return g.Tags[0]
	}
	return ""
}
