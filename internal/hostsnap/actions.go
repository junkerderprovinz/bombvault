package hostsnap

import (
	"context"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// Codes the page turns into sentences, besides the transport's own.
const (
	CodeBusy      = "busy"
	CodeHeld      = "held"
	CodeClones    = "clones"
	CodeNotFound  = "not-found"
	CodeExists    = "exists"
	CodeManaged   = "managed"
	CodeNoSupport = "not-supported"
	CodeFailed    = "failed"
	// CodeNeedsQcow2 is a VM whose disks cannot hold a snapshot of their own.
	CodeNeedsQcow2 = "needs-qcow2"
)

// Create takes a snapshot. recursive applies to ZFS alone, where it takes
// the dataset and every dataset below it in one atomic step.
func (h *Host) Create(ctx context.Context, src Source, name string, recursive bool) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	switch src.Kind {
	case KindZFS:
		if err := checkDataset(src.Name, name); err != nil {
			return err
		}
		args := []string{"snapshot"}
		if recursive {
			args = append(args, "-r")
		}
		return h.zfs(ctx, append(args, "--", src.Name+"@"+name)...)
	case KindBtrfs:
		if err := checkBtrfsPath(src.FS, src.Name, true); err != nil {
			return err
		}
		dir := path.Join(src.FS, BtrfsFolder, btrfsFolderName(src.Name))
		from := src.FS
		if src.Name != "" {
			from = path.Join(src.FS, src.Name)
		}
		script := preamble + `fs="$1"; from="$2"; dir="$3"; to="$3/$4"
[ "$(stat -f -c %T "$fs" 2>/dev/null)" = btrfs ] || { echo "not a btrfs filesystem" >&2; exit 3; }
[ -e "$to" ] && { echo "already exists" >&2; exit 4; }
mkdir -p -- "$dir" && "$BTRFS" subvolume snapshot -r -- "$from" "$to" >/dev/null
`
		return h.script(ctx, script, src.FS, from, dir, name)
	case KindVM:
		if err := checkDomain(src.Name); err != nil {
			return err
		}
		if h.vms == nil {
			return &Error{Code: CodeNoSupport}
		}
		if err := h.vms.SnapshotCreate(ctx, src.Name, name); err != nil {
			return virshError(err)
		}
		return nil
	}
	return &Error{Code: CodeNoSupport}
}

// btrfsFolderName is the folder below BtrfsFolder that holds the snapshots
// of one subvolume, its path with the slashes folded so the folder stays one
// level deep.
func btrfsFolderName(source string) string {
	if source == "" {
		return "top-level"
	}
	return strings.ReplaceAll(source, "/", "_")
}

// Rename gives a snapshot a new name. A snapshot a plan took keeps its place
// in the plan only as long as it keeps the plan's name, which is how someone
// keeps one for good.
func (h *Host) Rename(ctx context.Context, ref Ref, name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	if managedByRuns(path.Base(ref.Name)) {
		return &Error{Code: CodeManaged}
	}
	switch ref.Kind {
	case KindZFS:
		if err := checkDataset(ref.Source, ref.Name); err != nil {
			return err
		}
		if err := checkDataset(ref.Source, name); err != nil {
			return err
		}
		return h.zfs(ctx, "rename", "--", ref.Source+"@"+ref.Name, ref.Source+"@"+name)
	case KindBtrfs:
		if err := checkBtrfsPath(ref.FS, ref.Name, false); err != nil {
			return err
		}
		script := preamble + btrfsIsSnapshot + `to="$(dirname -- "$2")/$3"
[ -e "$to" ] && { echo "already exists" >&2; exit 4; }
mv -T -- "$2" "$to"
`
		return h.script(ctx, script, ref.FS, path.Join(ref.FS, ref.Name), name)
	}
	return &Error{Code: CodeNoSupport}
}

// btrfsIsSnapshot refuses a path that is not a snapshot, so a rename or a
// delete can never reach a subvolume someone works in. $2 is the path.
const btrfsIsSnapshot = `parent=$("$BTRFS" subvolume show -- "$2" 2>/dev/null | awk -F: '$1 ~ /Parent UUID/ { gsub(/[ \t]/, "", $2); print $2 }')
case "$parent" in ""|-) echo "not a snapshot" >&2; exit 5 ;; esac
`

// Hold places or releases BombVault's hold on a ZFS snapshot. Only HoldTag
// is ever released; a hold someone else placed is theirs to take away.
func (h *Host) Hold(ctx context.Context, ref Ref, on bool) error {
	if ref.Kind != KindZFS {
		return &Error{Code: CodeNoSupport}
	}
	if managedByRuns(ref.Name) {
		return &Error{Code: CodeManaged}
	}
	if err := checkDataset(ref.Source, ref.Name); err != nil {
		return err
	}
	verb := "release"
	if on {
		verb = "hold"
	}
	err := h.zfs(ctx, verb, "--", HoldTag, ref.Source+"@"+ref.Name)
	if CodeOf(err) == CodeExists || (CodeOf(err) == CodeNotFound && !on) {
		return nil
	}
	return err
}

// Result is the outcome of one snapshot in a delete.
type Result struct {
	Ref    Ref    `json:"ref"`
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
}

// Delete removes snapshots and answers for each. ZFS snapshots go out in one
// round trip, each in its own destroy so one that is busy does not hold back
// the rest.
func (h *Host) Delete(ctx context.Context, refs []Ref) []Result {
	out := make([]Result, 0, len(refs))
	var zfsRefs []Ref
	for _, ref := range refs {
		if managedByRuns(path.Base(ref.Name)) {
			out = append(out, Result{Ref: ref, Code: CodeManaged})
			continue
		}
		switch ref.Kind {
		case KindZFS:
			if err := checkDataset(ref.Source, ref.Name); err != nil {
				out = append(out, Result{Ref: ref, Code: CodeOf(err)})
				continue
			}
			zfsRefs = append(zfsRefs, ref)
		case KindBtrfs:
			out = append(out, h.deleteBtrfs(ctx, ref))
		case KindVM:
			out = append(out, h.deleteVM(ctx, ref))
		default:
			out = append(out, Result{Ref: ref, Code: CodeNoSupport})
		}
	}
	return append(out, h.deleteZFS(ctx, zfsRefs)...)
}

func (h *Host) deleteZFS(ctx context.Context, refs []Ref) []Result {
	if len(refs) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString(preamble)
	for i, ref := range refs {
		full := shQuote(ref.Source + "@" + ref.Name)
		b.WriteString(`if out=$("$ZFS" destroy -- ` + full + ` 2>&1); then echo "R` + strconv.Itoa(i) + `${tab}ok"; else printf 'R%s\tfail\t%s\n' ` + strconv.Itoa(i) + ` "$(printf '%s' "$out" | tr '\t\n' '  ')"; fi` + "\n")
	}
	stdout, stderr, err := h.sh.RunCapture(ctx, "sh", "-c", b.String(), "bombvault-snapshots")
	got := map[int]Result{}
	for _, line := range strings.Split(stdout, "\n") {
		f := strings.Split(line, "\t")
		if len(f) < 2 || !strings.HasPrefix(f[0], "R") {
			continue
		}
		i, convErr := strconv.Atoi(f[0][1:])
		if convErr != nil || i < 0 || i >= len(refs) {
			continue
		}
		if f[1] == "ok" {
			got[i] = Result{Ref: refs[i], Code: "ok"}
			continue
		}
		msg := ""
		if len(f) > 2 {
			msg = f[2]
		}
		got[i] = Result{Ref: refs[i], Code: zfsCode(msg, nil), Detail: clip(msg)}
	}
	out := make([]Result, len(refs))
	for i, ref := range refs {
		r, ok := got[i]
		if !ok {
			r = Result{Ref: ref, Code: zfs.Classify(stderr, err), Detail: clip(stderr)}
			if err == nil {
				r.Code = CodeFailed
			}
		}
		if r.Code == CodeNotFound {
			r.Code = "ok"
		}
		out[i] = r
	}
	return out
}

func (h *Host) deleteBtrfs(ctx context.Context, ref Ref) Result {
	if err := checkBtrfsPath(ref.FS, ref.Name, false); err != nil {
		return Result{Ref: ref, Code: CodeOf(err)}
	}
	script := preamble + btrfsIsSnapshot + `"$BTRFS" subvolume delete -- "$2" >/dev/null
`
	if err := h.script(ctx, script, ref.FS, path.Join(ref.FS, ref.Name)); err != nil {
		return Result{Ref: ref, Code: CodeOf(err), Detail: detailOf(err)}
	}
	return Result{Ref: ref, Code: "ok"}
}

func (h *Host) deleteVM(ctx context.Context, ref Ref) Result {
	if err := checkDomain(ref.Source); err != nil {
		return Result{Ref: ref, Code: CodeOf(err)}
	}
	if ref.Name == "" || strings.HasPrefix(ref.Name, "-") || strings.ContainsAny(ref.Name, "\n\t") {
		return Result{Ref: ref, Code: "invalid-name"}
	}
	if h.vms == nil {
		return Result{Ref: ref, Code: CodeNoSupport}
	}
	if err := h.vms.SnapshotDelete(ctx, ref.Source, ref.Name); err != nil {
		e := virshError(err)
		return Result{Ref: ref, Code: CodeOf(e), Detail: detailOf(e)}
	}
	return Result{Ref: ref, Code: "ok"}
}

// Estimate is how much deleting the ZFS snapshots among refs would free,
// as ZFS itself works it out for exactly that set. The page adds what btrfs
// quotas report, and says so where nothing can be known.
func (h *Host) Estimate(ctx context.Context, refs []Ref) (int64, error) {
	byDataset := map[string][]string{}
	var order []string
	for _, ref := range refs {
		if ref.Kind != KindZFS {
			continue
		}
		if err := checkDataset(ref.Source, ref.Name); err != nil {
			return 0, err
		}
		if _, ok := byDataset[ref.Source]; !ok {
			order = append(order, ref.Source)
		}
		byDataset[ref.Source] = append(byDataset[ref.Source], ref.Name)
	}
	var total int64
	for _, ds := range order {
		out, err := h.zfsOut(ctx, "destroy", "-n", "-v", "-p", "--", ds+"@"+strings.Join(byDataset[ds], ","))
		if err != nil {
			return 0, err
		}
		for _, line := range strings.Split(out, "\n") {
			f := strings.Fields(line)
			if len(f) == 2 && f[0] == "reclaim" {
				if n := num(f[1]); n > 0 {
					total += n
				}
			}
		}
	}
	return total, nil
}

// Mount is one place a snapshot is mounted.
type Mount struct {
	PID         int    `json:"pid"`
	Path        string `json:"path"`
	Host        bool   `json:"host"`
	ContainerID string `json:"containerId,omitempty"`
	// Container is filled in by the caller, which knows the names.
	Container string `json:"container,omitempty"`
	Code      string `json:"code,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

var containerIDRe = regexp.MustCompile(`(?:docker[-/]|libpod[-/]|containerd[-/]|cri-containerd-)([0-9a-f]{64})`)

// Where lists the mounts that keep ZFS from destroying a snapshot it calls
// busy, in every mount namespace on the host, so the page can say which
// container holds it.
func (h *Host) Where(ctx context.Context, ref Ref) ([]Mount, error) {
	if ref.Kind != KindZFS {
		return nil, &Error{Code: CodeNoSupport}
	}
	if err := checkDataset(ref.Source, ref.Name); err != nil {
		return nil, err
	}
	full := ref.Source + "@" + ref.Name
	out, stderr, err := h.sh.RunCapture(ctx, "sh", "-c", whereScript, "bombvault-snapshots", full, mountEscape(full))
	if err != nil {
		return nil, &Error{Code: zfs.Classify(stderr, err), Detail: clip(stderr)}
	}
	mounts := []Mount{}
	for _, line := range strings.Split(out, "\n") {
		f := strings.Split(line, "\t")
		if len(f) < 5 || f[0] != "M" {
			continue
		}
		pid, _ := strconv.Atoi(f[1])
		m := Mount{PID: pid, Path: mountUnescape(f[3]), Host: f[4] == "1"}
		if len(f) > 5 && !m.Host {
			if c := containerIDRe.FindStringSubmatch(f[5]); c != nil {
				m.ContainerID = c[1]
			}
		}
		mounts = append(mounts, m)
	}
	return mounts, nil
}

// FreeAndDelete unmounts a busy ZFS snapshot everywhere it is mounted and
// destroys it. It returns what each unmount did; the error is the destroy's.
func (h *Host) FreeAndDelete(ctx context.Context, ref Ref) ([]Mount, error) {
	if ref.Kind != KindZFS {
		return nil, &Error{Code: CodeNoSupport}
	}
	if managedByRuns(ref.Name) {
		return nil, &Error{Code: CodeManaged}
	}
	if err := checkDataset(ref.Source, ref.Name); err != nil {
		return nil, err
	}
	full := ref.Source + "@" + ref.Name
	out, stderr, err := h.sh.RunCapture(ctx, "sh", "-c", freeScript, "bombvault-snapshots", full, mountEscape(full))
	if err != nil && !strings.Contains(out, "\nR\t") && !strings.HasPrefix(out, "R\t") {
		return nil, &Error{Code: zfs.Classify(stderr, err), Detail: clip(stderr)}
	}
	mounts := []Mount{}
	var result error = &Error{Code: CodeFailed}
	for _, line := range strings.Split(out, "\n") {
		f := strings.Split(line, "\t")
		switch {
		case len(f) >= 6 && f[0] == "U":
			pid, _ := strconv.Atoi(f[1])
			m := Mount{PID: pid, Path: mountUnescape(f[3]), Host: f[4] == "1", Code: "ok"}
			if f[5] != "0" {
				m.Code = CodeFailed
				if len(f) > 6 {
					m.Detail = clip(f[6])
				}
			}
			if len(f) > 7 && !m.Host {
				if c := containerIDRe.FindStringSubmatch(f[7]); c != nil {
					m.ContainerID = c[1]
				}
			}
			mounts = append(mounts, m)
		case len(f) >= 2 && f[0] == "R":
			if f[1] == "ok" {
				result = nil
				continue
			}
			msg := ""
			if len(f) > 2 {
				msg = f[2]
			}
			if code := zfsCode(msg, nil); code == CodeNotFound {
				result = nil
			} else {
				result = &Error{Code: code, Detail: clip(msg)}
			}
		}
	}
	return mounts, result
}

func (h *Host) zfs(ctx context.Context, args ...string) error {
	_, err := h.zfsOut(ctx, args...)
	return err
}

func (h *Host) zfsOut(ctx context.Context, args ...string) (string, error) {
	script := preamble + `[ -n "$ZFS" ] || { echo "zfs: command not found" >&2; exit 127; }
"$ZFS" "$@"
`
	argv := append([]string{"sh", "-c", script, "bombvault-snapshots"}, args...)
	out, stderr, err := h.sh.RunCapture(ctx, argv...)
	if err != nil {
		return "", &Error{Code: zfsCode(stderr, err), Detail: clip(stderr)}
	}
	return out, nil
}

// script runs a script that was built from constants and takes everything
// variable as arguments.
func (h *Host) script(ctx context.Context, script string, args ...string) error {
	argv := append([]string{"sh", "-c", script, "bombvault-snapshots"}, args...)
	_, stderr, err := h.sh.RunCapture(ctx, argv...)
	if err == nil {
		return nil
	}
	low := strings.ToLower(stderr)
	switch {
	case strings.Contains(low, "already exists"), strings.Contains(low, "file exists"):
		return &Error{Code: CodeExists, Detail: clip(stderr)}
	case strings.Contains(low, "not a snapshot"), strings.Contains(low, "no such file"), strings.Contains(low, "not a btrfs"):
		return &Error{Code: CodeNotFound, Detail: clip(stderr)}
	}
	code := zfs.Classify(stderr, err)
	if code == "zfs-error" || code == "zfs-not-found" {
		code = CodeFailed
	}
	return &Error{Code: code, Detail: clip(stderr)}
}

// zfsCode sharpens the ZFS domain's classification for this page, where a
// hold, a mount and a clone each want their own answer.
func zfsCode(stderr string, err error) string {
	low := strings.ToLower(stderr)
	switch {
	case strings.Contains(low, "dependent clones"):
		return CodeClones
	case strings.Contains(low, "being held"), strings.Contains(low, "is held"):
		return CodeHeld
	}
	switch code := zfs.Classify(stderr, err); code {
	case "busy":
		return CodeBusy
	case "not-found":
		return CodeNotFound
	case "exists":
		return CodeExists
	default:
		return code
	}
}

func virshError(err error) error {
	msg := err.Error()
	low := strings.ToLower(msg)
	switch {
	case strings.Contains(low, "already exists"):
		return &Error{Code: CodeExists, Detail: clip(msg)}
	case strings.Contains(low, "not found"), strings.Contains(low, "no domain snapshot"):
		return &Error{Code: CodeNotFound, Detail: clip(msg)}
	case strings.Contains(low, "for storage type"), strings.Contains(low, "internal snapshot") && strings.Contains(low, "unsupported"):
		return &Error{Code: CodeNeedsQcow2, Detail: clip(msg)}
	case strings.Contains(low, "unsupported"), strings.Contains(low, "not supported"):
		return &Error{Code: CodeNoSupport, Detail: clip(msg)}
	}
	return &Error{Code: CodeFailed, Detail: clip(msg)}
}

func detailOf(err error) string {
	if e, ok := err.(*Error); ok {
		return e.Detail
	}
	return ""
}

// checkDataset refuses a dataset or snapshot name that is not one, so a
// destroy always carries an "@" and can never reach a dataset.
func checkDataset(dataset, snap string) error {
	if err := zfs.ValidateMemberName(dataset); err != nil {
		return ErrInvalidName
	}
	if ValidateName(snap) != nil && !validZFSSnapName(snap) {
		return ErrInvalidName
	}
	if len(dataset)+1+len(snap) > zfs.MaxSnapshotNameLen {
		return ErrInvalidName
	}
	return nil
}

// validZFSSnapName accepts a snapshot name someone else made, which can hold
// the characters ZFS allows beyond the ones this page offers.
func validZFSSnapName(s string) bool {
	if s == "" || s[0] == '-' || strings.ContainsAny(s, "@/,%\t\n") {
		return false
	}
	return zfs.ValidateMemberName(s) == nil
}

// checkBtrfsPath refuses a filesystem that is not an absolute clean path and
// a subvolume path that would climb out of it. An empty sub is the top level,
// which only a create may name.
func checkBtrfsPath(fs, sub string, create bool) error {
	if !strings.HasPrefix(fs, "/") || path.Clean(fs) != fs {
		return ErrInvalidName
	}
	if sub == "" {
		if create {
			return nil
		}
		return ErrInvalidName
	}
	if strings.HasPrefix(sub, "/") || path.Clean(sub) != sub || sub == ".." || strings.HasPrefix(sub, "../") || strings.HasPrefix(sub, "-") {
		return ErrInvalidName
	}
	return nil
}

func checkDomain(name string) error {
	if name == "" || strings.HasPrefix(name, "-") || strings.ContainsAny(name, "\n\t/") {
		return ErrInvalidName
	}
	return nil
}
