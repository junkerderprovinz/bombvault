package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The limits of a start test's copy. They are fixed, so a test can never take
// more of the host than this.
const (
	startTestCPUs   = 1_000_000_000
	startTestMemory = 2 << 30
	startTestPids   = 512
)

// How long a start test waits for its copy: a healthcheck gets five minutes
// (start periods can be long), the port check two, and a container with
// neither has to stay up for twenty seconds.
const (
	startTestHealthWait = 5 * time.Minute
	startTestPortWait   = 2 * time.Minute
	startTestCleanup    = 2 * time.Minute
)

// startTestSettle and startTestPoll are variables so a test does not wait
// out the real ones.
var (
	startTestSettle = 20 * time.Second
	startTestPoll   = 2 * time.Second
)

// startTestSandboxPrefix names the folder a start test restores into.
const startTestSandboxPrefix = "bombvault-starttest"

// errStartTestNoDocker is the answer where BombVault's Docker client cannot
// create networks and containers of its own.
var errStartTestNoDocker = errors.New("start tests need BombVault's own Docker access, which this setup does not have")

// startTestBlocked is why a container cannot run in isolation. Code is what
// the web interface translates.
type startTestBlocked struct{ Code string }

func (e startTestBlocked) Error() string {
	return "this container cannot be started in isolation (" + e.Code + ")"
}

// startTestBlocker returns why a container's recipe cannot run in an isolated
// copy, or "" when it can. The copy would need the host itself or another
// container, so it is listed as not testable instead of being tried.
func startTestBlocker(in model.Inspect) string {
	hc := in.HostConfig
	switch {
	case hc.NetworkMode == "host":
		return "host-network"
	case hc.Privileged:
		return "privileged"
	case len(hc.Devices) > 0:
		return "devices"
	case hc.PidMode == "host" || hc.IpcMode == "host" || hc.UsernsMode == "host":
		return "host-namespace"
	case strings.HasPrefix(hc.NetworkMode, "container:"),
		strings.HasPrefix(hc.PidMode, "container:"),
		strings.HasPrefix(hc.IpcMode, "container:"),
		strings.TrimSpace(in.Config.Labels["com.docker.compose.depends_on"]) != "":
		return "depends-on"
	}
	return ""
}

// startTestRecipe reads the recipe a restore would create a container from,
// and why it cannot be tested when it cannot.
func startTestRecipe(tg store.Target) (containerDefinition, string) {
	var def containerDefinition
	if tg.Definition == "" || json.Unmarshal([]byte(tg.Definition), &def) != nil || def.Inspect.Config.Image == "" {
		return def, "no-definition"
	}
	return def, startTestBlocker(def.Inspect)
}

// RunStartTest runs a start test of one container now. The result is on the
// screen of whoever asked, so it sends nothing.
func (s *Service) RunStartTest(ctx context.Context, targetID string) (store.StartTest, error) {
	tg, err := s.store.GetTargetByID(targetID)
	if err != nil {
		return store.StartTest{}, fmt.Errorf("no container with id %q", targetID)
	}
	return s.startTest(context.WithoutCancel(ctx), tg, "manual")
}

// runScheduledStartTest tests the container whose last start test is oldest,
// never-tested ones first, and notifies when it fails.
func (s *Service) runScheduledStartTest(ctx context.Context) error {
	targets, err := s.store.ListTargets()
	if err != nil {
		return err
	}
	latest, err := s.store.LatestStartTests()
	if err != nil {
		return err
	}
	self := s.selfContainerName(ctx)
	var pick *store.Target
	for i := range targets {
		tg := targets[i]
		if tg.ContainerName == self {
			continue
		}
		if _, blocked := startTestRecipe(tg); blocked != "" {
			continue
		}
		run, rErr := s.store.LastSuccessfulBackup(tg.ID)
		if rErr != nil || run == nil {
			continue
		}
		if pick == nil || latest[tg.ID].At < latest[pick.ID].At ||
			(latest[tg.ID].At == latest[pick.ID].At && tg.ContainerName < pick.ContainerName) {
			pick = &targets[i]
		}
	}
	if pick == nil {
		return nil
	}
	rec, err := s.startTest(ctx, *pick, "schedule")
	if err == nil && !rec.OK {
		s.notifyStartTestFailure(ctx, rec)
	}
	return err
}

// startTest restores a container's newest backup into an isolated copy,
// starts it, checks it and removes everything again. An error means the test
// could not run at all; a copy that did not come up is a recorded failure.
func (s *Service) startTest(ctx context.Context, tg store.Target, trigger string) (store.StartTest, error) {
	host, ok := s.docker.(dockercli.StartTestHost)
	if !ok {
		return store.StartTest{}, errStartTestNoDocker
	}
	def, blocked := startTestRecipe(tg)
	if blocked != "" {
		return store.StartTest{}, startTestBlocked{Code: blocked}
	}
	run, err := s.store.LastSuccessfulBackup(tg.ID)
	if err != nil {
		return store.StartTest{}, err
	}
	if run == nil {
		return store.StartTest{}, errors.New("this container has no backup yet")
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return store.StartTest{}, fmt.Errorf("read settings: %w", err)
	}

	key := "starttest:" + tg.ContainerName
	_, progStarted := s.progBegin(ctx, key, "maintenance")
	started := time.Now()
	method, testErr := s.runStartTestCopy(ctx, host, settings, tg, def, run.SnapshotID, trigger)
	s.progEnd(key, "maintenance", testErr == nil, progStarted)
	if errors.Is(testErr, errDomainBusy) {
		return store.StartTest{}, testErr
	}
	var setupErr startTestSetupError
	if errors.As(testErr, &setupErr) {
		return store.StartTest{}, setupErr.err
	}

	rec := store.StartTest{
		TargetID: tg.ID, Container: tg.ContainerName, At: time.Now().Unix(),
		OK: testErr == nil, Method: method, DurationMS: time.Since(started).Milliseconds(), Trigger: trigger,
	}
	if testErr != nil {
		rec.Detail = truncateDetail(scrubError(testErr))
	}
	if err := s.store.AddStartTest(rec); err != nil {
		return rec, fmt.Errorf("record start test: %w", err)
	}
	return rec, nil
}

// startTestSetupError is a failure before the copy existed, such as a busy
// domain or a restore folder that cannot be written. It says nothing about the
// container, so it is reported instead of recorded.
type startTestSetupError struct{ err error }

func (e startTestSetupError) Error() string { return e.err.Error() }

// runStartTestCopy does the work of one start test and returns how the copy
// was judged. Everything it creates is removed before it returns, whatever
// happened, on a context of its own so a cancelled test still cleans up.
func (s *Service) runStartTestCopy(ctx context.Context, host dockercli.StartTestHost, settings store.Settings, tg store.Target, def containerDefinition, snapshotID, trigger string) (string, error) {
	sandbox, cleanupSandbox, err := s.newDrillSandbox(settings, startTestSandboxPrefix+"-"+sanitizeName(tg.ContainerName))
	if err != nil {
		return "", startTestSetupError{err}
	}
	defer cleanupSandbox()

	if snapshotID != "" {
		if err := s.restoreForStartTest(ctx, settings, tg, sandbox, snapshotID, trigger); err != nil {
			return "", err
		}
	}

	suffix := randomSuffix()
	netName := dockercli.StartTestPrefix + "net-" + suffix
	name := dockercli.StartTestPrefix + truncateName(sanitizeName(tg.ContainerName), 40) + "-" + suffix
	defer func() {
		cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), startTestCleanup)
		defer cancel()
		if err := host.RemoveTestContainer(cctx, name); err != nil {
			log.Printf("api: start test %q: remove copy: %v", tg.ContainerName, err) //nolint:gosec // G706: %q-quoted
		}
		if err := host.RemoveTestNetwork(cctx, netName); err != nil {
			log.Printf("api: start test %q: remove network: %v", tg.ContainerName, err) //nolint:gosec // G706: %q-quoted
		}
	}()

	if err := host.CreateTestNetwork(ctx, netName); err != nil {
		return "", startTestSetupError{err}
	}
	spec := dockercli.IsolatedSpec{
		Name:        name,
		Network:     netName,
		From:        def.Inspect,
		Binds:       s.startTestBinds(def, sandbox),
		NanoCPUs:    startTestCPUs,
		MemoryBytes: startTestMemory,
		PidsLimit:   startTestPids,
		Labels:      map[string]string{dockercli.StartTestLabel + ".of": tg.ContainerName},
	}
	if err := host.StartIsolated(ctx, spec); err != nil {
		return "", withoutPrivileges(fmt.Errorf("the copy did not start: %w", err), def.Inspect)
	}
	method, err := s.judgeStartTest(ctx, host, name, netName, suffix)
	return method, withoutPrivileges(err, def.Inspect)
}

// withoutPrivileges adds to a failed start test what the copy ran without,
// since an app that needs an added capability fails for that reason alone.
func withoutPrivileges(err error, in model.Inspect) error {
	granted := dockercli.GrantedPrivileges(in)
	if err == nil || len(granted) == 0 {
		return err
	}
	return fmt.Errorf("%w; the copy ran without what the original is granted: %s", err, strings.Join(granted, "; "))
}

// restoreForStartTest restores the backup's data into the sandbox under the
// domain lock, which is released again before the copy starts.
func (s *Service) restoreForStartTest(ctx context.Context, settings store.Settings, tg store.Target, sandbox, snapshotID, trigger string) error {
	var unlock func()
	if trigger == "schedule" {
		u, ok := s.waitLockDomainFor("containers", "verify")
		if !ok {
			return errDomainBusy
		}
		unlock = u
	} else {
		u, ok := s.tryLockDomainFor("containers", "verify")
		if !ok {
			return errDomainBusy
		}
		unlock = u
	}
	defer unlock()

	repo, err := s.containerRepoPath(settings, tg)
	if err != nil {
		return startTestSetupError{err}
	}
	mode := s.primaryModeFor(settings, "containers", repo)
	s.unlockStale(ctx, repo, mode)
	rctx, cancel := context.WithTimeout(ctx, restoreTimeout)
	defer cancel()
	if _, want, sErr := s.engine.StatsRestoreSize(rctx, repo, snapshotID, mode); sErr == nil && want > 0 {
		if free, fErr := s.diskFreeFn()(sandbox); fErr == nil && free < uint64(want) {
			return startTestSetupError{fmt.Errorf("not enough free space in the restore folder: the backup needs %d bytes, %d are free", want, free)}
		}
	}
	if err := s.engine.RestoreAll(rctx, repo, snapshotID, sandbox, mode); err != nil && !errors.Is(err, restic.ErrRestoreMetadataOnly) {
		return fmt.Errorf("restore the backup: %w", err)
	}
	return nil
}

// startTestBinds points the recipe's bind mounts at the restored copy of the
// data. A bind outside the backed-up folders is dropped rather than mounted,
// so the copy never sees the original's files, and named volumes are dropped
// for the same reason.
func (s *Service) startTestBinds(def containerDefinition, sandbox string) []string {
	var out []string
	for _, b := range def.Inspect.HostConfig.Binds {
		src, rest, ok := strings.Cut(b, ":")
		if !ok || !strings.HasPrefix(src, "/") {
			continue
		}
		cp, reachable := s.toContainerPath(src)
		if !reachable || !slices.ContainsFunc(def.AppdataPaths, func(p string) bool { return cp == p || strings.HasPrefix(cp, p+"/") }) {
			continue
		}
		out = append(out, s.toHostPath(path.Join(sandbox, cp))+":"+rest)
	}
	return out
}

// judgeStartTest waits for the copy to prove itself: healthy by its own
// healthcheck, else answering on its first exposed TCP port, else still
// running after a while. It returns the method it used.
func (s *Service) judgeStartTest(ctx context.Context, host dockercli.StartTestHost, name, netName, suffix string) (string, error) {
	deadline := time.Now().Add(startTestHealthWait)
	var st dockercli.IsolatedState
	for {
		var err error
		st, err = host.IsolatedState(ctx, name)
		if err != nil {
			return "", err
		}
		if !st.Running {
			return methodFor(st), exitReason(st)
		}
		switch st.Health {
		case "":
		case "healthy":
			return "health", nil
		case "unhealthy":
			return "health", errors.New("its healthcheck reported unhealthy")
		default:
			if time.Now().After(deadline) {
				return "health", fmt.Errorf("its healthcheck did not report healthy within %s", startTestHealthWait)
			}
			if err := sleepCtx(ctx, startTestPoll); err != nil {
				return "health", err
			}
			continue
		}
		break
	}

	if port := firstTCPPort(st.ExposedPorts); port != "" {
		if image := s.proberImage(ctx); image != "" {
			code, err := host.RunProber(ctx, dockercli.ProberSpec{
				Name:       dockercli.StartTestPrefix + "probe-" + suffix,
				Image:      image,
				Network:    netName,
				Entrypoint: []string{imageBinaryPath},
				Cmd:        []string{"tcp-probe", name + ":" + port, strconv.Itoa(int(startTestPortWait.Seconds()))},
				Timeout:    startTestPortWait + 30*time.Second,
			})
			if err != nil {
				return "tcp", err
			}
			if code != 0 {
				if st, sErr := host.IsolatedState(ctx, name); sErr == nil && !st.Running {
					return "tcp", exitReason(st)
				}
				return "tcp", fmt.Errorf("nothing answered on port %s within %s", port, startTestPortWait)
			}
			return "tcp", nil
		}
	}

	if err := sleepCtx(ctx, startTestSettle); err != nil {
		return "running", err
	}
	st, err := host.IsolatedState(ctx, name)
	if err != nil {
		return "running", err
	}
	if !st.Running {
		return "running", exitReason(st)
	}
	return "running", nil
}

func methodFor(st dockercli.IsolatedState) string {
	if st.Health != "" {
		return "health"
	}
	return "running"
}

func exitReason(st dockercli.IsolatedState) error {
	if st.OOMKilled {
		return fmt.Errorf("it ran out of memory (the limit is %d MiB)", startTestMemory>>20)
	}
	return fmt.Errorf("it stopped with exit code %d", st.ExitCode)
}

// firstTCPPort is the lowest exposed TCP port, as a number.
func firstTCPPort(exposed []string) string {
	best := 0
	for _, p := range exposed {
		num, proto, _ := strings.Cut(p, "/")
		if proto != "" && proto != "tcp" {
			continue
		}
		n, err := strconv.Atoi(num)
		if err != nil || n <= 0 {
			continue
		}
		if best == 0 || n < best {
			best = n
		}
	}
	if best == 0 {
		return ""
	}
	return strconv.Itoa(best)
}

// proberImage is the image BombVault itself runs from, which carries the
// tcp-probe command. Empty when BombVault does not run in a container.
func (s *Service) proberImage(ctx context.Context) string {
	self := s.selfContainerName(ctx)
	if self == "" {
		return ""
	}
	in, err := s.docker.Inspect(ctx, self)
	if err != nil {
		return ""
	}
	return in.Image
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func randomSuffix() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// sanitizeName keeps what Docker accepts in a name.
func sanitizeName(name string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '.', r == '-':
			return r
		}
		return '-'
	}, name)
}

func truncateName(name string, n int) string {
	if len(name) > n {
		return name[:n]
	}
	return name
}

// CleanupStartTestLeftovers removes what start tests left behind when
// BombVault stopped in the middle of one: the copies and networks by label and
// name, the restored data by its marker.
func (s *Service) CleanupStartTestLeftovers(ctx context.Context) {
	if host, ok := s.docker.(dockercli.StartTestHost); ok {
		containers, networks, err := host.StartTestLeftovers(ctx)
		if err != nil {
			log.Printf("api: start test leftovers: %v", err)
		}
		for _, name := range containers {
			if err := host.RemoveTestContainer(ctx, name); err != nil {
				log.Printf("api: start test leftovers: %v", err)
			}
		}
		for _, name := range networks {
			if err := host.RemoveTestNetwork(ctx, name); err != nil {
				log.Printf("api: start test leftovers: %v", err)
			}
		}
		if n := len(containers) + len(networks); n > 0 {
			log.Printf("api: removed %d leftovers of an interrupted start test", n)
		}
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return
	}
	dir, err := paths.Resolve(s.cfg.HostMountRoot, settings.RestoreFolder)
	if err != nil {
		return
	}
	entries, err := os.ReadDir(dir) //nolint:gosec // G703: dir is resolved strictly under the host mount root by paths.Resolve
	if err != nil {
		return
	}
	for _, e := range entries {
		ours := strings.HasPrefix(e.Name(), startTestSandboxPrefix+"-") || strings.HasPrefix(e.Name(), "bombvault-probe-")
		if !e.IsDir() || !ours {
			continue
		}
		if err := cleanupDrillSandbox(filepath.Join(dir, e.Name())); err != nil {
			log.Printf("api: start test leftovers: %v", err)
		}
	}
}

// notifyStartTestFailure reports a failed scheduled start test.
func (s *Service) notifyStartTestFailure(ctx context.Context, rec store.StartTest) {
	c, err := s.NotifyConfig()
	if err != nil || c.On == "" || c.On == "never" {
		return
	}
	msg := fmt.Sprintf("The start test of %s failed: a copy restored from its backup did not come up (%s).", rec.Container, rec.Detail)
	notify.Send(ctx, c, "containers", notify.Event{Title: "BombVault", Message: msg, OK: false})
	if s.unraidGate(c.Unraid) {
		if e := s.sendUnraidNotify(ctx, "BombVault: start test failed", msg, "warning"); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}
