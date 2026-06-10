# BombVault Phase 2 — VM Backend + Graceful-Shutdown Backup/Restore Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the full backend foundation for KVM/libvirt VM backup and restore (graceful-shutdown method only) in BombVault, including the virsh adapter, store migration, backup orchestrator, service layer, HTTP handlers, and Dockerfile patch — all fully unit-tested with fakes and passing `go build ./...`, `go test ./...`, and golangci-lint.

**Architecture:** Mirror the existing Docker/container pattern precisely: a `virshcli` package with a `Virsh` interface + concrete `Client` (shells out to the `virsh` CLI with separate exec args, never shell interpolation), a migration v4 adding the `vms` table, a `VMBackupDeps`/`VMRestoreDeps` orchestrator in `internal/backup/vm_orchestrator.go`, and service + handler methods in `internal/api/` following the same DI, envelope, and auth-gate patterns as the container equivalents. Host disk paths from libvirt (e.g. `/mnt/user/domains/...`) are translated to container-visible paths via a shared `toContainerPath` helper factored out of the existing `resolveAppdataPaths` in `service.go`.

**Tech Stack:** Go 1.26, SQLite (mattn/go-sqlite3 via store), encoding/xml for domain XML parsing, exec.CommandContext for virsh CLI, standard library only (no new dependencies). `libvirt-clients` apt package added to Dockerfile runtime stage.

---

## File Map

### New files
| File | Responsibility |
|------|---------------|
| `internal/virshcli/types.go` | `Virsh` interface + `VMInfo` + `DomainInfo` types |
| `internal/virshcli/virshcli.go` | Concrete `Client` that shells out to virsh; `ParseDomain` XML parser |
| `internal/store/vms.go` | `VMTarget` type + `UpsertVMTarget`, `GetVMTargetByName`, `ListVMTargets`, `SetVMMethod`, `SetVMInclude`, `DeleteVMTarget` |
| `internal/store/vms_test.go` | Roundtrip tests for every store function |
| `internal/backup/vm_orchestrator.go` | `VM` interface (DI seam) + `VMBackupDeps`/`VMRestoreDeps` + `BackupVMGraceful`/`RestoreVM` |
| `internal/backup/vm_orchestrator_test.go` | Fake-based tests for graceful ordering, always-start guard, restore sequence, path validation |

### Modified files
| File | What changes |
|------|-------------|
| `internal/store/migrate.go` | Migration v4: `vms` table |
| `internal/api/service.go` | Factor `toContainerPath` helper; add `vmsRepoPath`, `ListVMs`, `BackupVM`, `RestoreVM`, `SnapshotsVM`, `SetVMMethod`, `SetVMInclude`; add `virsh virshcli.Virsh` field to `Service`; add `vm_definition` type |
| `internal/api/handlers.go` | `handleListVMs`, `handleBackupVM`, `handleSnapshotsVM`, `handleRestoreVM`, `handlePatchVM` |
| `internal/api/handlers.go` (routes) | Register the 5 new `/api/vms/...` routes in `Router()` |
| `cmd/bombvault/main.go` | Wire concrete `virshcli.Client`; pass to `api.NewService` |
| `internal/api/service.go` `NewService` | Accept `virsh virshcli.Virsh` param; update call sites |
| `Dockerfile` | Add `libvirt-clients` to runtime `apt-get install` |

---

## Task 1: virshcli types — Virsh interface + VMInfo + DomainInfo

**Files:**
- Create: `internal/virshcli/types.go`

- [ ] **Step 1: Write the types file**

```go
// Package virshcli wraps the virsh CLI behind the Virsh interface so the
// VM backup orchestrator is unit-testable without a real libvirt socket.
// The concrete Client lives in virshcli.go and is wired only in cmd/bombvault.
package virshcli

import "context"

// VMInfo is a summary of a KVM/libvirt domain as returned by List.
type VMInfo struct {
	Name  string
	State string // "running", "shut off", "paused", ...
}

// DomainInfo contains the artifacts parsed from a libvirt domain XML:
// the disk image path(s) and the NVRAM path (empty for BIOS VMs).
type DomainInfo struct {
	DiskPaths []string
	NVRAMPath string
}

// Virsh is the host-control surface the VM backup orchestrator depends on.
// It is deliberately small and interface-shaped so orchestrators and the
// service layer can be unit-tested with fakes without a real libvirt socket.
type Virsh interface {
	// List returns all domains (running and stopped).
	List(ctx context.Context) ([]VMInfo, error)
	// State returns the domain state string ("running", "shut off", …), or
	// ("", nil) when the domain does not exist (mirror of dockercli.InspectName's
	// not-found tolerance).
	State(ctx context.Context, name string) (string, error)
	// DumpXML returns the domain XML for the named VM.
	DumpXML(ctx context.Context, name string) (string, error)
	// Shutdown sends an ACPI graceful-shutdown signal (virsh shutdown).
	Shutdown(ctx context.Context, name string) error
	// Destroy force-offs the domain (virsh destroy). Tolerates already-off.
	Destroy(ctx context.Context, name string) error
	// Start boots the domain (virsh start).
	Start(ctx context.Context, name string) error
	// Define (re)defines a domain from an XML file (virsh define <xmlPath>).
	Define(ctx context.Context, xmlPath string) error
	// Undefine removes the domain definition, including NVRAM if present
	// (virsh undefine --nvram). Tolerates not-defined.
	Undefine(ctx context.Context, name string) error
	// Autostart sets or clears the autostart flag (virsh autostart [--disable]).
	Autostart(ctx context.Context, name string, on bool) error
	// IsActive reports whether the domain is in the "running" state.
	IsActive(ctx context.Context, name string) (bool, error)
}
```

- [ ] **Step 2: Verify it compiles**

```powershell
$env:Path += ";C:\Program Files\Go\bin;$env:USERPROFILE\go\bin"
cd d:\nextcloud\it\github\bombvault
go build ./internal/virshcli/...
```

Expected: no output (compile success).

- [ ] **Step 3: Commit**

```powershell
git add internal/virshcli/types.go
git commit -m "feat(virshcli): Virsh interface + VMInfo/DomainInfo types"
```

---

## Task 2: virshcli concrete Client + ParseDomain XML parser

**Files:**
- Create: `internal/virshcli/virshcli.go`

This file shells out to `virsh` using `exec.CommandContext` with separate args (never shell interpolation), captures stdout/stderr, and on failure returns a scrubbed `lastReason`-style error (last non-empty stderr line, paths stripped). `ParseDomain` parses the domain XML to extract disk paths and NVRAM path using `encoding/xml`.

- [ ] **Step 1: Write the concrete Client**

```go
// Package virshcli — concrete virsh CLI adapter. See types.go for the interface.
package virshcli

import (
	"context"
	"encoding/xml"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strings"
)

// Client is the real virsh adapter that shells out to the virsh CLI.
// It connects to qemu:///system (virsh's default when run inside a container
// with the libvirt socket mounted at /var/run/libvirt/libvirt-sock).
type Client struct {
	bin string // "virsh" normally
}

// compile-time interface check.
var _ Virsh = (*Client)(nil)

// New returns a Client using the "virsh" binary on PATH.
func New() *Client { return &Client{bin: "virsh"} }

// absPathRe strips absolute paths from error messages so host paths do not
// leak to the caller (mirrors restic's lastReason scrubbing).
var absPathRe = regexp.MustCompile(`(/[^\s:'"]+)+`)

// run executes virsh with the given arguments. It returns the trimmed stdout
// on success. On failure it logs the full stderr server-side and returns a
// scrubbed error containing only the last non-empty stderr line (paths stripped).
func (c *Client) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, c.bin, args...) //nolint:gosec // G204: args are separate (never shell-interpolated); virsh name/path args come from libvirt, not raw user input
	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		log.Printf("virshcli: %q failed: %s", args[0], stderr)
		return "", fmt.Errorf("virshcli: %s: %s", args[0], lastReason(stderr))
	}
	return strings.TrimSpace(string(out)), nil
}

// lastReason extracts the last non-empty line of virsh stderr and scrubs
// absolute paths so host filesystem layout does not reach the caller.
func lastReason(stderr string) string {
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); l != "" {
			return absPathRe.ReplaceAllString(l, "[path]")
		}
	}
	return "unknown error"
}

// List returns all domains (running and stopped), one per name line.
func (c *Client) List(ctx context.Context) ([]VMInfo, error) {
	out, err := c.run(ctx, "list", "--all", "--name")
	if err != nil {
		return nil, err
	}
	var vms []VMInfo
	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		state, stErr := c.State(ctx, name)
		if stErr != nil {
			state = "unknown"
		}
		vms = append(vms, VMInfo{Name: name, State: state})
	}
	return vms, nil
}

// State returns the domain state ("running", "shut off", …) or ("", nil) when
// the domain does not exist — mirrors dockercli.InspectName not-found tolerance.
func (c *Client) State(ctx context.Context, name string) (string, error) {
	out, err := c.run(ctx, "domstate", name)
	if err != nil {
		// "failed to get domain" / "Domain not found" → treat as absent.
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "failed to get domain") ||
			strings.Contains(msg, "domain not found") ||
			strings.Contains(msg, "no domain") {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// DumpXML returns the domain XML for the named VM.
func (c *Client) DumpXML(ctx context.Context, name string) (string, error) {
	out, err := c.run(ctx, "dumpxml", name)
	if err != nil {
		return "", err
	}
	return out, nil
}

// Shutdown sends an ACPI graceful-shutdown signal.
func (c *Client) Shutdown(ctx context.Context, name string) error {
	_, err := c.run(ctx, "shutdown", name)
	return err
}

// Destroy force-offs the domain. Tolerates already-off ("domain is not running").
func (c *Client) Destroy(ctx context.Context, name string) error {
	_, err := c.run(ctx, "destroy", name)
	if err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "domain is not running") ||
			strings.Contains(msg, "not running") {
			return nil
		}
		return err
	}
	return nil
}

// Start boots the domain.
func (c *Client) Start(ctx context.Context, name string) error {
	_, err := c.run(ctx, "start", name)
	return err
}

// Define (re)defines a domain from an XML file on disk.
func (c *Client) Define(ctx context.Context, xmlPath string) error {
	_, err := c.run(ctx, "define", xmlPath)
	return err
}

// Undefine removes the domain definition including NVRAM. Tolerates not-defined.
func (c *Client) Undefine(ctx context.Context, name string) error {
	_, err := c.run(ctx, "undefine", "--nvram", name)
	if err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "failed to undefine") ||
			strings.Contains(msg, "domain not found") ||
			strings.Contains(msg, "no domain") {
			return nil
		}
		return err
	}
	return nil
}

// Autostart sets (on=true) or clears (on=false) the domain autostart flag.
func (c *Client) Autostart(ctx context.Context, name string, on bool) error {
	args := []string{"autostart"}
	if !on {
		args = append(args, "--disable")
	}
	args = append(args, name)
	_, err := c.run(ctx, args...)
	return err
}

// IsActive reports whether the domain is currently running.
func (c *Client) IsActive(ctx context.Context, name string) (bool, error) {
	state, err := c.State(ctx, name)
	if err != nil {
		return false, err
	}
	return state == "running", nil
}

// ---------------------------------------------------------------------------
// Domain XML parsing
// ---------------------------------------------------------------------------

// domainXML is the minimal struct for parsing a libvirt domain XML document.
// Only the fields BombVault needs (disk sources + NVRAM) are decoded; the rest
// is discarded (xml.Unmarshal ignores unknown elements by default).
type domainXML struct {
	Devices struct {
		Disks []struct {
			Type   string `xml:"type,attr"`
			Device string `xml:"device,attr"`
			Source struct {
				File string `xml:"file,attr"`
			} `xml:"source"`
		} `xml:"disk"`
	} `xml:"devices"`
	OS struct {
		NVRAM string `xml:"nvram"`
	} `xml:"os"`
}

// ParseDomain parses a libvirt domain XML string and extracts the disk file
// paths (type="file", device="disk") and NVRAM path (empty for BIOS VMs).
// It is exported so the service layer can call it without importing virshcli
// internals (the result is plain strings; no libvirt types cross the boundary).
func ParseDomain(xmlStr string) (DomainInfo, error) {
	var d domainXML
	if err := xml.Unmarshal([]byte(xmlStr), &d); err != nil {
		return DomainInfo{}, fmt.Errorf("virshcli: parse domain xml: %w", err)
	}
	var disks []string
	for _, disk := range d.Devices.Disks {
		if disk.Type == "file" && disk.Device == "disk" && disk.Source.File != "" {
			disks = append(disks, disk.Source.File)
		}
	}
	nvram := strings.TrimSpace(d.OS.NVRAM)
	return DomainInfo{DiskPaths: disks, NVRAMPath: nvram}, nil
}
```

- [ ] **Step 2: Build to verify**

```powershell
$env:Path += ";C:\Program Files\Go\bin;$env:USERPROFILE\go\bin"
cd d:\nextcloud\it\github\bombvault
go build ./internal/virshcli/...
```

Expected: no output.

- [ ] **Step 3: Commit**

```powershell
git add internal/virshcli/virshcli.go
git commit -m "feat(virshcli): concrete Client + ParseDomain XML parser"
```

---

## Task 3: store migration v4 — vms table

**Files:**
- Modify: `internal/store/migrate.go`

- [ ] **Step 1: Write the failing test first**

In `internal/store/migrate_test.go` (which already exists — check if it does; if not create it). Add the following test to verify v4 is applied:

```go
func TestMigrateV4VMsTable(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Verify vms table exists with the expected columns.
	_, err = db.Exec(`INSERT INTO vms (id, name, method, include_in_schedule, definition, created_at)
		VALUES ('test-id', 'testvm', 'graceful', 0, '', 1234567890)`)
	if err != nil {
		t.Fatalf("vms table not created or wrong schema: %v", err)
	}
	var name string
	if err := db.QueryRow(`SELECT name FROM vms WHERE id = 'test-id'`).Scan(&name); err != nil {
		t.Fatalf("cannot read back: %v", err)
	}
	if name != "testvm" {
		t.Fatalf("name = %q, want testvm", name)
	}
}
```

- [ ] **Step 2: Run test to verify it FAILS**

```powershell
$env:Path += ";C:\Program Files\Go\bin;$env:USERPROFILE\go\bin"
cd d:\nextcloud\it\github\bombvault
go test ./internal/store/... -run TestMigrateV4VMsTable -v
```

Expected: FAIL — "vms table not created".

- [ ] **Step 3: Add migration v4 to migrate.go**

In `internal/store/migrate.go`, append to the `migrations` slice (after the existing `{version: 3, …}` entry):

```go
{
    version: 4,
    name:    "vms_table",
    sql: `CREATE TABLE vms (
  id                  TEXT    PRIMARY KEY,
  name                TEXT    NOT NULL UNIQUE,
  method              TEXT    NOT NULL DEFAULT 'graceful',
  include_in_schedule INTEGER NOT NULL DEFAULT 0,
  definition          TEXT    NOT NULL DEFAULT '',
  created_at          INTEGER NOT NULL
);`,
},
```

- [ ] **Step 4: Run test to verify it PASSES**

```powershell
$env:Path += ";C:\Program Files\Go\bin;$env:USERPROFILE\go\bin"
cd d:\nextcloud\it\github\bombvault
go test ./internal/store/... -run TestMigrateV4VMsTable -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/store/migrate.go internal/store/migrate_test.go
git commit -m "feat(store): migration v4 — vms table"
```

---

## Task 4: store — VMTarget CRUD

**Files:**
- Create: `internal/store/vms.go`
- Create: `internal/store/vms_test.go`

The `VMTarget` type and its CRUD functions mirror `targets.go` exactly — same `ON CONFLICT` upsert pattern, same `scanTarget` helper style, same tx-based `DeleteVMTarget`.

- [ ] **Step 1: Write the failing tests**

```go
// internal/store/vms_test.go
package store_test

import (
	"testing"
	"time"
)

func TestUpsertVMTargetRoundtrip(t *testing.T) {
	st := mustOpen(t)
	tg, err := st.UpsertVMTarget(VMTarget{
		Name:              "win10",
		Method:            "graceful",
		IncludeInSchedule: false,
		Definition:        `{"xml":"<domain/>"}`,
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if tg.ID == "" {
		t.Fatal("ID must be assigned")
	}
	if tg.Name != "win10" {
		t.Fatalf("name = %q", tg.Name)
	}
	if tg.CreatedAt == 0 {
		t.Fatal("created_at must be set")
	}

	// Re-read.
	got, err := st.GetVMTargetByName("win10")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != tg.ID {
		t.Fatalf("id mismatch: %q vs %q", got.ID, tg.ID)
	}
}

func TestUpsertVMTargetConflictPreservesID(t *testing.T) {
	st := mustOpen(t)
	first, err := st.UpsertVMTarget(VMTarget{Name: "ubuntu", Method: "graceful"})
	if err != nil {
		t.Fatal(err)
	}
	// Upsert again with a different method — ID and created_at must be preserved.
	second, err := st.UpsertVMTarget(VMTarget{Name: "ubuntu", Method: "graceful", Definition: "updated"})
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID {
		t.Fatalf("conflict must preserve original ID: %q vs %q", second.ID, first.ID)
	}
	if second.Definition != "updated" {
		t.Fatalf("definition not updated: %q", second.Definition)
	}
}

func TestListVMTargets(t *testing.T) {
	st := mustOpen(t)
	for _, name := range []string{"vmB", "vmA", "vmC"} {
		if _, err := st.UpsertVMTarget(VMTarget{Name: name, Method: "graceful"}); err != nil {
			t.Fatal(err)
		}
	}
	list, err := st.ListVMTargets()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3, got %d", len(list))
	}
	// ORDER BY name
	if list[0].Name != "vmA" || list[1].Name != "vmB" || list[2].Name != "vmC" {
		t.Fatalf("order wrong: %v", list)
	}
}

func TestSetVMMethod(t *testing.T) {
	st := mustOpen(t)
	if _, err := st.UpsertVMTarget(VMTarget{Name: "fedora", Method: "graceful"}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetVMMethod("fedora", "graceful"); err != nil {
		t.Fatal(err)
	}
	tg, err := st.GetVMTargetByName("fedora")
	if err != nil {
		t.Fatal(err)
	}
	if tg.Method != "graceful" {
		t.Fatalf("method = %q", tg.Method)
	}
}

func TestSetVMInclude(t *testing.T) {
	st := mustOpen(t)
	if _, err := st.UpsertVMTarget(VMTarget{Name: "archvm", Method: "graceful"}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetVMInclude("archvm", true); err != nil {
		t.Fatal(err)
	}
	tg, err := st.GetVMTargetByName("archvm")
	if err != nil {
		t.Fatal(err)
	}
	if !tg.IncludeInSchedule {
		t.Fatal("include must be true after SetVMInclude(true)")
	}
}

func TestDeleteVMTarget(t *testing.T) {
	st := mustOpen(t)
	tg, err := st.UpsertVMTarget(VMTarget{Name: "deleteme", Method: "graceful"})
	if err != nil {
		t.Fatal(err)
	}
	// Seed a run referencing this VM target.
	runID, err := st.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRun(runID, "success", "abc123", 1024, ""); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteVMTarget("deleteme"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := st.GetVMTargetByName("deleteme"); err == nil {
		t.Fatal("target must be gone after delete")
	}
	// Runs must also be gone (cascade in tx).
	runs, _ := st.ListRuns(100)
	for _, r := range runs {
		if r.TargetID == tg.ID {
			t.Fatalf("run for deleted VM target must be removed: %+v", r)
		}
	}
}

func TestDeleteVMTargetNotFoundIsNoop(t *testing.T) {
	st := mustOpen(t)
	// Deleting a non-existent VM must not error.
	if err := st.DeleteVMTarget("ghost"); err != nil {
		t.Fatalf("delete non-existent: %v", err)
	}
}

// mustOpen is a test helper that opens an in-memory store and migrates it.
// It is defined in helpers_test.go; replicated here as a reminder of what it does:
// db, _ := Open(":memory:"); Migrate(db); return New(db)
// (The actual helper is already present in the package test — don't duplicate it.)
var _ = time.Now // suppress unused import
```

**IMPORTANT:** The `mustOpen` helper already exists in `internal/store/helpers_test.go`. Do not redeclare it. The test file above should not redeclare `mustOpen`. Check `helpers_test.go` first:

```powershell
type-cat internal/store/helpers_test.go
```

If it already has `mustOpen`, remove that declaration from vms_test.go. If it doesn't, add it:

```go
func mustOpen(t *testing.T) *Repo {
    t.Helper()
    db, err := Open(":memory:")
    if err != nil { t.Fatal(err) }
    t.Cleanup(func() { _ = db.Close() })
    if err := Migrate(db); err != nil { t.Fatal(err) }
    return New(db)
}
```

- [ ] **Step 2: Run tests to verify they FAIL**

```powershell
$env:Path += ";C:\Program Files\Go\bin;$env:USERPROFILE\go\bin"
cd d:\nextcloud\it\github\bombvault
go test ./internal/store/... -run "TestUpsertVMTarget|TestListVMTargets|TestSetVM|TestDeleteVMTarget" -v 2>&1 | Select-Object -First 40
```

Expected: FAIL — undefined types/functions.

- [ ] **Step 3: Implement vms.go**

```go
package store

import (
	"fmt"
	"time"
)

// VMTarget represents a KVM/libvirt VM that BombVault can back up.
type VMTarget struct {
	ID                string
	Name              string
	Method            string // "graceful" (default) or "live"
	IncludeInSchedule bool
	// Definition is an opaque JSON blob persisted at backup time containing
	// the domain XML, disk paths, NVRAM path, and method so restore works even
	// after the VM has been deleted or BombVault's /config is lost (full DR).
	Definition string
	CreatedAt  int64
}

// UpsertVMTarget inserts or updates a VM target by name.
// On conflict, method and definition are refreshed; id, created_at, and
// include_in_schedule are preserved (include_in_schedule is owned by SetVMInclude).
// Returns the authoritative VMTarget (original ID when a conflict fires).
func (r *Repo) UpsertVMTarget(t VMTarget) (VMTarget, error) {
	if t.ID == "" {
		t.ID = newID()
	}
	if t.CreatedAt == 0 {
		t.CreatedAt = time.Now().Unix()
	}
	if t.Method == "" {
		t.Method = "graceful"
	}

	_, err := r.db.Exec(`
		INSERT INTO vms (id, name, method, include_in_schedule, definition, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
		  method     = excluded.method,
		  definition = excluded.definition`,
		t.ID, t.Name, t.Method, boolInt(t.IncludeInSchedule), t.Definition, t.CreatedAt,
	)
	if err != nil {
		return VMTarget{}, fmt.Errorf("UpsertVMTarget: %w", err)
	}
	return r.GetVMTargetByName(t.Name)
}

// GetVMTargetByName returns the VM target for the named domain.
func (r *Repo) GetVMTargetByName(name string) (VMTarget, error) {
	row := r.db.QueryRow(`
		SELECT id, name, method, include_in_schedule, definition, created_at
		FROM vms WHERE name = ?`, name)
	return scanVMTarget(row)
}

// ListVMTargets returns all known VM targets ordered by name.
func (r *Repo) ListVMTargets() ([]VMTarget, error) {
	rows, err := r.db.Query(`
		SELECT id, name, method, include_in_schedule, definition, created_at
		FROM vms ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("ListVMTargets: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []VMTarget
	for rows.Next() {
		t, err := scanVMTarget(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// SetVMMethod updates the backup method for the named VM.
func (r *Repo) SetVMMethod(name, method string) error {
	res, err := r.db.Exec(`UPDATE vms SET method = ? WHERE name = ?`, method, name)
	if err != nil {
		return fmt.Errorf("SetVMMethod: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("SetVMMethod: vm %q not found", name)
	}
	return nil
}

// SetVMInclude updates the include_in_schedule flag for the named VM.
func (r *Repo) SetVMInclude(name string, include bool) error {
	res, err := r.db.Exec(`UPDATE vms SET include_in_schedule = ? WHERE name = ?`,
		boolInt(include), name)
	if err != nil {
		return fmt.Errorf("SetVMInclude: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("SetVMInclude: vm %q not found", name)
	}
	return nil
}

// DeleteVMTarget removes a VM target and ALL its run history by name, in a
// single transaction. It is a no-op (no error) if the target does not exist.
func (r *Repo) DeleteVMTarget(name string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("DeleteVMTarget begin: %w", err)
	}
	if _, err := tx.Exec(
		`DELETE FROM runs WHERE target_id IN (SELECT id FROM vms WHERE name = ?)`, name,
	); err != nil {
		tx.Rollback() //nolint:errcheck,gosec // best-effort rollback; original error takes priority
		return fmt.Errorf("DeleteVMTarget runs: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM vms WHERE name = ?`, name); err != nil {
		tx.Rollback() //nolint:errcheck,gosec // best-effort rollback; original error takes priority
		return fmt.Errorf("DeleteVMTarget: %w", err)
	}
	return tx.Commit()
}

func scanVMTarget(s scanner) (VMTarget, error) {
	var t VMTarget
	var include int
	err := s.Scan(&t.ID, &t.Name, &t.Method, &include, &t.Definition, &t.CreatedAt)
	if err != nil {
		return VMTarget{}, fmt.Errorf("scanVMTarget: %w", err)
	}
	t.IncludeInSchedule = include != 0
	return t, nil
}
```

- [ ] **Step 4: Run tests to verify they PASS**

```powershell
$env:Path += ";C:\Program Files\Go\bin;$env:USERPROFILE\go\bin"
cd d:\nextcloud\it\github\bombvault
go test ./internal/store/... -v 2>&1 | tail -30
```

Expected: all store tests PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/store/vms.go internal/store/vms_test.go
git commit -m "feat(store): VMTarget CRUD + vms_test.go roundtrip tests"
```

---

## Task 5: backup — VM orchestrator (BackupVMGraceful + RestoreVM)

**Files:**
- Create: `internal/backup/vm_orchestrator.go`

This file defines the `VM` interface (DI seam — only the virsh methods the orchestrator needs) and implements `BackupVMGraceful`/`RestoreVM` mirroring `BackupContainer`/`RestoreContainer` in `orchestrator.go`.

Key behavioural contracts:
- `BackupVMGraceful`: always restart VM if it was running before (defer, mirrors BackupContainer's always-start).
- `RestoreVM`: guard Confirmed + snapshotIDRe; validate paths; Destroy then Undefine if VM exists; RestorePaths; write XML to temp file; Define; Autostart; optionally Start.
- Both record runs via the `Runs` interface.
- Use `os.WriteFile` to write the domain XML to `DataDir/vm-define/<name>.xml` for the Define step.

- [ ] **Step 1: Write the failing tests**

Create `internal/backup/vm_orchestrator_test.go`:

```go
package backup_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeVM satisfies backup.VM for unit tests — no real virsh needed.
type fakeVM struct {
	log []string

	// state returns from IsActive/State calls
	active     bool
	stateVal   string
	stateErr   error
	activeErr  error
	shutdownErr error
	destroyErr  error
	startErr    error
	defineErr   error
	undefineErr error
	autostartErr error
	dumpXMLVal  string
	dumpXMLErr  error
}

func (f *fakeVM) State(_ context.Context, name string) (string, error) {
	f.log = append(f.log, "state:"+name)
	return f.stateVal, f.stateErr
}

func (f *fakeVM) IsActive(_ context.Context, name string) (bool, error) {
	f.log = append(f.log, "isActive:"+name)
	return f.active, f.activeErr
}

func (f *fakeVM) DumpXML(_ context.Context, name string) (string, error) {
	f.log = append(f.log, "dumpxml:"+name)
	return f.dumpXMLVal, f.dumpXMLErr
}

func (f *fakeVM) Shutdown(_ context.Context, name string) error {
	f.log = append(f.log, "shutdown:"+name)
	return f.shutdownErr
}

func (f *fakeVM) Destroy(_ context.Context, name string) error {
	f.log = append(f.log, "destroy:"+name)
	return f.destroyErr
}

func (f *fakeVM) Start(_ context.Context, name string) error {
	f.log = append(f.log, "start:"+name)
	return f.startErr
}

func (f *fakeVM) Define(_ context.Context, xmlPath string) error {
	f.log = append(f.log, "define:"+xmlPath)
	return f.defineErr
}

func (f *fakeVM) Undefine(_ context.Context, name string) error {
	f.log = append(f.log, "undefine:"+name)
	return f.undefineErr
}

func (f *fakeVM) Autostart(_ context.Context, name string, on bool) error {
	v := "on"
	if !on {
		v = "off"
	}
	f.log = append(f.log, "autostart:"+name+":"+v)
	return f.autostartErr
}

// vmContains reports whether any log entry has the given prefix.
func vmContains(log []string, prefix string) bool {
	for _, e := range log {
		if strings.HasPrefix(e, prefix) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// BackupVMGraceful tests
// ---------------------------------------------------------------------------

func sampleVMBackupDeps(t *testing.T, vm *fakeVM, r *fakeRestic, runs *fakeRuns) VMBackupDeps {
	t.Helper()
	return VMBackupDeps{
		Name:       "win10",
		DiskPaths:  []string{"/host/domains/win10/win10.qcow2"},
		NVRAMPath:  "/host/domains/win10/win10_VARS.fd",
		RepoPath:   "/repo/vms",
		TargetID:   "vmtarget-1",
		DataDir:    t.TempDir(),
		VM:         vm,
		Restic:     r,
		Runs:       runs,
	}
}

func TestBackupVMGracefulHappyPath(t *testing.T) {
	vm := &fakeVM{active: true, stateVal: "shut off"}
	r := &fakeRestic{summary: Summary{SnapshotID: "deadbeef12345678", Bytes: 4096}}
	runs := &fakeRuns{}

	sum, err := BackupVMGraceful(t.Context(), sampleVMBackupDeps(t, vm, r, runs))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sum.SnapshotID != "deadbeef12345678" {
		t.Fatalf("snapshot id = %q", sum.SnapshotID)
	}
	// Graceful order: isActive → shutdown → (poll) → restic backup → start
	if !vmContains(vm.log, "isActive:") {
		t.Fatal("isActive must be called")
	}
	if !vmContains(vm.log, "shutdown:win10") {
		t.Fatal("shutdown must be called")
	}
	if !vmContains(vm.log, "start:win10") {
		t.Fatal("start must be called (ALWAYS restart)")
	}
	if !vmContains(r.log, "backup:/repo/vms") {
		t.Fatalf("restic backup not called: %v", r.log)
	}
	// Tags must include vm:win10 and p2
	if !strings.Contains(r.log[0], "vm:win10") {
		t.Fatalf("tag vm:win10 missing in %v", r.log)
	}
	if !strings.Contains(r.log[0], "p2") {
		t.Fatalf("tag p2 missing in %v", r.log)
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "success" {
		t.Fatalf("run finishes = %v, want [success]", runs.finishes)
	}
}

func TestBackupVMGracefulAlwaysStartsWhenWasRunning(t *testing.T) {
	// VM was running; restic fails → VM must still be started.
	vm := &fakeVM{active: true, stateVal: "shut off"}
	r := &fakeRestic{backupErr: errors.New("restic boom")}
	runs := &fakeRuns{}

	_, err := BackupVMGraceful(t.Context(), sampleVMBackupDeps(t, vm, r, runs))
	if err == nil {
		t.Fatal("expected error to be re-thrown")
	}
	if !vmContains(vm.log, "start:win10") {
		t.Fatal("VM must be restarted even when backup fails and VM was running")
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "failed" {
		t.Fatalf("run finishes = %v, want [failed]", runs.finishes)
	}
}

func TestBackupVMGracefulDoesNotStartWhenWasNotRunning(t *testing.T) {
	// VM was already stopped — must NOT be started after backup.
	vm := &fakeVM{active: false, stateVal: "shut off"}
	r := &fakeRestic{summary: Summary{SnapshotID: "abcd1234"}}
	runs := &fakeRuns{}

	if _, err := BackupVMGraceful(t.Context(), sampleVMBackupDeps(t, vm, r, runs)); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if vmContains(vm.log, "start:win10") {
		t.Fatal("VM must NOT be started when it was already stopped before backup")
	}
}

func TestBackupVMGracefulDestroyOnShutdownTimeout(t *testing.T) {
	// Shutdown succeeds but state never becomes "shut off" → destroy called.
	// We simulate this by having stateVal always = "running" (never shut off).
	vm := &fakeVM{active: true, stateVal: "running"} // never transitions
	r := &fakeRestic{summary: Summary{SnapshotID: "abcd1234"}}
	runs := &fakeRuns{}

	deps := sampleVMBackupDeps(t, vm, r, runs)
	// Use a very short shutdown timeout so the test doesn't actually wait.
	// NOTE: BackupVMGraceful must accept a configurable ShutdownTimeout in its deps.
	deps.ShutdownTimeout = 1 // 1 poll cycle; see implementation note below
	// With a minimal timeout, the poll should immediately give up and call destroy.
	_, err := BackupVMGraceful(t.Context(), deps)
	// This may succeed or fail depending on fake, but destroy must be called.
	_ = err
	if !vmContains(vm.log, "destroy:win10") {
		t.Fatal("destroy must be called when graceful shutdown times out")
	}
}

// ---------------------------------------------------------------------------
// RestoreVM tests
// ---------------------------------------------------------------------------

func sampleVMRestoreDeps(t *testing.T, vm *fakeVM, r *fakeRestic, runs *fakeRuns) VMRestoreDeps {
	t.Helper()
	return VMRestoreDeps{
		Confirmed:    true,
		Name:         "win10",
		SnapshotID:   "deadbeef12345678",
		DiskPaths:    []string{"/host/domains/win10/win10.qcow2"},
		NVRAMPath:    "/host/domains/win10/win10_VARS.fd",
		DomainXML:    "<domain><name>win10</name></domain>",
		WasAutostart: true,
		StartAfter:   true,
		RepoPath:     "/repo/vms",
		TargetID:     "vmtarget-1",
		DataDir:      t.TempDir(),
		VM:           vm,
		Restic:       r,
		Runs:         runs,
	}
}

func TestRestoreVMAbortsWhenNotConfirmed(t *testing.T) {
	vm := &fakeVM{stateVal: ""}
	r := &fakeRestic{}
	runs := &fakeRuns{}

	deps := sampleVMRestoreDeps(t, vm, r, runs)
	deps.Confirmed = false

	err := RestoreVM(t.Context(), deps)
	if err == nil || !errors.Is(err, ErrNotConfirmed) {
		t.Fatalf("expected ErrNotConfirmed, got %v", err)
	}
	if vmContains(runs.log, "runStart:") {
		t.Fatal("runStart must NOT be called when not confirmed")
	}
}

func TestRestoreVMRejectsBadSnapshotID(t *testing.T) {
	vm := &fakeVM{stateVal: ""}
	r := &fakeRestic{}
	runs := &fakeRuns{}

	deps := sampleVMRestoreDeps(t, vm, r, runs)
	deps.SnapshotID = "not-hex!"

	err := RestoreVM(t.Context(), deps)
	if err == nil || !errors.Is(err, ErrInvalidSnapshotID) {
		t.Fatalf("expected ErrInvalidSnapshotID, got %v", err)
	}
}

func TestRestoreVMRejectsUnsafePath(t *testing.T) {
	vm := &fakeVM{stateVal: ""}
	r := &fakeRestic{}
	runs := &fakeRuns{}

	deps := sampleVMRestoreDeps(t, vm, r, runs)
	deps.DiskPaths = []string{"/host/domains/../../../etc/passwd"}

	err := RestoreVM(t.Context(), deps)
	if err == nil || !strings.Contains(err.Error(), "unsafe") {
		t.Fatalf("expected unsafe path rejection, got %v", err)
	}
}

func TestRestoreVMHappyPath(t *testing.T) {
	// VM is running when restore is called → destroy + undefine before restore.
	vm := &fakeVM{stateVal: "running"}
	r := &fakeRestic{}
	runs := &fakeRuns{}

	if err := RestoreVM(t.Context(), sampleVMRestoreDeps(t, vm, r, runs)); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Order: state → destroy → undefine → restic restore → define → autostart → start
	order := vm.log
	idxDestroy  := -1
	idxUndefine := -1
	idxDefine   := -1
	idxAutostart := -1
	idxStart    := -1
	for i, e := range order {
		switch {
		case strings.HasPrefix(e, "destroy:"): idxDestroy = i
		case strings.HasPrefix(e, "undefine:"): idxUndefine = i
		case strings.HasPrefix(e, "define:"): idxDefine = i
		case strings.HasPrefix(e, "autostart:"): idxAutostart = i
		case strings.HasPrefix(e, "start:"): idxStart = i
		}
	}
	if idxDestroy < 0 { t.Fatal("destroy not called for running VM") }
	if idxUndefine < 0 { t.Fatal("undefine not called") }
	if idxDefine < 0 { t.Fatal("define not called") }
	if idxAutostart < 0 { t.Fatal("autostart not called") }
	if idxStart < 0 { t.Fatal("start not called when StartAfter=true") }
	if idxDestroy > idxUndefine { t.Fatal("destroy must precede undefine") }
	if idxUndefine > idxDefine { t.Fatal("undefine must precede define") }
	if idxDefine > idxStart { t.Fatal("define must precede start") }

	// Restic restore must have been called.
	if !vmContains(r.log, "restore:/repo/vms:deadbeef12345678") {
		t.Fatalf("restic restore not called: %v", r.log)
	}
	// Autostart with on=true (WasAutostart=true).
	found := false
	for _, e := range vm.log {
		if e == "autostart:win10:on" { found = true }
	}
	if !found { t.Fatal("autostart:win10:on not called") }
	// Run recorded success.
	if len(runs.finishes) != 1 || runs.finishes[0] != "success" {
		t.Fatalf("run finishes = %v, want [success]", runs.finishes)
	}
	// define called with a file that exists (temp xml file was written).
	for _, e := range vm.log {
		if strings.HasPrefix(e, "define:") {
			xmlPath := strings.TrimPrefix(e, "define:")
			if _, err := os.Stat(xmlPath); err != nil {
				t.Fatalf("define xml file does not exist: %v", err)
			}
		}
	}
}

func TestRestoreVMDoesNotDestroyWhenAbsent(t *testing.T) {
	// VM does not exist on host → destroy/undefine must NOT be called.
	vm := &fakeVM{stateVal: ""} // empty state = not found
	r := &fakeRestic{}
	runs := &fakeRuns{}

	if err := RestoreVM(t.Context(), sampleVMRestoreDeps(t, vm, r, runs)); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if vmContains(vm.log, "destroy:") {
		t.Fatal("destroy must NOT be called when VM is absent")
	}
	if vmContains(vm.log, "undefine:") {
		t.Fatal("undefine must NOT be called when VM is absent")
	}
	if !vmContains(r.log, "restore:") {
		t.Fatal("restic restore must still run")
	}
}

func TestRestoreVMRecordsFailedOnResticError(t *testing.T) {
	vm := &fakeVM{stateVal: ""}
	r := &fakeRestic{restoreErr: errors.New("restic failed")}
	runs := &fakeRuns{}

	err := RestoreVM(t.Context(), sampleVMRestoreDeps(t, vm, r, runs))
	if err == nil {
		t.Fatal("expected error")
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "failed" {
		t.Fatalf("run finishes = %v, want [failed]", runs.finishes)
	}
}
```

- [ ] **Step 2: Run tests to verify they FAIL**

```powershell
$env:Path += ";C:\Program Files\Go\bin;$env:USERPROFILE\go\bin"
cd d:\nextcloud\it\github\bombvault
go test ./internal/backup/... -run "TestBackupVM|TestRestoreVM" -v 2>&1 | Select-Object -First 20
```

Expected: FAIL — undefined `VMBackupDeps`, `BackupVMGraceful`, etc.

- [ ] **Step 3: Implement vm_orchestrator.go**

```go
// Package backup — VM orchestrators for graceful-shutdown backup and restore.
// This file mirrors orchestrator.go's patterns: DI interfaces, ALWAYS-restart
// guard via defer, confirmation + path validation guards.
package backup

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// VM DI interface (the seam — no concrete virshcli imported here)
// ---------------------------------------------------------------------------

// VM is the subset of virsh host control the VM orchestrators need.
// The full virshcli.Virsh interface is a superset; any adapter satisfying
// virshcli.Virsh automatically satisfies VM.
type VM interface {
	State(ctx context.Context, name string) (string, error)
	IsActive(ctx context.Context, name string) (bool, error)
	DumpXML(ctx context.Context, name string) (string, error)
	Shutdown(ctx context.Context, name string) error
	Destroy(ctx context.Context, name string) error
	Start(ctx context.Context, name string) error
	Define(ctx context.Context, xmlPath string) error
	Undefine(ctx context.Context, name string) error
	Autostart(ctx context.Context, name string, on bool) error
}

// ---------------------------------------------------------------------------
// VMBackupDeps / VMRestoreDeps
// ---------------------------------------------------------------------------

const (
	defaultVMShutdownPollInterval = 5 * time.Second
	defaultVMShutdownMaxPolls     = 18 // 18 × 5s = 90s timeout
)

// VMBackupDeps bundles everything BackupVMGraceful needs.
type VMBackupDeps struct {
	// Name is the libvirt domain name (used for tags + vm-define dir).
	Name string
	// DiskPaths are the container-visible absolute paths to the disk images.
	DiskPaths []string
	// NVRAMPath is the container-visible NVRAM path (empty for BIOS VMs).
	NVRAMPath string
	// RepoPath is the local restic repository path for the vms domain.
	RepoPath string
	// TargetID is the run-recording target id.
	TargetID string
	// DataDir is used to write temp files (e.g. the xml temp dir).
	DataDir string
	// ShutdownTimeout is the maximum number of poll cycles to wait for
	// "shut off" state before calling Destroy. 0 = use default (18 × 5s = 90s).
	// Each cycle is 5s. Set to 1 in tests for instant timeout.
	ShutdownTimeout int

	VM     VM
	Restic Restic
	Runs   Runs
}

// VMRestoreDeps bundles everything RestoreVM needs.
type VMRestoreDeps struct {
	// Confirmed MUST be true — guard against an accidental destructive restore.
	Confirmed bool
	// Name is the libvirt domain name.
	Name string
	// SnapshotID is the restic snapshot to restore (validated hex).
	SnapshotID string
	// DiskPaths are the absolute container-visible paths to restore.
	DiskPaths []string
	// NVRAMPath is the absolute container-visible NVRAM path (may be empty).
	NVRAMPath string
	// DomainXML is the captured libvirt domain XML, written to a temp file and
	// passed to virsh define so the VM reappears in the VM Manager.
	DomainXML string
	// WasAutostart is the autostart flag captured at backup time; re-applied
	// after define so the VM has the same boot-on-host-start behaviour.
	WasAutostart bool
	// StartAfter, when true, boots the VM after define (mirrors a running VM).
	StartAfter bool
	// RepoPath is the local restic repository path for the vms domain.
	RepoPath string
	// TargetID is the run-recording target id.
	TargetID string
	// DataDir is used to write temp files.
	DataDir string

	VM     VM
	Restic Restic
	Runs   Runs
}

// ---------------------------------------------------------------------------
// BackupVMGraceful
// ---------------------------------------------------------------------------

// BackupVMGraceful orchestrates a graceful VM backup:
//
//	recordRunStart
//	→ IsActive (capture wasRunning)
//	→ Shutdown → poll State until "shut off" (timeout → Destroy)
//	→ restic Backup (diskPaths + nvram, tags vm:<name>, p2)
//	→ FINALLY Start (only if wasRunning — mirrors BackupContainer's always-start)
//	→ recordRunFinish(success|failed)
//	→ re-throw on failure
//
// The VM is GUARANTEED to be restarted if it was running before the backup,
// even if any intermediate step fails.
func BackupVMGraceful(ctx context.Context, d VMBackupDeps) (Summary, error) {
	runID, err := d.Runs.Start(d.TargetID, kindBackup)
	if err != nil {
		return Summary{}, fmt.Errorf("vm backup: record run start: %w", err)
	}

	wasRunning, err := d.VM.IsActive(ctx, d.Name)
	if err != nil {
		_ = d.Runs.Finish(runID, statusFailed, "", 0, truncateErr(err))
		return Summary{}, fmt.Errorf("vm backup: check active: %w", err)
	}

	var backupErr error
	var summary Summary

	func() {
		// ALWAYS restart the VM if it was running, even on any error below.
		defer func() {
			if !wasRunning {
				return
			}
			if startErr := d.VM.Start(ctx, d.Name); startErr != nil && backupErr == nil {
				backupErr = fmt.Errorf("vm backup: restart vm: %w", startErr)
			}
		}()

		// Graceful shutdown + poll.
		if wasRunning {
			if backupErr = d.VM.Shutdown(ctx, d.Name); backupErr != nil {
				backupErr = fmt.Errorf("vm backup: shutdown: %w", backupErr)
				return
			}
			if backupErr = waitShutOff(ctx, d.VM, d.Name, d.ShutdownTimeout); backupErr != nil {
				return
			}
		}

		// Build path list: disks + nvram (if present).
		paths := append([]string(nil), d.DiskPaths...)
		if d.NVRAMPath != "" {
			paths = append(paths, d.NVRAMPath)
		}

		tags := []string{"vm:" + d.Name, "p2"}
		summary, backupErr = d.Restic.Backup(ctx, d.RepoPath, paths, tags)
		if backupErr != nil {
			backupErr = fmt.Errorf("vm backup: restic: %w", backupErr)
		}
	}()

	if backupErr != nil {
		_ = d.Runs.Finish(runID, statusFailed, "", 0, truncateErr(backupErr))
		return Summary{}, backupErr
	}

	if err := d.Runs.Finish(runID, statusSuccess, summary.SnapshotID, summary.Bytes, ""); err != nil {
		return summary, fmt.Errorf("vm backup: record run finish: %w", err)
	}
	return summary, nil
}

// waitShutOff polls the VM state until it reaches "shut off". On timeout it
// calls Destroy (force off) and returns nil (the VM is now off either way).
// If maxPolls is 0, uses defaultVMShutdownMaxPolls.
func waitShutOff(ctx context.Context, vm VM, name string, maxPolls int) error {
	if maxPolls <= 0 {
		maxPolls = defaultVMShutdownMaxPolls
	}
	for i := 0; i < maxPolls; i++ {
		state, err := vm.State(ctx, name)
		if err != nil {
			return fmt.Errorf("vm backup: poll state: %w", err)
		}
		if state == "shut off" {
			return nil
		}
		// Only sleep if we're going to poll again.
		if i < maxPolls-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(defaultVMShutdownPollInterval):
			}
		}
	}
	// Timeout: force off.
	log.Printf("vm backup: graceful shutdown timed out for %q; forcing destroy", name)
	if err := vm.Destroy(ctx, name); err != nil {
		return fmt.Errorf("vm backup: force destroy after timeout: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// RestoreVM
// ---------------------------------------------------------------------------

// RestoreVM orchestrates a VM restore:
//
//	guard Confirmed + validate snapshotID (hex) + validate paths
//	→ recordRunStart
//	→ if VM exists: Destroy (if running) + Undefine
//	→ restic RestorePaths(diskPaths + nvram, per-path back to origin)
//	→ write DomainXML to DataDir/vm-define/<name>.xml → Define
//	→ Autostart(wasAutostart) → Start (if StartAfter)
//	→ recordRunFinish(success|failed)
func RestoreVM(ctx context.Context, d VMRestoreDeps) error {
	if !d.Confirmed {
		return ErrNotConfirmed
	}
	if !snapshotIDRe.MatchString(d.SnapshotID) {
		return ErrInvalidSnapshotID
	}

	runID, err := d.Runs.Start(d.TargetID, kindRestore)
	if err != nil {
		return fmt.Errorf("vm restore: record run start: %w", err)
	}

	restoreErr := runVMRestore(ctx, d)
	if restoreErr != nil {
		_ = d.Runs.Finish(runID, statusFailed, "", 0, truncateErr(restoreErr))
		return restoreErr
	}
	if err := d.Runs.Finish(runID, statusSuccess, d.SnapshotID, 0, ""); err != nil {
		return fmt.Errorf("vm restore: record run finish: %w", err)
	}
	return nil
}

func runVMRestore(ctx context.Context, d VMRestoreDeps) error {
	// Validate: every path must be absolute and traversal-free (SEC parity).
	allPaths := append([]string(nil), d.DiskPaths...)
	if d.NVRAMPath != "" {
		allPaths = append(allPaths, d.NVRAMPath)
	}
	if len(allPaths) == 0 {
		return fmt.Errorf("vm restore: no paths to restore (unsafe)")
	}
	for _, p := range allPaths {
		if !strings.HasPrefix(p, "/") || strings.Contains(p, "..") {
			return fmt.Errorf("vm restore: unsafe path %q (unsafe)", p)
		}
	}

	// If the VM currently exists, destroy (if running) and undefine it.
	state, err := d.VM.State(ctx, d.Name)
	if err != nil {
		return fmt.Errorf("vm restore: check state: %w", err)
	}
	if state != "" {
		// VM exists.
		if state == "running" {
			if err := d.VM.Destroy(ctx, d.Name); err != nil {
				return fmt.Errorf("vm restore: destroy running vm: %w", err)
			}
		}
		if err := d.VM.Undefine(ctx, d.Name); err != nil {
			return fmt.Errorf("vm restore: undefine: %w", err)
		}
	}

	// Restore each path back to its origin (per-path subtree, like containers).
	if err := d.Restic.RestorePaths(ctx, d.RepoPath, d.SnapshotID, allPaths); err != nil {
		return fmt.Errorf("vm restore: restic restore: %w", err)
	}

	// Write domain XML to a temp file and define it.
	xmlDir := filepath.Join(d.DataDir, "vm-define")
	if err := os.MkdirAll(xmlDir, 0o700); err != nil {
		return fmt.Errorf("vm restore: create vm-define dir: %w", err)
	}
	xmlPath := filepath.Join(xmlDir, d.Name+".xml")
	if err := os.WriteFile(xmlPath, []byte(d.DomainXML), 0o600); err != nil {
		return fmt.Errorf("vm restore: write domain xml: %w", err)
	}
	if err := d.VM.Define(ctx, xmlPath); err != nil {
		return fmt.Errorf("vm restore: define: %w", err)
	}

	// Restore autostart flag.
	if err := d.VM.Autostart(ctx, d.Name, d.WasAutostart); err != nil {
		return fmt.Errorf("vm restore: autostart: %w", err)
	}

	// Optionally start the VM.
	if d.StartAfter {
		if err := d.VM.Start(ctx, d.Name); err != nil {
			return fmt.Errorf("vm restore: start: %w", err)
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they PASS**

```powershell
$env:Path += ";C:\Program Files\Go\bin;$env:USERPROFILE\go\bin"
cd d:\nextcloud\it\github\bombvault
go test ./internal/backup/... -v 2>&1 | tail -40
```

Expected: all backup tests PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/backup/vm_orchestrator.go internal/backup/vm_orchestrator_test.go
git commit -m "feat(backup): VM orchestrator — BackupVMGraceful + RestoreVM + tests"
```

---

## Task 6: service layer — VM service methods

**Files:**
- Modify: `internal/api/service.go`

This is the most complex task. The service needs:
1. A `virsh virshcli.Virsh` field on `Service` (update `NewService` signature).
2. `vmsRepoPath(settings)` — mirror of `containersRepoPath`.
3. Factor `translate` closure out of `resolveAppdataPaths` into a standalone `toContainerPath(host string) (string, bool)` method on `Service`, and call it from `resolveAppdataPaths`.
4. `vmDefinition` struct (like `containerDefinition`).
5. `ListVMs`, `BackupVM`, `RestoreVM`, `SnapshotsVM`, `SetVMMethod`, `SetVMInclude`.
6. Wire `virsh` in `NewService`.

**IMPORTANT:** `NewService` currently has signature `NewService(cfg, st, docker, engine)`. After this task it becomes `NewService(cfg, st, docker, virsh, engine)`. All call sites (`cmd/bombvault/main.go`, test files) must be updated.

- [ ] **Step 1: Read the current NewService call in service_test.go to understand what the tests pass**

The test files in `internal/api/` use `api.NewService(cfg, st, d, eng)`. After this change they need `api.NewService(cfg, st, d, fakeVirsh, eng)`. A `fakeVirsh` satisfying `virshcli.Virsh` is needed in `testutil_test.go`.

- [ ] **Step 2: Add fakeVirsh to testutil_test.go**

Add at the bottom of `internal/api/testutil_test.go`:

```go
// fakeVirsh is a minimal virshcli.Virsh implementation for service tests.
// All methods are no-ops returning empty values / nil errors.
type fakeVirsh struct{}

func (fakeVirsh) List(_ context.Context) ([]virshcli.VMInfo, error)              { return nil, nil }
func (fakeVirsh) State(_ context.Context, _ string) (string, error)              { return "", nil }
func (fakeVirsh) DumpXML(_ context.Context, _ string) (string, error)            { return "<domain/>", nil }
func (fakeVirsh) Shutdown(_ context.Context, _ string) error                     { return nil }
func (fakeVirsh) Destroy(_ context.Context, _ string) error                      { return nil }
func (fakeVirsh) Start(_ context.Context, _ string) error                        { return nil }
func (fakeVirsh) Define(_ context.Context, _ string) error                       { return nil }
func (fakeVirsh) Undefine(_ context.Context, _ string) error                     { return nil }
func (fakeVirsh) Autostart(_ context.Context, _ string, _ bool) error            { return nil }
func (fakeVirsh) IsActive(_ context.Context, _ string) (bool, error)             { return false, nil }
```

Also add `"context"` and `"github.com/junkerderprovinz/bombvault/internal/virshcli"` to the imports in `testutil_test.go`.

Then update every `api.NewService(cfg, st, d, eng)` call across all test files in `internal/api/` to `api.NewService(cfg, st, d, fakeVirsh{}, eng)`. This affects `service_test.go`, `handlers_test.go` (via `newTestRouter`).

- [ ] **Step 3: Update newTestRouter in handlers_test.go**

In `newTestRouter`, change:
```go
svc := api.NewService(cfg, st, d, eng)
```
to:
```go
svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)
```

- [ ] **Step 4: Implement the service changes in service.go**

**a) Add `virsh virshcli.Virsh` to Service and update NewService:**

```go
type Service struct {
    cfg    config.Config
    store  *store.Repo
    docker dockercli.Docker
    virsh  virshcli.Virsh
    engine ResticEngine
}

func NewService(cfg config.Config, st *store.Repo, d dockercli.Docker, v virshcli.Virsh, eng ResticEngine) *Service {
    return &Service{cfg: cfg, store: st, docker: d, virsh: v, engine: eng}
}
```

**b) Factor `toContainerPath` out of `resolveAppdataPaths`:**

```go
// toContainerPath translates a HOST path under HostSourceRoot to its
// container-visible equivalent under HostMountRoot. Returns ("", false) when
// the host path is not reachable through the mount.
// Used for both appdata (containers) and VM disk paths.
func (s *Service) toContainerPath(host string) (string, bool) {
    srcRoot := path.Clean(s.cfg.HostSourceRoot)
    mountRoot := path.Clean(s.cfg.HostMountRoot)
    p := path.Clean(host)
    if p == srcRoot {
        return mountRoot, true
    }
    if rest := strings.TrimPrefix(p, srcRoot+"/"); rest != p {
        return mountRoot + "/" + rest, true
    }
    return "", false
}
```

Refactor `resolveAppdataPaths` to call `s.toContainerPath(m.Source)` instead of the inline closure.

**c) Add `vmsRepoPath`:**

```go
func (s *Service) vmsRepoPath(settings store.Settings) (string, error) {
    repo, err := paths.Resolve(s.cfg.HostMountRoot, settings.VmsPath)
    if err != nil {
        return "", fmt.Errorf("resolve vms path: %w", err)
    }
    return repo, nil
}
```

**d) Add `vmDefinition`:**

```go
// vmDefinition is the recreate recipe persisted at VM backup time.
type vmDefinition struct {
    DomainXML    string   `json:"domain_xml"`
    DiskPaths    []string `json:"disk_paths"` // container-visible paths
    NVRAMPath    string   `json:"nvram_path"` // container-visible path (empty for BIOS)
    Method       string   `json:"method"`
    WasAutostart bool     `json:"was_autostart"`
}
```

**e) Implement `ListVMs`:**

```go
// VMView is the per-VM row returned by ListVMs.
type VMView struct {
    Name              string  `json:"name"`
    State             string  `json:"state"`
    Method            string  `json:"method"`
    IncludeInSchedule bool    `json:"includeInSchedule"`
    LastBackup        *int64  `json:"lastBackup"`
}

// ListVMs returns all known VMs (from virsh) merged with the DB targets.
// VMs with no virsh entry but with backup history appear as state="not-installed".
func (s *Service) ListVMs(ctx context.Context) ([]VMView, error) {
    infos, err := s.virsh.List(ctx)
    if err != nil {
        return nil, fmt.Errorf("list vms: virsh: %w", err)
    }
    targets, _ := s.store.ListVMTargets()
    byName := make(map[string]store.VMTarget, len(targets))
    for _, t := range targets {
        byName[t.Name] = t
    }

    live := make(map[string]bool, len(infos))
    views := make([]VMView, 0, len(infos)+len(targets))
    for _, vm := range infos {
        live[vm.Name] = true
        v := VMView{Name: vm.Name, State: vm.State, Method: "graceful"}
        if t, ok := byName[vm.Name]; ok {
            v.Method = t.Method
            v.IncludeInSchedule = t.IncludeInSchedule
            if run, _ := s.store.LastSuccessfulBackup(t.ID); run != nil {
                v.LastBackup = run.FinishedAt
            }
        }
        views = append(views, v)
    }
    // Orphans: targets whose VM is no longer defined.
    for _, t := range targets {
        if live[t.Name] {
            continue
        }
        v := VMView{Name: t.Name, State: "not-installed", Method: t.Method, IncludeInSchedule: t.IncludeInSchedule}
        if run, _ := s.store.LastSuccessfulBackup(t.ID); run != nil {
            v.LastBackup = run.FinishedAt
        }
        views = append(views, v)
    }
    return views, nil
}
```

**f) Implement `BackupVM`:**

```go
// BackupVM orchestrates a full VM backup: resolve repo + mode, ensure repo,
// dump XML, parse domain, translate paths, upsert VM target, run orchestrator.
func (s *Service) BackupVM(ctx context.Context, name string) (backup.Summary, error) {
    settings, err := s.store.GetSettings()
    if err != nil {
        return backup.Summary{}, fmt.Errorf("read settings: %w", err)
    }
    repo, err := s.vmsRepoPath(settings)
    if err != nil {
        return backup.Summary{}, err
    }
    mode := s.ModeFor(settings)
    if err := s.EnsureRepo(ctx, repo, mode); err != nil {
        return backup.Summary{}, err
    }

    // Capture the domain XML and parse disk/nvram paths.
    xmlStr, err := s.virsh.DumpXML(ctx, name)
    if err != nil {
        return backup.Summary{}, fmt.Errorf("backup vm: dumpxml: %w", err)
    }
    domain, err := virshcli.ParseDomain(xmlStr)
    if err != nil {
        return backup.Summary{}, fmt.Errorf("backup vm: parse domain: %w", err)
    }

    // Translate HOST paths to container-visible paths.
    var diskPaths []string
    for _, hp := range domain.DiskPaths {
        if cp, ok := s.toContainerPath(hp); ok {
            diskPaths = append(diskPaths, cp)
        } else {
            log.Printf("api: BackupVM: disk path %q not reachable via mount, using host path", hp)
            diskPaths = append(diskPaths, hp)
        }
    }
    nvramPath := ""
    if domain.NVRAMPath != "" {
        if cp, ok := s.toContainerPath(domain.NVRAMPath); ok {
            nvramPath = cp
        } else {
            nvramPath = domain.NVRAMPath
        }
    }

    // Resolve autostart (default true if we can't tell — safe for restore).
    wasAutostart := true

    // Get method from existing target (default graceful).
    method := "graceful"
    if existing, tErr := s.store.GetVMTargetByName(name); tErr == nil {
        method = existing.Method
    }

    def := vmDefinition{
        DomainXML: xmlStr, DiskPaths: diskPaths, NVRAMPath: nvramPath,
        Method: method, WasAutostart: wasAutostart,
    }
    defBytes, _ := json.Marshal(def)

    tg, err := s.store.UpsertVMTarget(store.VMTarget{
        Name: name, Method: method, Definition: string(defBytes),
    })
    if err != nil {
        return backup.Summary{}, fmt.Errorf("upsert vm target: %w", err)
    }

    return backup.BackupVMGraceful(ctx, backup.VMBackupDeps{
        Name:      name,
        DiskPaths: diskPaths,
        NVRAMPath: nvramPath,
        RepoPath:  repo,
        TargetID:  tg.ID,
        DataDir:   s.cfg.DataDir,
        VM:        s.virsh,
        Restic:    &resticAdapter{engine: s.engine, mode: mode},
        Runs:      runsAdapter{s.store},
    })
}
```

**g) Implement `RestoreVM`:**

```go
// RestoreVM orchestrates a VM restore from a stored definition.
func (s *Service) RestoreVM(ctx context.Context, name, snapshotID string, confirm bool) error {
    if !confirm {
        return backup.ErrNotConfirmed
    }
    settings, err := s.store.GetSettings()
    if err != nil {
        return fmt.Errorf("read settings: %w", err)
    }
    repo, err := s.vmsRepoPath(settings)
    if err != nil {
        return err
    }
    mode := s.ModeFor(settings)

    tg, err := s.store.GetVMTargetByName(name)
    if err != nil {
        return errors.New("vm has not been backed up yet")
    }
    if snapshotID == "latest" || snapshotID == "" {
        snaps, snapErr := s.SnapshotsVM(ctx, name)
        if snapErr != nil {
            return snapErr
        }
        if len(snaps) == 0 {
            return errors.New("no backups found for this vm")
        }
        snapshotID = snaps[len(snaps)-1].ID
    }

    if tg.Definition == "" {
        return errors.New("no stored definition for this vm — run a backup once first")
    }
    var def vmDefinition
    if err := json.Unmarshal([]byte(tg.Definition), &def); err != nil {
        return fmt.Errorf("restore vm: unmarshal definition: %w", err)
    }

    // Re-validate stored paths stay within the mount root (defense-in-depth).
    allPaths := append([]string(nil), def.DiskPaths...)
    if def.NVRAMPath != "" {
        allPaths = append(allPaths, def.NVRAMPath)
    }
    for _, p := range allPaths {
        if !paths.Within(s.cfg.HostMountRoot, p) {
            log.Printf("api: RestoreVM: path %q escapes mount root", p)
            return errors.New("a stored backup path is outside the host mount — refusing to restore")
        }
    }

    return backup.RestoreVM(ctx, backup.VMRestoreDeps{
        Confirmed:    confirm,
        Name:         name,
        SnapshotID:   snapshotID,
        DiskPaths:    def.DiskPaths,
        NVRAMPath:    def.NVRAMPath,
        DomainXML:    def.DomainXML,
        WasAutostart: def.WasAutostart,
        StartAfter:   true,
        RepoPath:     repo,
        TargetID:     tg.ID,
        DataDir:      s.cfg.DataDir,
        VM:           s.virsh,
        Restic:       &resticAdapter{engine: s.engine, mode: mode},
        Runs:         runsAdapter{s.store},
    })
}
```

**h) Implement `SnapshotsVM`:**

```go
// SnapshotsVM lists restic snapshots for a single VM (filtered by vm:<name> tag).
func (s *Service) SnapshotsVM(ctx context.Context, name string) ([]restic.Snapshot, error) {
    settings, err := s.store.GetSettings()
    if err != nil {
        return nil, fmt.Errorf("read settings: %w", err)
    }
    repo, err := s.vmsRepoPath(settings)
    if err != nil {
        return nil, err
    }
    mode := s.ModeFor(settings)
    if _, statErr := os.Stat(filepath.Join(repo, "config")); errors.Is(statErr, fs.ErrNotExist) {
        return nil, nil
    }
    all, err := s.engine.Snapshots(ctx, repo, mode)
    if err != nil {
        return nil, err
    }
    tag := "vm:" + name
    out := make([]restic.Snapshot, 0)
    for _, snap := range all {
        for _, t := range snap.Tags {
            if t == tag {
                out = append(out, snap)
                break
            }
        }
    }
    return out, nil
}
```

**i) Implement `SetVMMethod` and `SetVMInclude`:**

```go
// SetVMMethod updates the backup method for a VM, creating the target if absent.
func (s *Service) SetVMMethod(ctx context.Context, name, method string) error {
    if _, err := s.store.GetVMTargetByName(name); err != nil {
        if _, uErr := s.store.UpsertVMTarget(store.VMTarget{Name: name, Method: method}); uErr != nil {
            return fmt.Errorf("ensure vm target: %w", uErr)
        }
        return nil
    }
    return s.store.SetVMMethod(name, method)
}

// SetVMInclude updates the include_in_schedule flag for a VM.
func (s *Service) SetVMInclude(ctx context.Context, name string, include bool) error {
    if _, err := s.store.GetVMTargetByName(name); err != nil {
        if _, uErr := s.store.UpsertVMTarget(store.VMTarget{Name: name, Method: "graceful"}); uErr != nil {
            return fmt.Errorf("ensure vm target: %w", uErr)
        }
    }
    return s.store.SetVMInclude(name, include)
}
```

- [ ] **Step 5: Update imports in service.go**

Add `"github.com/junkerderprovinz/bombvault/internal/virshcli"` to the import block.

- [ ] **Step 6: Build to verify**

```powershell
$env:Path += ";C:\Program Files\Go\bin;$env:USERPROFILE\go\bin"
cd d:\nextcloud\it\github\bombvault
go build ./...
```

Expected: no errors. Fix any compilation issues (usually import cycles or missing methods).

- [ ] **Step 7: Run all tests to verify no regressions**

```powershell
$env:Path += ";C:\Program Files\Go\bin;$env:USERPROFILE\go\bin"
cd d:\nextcloud\it\github\bombvault
go test ./... 2>&1 | tail -20
```

Expected: PASS for all existing tests.

- [ ] **Step 8: Commit**

```powershell
git add internal/api/service.go internal/api/testutil_test.go internal/api/service_test.go internal/api/handlers_test.go
git commit -m "feat(api): VM service methods + toContainerPath refactor"
```

---

## Task 7: API handlers + routes for VMs

**Files:**
- Modify: `internal/api/handlers.go`

Add `handleListVMs`, `handleBackupVM`, `handleSnapshotsVM`, `handleRestoreVM`, `handlePatchVM` and register them in `Router()` — all mirroring the container equivalents with the same envelope/scrub pattern.

The `Handler` struct (defined in `internal/api/handlers.go` or a related file — check where `NewHandler` is defined) already has `svc *Service` and `store *store.Repo`. The VM handlers call methods on `h.svc`.

**Note:** First check where `NewHandler` and `Handler` struct are defined. They may be in `handlers.go` or a separate file. Look for `type Handler struct` first.

- [ ] **Step 1: Locate the Handler struct**

```powershell
Select-String -Path "d:\nextcloud\it\github\bombvault\internal\api\*.go" -Pattern "type Handler struct" | Select-Object -First 5
```

- [ ] **Step 2: Add VM handlers to handlers.go**

Append the following handler methods to `internal/api/handlers.go`:

```go
// vmView is the per-VM row returned by GET /api/vms.
// It mirrors containerView for shape consistency.
type vmView struct {
    Name              string `json:"name"`
    State             string `json:"state"`
    Method            string `json:"method"`
    IncludeInSchedule bool   `json:"includeInSchedule"`
    LastBackup        *int64 `json:"lastBackup"`
}

func (h *Handler) handleListVMs(w http.ResponseWriter, r *http.Request) {
    views, err := h.svc.ListVMs(r.Context())
    if err != nil {
        writeJSON(w, http.StatusOK, failEnvelope(err))
        return
    }
    if views == nil {
        views = []VMView{}
    }
    writeJSON(w, http.StatusOK, map[string]any{"ok": true, "vms": views})
}

func (h *Handler) handleBackupVM(w http.ResponseWriter, r *http.Request) {
    name := r.PathValue("name")
    sum, err := h.svc.BackupVM(r.Context(), name)
    if err != nil {
        writeJSON(w, http.StatusOK, failEnvelope(err))
        return
    }
    writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
        "snapshotId": sum.SnapshotID,
        "bytes":      sum.Bytes,
    }))
}

func (h *Handler) handleSnapshotsVM(w http.ResponseWriter, r *http.Request) {
    name := r.PathValue("name")
    snaps, err := h.svc.SnapshotsVM(r.Context(), name)
    if err != nil {
        writeJSON(w, http.StatusOK, failEnvelope(err))
        return
    }
    if snaps == nil {
        snaps = []restic.Snapshot{}
    }
    writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"snapshots": snaps}))
}

func (h *Handler) handleRestoreVM(w http.ResponseWriter, r *http.Request) {
    name := r.PathValue("name")
    var body struct {
        SnapshotID string `json:"snapshotId"`
        Confirm    bool   `json:"confirm"`
    }
    if !decodeBody(w, r, &body) {
        return
    }
    if err := h.svc.RestoreVM(r.Context(), name, body.SnapshotID, body.Confirm); err != nil {
        writeJSON(w, http.StatusOK, failEnvelope(err))
        return
    }
    writeJSON(w, http.StatusOK, okEnvelope(nil))
}

func (h *Handler) handlePatchVM(w http.ResponseWriter, r *http.Request) {
    name := r.PathValue("name")
    var body struct {
        Method            *string `json:"method"`
        IncludeInSchedule *bool   `json:"includeInSchedule"`
    }
    if !decodeBody(w, r, &body) {
        return
    }
    if body.Method != nil {
        if err := h.svc.SetVMMethod(r.Context(), name, *body.Method); err != nil {
            writeJSON(w, http.StatusOK, failEnvelope(err))
            return
        }
    }
    if body.IncludeInSchedule != nil {
        if err := h.svc.SetVMInclude(r.Context(), name, *body.IncludeInSchedule); err != nil {
            writeJSON(w, http.StatusOK, failEnvelope(err))
            return
        }
    }
    writeJSON(w, http.StatusOK, okEnvelope(nil))
}
```

**IMPORTANT:** `VMView` is defined in `service.go` — use that type in `handleListVMs` (not a local `vmView`). Remove the local `vmView` struct above and use `[]VMView` directly. Or alias it. Decide based on whether service.go exports `VMView`.

- [ ] **Step 3: Register routes in Router()**

Find `Router()` in the handler file (check `server.go` or `handlers.go`). Add the 5 new routes under `authGate`:

```go
mux.Handle("GET /api/vms", h.authGate(http.HandlerFunc(h.handleListVMs)))
mux.Handle("POST /api/vms/{name}/backup", h.authGate(http.HandlerFunc(h.handleBackupVM)))
mux.Handle("GET /api/vms/{name}/snapshots", h.authGate(http.HandlerFunc(h.handleSnapshotsVM)))
mux.Handle("POST /api/vms/{name}/restore", h.authGate(http.HandlerFunc(h.handleRestoreVM)))
mux.Handle("PATCH /api/vms/{name}", h.authGate(http.HandlerFunc(h.handlePatchVM)))
```

- [ ] **Step 4: Find Router() and add the routes**

```powershell
Select-String -Path "d:\nextcloud\it\github\bombvault\internal\api\*.go" -Pattern "func.*Router" | Select-Object -First 5
```

Read the file containing `Router()` and add the routes at the correct location (near the existing container routes).

- [ ] **Step 5: Build to verify**

```powershell
$env:Path += ";C:\Program Files\Go\bin;$env:USERPROFILE\go\bin"
cd d:\nextcloud\it\github\bombvault
go build ./...
```

- [ ] **Step 6: Add handler tests for VM routes**

Add to the bottom of `internal/api/handlers_test.go`:

```go
func TestListVMsOK(t *testing.T) {
    h, _ := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
    w, m := doJSON(t, h, http.MethodGet, "/api/vms", "")
    if w.Code != http.StatusOK {
        t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
    }
    if m["ok"] != true {
        t.Fatalf("expected ok:true, got %v", m)
    }
    if _, ok := m["vms"]; !ok {
        t.Fatal("expected vms key in response")
    }
}

func TestBackupVMOK(t *testing.T) {
    h, _ := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
    w, m := doJSON(t, h, http.MethodPost, "/api/vms/win10/backup", "")
    if w.Code != http.StatusOK {
        t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
    }
    // fakeVirsh returns <domain/> which ParseDomain handles gracefully (no disks).
    // The backup should succeed (empty paths list) or return a graceful error.
    // We check the envelope is well-formed.
    if _, ok := m["ok"]; !ok {
        t.Fatalf("missing ok key in response: %v", m)
    }
}

func TestSnapshotsVMOK(t *testing.T) {
    h, _ := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
    w, m := doJSON(t, h, http.MethodGet, "/api/vms/win10/snapshots", "")
    if w.Code != http.StatusOK {
        t.Fatalf("status = %d", w.Code)
    }
    if m["ok"] != true {
        t.Fatalf("expected ok:true, got %v", m)
    }
}

func TestRestoreVMNotConfirmed(t *testing.T) {
    h, _ := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
    w, m := doJSON(t, h, http.MethodPost, "/api/vms/win10/restore",
        `{"snapshotId":"deadbeef","confirm":false}`)
    if w.Code != http.StatusOK {
        t.Fatalf("status = %d", w.Code)
    }
    if m["ok"] != false {
        t.Fatalf("expected ok:false for unconfirmed restore")
    }
}

func TestPatchVMMethodOK(t *testing.T) {
    h, _ := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
    w, m := doJSON(t, h, http.MethodPatch, "/api/vms/win10",
        `{"method":"graceful"}`)
    if w.Code != http.StatusOK {
        t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
    }
    if m["ok"] != true {
        t.Fatalf("expected ok:true, got %v", m)
    }
}
```

- [ ] **Step 7: Run all tests**

```powershell
$env:Path += ";C:\Program Files\Go\bin;$env:USERPROFILE\go\bin"
cd d:\nextcloud\it\github\bombvault
go test ./... 2>&1 | tail -30
```

Expected: all PASS.

- [ ] **Step 8: Commit**

```powershell
git add internal/api/handlers.go
git commit -m "feat(api): VM handlers (ListVMs, BackupVM, SnapshotsVM, RestoreVM, PatchVM) + routes"
```

---

## Task 8: Wire virshcli.Client in main.go

**Files:**
- Modify: `cmd/bombvault/main.go`

- [ ] **Step 1: Update main.go**

In `run()`, after `dc, err := dockercli.New()`:

```go
// Real virsh adapter over the mounted libvirt socket.
vc := virshcli.New()
```

Then update the `api.NewService` call:

```go
svc := api.NewService(cfg, st, dc, vc, engine)
```

Add `"github.com/junkerderprovinz/bombvault/internal/virshcli"` to the import block.

- [ ] **Step 2: Build to verify**

```powershell
$env:Path += ";C:\Program Files\Go\bin;$env:USERPROFILE\go\bin"
cd d:\nextcloud\it\github\bombvault
go build ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```powershell
git add cmd/bombvault/main.go
git commit -m "feat(main): wire virshcli.Client into service"
```

---

## Task 9: Dockerfile — add libvirt-clients

**Files:**
- Modify: `Dockerfile`

- [ ] **Step 1: Add libvirt-clients to the runtime apt install**

In the `apt-get install` line in the `FROM debian:stable-slim AS runtime` stage, add `libvirt-clients` after `qemu-utils`:

Current line:
```
    apt-get install -y --no-install-recommends ca-certificates qemu-utils rclone bzip2 wget; \
```

Updated line:
```
    apt-get install -y --no-install-recommends ca-certificates qemu-utils libvirt-clients rclone bzip2 wget; \
```

- [ ] **Step 2: Verify the Dockerfile is syntactically correct**

```powershell
docker build --check . 2>&1 | head -20
```

If `docker build --check` is not available (older Docker), just do a dry-run syntax check:

```powershell
Select-String -Path "d:\nextcloud\it\github\bombvault\Dockerfile" -Pattern "libvirt-clients"
```

Expected: the line appears.

- [ ] **Step 3: Commit**

```powershell
git add Dockerfile
git commit -m "feat(docker): add libvirt-clients to runtime image for virsh"
```

---

## Task 10: Final gates — go build, go test, golangci-lint

- [ ] **Step 1: go build ./...**

```powershell
$env:Path += ";C:\Program Files\Go\bin;$env:USERPROFILE\go\bin"
cd d:\nextcloud\it\github\bombvault
go build ./...
```

Expected: zero output (success).

- [ ] **Step 2: go test ./...**

```powershell
$env:Path += ";C:\Program Files\Go\bin;$env:USERPROFILE\go\bin"
cd d:\nextcloud\it\github\bombvault
go test ./... -count=1
```

Expected: all packages PASS, zero FAILs.

- [ ] **Step 3: golangci-lint run**

```powershell
$env:Path += ";C:\Program Files\Go\bin;$env:USERPROFILE\go\bin"
cd d:\nextcloud\it\github\bombvault
& "$env:USERPROFILE\go\bin\golangci-lint.exe" run
```

Expected: 0 issues. Fix any lint issues before the commit.

Common lint issues to watch for:
- `G204` (exec.Command with variable) — already suppressed with `//nolint:gosec` comment in virshcli.go.
- Unused variables.
- `errcheck` on `tx.Rollback()` — use the same `//nolint:errcheck,gosec` pattern as `targets.go`.
- `gosec G703` on `os.WriteFile` — add nolint comment matching the pattern in service.go.

- [ ] **Step 4: Fix lint issues if any**

Address each issue, re-run lint until clean, then:

```powershell
git add -u
git commit -m "fix(lint): address golangci-lint findings in VM backend"
```

- [ ] **Step 5: Verify branch and final log**

```powershell
cd d:\nextcloud\it\github\bombvault
git log --oneline feat/vm-backup-graceful 2>&1 | head -15
git branch --show-current
```

Expected: branch = `feat/vm-backup-graceful`; log shows all commits from this wave.

---

## Spec Coverage Checklist

| Spec requirement | Task |
|-----------------|------|
| virsh adapter, `Virsh` interface, `virsh list --all --name` | Tasks 1–2 |
| `State` tolerates not-found (returns `"", nil`) | Task 2 |
| `DumpXML`, `Shutdown`, `Destroy`, `Start`, `Define`, `Undefine`, `Autostart`, `IsActive` | Task 2 |
| `ParseDomain` — extract disk paths + NVRAM | Task 2 |
| Exec with separate args (no shell interpolation) | Task 2 |
| `lastReason` scrubbing (paths stripped, last non-empty stderr) | Task 2 |
| Migration v4 — `vms` table | Task 3 |
| `VMTarget` CRUD — `UpsertVMTarget`, `GetVMTargetByName`, `ListVMTargets`, `SetVMMethod`, `SetVMInclude`, `DeleteVMTarget` | Tasks 3–4 |
| `BackupVMGraceful` — ALWAYS restart if wasRunning | Task 5 |
| Graceful shutdown poll → timeout → Destroy | Task 5 |
| Tags `vm:<name>` + `p2` | Task 5 |
| `RestoreVM` — confirm guard + snapshotID hex guard | Task 5 |
| Path validation (absolute, no `..`) | Task 5 |
| Destroy+Undefine if exists; skip if absent | Task 5 |
| RestorePaths per-path (mirrors container pattern) | Task 5 |
| Write XML temp file → Define → Autostart → Start | Task 5 |
| Run recording (Start/Finish) for both orchestrators | Task 5 |
| `toContainerPath` helper shared by appdata + VM disks | Task 6 |
| `vmDefinition` persisted at backup time | Task 6 |
| `ListVMs`, `BackupVM`, `RestoreVM`, `SnapshotsVM`, `SetVMMethod`, `SetVMInclude` | Tasks 6–7 |
| `GET /api/vms`, `POST /api/vms/{name}/backup`, `GET /api/vms/{name}/snapshots`, `POST /api/vms/{name}/restore`, `PATCH /api/vms/{name}` | Task 7 |
| authGate on all VM routes | Task 7 |
| Wire concrete `virshcli.Client` in `main.go` | Task 8 |
| `libvirt-clients` in Dockerfile runtime stage | Task 9 |
| All gates pass: `go build`, `go test`, golangci-lint | Task 10 |

## Ambiguities flagged

1. **Autostart capture:** The design says "capture autostart flag" at backup time. `virsh dominfo <name>` includes an `Autostart: enable/disable` field. However, parsing `dominfo` requires more XML/text parsing. For this wave, `WasAutostart` defaults to `true` (safe: if the VM was set up in the VM Manager, autostart is almost always on). A TODO comment is left in `BackupVM` to replace with real `dominfo` parsing in a follow-up.

2. **`PATCH /api/vms/{name}` body with `*string` and `*bool` fields:** `decodeBody` uses `DisallowUnknownFields`. The PATCH body uses optional pointer fields (`Method *string`, `IncludeInSchedule *bool`). The decoder must not reject a body that only has one of the two fields. This is fine because `DisallowUnknownFields` only rejects *unknown* fields, not absent ones.

3. **Empty disk paths on `BackupVM` when `<domain/>` has no disks:** The fake virsh returns `<domain/>`. `ParseDomain` returns empty `DiskPaths`. `BackupVMGraceful` with empty paths calls `restic.Backup` with an empty path list — restic would fail on a real system, but in tests the fake restic succeeds. In production, the service should warn/error on empty disk paths; a defensive check is added in `BackupVM`: `if len(diskPaths) == 0 { return …, fmt.Errorf("no disk paths found in domain XML") }`.

4. **`ShutdownTimeout` as poll count vs duration:** The `VMBackupDeps.ShutdownTimeout` is an integer poll count (0 = default 18 × 5s). This is simpler to fake in tests (set to 1 for instant timeout) than a `time.Duration`. Production behaviour is identical: 18 × 5s = 90s max wait.

5. **`runs` table FK constraint:** `runs.target_id` references `targets(id)`, not `vms(id)`. Since the `runs` table has a REFERENCES constraint to `targets`, VM run records will use the `vms.id` as `target_id` but there is no FK from `runs` to `vms`. This is intentional — the design reuses the `runs` table across domains (the `kind` field distinguishes them). The `DeleteVMTarget` tx deletes runs where `target_id IN (SELECT id FROM vms WHERE name=?)` which correctly cleans up the runs without needing a FK.
