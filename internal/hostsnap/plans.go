package hostsnap

import (
	"context"
	"path"
	"sort"
	"strings"
	"time"
)

// Plan is a schedule's snapshot of one source and how many of them it keeps.
type Plan struct {
	ID        string
	Kind      Kind
	FS        string
	Source    string
	Recursive bool
	Keep      int
	// SkipAsleep leaves a pool whose disks sleep alone. Nothing was written
	// to it while they slept, so the snapshot would hold nothing new.
	SkipAsleep bool
}

// RunResult is what one run of a plan did.
type RunResult struct {
	Code    string   `json:"code"`
	Created string   `json:"created,omitempty"`
	Removed []string `json:"removed"`
	Detail  string   `json:"detail,omitempty"`
}

// Run codes besides "ok" and the error codes.
const (
	CodeSkippedAsleep = "skipped-asleep"
	CodeKeptSome      = "kept-some"
)

// Expired picks the snapshots the plan no longer keeps: its own, past the
// newest keep of them. A held snapshot is left out of the count and is never
// picked, and a snapshot the plan did not take is never looked at.
func Expired(snaps []Snapshot, planID string, keep int) []Snapshot {
	if keep < 1 {
		keep = 1
	}
	var own []Snapshot
	for _, s := range snaps {
		if len(s.Holds) == 0 && OwnedByPlan(labelOf(s), planID) {
			own = append(own, s)
		}
	}
	sort.SliceStable(own, func(i, j int) bool {
		if own[i].CreatedAt != own[j].CreatedAt {
			return own[i].CreatedAt > own[j].CreatedAt
		}
		return labelOf(own[i]) > labelOf(own[j])
	})
	if len(own) <= keep {
		return nil
	}
	return own[keep:]
}

func labelOf(s Snapshot) string {
	if s.Label != "" {
		return s.Label
	}
	return path.Base(s.Name)
}

// RunPlan takes the plan's snapshot and removes the ones it no longer keeps.
func (h *Host) RunPlan(ctx context.Context, p Plan, now time.Time) RunResult {
	res := RunResult{Code: "ok", Removed: []string{}}
	if p.SkipAsleep && (p.Kind == KindZFS || p.Kind == KindBtrfs) && h.asleep(ctx, p) {
		res.Code = CodeSkippedAsleep
		return res
	}
	name := PlanSnapshotName(p.ID, now)
	src := Source{Kind: p.Kind, FS: p.FS, Name: p.Source}
	if err := h.Create(ctx, src, name, p.Recursive && p.Kind == KindZFS); err != nil {
		res.Code, res.Detail = CodeOf(err), detailOf(err)
		return res
	}
	res.Created = name
	snaps, err := h.planSnapshots(ctx, p)
	if err != nil {
		res.Code, res.Detail = CodeOf(err), detailOf(err)
		return res
	}
	for _, s := range Expired(snaps, p.ID, p.Keep) {
		var code, detail string
		if p.Kind == KindZFS && p.Recursive {
			err := h.zfs(ctx, "destroy", "-r", "--", s.Source+"@"+s.Name)
			code, detail = CodeOf(err), detailOf(err)
			if err == nil || code == CodeNotFound {
				code = "ok"
			}
		} else {
			r := h.Delete(ctx, []Ref{s.Ref})[0]
			code, detail = r.Code, r.Detail
		}
		if code == "ok" {
			res.Removed = append(res.Removed, labelOf(s))
			continue
		}
		res.Code, res.Detail = CodeKeptSome, detail
	}
	return res
}

// asleep reports whether the disks of the plan's pool sleep.
func (h *Host) asleep(ctx context.Context, p Plan) bool {
	out, _, err := h.sh.RunCapture(ctx, "sh", "-c", surveyScript, "bombvault-snapshots", "0", "states")
	if err != nil {
		return false
	}
	raw := parseSurvey(out)
	if p.Kind == KindZFS {
		pool, _, _ := strings.Cut(p.Source, "/")
		return raw.asleep[poolKey(KindZFS, pool)]
	}
	return raw.asleep[poolKey(KindBtrfs, p.FS)]
}

// planSnapshots lists the snapshots of the plan's source only.
func (h *Host) planSnapshots(ctx context.Context, p Plan) ([]Snapshot, error) {
	switch p.Kind {
	case KindZFS:
		out, err := h.zfsOut(ctx, "list", "-H", "-p", "-t", "snapshot", "-d", "1", "-o", "name,creation,used,referenced,userrefs,clones", "--", p.Source)
		if err != nil {
			return nil, err
		}
		var snaps []Snapshot
		for _, line := range strings.Split(out, "\n") {
			if sn, ok := parseZFSSnap(strings.Split(line, "\t")); ok && sn.dataset == p.Source {
				snaps = append(snaps, zfsSnapshot(sn))
			}
		}
		return snaps, nil
	case KindBtrfs:
		dir := path.Join(BtrfsFolder, btrfsFolderName(p.Source))
		script := `ls -1 -- "$1" 2>/dev/null; exit 0`
		out, _, err := h.sh.RunCapture(ctx, "sh", "-c", script, "bombvault-snapshots", path.Join(p.FS, dir))
		if err != nil {
			return nil, &Error{Code: CodeFailed}
		}
		var snaps []Snapshot
		for _, name := range strings.Split(out, "\n") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			snaps = append(snaps, Snapshot{
				Ref:   Ref{Kind: KindBtrfs, FS: p.FS, Source: p.Source, Name: path.Join(dir, name)},
				Label: name, CreatedAt: stampOf(name), Holds: []string{},
			})
		}
		return snaps, nil
	case KindVM:
		table, err := h.vms.SnapshotTable(ctx, p.Source)
		if err != nil {
			return nil, virshError(err)
		}
		return parseVMSnapshots(p.Source, table), nil
	}
	return nil, &Error{Code: CodeNoSupport}
}

// stampOf reads the instant a plan's snapshot name carries.
func stampOf(name string) int64 {
	m := planNameRe.FindStringSubmatch(name)
	if m == nil {
		return 0
	}
	t, err := time.Parse(stampLayout, m[2])
	if err != nil {
		return 0
	}
	return t.Unix()
}
