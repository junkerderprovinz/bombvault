package api

import (
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/hostload"
	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// procDir and cgroupDir are where the kernel counters are read.
var (
	procDir   = "/proc"
	cgroupDir = "/sys/fs/cgroup"
)

// loadEvery is how often the counters are read while a backup runs.
const loadEvery = 5 * time.Second

// A run counts as slow when restic took this much longer than the median of
// the item's recent successful runs, and at least slowMinExtra longer.
const (
	slowFactor   = 1.5
	slowMinExtra = 60 * time.Second
	slowHistory  = 10
	slowMinRuns  = 3
)

// runLoad is what a backup run records about the host. Cause is set only on
// a slow run with a single clear brake.
type runLoad struct {
	hostload.Summary
	Slow  bool            `json:"slow,omitempty"`
	Cause *hostload.Cause `json:"cause,omitempty"`
}

func (s *Service) runStarted(ev store.RunStarted) {
	if ev.Kind == "backup" {
		s.load.Begin(ev.RunID)
	}
}

// runEnded runs on the goroutine that holds the database, so the summary is
// written from a goroutine of its own.
func (s *Service) runEnded(ev store.RunFinished) {
	if ev.RunID == "" {
		return
	}
	go s.refreshBreakdownAfter(ev.RunID)
	sum, ok := s.load.End(ev.RunID)
	if !ok {
		return
	}
	go s.recordRunLoad(ev.RunID, sum)
}

func (s *Service) recordRunLoad(runID string, sum hostload.Summary) {
	run, err := s.store.GetRun(runID)
	if err != nil {
		return
	}
	item := s.loadItemOf(run.TargetID)
	hostload.NewResolver(procDir, s.cfg.HostMountRoot).Annotate(&sum, item.sources, item.target)
	load := runLoad{Summary: sum}
	if run.Status == "success" && s.runWasSlow(run) {
		load.Slow = true
		load.Cause = hostload.CauseOf(sum, item.uploadLimitBps)
	}
	b, err := json.Marshal(load)
	if err != nil {
		return
	}
	if err := s.store.SetRunLoad(runID, string(b)); err != nil {
		log.Printf("api: run %s: storing how busy the host was: %v", runID, err)
	}
}

// runWasSlow compares restic's time for the run with the item's recent
// successful runs.
func (s *Service) runWasSlow(run store.Run) bool {
	series, err := s.store.ItemSeries(run.TargetID, run.Kind, run.StartedAt, slowHistory+1)
	if err != nil {
		return false
	}
	var current *int64
	var prior []float64
	for _, r := range series {
		switch {
		case r.ID == run.ID:
			current = r.ResticMS
		case r.Status == "success" && r.ResticMS != nil && len(prior) < slowHistory:
			prior = append(prior, float64(*r.ResticMS))
		}
	}
	if current == nil || len(prior) < slowMinRuns {
		return false
	}
	slices.Sort(prior)
	med := prior[len(prior)/2]
	if len(prior)%2 == 0 {
		med = (prior[len(prior)/2-1] + med) / 2
	}
	cur := float64(*current)
	return cur >= slowFactor*med && cur-med >= float64(slowMinExtra.Milliseconds())
}

// loadItem is where a run read from and wrote to. target is empty for a
// remote repository.
type loadItem struct {
	sources        []string
	target         string
	uploadLimitBps float64
}

func (s *Service) loadItemOf(targetID string) loadItem {
	var item loadItem
	settings, err := s.store.GetSettings()
	if err != nil {
		return item
	}
	var domain, repo string
	switch targetID {
	case store.FlashTargetID:
		domain, item.sources = "flash", []string{s.cfg.FlashDir}
		repo, _ = s.repoFor(settings, domain, "local")
	case store.ConfigTargetID:
		domain = "config"
		repo, _ = s.repoFor(settings, domain, "local")
	default:
		if t, err := s.store.GetTargetByID(targetID); err == nil {
			domain, item.sources = "containers", t.AppdataPaths
			repo, _ = s.containerRepoForName(settings, t.ContainerName, "local")
		} else if set, err := s.store.GetFileSet(targetID); err == nil {
			domain = "files"
			if src, err := paths.Resolve(s.cfg.HostMountRoot, set.Path); err == nil {
				item.sources = []string{src}
			}
			repo, _ = s.fileSetRepoFor(settings, set, "local")
		} else if vm, err := s.store.GetVMTargetByID(targetID); err == nil {
			domain = "vms"
			repo, _ = s.vmRepoForName(settings, vm.Name, "local")
		} else if d, err := s.store.GetZFSDataset(targetID); err == nil {
			domain = zfsDomain
			repo, _ = s.zfsDatasetRepoFor(settings, d, "local")
		}
	}
	if repo == "" {
		return item
	}
	if restic.IsRemoteRepo(repo) {
		item.uploadLimitBps = float64(s.primaryModeFor(settings, domain, repo).Limits.UploadKBps) * 1024
	} else {
		item.target = repo
	}
	return item
}

// runBottleneck is the cause stored with a run, if it has one.
func runBottleneck(load string) *hostload.Cause {
	if load == "" {
		return nil
	}
	var l runLoad
	if json.Unmarshal([]byte(load), &l) != nil || !l.Slow {
		return nil
	}
	return l.Cause
}

// bottleneckSentence words a cause for an assistant.
func bottleneckSentence(c *hostload.Cause) string {
	pct := int(c.Share*100 + 0.5)
	switch c.Kind {
	case hostload.CauseCPU:
		return fmt.Sprintf("The CPU was %d%% busy.", pct)
	case hostload.CauseCPULimit:
		return fmt.Sprintf("BombVault used %d%% of the CPU limit of its container.", pct)
	case hostload.CauseUpload:
		return fmt.Sprintf("The upload ran at %d%% of the repository's upload limit.", pct)
	}
	switch c.Role {
	case hostload.RoleSource:
		return fmt.Sprintf("The source disk %s was %d%% busy.", c.Name, pct)
	case hostload.RoleTarget:
		return fmt.Sprintf("The target disk %s was %d%% busy.", c.Name, pct)
	case hostload.RoleBoth:
		return fmt.Sprintf("The disk %s, which holds both the source and the target, was %d%% busy.", c.Name, pct)
	}
	return fmt.Sprintf("The disk %s was %d%% busy.", c.Name, pct)
}
