# BombVault v8.0.0, Direction 4/4: Platform Expansion Beyond Unraid — Design

**Goal:** run BombVault on TrueNAS Scale (containers + VMs) and on a generic Docker host (containers), in addition to Unraid, from one binary — without turning it into a different product.

**Non-goal:** Proxmox VE support. Proxmox has no libvirtd, no domain XML, no host-level Docker daemon, and Proxmox Backup Server already covers raw VM backup on that platform. It is a separate product's worth of work for a market already served; deferred to a possible future initiative positioned as a PBS complement (restore drills, off-site, node-config backup) — never a PBS replacement.

**Non-goal (this release):** a platform-neutral marketing rebrand. The target *identity* is platform-neutral (see Positioning below), but README/support-thread/CA-listing stay Unraid-first until TrueNAS support is live and has real users.

---

## 1. Current Unraid coupling (from a full codebase audit)

Smaller than expected, and concentrated in a few places:

- **`resolveAppdataPaths`** (`internal/api/service.go:2727`) is the actual blocker: it only keeps a container's bind mount if the host source path contains a segment literally named `appdata`, and otherwise falls back to the hardcoded `/mnt/user/appdata/<name>`. Off Unraid, neither ever matches — every container is silently classified stateless (definition-only backup, no data) unless the user manually picks folders.
- **Named volumes are invisible.** Only bind mounts (`m.Source`) are walked; `Type=="volume"` is never considered. This is a real gap on Unraid too, not just elsewhere.
- **Single host root.** `HOST_SOURCE_ROOT=/mnt` → `HOST_MOUNT_ROOT=/host/user`, enforced by `paths.Resolve/Within` and `mountinfo.go`. No representation for multiple independent data roots (e.g. `/opt/stacks` + `/mnt/backup` on a generic host).
- **Unraid share literals in code, not config:** `foreignContainerDestBase` → `user/appdata`, `foreignVMDestBase` → `user/domains` (`service.go:6379-6419`), default repo paths `user/bombvault/*` (`migrate.go:107`).
- **`DOCKER_HOST` is silently ignored** — `client.FromEnv` is called, then immediately overridden by `client.WithHost("unix:///var/run/docker.sock")` (`internal/dockercli/dockercli.go:40-50`). Breaks rootless Docker/Podman today, on any platform including Unraid. A real bug, not a platform gap.
- **Genuinely Unraid-only, and already fail-soft** (degrade gracefully, never break the backup): dockerMan template XML capture/restore (`internal/template/*`), the flash/USB domain, the USB-creator-compatible zip export, `dashplugin.go` (.plg push over SSH), `sendUnraidNotify`, the emhttp PHP update-status reconcile (`service.go:3288`), `defaultOVMFVars = /usr/share/qemu/ovmf-x64/...`.
- **Not Unraid-coupled at all:** `internal/dockercli` (talks to the standard Docker Engine API only), `internal/backup/*`, `internal/compose`, the config domain, the `sshconn`/`virshcli` transport layer.

## 2. Platform fit

**Generic Docker host** — best fit, lowest effort. The structural model already works; cost is fixing the four items in §3 plus feature-gating the Unraid-only extras (VMs/flash default off) and shipping a compose file.

**TrueNAS Scale** — near-free once generic works. Since 24.10 it runs plain Docker + Compose with the standard `/var/run/docker.sock` path; first-party apps mount it read-write. VMs are libvirt again as of 25.04.2+, so the existing `qemu+ssh` transport works in principle, but needs a `LIBVIRT_URI` override for TrueNAS's non-standard socket path (`?socket=/run/truenas_libvirt/libvirt-sock`), and the VM path needs to handle zvol-backed disks (block devices, not files) and TrueNAS's domain-naming scheme (UUID-based in v26, not name-based).

**Proxmox VE** — a different product, not a port (see Non-goal above).

## 3. Concrete fixes (Approach A, ships first)

1. Generalize appdata/data-root discovery: configurable data roots instead of the hardcoded `appdata` segment filter; use Docker Compose labels (`com.docker.compose.project.working_dir`) as an additional, mechanically reliable discovery source, following the pattern used by Nautical Backup (`SOURCE_LOCATION` + per-container label overrides).
2. Support Docker named volumes (`Type=="volume"`, resolve via `/var/lib/docker/volumes/<name>/_data` or the daemon's reported mountpoint) — fixes a real gap on every platform including Unraid.
3. Support multiple host roots instead of a single `/mnt`. On `generic`/`truenas`, default to an identity bind (`HOST_SOURCE_ROOT == HOST_MOUNT_ROOT`, i.e. no path translation) instead of Unraid's split `/mnt` → `/host/user` scheme — it's the simpler configuration for a plain Docker host and matches how comparable tools (e.g. Nautical Backup) do it. Unraid's split-root behavior is unchanged.
4. Fix `DOCKER_HOST` handling so rootless Docker/Podman work — drop the `WithHost` override when `DOCKER_HOST` is set.

These four ship as one pass and already unlock generic-Docker containers and TrueNAS-Scale containers simultaneously.

## 4. Architecture: a thin Platform adapter (Approach B)

Introduce a `Platform` interface covering exactly the seam points identified in §1:

- data-root / appdata discovery (§3.1, generalized either way, but the interface lets a platform supply its own hints)
- restore-destination defaults (replaces the hardcoded `user/appdata` / `user/domains` literals)
- host-config / notify reconcile (the emhttp-specific step becomes a no-op outside Unraid, exactly as it already fails soft today)

Implementations: `unraid` (existing behavior, unchanged), `generic` (new), `truenas` (new; embeds `generic`, adds the `LIBVIRT_URI` override and zvol/domain-naming handling for the VM path).

**Detection:** automatic at runtime (probe for Unraid-specific paths/`emhttp`; probe for TrueNAS-specific markers; fall back to `generic`), not a required config flag. One binary, one image, one CA template / one TrueNAS catalog entry / one generic compose file.

## 5. TrueNAS Scale specifics

- Containers: work via the §3 fixes directly, no TrueNAS-specific code needed beyond the `Platform` detection itself. Confirmed against the current stable release (25.10 "Goldeye"): Apps moved from Kubernetes/k3s to plain Docker + Compose in 24.10, the host `docker.sock` is reachable at the standard path, and the official catalog's own library mounts it (read-write for apps like Dockge) — sibling-container discovery/control works the same way it does on Unraid.
- VMs: confirmed libvirt + QEMU/KVM on the current stable release (25.04 briefly shipped an Incus-based "Instances" system; 25.04.2 restored libvirt; 25.10 removes Incus from the base system entirely, libvirt now drives both VMs and LXC). This is materially more work than a single connection-string fix:
  - `LIBVIRT_URI` override for TrueNAS's non-standard libvirt socket (`qemu+ssh://<user>@<host>/system?socket=/run/truenas_libvirt/libvirt-sock`).
  - **VM disks are zvols (ZFS block devices), not qcow2 files.** BombVault's existing VM backup path assumes file-based disk images that restic can back up directly; a zvol needs a ZFS-snapshot-based backup path instead — a genuinely separate code path, not a config flag.
  - **UEFI NVRAM and TPM state live outside the zvol**, at fixed paths (`/var/db/system/vm/nvram/{id}_{name}_VARS.fd`, `/var/db/system/vm/tpm/{id}_{name}_tpm_state`) — must be captured and restored alongside the disk for a Secure-Boot/vTPM guest to actually come back up.
  - **Domain naming differs by version**: `{id}_{name}` on 25.10, VM UUID on the upcoming 26 release — any name-based VM lookup needs to handle both.
  - TrueNAS's middleware is the idiomatic control path; changes made directly via `virsh` are invisible to it and may be reconciled/overwritten, a sharper version of the same risk Unraid's own VM Manager already poses.
- **Official catalog listing is in scope for this release**, not deferred: a TrueNAS Apps community-train submission (`app.yaml` + `ix_values.yaml` + `questions.yaml` + a Jinja2-templated `templates/docker-compose.yaml`, per `github.com/truenas/apps`'s contribution process) — the structural equivalent of, and a good deal more work than, the Unraid CA template XML. This is an ongoing maintenance commitment (catalog PRs, keeping the manifest current across TrueNAS releases) that BombVault is taking on deliberately, the same way the CA template is maintained today.

## 6. Positioning

Target *identity* going forward is platform-neutral: new user-facing strings should not bake in new Unraid-only wording where a neutral term works equally well. Public positioning (README, support thread, CA listing copy) stays Unraid-first until TrueNAS support has shipped and has real users — the actual rebrand is a deliberate, separate step taken once the product has proven itself on a second platform, not before.

## 7. Competitive validation (Duplicati / Duplicacy research, 2026-08-16)

Both are file-level-only tools with no container or VM awareness. Neither has a native append-only/immutable destination (both maintainers cite the same structural reason: their own prune/maintenance operations modify existing objects, which is incompatible with WORM retention — an open, unresolved gap in both projects for years). Neither has a true automated restore-drill (Duplicati verifies one random sampled file per run; Duplicacy's `check` verifies chunk presence/hashes but never exercises the actual restore path). Neither has a free-tier fleet dashboard (Duplicati gates it behind a paid, per-machine-priced product; Duplicacy's Web Edition is explicitly one instance per machine, "bookmarks" is the maintainers' own suggested workaround). BombVault's four pillars — drills, append-only/immutable, protection scorecard, fleet-view — are confirmed real gaps in both competitors, not assumed ones. Two ideas worth a future look, unrelated to this direction: Duplicati's GFS-bucket retention UI phrasing, and Duplicacy's lock-free/fossil-collection chunk design as a reference if restic repo-locking ever becomes a real support burden.

## 8. Testing & verification plan

- Unit tests for the `Platform` interface and each implementation (data-root discovery, restore-destination defaults) mirroring the existing coverage style in `internal/api`.
- Named-volume and multi-root fixes get regression tests against the current Unraid behavior first (must not change), then generic/TrueNAS-specific cases.
- Live verification on Bottich: a generic-Docker-flavored test container (no Unraid paths available) exercising the new discovery path end-to-end, mirroring the isolated-instance discipline used for Fleet/Mesh this cycle.
- TrueNAS Scale live verification requires an actual TrueNAS Scale test instance (not available on Bottich) — flag this as a real gap to close before claiming TrueNAS support works, not just "should work per the docs."

## 9. Sequencing

Ship §3 (generic-Docker fixes) and §4 (the adapter seam) together in one pass — the seam costs little extra once the fixes are being made anyway, and avoids a later fork-shaped refactor. TrueNAS specifics (§5) follow as a second pass on top of the same seam. Proxmox and the marketing rebrand stay out of v8.0.0 entirely (§ Non-goals).
