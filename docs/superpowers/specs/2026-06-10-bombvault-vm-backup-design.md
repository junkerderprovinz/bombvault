# BombVault Phase 2 — VM Backup & Restore (KVM/libvirt) — Design

**Status:** draft for review
**Goal:** Back up and fully restore Unraid KVM/libvirt VMs (disk image(s) + NVRAM + the
libvirt XML), so a restored VM reappears in the Unraid **VM Manager** exactly as before —
mirroring what Phase 1 does for containers. Engine stays **restic**.

**User decision (2026-06-10):** support **both** backup methods, selectable per VM —
**Graceful Shutdown** (default) **and** **Live Snapshot** (no downtime).

---

## 1. Architecture

- **libvirt access via the `virsh` CLI** (shell out, exactly like the restic adapter), over the
  already-mounted libvirt socket (`/var/run/libvirt/libvirt-sock`, advanced mount in the template).
  No cgo/libvirt-dev. Add `libvirt-clients` (virsh) to the runtime image (qemu-utils already present).
- **A `virsh` interface** (like the `Docker` interface) so the orchestrator is mockable in tests;
  the concrete adapter lives in `internal/virshcli`. CI can't run libvirt → unit tests mock virsh,
  real behaviour is **box-gated** on the Unraid host.
- **Engine:** restic, into the **vms** repo path (`settings.vms_path`, already exists). Same
  encryption/mode as containers (APP_KEY-derived). Snapshots tagged `vm:<name>`.
- **VM disks** live under `/mnt/user/domains/<vm>/…` on Unraid → reachable through the existing
  broad `/mnt → /host/user` mount (same host→container path translation as appdata).

## 2. What is captured (the recreate recipe)

Per VM, captured at backup time and persisted (DB `vms` table + encrypted mirror on the backup
storage for full-DR, exactly like container definitions):
- the **libvirt domain XML** (`virsh dumpxml <vm>`),
- the **disk image path(s)** parsed from the XML (`<disk type='file'><source file='…'/>`),
- the **NVRAM** path (`<os><nvram>…</nvram>`) for UEFI/OVMF VMs,
- the chosen **method** (graceful|live) + include-in-schedule flag.

restic backs up the disk file(s) + NVRAM (per-path subtree restore, like containers — never
reconcile the shared `domains` parent). The XML travels in the definition.

## 3. Backup flows

### 3a. Graceful Shutdown (default)
```
record run start
→ virsh shutdown <vm>  (poll until state==shut off; timeout → virsh destroy)
→ restic backup <disk(s)> <nvram>  (tag vm:<name>)
→ FINALLY virsh start <vm>  (always restart, even on error — mirror container BackupContainer)
→ record finish
```
Consistent (VM is off during copy). Downtime = the copy duration.

### 3b. Live Snapshot (no downtime)
```
record run start
→ virsh snapshot-create-as <vm> bombvault-tmp --disk-only --atomic [--quiesce if guest-agent]
   (writes now go to a NEW overlay; the ORIGINAL disk becomes a stable backing file)
→ restic backup <original backing disk(s)> <nvram>  (the backing files are now read-only-stable)
→ ALWAYS: virsh blockcommit <vm> <target> --active --verbose --pivot --wait
   (merge the overlay back into the original, pivot the live VM onto it)
→ delete the snapshot metadata; verify no overlay left dangling
→ record finish
```
- `--quiesce` only when the qemu-guest-agent is present (probe `virsh domfsinfo`/agent ping);
  otherwise crash-consistent (still valid for a journaling FS).
- **SEC/safety (critical):** the blockcommit/pivot + overlay cleanup MUST run even if the restic
  backup fails, so the VM is never left running on a temporary overlay. If blockcommit fails,
  surface a LOUD error and leave the snapshot in place (do not delete it) so the VM stays usable
  and the operator can recover. Never `destroy` a running VM in the live path.

## 4. Restore (same for both methods)
```
confirm → validate snapshot id (hex) + disk paths within mount root
→ if the VM exists: virsh destroy (if running) + virsh undefine --nvram
→ restic restore <disk(s)> + <nvram> back to their original paths (per-path subtree)
→ write the captured XML to a temp file → virsh define <xml>
→ virsh autostart <vm> (preserve the original autostart flag) ; optionally virsh start
→ record finish
```
Restored VM reappears in the VM Manager with its disk(s) attached. Passthrough devices
(PCI/GPU/USB) are preserved verbatim in the XML — note in the UI that the same hardware must be
present on restore (passthrough guard).

## 5. Data model + API + UI

- **DB:** new `vms` table (id, name, method, include_in_schedule, definition, created_at) +
  reuse `runs` (kind already free-form). Migration v4.
- **virsh interface:** List, DumpXML, State, Shutdown, Destroy, Start, Define, Undefine,
  SnapshotCreateDiskOnly, Blockcommit, SnapshotDelete, Autostart, AgentPing.
- **Service:** `BackupVM` / `RestoreVM` orchestrators (mirroring container ones, with the
  graceful/live branch), `ListVMs`, `Discover` extended to vm: snapshots + vm defs.
- **API:** `GET /api/vms`, `POST /api/vms/{name}/backup`, `GET /api/vms/{name}/snapshots`,
  `POST /api/vms/{name}/restore`, `PATCH /api/vms/{name}` (method + include), bulk like containers.
- **UI:** the **VMs** tab (already a sidebar placeholder) → list VMs with state, per-VM method
  dropdown (graceful|live), backup/restore/last-backup, multi-select bulk, in the same style as
  Containers. The Jobs tab + dashboard stat cards already have VM slots (currently 0).

## 6. Image / template

- Dockerfile runtime: add `libvirt-clients` (virsh). qemu-utils already present.
- Template: the libvirt socket mount is already there (advanced) — promote its description; VM
  disks are covered by the existing `/mnt` mount. No new required mount.

## 7. Testing

- Unit: mock the virsh interface; test the graceful + live orchestration ordering, the
  always-restart / always-blockcommit guards, path validation, restore sequence. (restic roundtrip
  stays as-is.)
- **Box-gate (required):** real libvirt is not in CI. Validate on the Unraid host: graceful + live
  backup of a real VM, restore into the VM Manager, UEFI (NVRAM) VM, a VM with passthrough.

## 8. Phasing (delivered together, built in this order)

1. virsh adapter + interface + `vms` table/migration + ListVMs + the VMs tab (read-only list).
2. Graceful backup + restore end-to-end (simplest, proves the pipeline) → box-gate.
3. Live snapshot + blockcommit (+ guest-agent quiesce) → box-gate carefully (VM-safety critical).
4. Bulk + Discover (vm defs on storage) + dashboard/Jobs wthat wire the VM counts.

## 9. Risks

- **Live snapshot/blockcommit is VM-safety-critical** — a mid-operation failure must leave the VM
  runnable. Extensive defensive handling + box-testing before declaring it done. Graceful is the
  safe default and ships first.
- Large VM disks: restic dedup/incremental keeps repo growth sane; the first backup is large.
- Passthrough hardware must exist on restore (documented; XML preserved verbatim).
