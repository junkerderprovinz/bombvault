# BombVault VM Backup over SSH — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make BombVault's VM backup/restore (graceful + live snapshot) work mount-free by reaching libvirt over `qemu+ssh://`, so the container can never break the host VM Manager.

**Architecture:** BombVault runs `virsh` on the Unraid host via SSH for all control ops; restic reads/writes the VM disks through the existing `/mnt`→`/host/user` mount; NVRAM is read/written on the host over SSH. No libvirt bind mount exists.

**Tech Stack:** Go (cmd/bombvault + internal/*), `virsh`/`ssh`/`ssh-keygen` CLIs, restic, React/Vite SPA.

**Spec:** `docs/superpowers/specs/2026-06-15-bombvault-vm-backup-ssh-design.md`

**Per-task gates:** `go build ./...`, `go test ./...`, `golangci-lint run ./...` clean (and `npm --prefix web run build` for React tasks), then commit (no AI attribution). Wave-end: `/code-review` on the diff. Phase-end: `/security-review`. libvirt/SSH are NOT testable in CI → box-gate on the real Unraid host.

---

## File Structure

- **Create** `internal/sshconn/sshconn.go` — SSH key management + connection helper: keygen/reuse, `VirshURI()`, `Run()` (raw ssh exec), `ReadFile()`/`WriteFile()` (NVRAM over ssh), `PublicKey()`, `Test()`.
- **Create** `internal/sshconn/sshconn_test.go`.
- **Modify** `internal/config/config.go` — add `LibvirtHost`, `LibvirtSSHUser`; SSH dir derived from `DataDir`.
- **Modify** `internal/virshcli/virshcli.go` — `New(uri string)`; every `run` becomes `virsh -c <uri> …`; add `SnapshotCreateDiskOnly`, `BlockCommitActivePivot`, `GuestAgentPing`.
- **Modify** `internal/virshcli/types.go` — extend the `Virsh` interface with the three new methods.
- **Delete** `internal/virshcli/link.go` + `internal/virshcli/link_test.go` — the symlink helper was for the (removed) mount approach.
- **Modify** `internal/backup/vm_orchestrator.go` — extend the `VM` interface; add `BackupVMLive`.
- **Modify** `internal/api/service.go` — wire sshconn; `BackupVM` dispatches graceful/live + stages NVRAM via ssh; `RestoreVM` writes NVRAM back via ssh.
- **Modify** `cmd/bombvault/main.go` — build sshconn, ensure key, pass URI to `virshcli.New`; drop the `LinkSocket` call.
- **Modify** `internal/api/handlers.go` + `api.go` — `GET /api/vm/ssh` (pubkey + host), `POST /api/vm/ssh/test`.
- **Modify** `web/src/pages/Settings.tsx`, `web/src/pages/VMs.tsx`, `web/src/lib/api.ts`, `web/src/lib/i18n.ts` (+ locale files via the existing key-parity build gate).
- **Modify** `Dockerfile` — add `openssh-client`.
- **Modify** `templates/my-BombVault.xml` — `ExtraParams` `--add-host=host.docker.internal:host-gateway` + `LIBVIRT_HOST`/`LIBVIRT_SSH_USER` vars; deliver to `d:\nextcloud\it\github\my-BombVault.xml`.
- **Modify** `README.md` — VM backup over SSH setup + recovery note.

---

## Wave A — SSH foundation

### Task 1: Dockerfile + config fields

**Files:**
- Modify: `Dockerfile:65`
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Add the ssh client to the runtime image.** In `Dockerfile`, add `openssh-client` to the apt install line (currently `ca-certificates libvirt-clients qemu-utils rclone bzip2 wget`):

```dockerfile
    apt-get install -y --no-install-recommends ca-certificates libvirt-clients qemu-utils openssh-client rclone bzip2 wget; \
```

- [ ] **Step 2: Write the failing test** in `internal/config/config_test.go`:

```go
func TestLoadLibvirtDefaults(t *testing.T) {
	c, err := Load(map[string]string{"APP_KEY": strings.Repeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	if c.LibvirtHost != "host.docker.internal" {
		t.Errorf("LibvirtHost = %q, want host.docker.internal", c.LibvirtHost)
	}
	if c.LibvirtSSHUser != "root" {
		t.Errorf("LibvirtSSHUser = %q, want root", c.LibvirtSSHUser)
	}
}
```

- [ ] **Step 3: Run it, expect FAIL** (`undefined field LibvirtHost`): `go test ./internal/config/...`

- [ ] **Step 4: Add the fields.** In `internal/config/config.go`, add to `Config`:

```go
	LibvirtHost    string
	LibvirtSSHUser string
```

and in `Load`, alongside the other defaults:

```go
		// libvirt is reached over SSH (qemu+ssh://). No filesystem mount.
		LibvirtHost:    stringOr(env["LIBVIRT_HOST"], "host.docker.internal"),
		LibvirtSSHUser: stringOr(env["LIBVIRT_SSH_USER"], "root"),
```

- [ ] **Step 5: Run it, expect PASS:** `go test ./internal/config/...`

- [ ] **Step 6: Commit:** `git add internal/config Dockerfile && git commit -m "feat(config): libvirt-over-ssh host/user config + ssh client in image"`

---

### Task 2: `internal/sshconn` — key management + SSH helper

**Files:**
- Create: `internal/sshconn/sshconn.go`
- Test: `internal/sshconn/sshconn_test.go`

**Responsibility:** generate/reuse an ed25519 keypair under `<dataDir>/ssh/`, build the `qemu+ssh://` URI, run raw ssh commands (for NVRAM), and report the public key + connection health.

- [ ] **Step 1: Write the failing tests** in `internal/sshconn/sshconn_test.go`:

```go
package sshconn

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureKeyGeneratesAndReuses(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not available")
	}
	dir := t.TempDir()
	c := &Conn{Host: "host.docker.internal", User: "root", dir: filepath.Join(dir, "ssh")}

	if err := c.EnsureKey(); err != nil {
		t.Fatalf("EnsureKey: %v", err)
	}
	pub, err := c.PublicKey()
	if err != nil || !strings.HasPrefix(pub, "ssh-ed25519 ") {
		t.Fatalf("PublicKey = %q, err=%v", pub, err)
	}
	// Reuse: second call keeps the same key.
	first, _ := os.ReadFile(filepath.Join(c.dir, "id_ed25519"))
	if err := c.EnsureKey(); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(filepath.Join(c.dir, "id_ed25519"))
	if string(first) != string(second) {
		t.Fatal("EnsureKey regenerated the key instead of reusing it")
	}
}

func TestVirshURI(t *testing.T) {
	c := &Conn{Host: "1.2.3.4", User: "root", dir: "/config/ssh"}
	got := c.VirshURI()
	want := "qemu+ssh://root@1.2.3.4/system?keyfile=/config/ssh/id_ed25519&known_hosts=/config/ssh/known_hosts&known_hosts_verify=normal"
	if got != want {
		t.Fatalf("VirshURI = %q, want %q", got, want)
	}
}
```

(add `"os/exec"` import as `exec` in the test)

- [ ] **Step 2: Run it, expect FAIL** (package missing): `go test ./internal/sshconn/...`

- [ ] **Step 3: Implement** `internal/sshconn/sshconn.go`:

```go
// Package sshconn manages BombVault's SSH access to the Unraid host for libvirt
// control (qemu+ssh://) and NVRAM file transfer. No libvirt path is ever
// bind-mounted; the container runs virsh ON the host over SSH.
package sshconn

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Conn holds the SSH identity + target for reaching the host's libvirt.
type Conn struct {
	Host string // e.g. "host.docker.internal"
	User string // e.g. "root"
	dir  string // <dataDir>/ssh
}

// New returns a Conn storing its key material under dataDir/ssh.
func New(host, user, dataDir string) *Conn {
	return &Conn{Host: host, User: user, dir: filepath.Join(dataDir, "ssh")}
}

func (c *Conn) keyPath() string        { return filepath.Join(c.dir, "id_ed25519") }
func (c *Conn) knownHostsPath() string { return filepath.Join(c.dir, "known_hosts") }

// EnsureKey generates an ed25519 keypair on first use and reuses it thereafter.
func (c *Conn) EnsureKey() error {
	if err := os.MkdirAll(c.dir, 0o700); err != nil {
		return fmt.Errorf("sshconn: mkdir: %w", err)
	}
	if _, err := os.Stat(c.keyPath()); err == nil {
		return nil // already present
	}
	cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-N", "", "-C", "bombvault", "-f", c.keyPath()) //nolint:gosec // fixed args
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sshconn: keygen: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// PublicKey returns the authorized_keys line to add on the host.
func (c *Conn) PublicKey() (string, error) {
	b, err := os.ReadFile(c.keyPath() + ".pub")
	if err != nil {
		return "", fmt.Errorf("sshconn: read pubkey: %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}

// VirshURI is the libvirt connection URI for `virsh -c`.
func (c *Conn) VirshURI() string {
	return fmt.Sprintf("qemu+ssh://%s@%s/system?keyfile=%s&known_hosts=%s&known_hosts_verify=normal",
		c.User, c.Host, c.keyPath(), c.knownHostsPath())
}

// sshArgs are the common ssh options (key, pinned known_hosts, no prompts).
func (c *Conn) sshArgs() []string {
	return []string{
		"-i", c.keyPath(),
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "UserKnownHostsFile=" + c.knownHostsPath(),
		c.User + "@" + c.Host,
	}
}

// Run executes a command on the host over SSH and returns trimmed stdout.
func (c *Conn) Run(ctx context.Context, args ...string) (string, error) {
	full := append(c.sshArgs(), append([]string{"--"}, args...)...)
	out, err := exec.CommandContext(ctx, "ssh", full...).Output() //nolint:gosec // argv-separated; host/user from config
	if err != nil {
		return "", fmt.Errorf("sshconn: run %q: %w", args[0], err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ReadFile returns the bytes of a file on the host (used for NVRAM).
func (c *Conn) ReadFile(ctx context.Context, path string) ([]byte, error) {
	full := append(c.sshArgs(), "--", "cat", path)
	out, err := exec.CommandContext(ctx, "ssh", full...).Output() //nolint:gosec // argv-separated
	if err != nil {
		return nil, fmt.Errorf("sshconn: read %q: %w", filepath.Base(path), err)
	}
	return out, nil
}

// WriteFile writes data to a file on the host (used to restore NVRAM). It pipes
// data to `cat > path` over SSH (mkdir -p the parent first).
func (c *Conn) WriteFile(ctx context.Context, path string, data []byte) error {
	dir := filepath.Dir(path)
	sh := fmt.Sprintf("mkdir -p %q && cat > %q", dir, path)
	full := append(c.sshArgs(), "--", "sh", "-c", sh)
	cmd := exec.CommandContext(ctx, "ssh", full...) //nolint:gosec // argv-separated; path quoted in sh
	cmd.Stdin = strings.NewReader(string(data))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sshconn: write %q: %s", filepath.Base(path), strings.TrimSpace(string(out)))
	}
	return nil
}

// Test verifies the SSH path reaches libvirt: runs `virsh -c <uri> list --all`.
func (c *Conn) Test(ctx context.Context) error {
	out, err := exec.CommandContext(ctx, "virsh", "-c", c.VirshURI(), "list", "--all").CombinedOutput() //nolint:gosec // uri from config
	if err != nil {
		return fmt.Errorf("libvirt over SSH not reachable: %s", lastLine(string(out)))
	}
	return nil
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); l != "" {
			return l
		}
	}
	return "unknown error"
}
```

- [ ] **Step 4: Run it, expect PASS:** `go test ./internal/sshconn/...`
- [ ] **Step 5: Lint:** `golangci-lint run ./internal/sshconn/...` → 0 issues.
- [ ] **Step 6: Commit:** `git add internal/sshconn && git commit -m "feat(sshconn): ed25519 key mgmt + qemu+ssh URI + ssh exec/read/write/test"`

---

## Wave B — virshcli over SSH

### Task 3: virshcli connects via the SSH URI

**Files:**
- Modify: `internal/virshcli/virshcli.go:17-46` (Client, New, run)
- Delete: `internal/virshcli/link.go`, `internal/virshcli/link_test.go`
- Test: `internal/virshcli/virshcli_uri_test.go` (new)

- [ ] **Step 1: Delete the obsolete symlink helper** (was for the removed mount): `git rm internal/virshcli/link.go internal/virshcli/link_test.go`

- [ ] **Step 2: Write the failing test** `internal/virshcli/virshcli_uri_test.go`:

```go
package virshcli

import "testing"

func TestClientUsesConnectionURI(t *testing.T) {
	c := New("qemu+ssh://root@h/system")
	got := c.baseArgs("list", "--all")
	want := []string{"-c", "qemu+ssh://root@h/system", "list", "--all"}
	if len(got) != len(want) {
		t.Fatalf("baseArgs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("baseArgs[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestClientEmptyURIHasNoConnFlag(t *testing.T) {
	c := New("")
	got := c.baseArgs("list")
	if len(got) != 1 || got[0] != "list" {
		t.Fatalf("baseArgs = %v, want [list]", got)
	}
}
```

- [ ] **Step 3: Run it, expect FAIL** (`New` takes no args / no `baseArgs`): `go test ./internal/virshcli/...`

- [ ] **Step 4: Implement.** In `internal/virshcli/virshcli.go` change the Client + New + run:

```go
type Client struct {
	bin string // "virsh"
	uri string // qemu+ssh://… ("" = local default, used only in tests/legacy)
}

// New returns a Client that connects via the given libvirt URI (qemu+ssh://…).
// An empty URI uses virsh's default (local) connection.
func New(uri string) *Client { return &Client{bin: "virsh", uri: uri} }

// baseArgs prefixes "-c <uri>" when a URI is configured.
func (c *Client) baseArgs(args ...string) []string {
	if c.uri == "" {
		return args
	}
	return append([]string{"-c", c.uri}, args...)
}
```

and in `run`, build the command from `baseArgs`:

```go
func (c *Client) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, c.bin, c.baseArgs(args...)...) //nolint:gosec // G204: args separate; uri/name from config + libvirt
	out, err := cmd.Output()
	...
```

- [ ] **Step 5: Run it, expect PASS:** `go test ./internal/virshcli/...`
- [ ] **Step 6: Build the whole module** (the `New()` signature changed — main.go won't compile yet; that's Task 4): note expected break, do NOT commit until Task 4. Run `go build ./internal/...` (should pass; only cmd breaks).
- [ ] **Step 7: Commit after Task 4** (combined), or commit now with `//nolint` stub if needed. Prefer combine.

---

### Task 4: wire sshconn → virshcli in main

**Files:**
- Modify: `cmd/bombvault/main.go:57-58`

- [ ] **Step 1: Replace the LinkSocket + `virshcli.New()` block** with sshconn wiring:

```go
	// libvirt over SSH (qemu+ssh://) — no filesystem mount. Generate the key on
	// first run; the user authorizes the public key on the host (Settings shows it).
	sc := sshconn.New(cfg.LibvirtHost, cfg.LibvirtSSHUser, cfg.DataDir)
	if err := sc.EnsureKey(); err != nil {
		log.Printf("sshconn: ensure key: %v", err) // non-fatal: VM backup just stays unavailable
	}
	vc := virshcli.New(sc.VirshURI())
```

Add the `sshconn` import. Remove the old `virshcli.LinkSocket(...)` lines and any now-unused `cfg.HostRunRoot` reference (and remove `HostRunRoot` from config if nothing else uses it — grep first).

- [ ] **Step 2: Build:** `go build ./...` → passes.
- [ ] **Step 3: Test + lint:** `go test ./... && golangci-lint run ./...` → green/0.
- [ ] **Step 4: Commit:** `git add internal/virshcli cmd internal/config && git commit -m "feat(virshcli): connect via qemu+ssh URI; wire sshconn; drop the mount-era socket symlink"`

---

## Wave C — NVRAM over SSH

### Task 5: capture + restore NVRAM via SSH

The service already parses `domain.NVRAMPath` (host path). Instead of (un)reachable mounts, read/write it over SSH and ship it inside the restic snapshot via a staging dir under `DataDir`.

**Files:**
- Modify: `internal/api/service.go` (`BackupVM`, `RestoreVM`, add `sshconn` field + wiring)
- Modify: `cmd/bombvault/main.go` (pass `sc` into the service)
- Test: `internal/api/service_test.go`

- [ ] **Step 1:** Add an `ssh` dependency to `Service` (interface for testability):

```go
// HostSSH is the subset of sshconn used by the service (NVRAM transfer).
type HostSSH interface {
	ReadFile(ctx context.Context, path string) ([]byte, error)
	WriteFile(ctx context.Context, path string, data []byte) error
}
```

Add `ssh HostSSH` to `Service` and a setter/constructor arg; wire `sc` in main. A nil `ssh` means NVRAM transfer is skipped (graceful degradation).

- [ ] **Step 2: BackupVM — stage NVRAM.** After resolving `diskPaths`, when `domain.NVRAMPath != "" && s.ssh != nil`: read the bytes over SSH and write them to `stage := filepath.Join(s.cfg.DataDir, "vm-stage", name, "nvram.fd")`; add `stage` to the restic backup `paths`; store the host NVRAM path in the definition as `NVRAMHostPath`. On read error: log + skip (fall back to template regeneration), never fail the backup.

```go
nvramHostPath := domain.NVRAMPath
nvramStage := ""
if nvramHostPath != "" && s.ssh != nil {
	if b, rerr := s.ssh.ReadFile(ctx, nvramHostPath); rerr == nil {
		nvramStage = filepath.Join(s.cfg.DataDir, "vm-stage", name, "nvram.fd")
		if werr := os.MkdirAll(filepath.Dir(nvramStage), 0o700); werr == nil {
			if werr = os.WriteFile(nvramStage, b, 0o600); werr != nil {
				nvramStage = ""
			}
		} else {
			nvramStage = ""
		}
	} else {
		log.Printf("api: BackupVM: NVRAM read over SSH failed; UEFI restore will regenerate it: %v", rerr)
	}
}
```

Include `nvramStage` in the backup path list (when non-empty) and persist `NVRAMHostPath: nvramHostPath` + `NVRAMStage: nvramStage` in `vmDefinition`.

- [ ] **Step 3: RestoreVM — write NVRAM back.** After restic restores (the stage file lands at its origin path) and BEFORE `virsh define`: if `def.NVRAMStage` restored and `s.ssh != nil`, read the staged bytes and `s.ssh.WriteFile(ctx, def.NVRAMHostPath, bytes)`. On any failure: log + continue (EnsureNVRAMTemplate then regenerates). The disk-restore path list must include the stage path so restic brings it back.

- [ ] **Step 4: Tests** in `internal/api/service_test.go` with a fake `HostSSH` (records ReadFile/WriteFile): assert BackupVM reads the NVRAM host path and stages it; assert RestoreVM writes the staged bytes to the NVRAM host path before define. Use the existing fakeVirsh + fakes.

- [ ] **Step 5: Run + lint + build:** `go test ./internal/api/... && golangci-lint run ./... && go build ./...`
- [ ] **Step 6: Commit:** `git add internal/api cmd && git commit -m "feat(vm): capture+restore NVRAM over SSH (perfect UEFI restore; template fallback)"`

---

## Wave D — Live snapshot

### Task 6: virsh snapshot + blockcommit + guest-agent ping

**Files:**
- Modify: `internal/virshcli/types.go` (extend `Virsh`)
- Modify: `internal/virshcli/virshcli.go` (implement)
- Modify: `internal/backup/vm_orchestrator.go` (extend `VM`)
- Test: `internal/virshcli/virshcli_uri_test.go`

- [ ] **Step 1: Add to the `Virsh` interface** (`types.go`) and the `VM` interface (`vm_orchestrator.go`):

```go
	// SnapshotCreateDiskOnly creates an external, atomic, disk-only snapshot
	// (the VM keeps running and writes to a fresh overlay). quiesce uses the
	// qemu guest agent for app-consistency when available.
	SnapshotCreateDiskOnly(ctx context.Context, name, snapName string, quiesce bool) error
	// BlockCommitActivePivot commits the active overlay back into its base and
	// pivots the running VM onto the base (virsh blockcommit --active --pivot --wait).
	BlockCommitActivePivot(ctx context.Context, name, device string) error
	// GuestAgentPing reports whether the qemu guest agent answers in the VM.
	GuestAgentPing(ctx context.Context, name string) bool
```

- [ ] **Step 2: Implement in `virshcli.go`:**

```go
func (c *Client) SnapshotCreateDiskOnly(ctx context.Context, name, snapName string, quiesce bool) error {
	args := []string{"snapshot-create-as", "--domain", name, snapName,
		"--disk-only", "--atomic", "--no-metadata"}
	if quiesce {
		args = append(args, "--quiesce")
	}
	_, err := c.run(ctx, args...)
	return err
}

func (c *Client) BlockCommitActivePivot(ctx context.Context, name, device string) error {
	_, err := c.run(ctx, "blockcommit", name, device, "--active", "--pivot", "--wait")
	return err
}

func (c *Client) GuestAgentPing(ctx context.Context, name string) bool {
	_, err := c.run(ctx, "qemu-agent-command", name, `{"execute":"guest-ping"}`)
	return err == nil
}
```

- [ ] **Step 3:** Update fakes (`fakeVirsh` in api tests, `fakeVM` in backup tests) to satisfy the extended interfaces (record calls; configurable errors).
- [ ] **Step 4: Build + test:** `go build ./... && go test ./...`
- [ ] **Step 5: Commit:** `git add internal/virshcli internal/backup internal/api && git commit -m "feat(virshcli): disk-only snapshot, active blockcommit, guest-agent ping"`

---

### Task 7: `BackupVMLive` orchestrator (safety-critical)

**Files:**
- Modify: `internal/backup/vm_orchestrator.go`
- Test: `internal/backup/vm_orchestrator_test.go`

- [ ] **Step 1: Write failing tests** proving the safety guarantee:

```go
func TestBackupVMLiveHappyPath(t *testing.T) {
	d := newLiveDeps()
	_, err := BackupVMLive(t.Context(), d.deps())
	if err != nil { t.Fatalf("unexpected: %v", err) }
	// order: snapshot → restic backup → blockcommit → run success
	wantSeq(t, d.vm.log, "snapshot", "blockcommit")
	if !contains(d.restic.log, "backup:") { t.Fatal("base not backed up") }
	if d.runs.finishes[0] != "success" { t.Fatalf("runs=%v", d.runs.finishes) }
}

func TestBackupVMLiveCommitFailsLeavesVMRunning(t *testing.T) {
	d := newLiveDeps()
	d.vm.blockcommitErr = errors.New("commit boom")
	_, err := BackupVMLive(t.Context(), d.deps())
	if err == nil { t.Fatal("expected error") }
	// VM must NOT be destroyed/undefined; run recorded failed; clear message.
	if contains(d.vm.log, "destroy") || contains(d.vm.log, "undefine") {
		t.Fatalf("must never tear down the VM on commit failure: %v", d.vm.log)
	}
	if d.runs.finishes[0] != "failed" { t.Fatalf("runs=%v", d.runs.finishes) }
	if !strings.Contains(err.Error(), "still running") {
		t.Fatalf("error must reassure the VM is usable: %v", err)
	}
}
```

- [ ] **Step 2: Run, expect FAIL** (`BackupVMLive` undefined): `go test ./internal/backup/...`

- [ ] **Step 3: Implement `BackupVMLive`:**

```go
// BackupVMLive backs up a RUNNING VM without shutting it down:
//   snapshot-create-as --disk-only --atomic (the VM writes to an overlay)
//   → restic backs up the now-static base disk(s) [+ nvram]
//   → blockcommit --active --pivot (merge overlay back, pivot the live VM)
// Safety: on ANY failure the VM is left RUNNING and usable — it is never
// destroyed or undefined. A commit failure surfaces a clear, actionable error
// (the overlay remains; the VM still runs on it) and records the run failed.
func BackupVMLive(ctx context.Context, d VMBackupDeps) (Summary, error) {
	runID, err := d.Runs.Start(d.TargetID, kindBackup)
	if err != nil {
		return Summary{}, fmt.Errorf("vm live backup: record run start: %w", err)
	}

	const snap = "bombvault-tmp"
	quiesce := d.VM.GuestAgentPing(ctx, d.Name)

	if err := d.VM.SnapshotCreateDiskOnly(ctx, d.Name, snap, quiesce); err != nil {
		e := fmt.Errorf("vm live backup: snapshot: %w", err)
		_ = d.Runs.Finish(runID, statusFailed, "", 0, truncateErr(e))
		return Summary{}, e // nothing changed; VM untouched
	}

	paths := append([]string(nil), d.DiskPaths...)
	if d.NVRAMPath != "" {
		paths = append(paths, d.NVRAMPath)
	}
	tags := []string{"vm:" + d.Name, "p2", "live"}
	summary, backupErr := d.Restic.Backup(ctx, d.RepoPath, paths, tags)

	// ALWAYS attempt to commit the overlay back, even if the backup failed, so the
	// VM does not keep diverging on the overlay.
	if commitErr := d.VM.BlockCommitActivePivot(ctx, d.Name, d.DiskDevice); commitErr != nil {
		e := fmt.Errorf("vm live backup: blockcommit failed — the VM is STILL RUNNING on its snapshot overlay (no data lost); resolve the overlay before the next backup: %w", commitErr)
		_ = d.Runs.Finish(runID, statusFailed, "", 0, truncateErr(e))
		return Summary{}, e
	}
	if backupErr != nil {
		e := fmt.Errorf("vm live backup: restic: %w", backupErr)
		_ = d.Runs.Finish(runID, statusFailed, "", 0, truncateErr(e))
		return Summary{}, e
	}
	if err := d.Runs.Finish(runID, statusSuccess, summary.SnapshotID, summary.Bytes, ""); err != nil {
		return summary, fmt.Errorf("vm live backup: record run finish: %w", err)
	}
	return summary, nil
}
```

Add `DiskDevice string` to `VMBackupDeps` (the target device for blockcommit, e.g. `vda`/`hdc`; parsed from the domain XML — extend `ParseDomain`/`DomainInfo` to return the first disk's `target dev` in Task 6 or here).

- [ ] **Step 4: Run, expect PASS:** `go test ./internal/backup/...`
- [ ] **Step 5: Lint:** `golangci-lint run ./internal/backup/...`
- [ ] **Step 6: Commit:** `git add internal/backup && git commit -m "feat(vm): live-snapshot backup (snapshot→restic→blockcommit) with always-running safety guarantee"`

---

### Task 8: service dispatches graceful vs live

**Files:**
- Modify: `internal/api/service.go` (`BackupVM`)
- Test: `internal/api/service_test.go`

- [ ] **Step 1: Write the failing test:** a VM whose stored `Method == "live"` routes to `BackupVMLive` (assert the snapshot call happened, no shutdown); `Method == "graceful"` routes to `BackupVMGraceful` (assert shutdown happened, no snapshot). Use the fake virsh call log.

- [ ] **Step 2: Implement the dispatch** in `BackupVM`: read the method (`existing.Method`, default graceful), and call `backup.BackupVMLive` when `method == "live"`, else `backup.BackupVMGraceful`. Pass `DiskDevice` (first disk target from the parsed domain) into the deps.

- [ ] **Step 3: Run + lint + build.**
- [ ] **Step 4: Commit:** `git add internal/api && git commit -m "feat(vm): route backup to graceful or live per the VM's stored method"`

---

## Wave E — UI + template

### Task 9: SSH setup UI + per-VM method

**Files:**
- Modify: `internal/api/handlers.go`, `internal/api/api.go` (routes)
- Modify: `web/src/lib/api.ts`, `web/src/pages/Settings.tsx`, `web/src/pages/VMs.tsx`, `web/src/lib/i18n.ts`

- [ ] **Step 1: API — `GET /api/vm/ssh`** returns `{host, publicKey}` (from `sshconn.PublicKey()` + `cfg.LibvirtHost`); **`POST /api/vm/ssh/test`** runs `sshconn.Test` and returns `{ok, error?}`. Add `Test(ctx)` to the service via the `sshconn` (extend `HostSSH` or add a separate `Test`/`PublicKey` seam). Per-VM method already has `store.SetVMMethod`; expose `POST /api/vms/method {name, method}` if not present.

- [ ] **Step 2: Settings.tsx — "VM Backup (SSH)" card:** show the host field (read-only display of `LIBVIRT_HOST`), the public key in a `<code>` block with a copy button + the one-line instruction *"Append to Unraid `/root/.ssh/authorized_keys`"*, and a **Test connection** button calling `POST /api/vm/ssh/test` (green ✓ / red message). All strings via `t(...)` (add `vm.ssh.*` keys to `en` + `de` in `i18n.ts`; the 24 locale files inherit/fall back — the build's key-parity type only requires en's keys to exist, so add the keys to every locale or keep them en/de and let others fall back; simplest: add to `en` only is a type error, so add to `en` + `de` and add the same keys to the 24 locale files with the English value via a quick script, OR temporarily widen — pick: add to en+de and run the build; if it errors, add the keys to all locales).

- [ ] **Step 3: VMs.tsx — per-VM method dropdown** (Graceful / Live) calling `POST /api/vms/method`; default Graceful; tooltip noting Live needs the qemu guest agent + disks on `/mnt/cache`.

- [ ] **Step 4: Build the web:** `npm --prefix web run build` → passes (key parity enforced).
- [ ] **Step 5: Commit:** `git add internal/api web && git commit -m "feat(ui): VM-backup SSH setup card (pubkey + test) and per-VM graceful/live selector"`

---

### Task 10: template + Dockerfile delivery + README

**Files:**
- Modify: `templates/my-BombVault.xml`
- Modify: `README.md`

- [ ] **Step 1:** In `templates/my-BombVault.xml`, set `ExtraParams` to `--restart unless-stopped --add-host=host.docker.internal:host-gateway`; add advanced, optional vars `LIBVIRT_HOST` (default `host.docker.internal`) and `LIBVIRT_SSH_USER` (default `root`). No libvirt mount.
- [ ] **Step 2:** README — replace the "VM backup experimental opt-in (mount)" note with the SSH setup: enable in Settings → copy the public key → add to Unraid `/root/.ssh/authorized_keys` → Test connection. Keep the recovery note (GUI/reboot, never manual mount). Note Live needs qemu-guest-agent + disks on `/mnt/cache`.
- [ ] **Step 3:** Validate XML well-formed; **deliver** a copy to `d:\nextcloud\it\github\my-BombVault.xml`.
- [ ] **Step 4: Commit:** `git add templates README.md && git commit -m "feat(template): SSH VM backup (add-host host-gateway + LIBVIRT_HOST/USER); README setup"`

---

## Wave F — review + box-gate

### Task 11: review gates + box-gate

- [ ] **Step 1:** `/code-review` on the full diff (`main...HEAD`); fix findings.
- [ ] **Step 2:** `/security-review` (focus: SSH key handling, argv safety, host-key pinning, NVRAM path handling, the live-snapshot always-running guarantee).
- [ ] **Step 3:** Push; confirm CI green (build multi-arch + lint).
- [ ] **Step 4: Box-gate (user, real Unraid host):**
  1. Update `:latest` + import the new template (no libvirt mount; `--add-host` present).
  2. Settings → VM Backup → copy the public key → on Unraid: append to `/root/.ssh/authorized_keys` (enable SSH if needed) → **Test connection** = green.
  3. **Graceful:** a VM with method=graceful → back up → delete → restore → boots (UEFI keeps boot entries via NVRAM round-trip).
  4. **Live:** a running VM with method=live + qemu-guest-agent, disk on `/mnt/cache` → back up while running → restore → boots.
  5. **Crucially:** toggling the Unraid VM Manager has ZERO effect on BombVault (no mount) — confirm the host never breaks.

---

## Self-Review

**Spec coverage:** SSH transport (Tasks 1–4) ✓; NVRAM over SSH (Task 5) ✓; graceful (existing, transport-swapped via Task 4) ✓; live snapshot + safety (Tasks 6–8) ✓; per-VM method (Tasks 8–9) ✓; Settings UI + test button (Task 9) ✓; template/Dockerfile/README (Tasks 1, 10) ✓; security + box-gate (Task 11) ✓.

**Placeholders:** none — each step has concrete code/commands. The one judgement call (i18n key parity in Task 9, Step 2) is spelled out with the fallback.

**Type consistency:** `Conn` (sshconn) methods `EnsureKey/PublicKey/VirshURI/Run/ReadFile/WriteFile/Test`; `virshcli.New(uri string)` + `baseArgs`; `Virsh`/`VM` gain `SnapshotCreateDiskOnly/BlockCommitActivePivot/GuestAgentPing`; `VMBackupDeps.DiskDevice` used by `BackupVMLive`; `HostSSH` service seam (`ReadFile/WriteFile`); `vmDefinition` gains `NVRAMHostPath`/`NVRAMStage`. Consistent across tasks.

**Open follow-ups (out of scope, noted):** SSH ControlMaster multiplexing (speed), per-snapshot vs latest NVRAM edge cases, off-site/retention.
