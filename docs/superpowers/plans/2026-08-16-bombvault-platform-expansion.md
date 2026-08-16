# BombVault v8.0.0, Direction 4/4: Platform Expansion — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Run BombVault on TrueNAS Scale (containers + VMs) and a generic Docker host (containers) in addition to Unraid, from one binary, without changing Unraid's existing behavior.

**Architecture:** Two independently shippable phases, per the design spec's sequencing.
- **Phase A** (own branch/PR, `feature/platform-generic-docker`): generalize container data-root discovery (the one blocking defect), add named-volume support, add multi-root support, fix `DOCKER_HOST`, introduce a thin `Platform` adapter interface (`unraid`/`generic`), feature-gate the Unraid-only extras, ship a generic `docker-compose.yml`. This alone unlocks generic-Docker hosts and TrueNAS-Scale *containers*.
- **Phase B** (own branch/PR, `feature/platform-truenas`, starts after Phase A merges): `truenas` `Platform` implementation, the TrueNAS libvirt socket override, zvol-aware VM disk backup, NVRAM/TPM capture for TrueNAS VMs, VM domain-name version handling, and the TrueNAS Apps community-train catalog submission.

Proxmox and the marketing rebrand are explicitly out of scope (see the design spec's Non-goals).

**Tech Stack:** Go backend (`internal/dockercli`, `internal/api/service.go`, new `internal/platform`), React/TS frontend only where a new setting/label needs surfacing, restic engine, libvirt via `qemu+ssh`.

**Design spec:** `docs/superpowers/specs/2026-08-16-bombvault-platform-expansion-design.md` — read it first, it has the full Unraid-coupling audit this plan's file:line references come from.

---

## Global Constraints

- 3-digit SemVer. NEVER tag/release without explicit approval.
- Before every push: `go build ./...`, `go vet ./...`, `gofmt -l .` (must be empty), `golangci-lint run ./...`, `go test ./internal/...`. Frontend changes additionally need `npx tsc --noEmit`, `npm run lint`, `npx vitest run`, `npm run build` (revert the incidental `web/dist/index.html` diff after).
- **Every task in this plan must preserve Unraid's current behavior exactly.** Each behavior-changing task adds a regression test that pins the existing Unraid-shaped output BEFORE adding the new generic/TrueNAS path, so a accidental change to the Unraid default is caught by CI, not discovered live.
- New env vars are added to `internal/config/config.go` following the existing `stringOr(env["X"], default)` pattern and documented in `docs/configuration.md` (English) — do not touch the 25 translated `configuration.*.md` copies for internal/dev-facing env vars (those docs are user-facing feature docs, not the full env var reference; check the existing file before assuming).
- Any new i18n key goes into `web/src/lib/i18n.ts` (en + de) immediately, then propagated to all 24 external locale files under `web/src/lib/locales/` before merge — the parity test (`web/src/lib/i18n.parity.test.ts`) enforces exact key parity across all 26 locales.
- No direct commits to `main` for code changes — branch per phase, PR, CI green, then merge (docs-only spec/plan commits are the one exception already established in this repo's history).
- Public repo; no real user data/IPs; repo prose in English.
- Live verification on Bottich (root@192.168.10.10:1004) for Phase A, mirroring the isolated-clone/throwaway-image/cleanup discipline used for Fleet/Mesh this cycle. **Phase B (TrueNAS specifics) cannot be live-verified on Bottich — flag this explicitly in the Phase B PR description as an unverified-against-real-hardware risk**, since no TrueNAS Scale test instance is available; the zvol/NVRAM/TPM code paths are reasoned from TrueNAS's public middleware source, not tested against a live box.

---

# Phase A — Generic Docker + adapter seam

### Task 1: Honor `DOCKER_HOST` (the one true bug, fixes Unraid too)

**Files:**
- Modify: `internal/dockercli/dockercli.go:40-50` (func `New`)
- Test: `internal/dockercli/dockercli_test.go` (create if it doesn't exist)

**Interfaces:** `New(...) (*Client, error)` signature unchanged.

- [ ] **Step 1: Write a failing test** asserting that when `DOCKER_HOST` is set in the environment, the client is NOT forced onto `unix:///var/run/docker.sock`. Since `client.New(...)` opts are applied in order and the SDK doesn't expose the resolved host directly, test this by asserting the *option order*: capture that `client.WithHost(...)` is only added when `os.Getenv("DOCKER_HOST") == ""`. Structure the function so this is testable — extract the option-building into a small unexported `dockerClientOpts() []client.Opt` that the test calls directly and inspects for the presence/absence of a `WithHost` option via a documented, exported sentinel (or, simpler: refactor `New` to accept the host string as a parameter with `""` meaning "let the SDK/env decide", and unit-test that parameter's plumbing instead of trying to introspect `client.Opt` internals).
- [ ] **Step 2: Run it, verify it fails** (`go test ./internal/dockercli/... -run TestDockerHost`).
- [ ] **Step 3: Fix `New`** — replace the unconditional `client.WithHost("unix:///var/run/docker.sock")` with: only call `client.WithHost(...)` when `os.Getenv("DOCKER_HOST") == ""`, defaulting to the current hardcoded socket path in that case (`client.FromEnv` already reads `DOCKER_HOST` when present and takes precedence if applied after; keep `client.FromEnv` as the last option applied so it wins). Add a doc comment: "DOCKER_HOST is honored when set (rootless Docker, Podman); otherwise defaults to the standard /var/run/docker.sock, matching every prior BombVault release."
- [ ] **Step 4: Run it, verify it passes.**
- [ ] **Step 5: `go build ./... && gofmt -l . && go vet ./...`; commit** (`fix(dockercli): honor DOCKER_HOST instead of always forcing /var/run/docker.sock`).

### Task 2: Support Docker named volumes in appdata discovery

**Files:**
- Modify: `internal/api/service.go:2727-2755` (`resolveAppdataPaths`)
- Test: `internal/api/service_test.go` (or the appropriate existing test file covering `resolveAppdataPaths` — check for one first with `grep -rn resolveAppdataPaths internal/api/*_test.go`)

**Interfaces:** `resolveAppdataPaths(...)` signature unchanged; it already receives `in.Mounts` (Docker inspect `[]types.MountPoint`). Each `MountPoint` has a `Type` field (`"bind"` or `"volume"`) and, for `Type=="volume"`, a `Name` field but no reliable `Source` — the host-side path for a named volume must be resolved separately via `client.VolumeInspect(ctx, name)` → `.Mountpoint` (e.g. `/var/lib/docker/volumes/<name>/_data`), which is itself under the Docker data-root and needs its own containment/translation the same way bind sources do.

- [ ] **Step 1: Write a failing test** that constructs a container inspect result with one `Type=="volume"` mount (no `hasSegment(..., "appdata")` match possible since volumes have no host source path in the mount struct) and asserts `resolveAppdataPaths` returns a non-empty path for it once the volume's real host mountpoint is resolved.
- [ ] **Step 2: Run it, verify it fails.**
- [ ] **Step 3: Add volume resolution.** In `resolveAppdataPaths`, for each `m := range in.Mounts` where `m.Type == "volume"`, call the Docker client's `VolumeInspect(ctx, m.Name)` (add this method to `internal/dockercli` if it doesn't already exist — check first) and use `.Mountpoint` as the host source path, running it through the same `toContainerPath`/translation logic already applied to bind sources. Named volumes are always "persistent data" by construction (there is no equivalent of a throwaway bind), so do NOT apply the `hasSegment(..., "appdata")` filter to them — include every named volume unconditionally, matching how Nautical Backup and comparable tools treat volumes.
- [ ] **Step 4: Run it, verify it passes.** Also run the FULL existing `resolveAppdataPaths` test suite to confirm bind-mount (Unraid) behavior is byte-identical to before.
- [ ] **Step 5: `go build/vet/gofmt`; `go test ./internal/api/...`; commit** (`feat(containers): back up Docker named volumes, not just appdata-segment binds`).

### Task 3: Configurable data-root discovery (the blocking defect)

This is the change that decides whether the port works at all off Unraid. Replace the hardcoded `hasSegment(path, "appdata")` filter with: (a) a configurable list of data-root segment names (default `["appdata"]`, so Unraid behavior is unchanged), (b) Docker Compose label discovery as an always-on additional source, (c) a documented per-container label override for the rare case neither convention fits.

**Files:**
- Modify: `internal/api/service.go:2727-2755` (`resolveAppdataPaths`)
- Modify: `internal/config/config.go` — add `DataRootSegments []string` (env `DATA_ROOT_SEGMENTS`, comma-separated, default `"appdata"`)
- Test: the same test file as Task 2

**Interfaces:**
- `hasSegment(path, segment string) bool` (existing) stays; call it once per configured segment instead of the single literal `"appdata"`.
- New: `composeProjectDataDir(labels map[string]string) (string, bool)` — reads `com.docker.compose.project.working_dir`; returns it (as an additional host-path candidate, translated the same way as a bind source) when present and non-empty.
- New label read (not a new mechanism, just an additional match in the existing bind-walk): a bind whose container mount carries the label key `bombvault.data` (checked on the container's `Config.Labels`, not per-mount) is included unconditionally, regardless of segment match — the documented escape hatch for `/srv/plex/config`-style layouts.

- [ ] **Step 1: Write failing tests** for: (a) a bind source matching a NON-default configured segment (e.g. `DATA_ROOT_SEGMENTS=appdata,config` and a bind under `/srv/plex/config`) is included; (b) a container with `com.docker.compose.project.working_dir` label but no matching bind is still assigned that directory as a data path; (c) a container with the `bombvault.data` label override on an otherwise-non-matching bind is included; (d) the DEFAULT config (`DATA_ROOT_SEGMENTS` unset) reproduces the exact current Unraid-only-`appdata`-segment behavior byte-for-byte (pin this as a regression test using the existing Unraid-shaped fixture).
- [ ] **Step 2: Run them, verify all four fail except (d)** (which should already pass, proving you understand the current behavior before touching it — if it fails, your test fixture is wrong, fix the test not the code).
- [ ] **Step 3: Add `DataRootSegments` to `Config`** in `internal/config/config.go`, parsed as a comma-split, trimmed, lower-cased list, default `["appdata"]` when unset. Add `composeProjectDataDir` as a small helper reading the label map. Rewrite `resolveAppdataPaths`'s bind-walk to: for each mount, include it if ANY configured segment matches (`hasSegment` loop) OR the container has the `bombvault.data` label truthy; separately, if `composeProjectDataDir` returns a path, add it as a candidate (deduplicate against already-included paths by cleaned absolute path). Keep the existing single-fallback-path behavior (Task 4 changes what that fallback IS, not whether one exists).
- [ ] **Step 4: Run all four tests, verify they pass.** Run the full `internal/api` test suite to confirm nothing else regressed.
- [ ] **Step 5: `go build/vet/gofmt`; `go test ./internal/api/...`; commit** (`feat(containers): configurable data-root segments + compose-label discovery + per-container override`).

### Task 4: Multi-root support + identity-bind default off Unraid

**Files:**
- Modify: `internal/config/config.go` — the `HostSourceRoot`/`HostMountRoot` handling
- Modify: `internal/api/mountinfo.go` — the exclude-root rule (confirm it still holds for multi-root; adjust only if the new identity-bind default actually collides, per the design spec's flagged risk)
- Modify: `internal/api/service.go:2741-2753, :5340, :6400-6419` (the `/mnt/user/appdata`-shaped fallback and `foreignContainerDestBase`) — see Task 5, these move into the `Platform` interface instead of being fixed here
- Test: `internal/paths/paths_test.go` and `internal/api/mountinfo_test.go` (extend existing coverage)

**Interfaces:** No new exported type here — this task specifically makes the EXISTING `HostSourceRoot`/`HostMountRoot` pair's defaults platform-dependent, via the `Platform` interface introduced in Task 5. Land Task 4 as: (a) confirm `paths.Resolve`/`paths.Within` already handle an identity root (`HostSourceRoot == HostMountRoot`) correctly with a test (no code change if it already works — the mechanism was already assessed as generic in the design spec's coupling audit); (b) if `mountinfo.go`'s exclude-root rule breaks under an identity root, fix it to compare against the configured `HostMountRoot` value rather than assuming it differs from `HostSourceRoot`.

- [ ] **Step 1: Write a failing test** (or a passing one that documents the assumption, if it already works) asserting `paths.Resolve`/`paths.Within` behave correctly when `HostSourceRoot == HostMountRoot` (an identity bind) — a relative path resolves and is contained exactly as it would under Unraid's split-root config.
- [ ] **Step 2: Run it.** If it already passes, this task is documentation-only — write a short doc comment on `paths.Resolve` stating "supports both a split root (Unraid: /mnt vs /host/user) and an identity root (generic/TrueNAS default)" and skip to Step 5. If it fails, continue.
- [ ] **Step 3 (only if Step 2 failed):** Fix `mountinfo.go`'s discriminator to key off `HostMountRoot` alone rather than any assumption of a distinct `HostSourceRoot`.
- [ ] **Step 4: Run the test, verify it passes; run the full `internal/paths` and `internal/api` mountinfo suites** to confirm Unraid's split-root behavior is unaffected.
- [ ] **Step 5: `go build/vet/gofmt`; `go test ./internal/paths/... ./internal/api/...`; commit** (`test(paths): confirm identity-root (generic/TrueNAS) resolution works alongside Unraid's split root` — or `fix(mountinfo): ...` if Step 3 was needed).

### Task 5: The `Platform` adapter interface

**Files:**
- Create: `internal/platform/platform.go`
- Create: `internal/platform/unraid.go`
- Create: `internal/platform/generic.go`
- Create: `internal/platform/detect.go`
- Create: `internal/platform/platform_test.go`
- Modify: `internal/config/config.go` — add `PlatformOverride string` (env `PLATFORM`, one of `""`/`unraid`/`truenas`/`generic`, default `""` meaning auto-detect)
- Modify: `internal/api/service.go` — replace the three hardcoded-literal call sites with calls through a `Platform` field on `Service`
- Modify: `cmd/bombvault/main.go` — construct and inject the detected `Platform` at startup

**Interfaces:**
```go
package platform

type Kind string

const (
    KindUnraid  Kind = "unraid"
    KindTrueNAS Kind = "truenas"
    KindGeneric Kind = "generic"
)

type Platform interface {
    Kind() Kind
    // AppdataFallback returns the last-resort absolute HOST path to try for
    // a container's persistent data when no bind/volume/compose-label/label-
    // override candidate matched anything. Empty string means "give up".
    AppdataFallback(hostMountRoot, containerName string) string
    // ForeignContainerDestBase returns the default cross-instance restore
    // destination for the containers domain when no explicit target/
    // RestoreFolder is configured.
    ForeignContainerDestBase(hostMountRoot string) string
    // ForeignVMDestBase returns the default cross-instance restore
    // destination for the vms domain when no explicit target/RestoreFolder
    // is configured.
    ForeignVMDestBase(hostMountRoot string) string
    // ReconcileContainerUpdateStatus runs whatever host-side step (if any)
    // makes the host's own UI reflect a post-backup image update. A no-op
    // on any platform without one.
    ReconcileContainerUpdateStatus(ctx context.Context, ssh *sshconn.Conn, containerName string) error
}
```
- `unraid.go`: `Unraid{}` implementing the four methods with EXACTLY today's literals — `AppdataFallback` returns `path.Join("/mnt/user/appdata", containerName)`, `ForeignContainerDestBase` returns `path.Join(hostMountRoot, "user/appdata")`, `ForeignVMDestBase` returns `path.Join(hostMountRoot, "user/domains")`, `ReconcileContainerUpdateStatus` runs today's `unraidReconcileUpdateStatusPHP` body (move it here verbatim from `service.go:3288-3318`).
- `generic.go`: `Generic{}` — `AppdataFallback` returns `""` (no convention to fall back to; rely on bind/volume/compose-label/override discovery from Task 2/3), `ForeignContainerDestBase`/`ForeignVMDestBase` return `hostMountRoot` itself (identity default, no assumed subpath), `ReconcileContainerUpdateStatus` is a no-op returning `nil`.
- `detect.go`: `func Detect(ctx context.Context, override string, flashDir string) Kind` — if `override != ""` return it directly (validated against the three known Kinds, falling back to `KindGeneric` with a logged warning on an unrecognized value); else probe `filepath.Join(flashDir, "config/plugins/dockerMan")` (the exact Unraid dockerMan marker already referenced by `FlashTemplatesDir`'s default) — if it exists, `KindUnraid`; otherwise `KindGeneric`. **Do not attempt automatic TrueNAS detection in Phase A** — there is no reliable filesystem marker visible from inside a Docker-socket-only container (TrueNAS's `ix-apps` dataset and libvirt socket are not guaranteed mounted, especially when VMs are disabled), so TrueNAS selection is explicit-only via `PLATFORM=truenas` until Phase B's `truenas` implementation exists to select. Document this honestly in the doc comment — do not claim detection that isn't real.
- `main.go`: call `platform.Detect(ctx, cfg.PlatformOverride, cfg.FlashDir)`, map `Kind` to a concrete `Platform` (Phase A: `unraid`/`generic` only; `truenas` added in Phase B; an unmapped Kind — i.e. `truenas` selected before Phase B ships — falls back to `Generic{}` with a logged warning), inject into `Service`.

- [ ] **Step 1: Write failing tests** in `platform_test.go`: `Unraid{}` returns the exact pre-existing literals for each method (pin them as constants in the test so a future edit to the literal is caught); `Generic{}` returns `""`/`hostMountRoot`/`hostMountRoot`/nil-error; `Detect` returns `KindUnraid` when the dockerMan marker directory exists under the given flash dir, `KindGeneric` when it doesn't, and returns the override verbatim (mapped) when one is set including the "unrecognized value logs a warning and falls back to generic" case.
- [ ] **Step 2: Run them, verify FAIL** (package doesn't exist yet).
- [ ] **Step 3: Create the four files** implementing the interface and detection exactly as specified above.
- [ ] **Step 4: Wire into `service.go`** — add a `platform platform.Platform` field to `Service`, replace the three call sites: `resolveAppdataPaths`'s fallback (`internal/api/service.go:2741-2753`) calls `s.platform.AppdataFallback(...)` instead of the hardcoded `/mnt/user/appdata` join; `foreignContainerDestBase` (`:6400-6419`) and `foreignVMDestBase` (`:6379-6398`) call `s.platform.ForeignContainerDestBase`/`ForeignVMDestBase` instead of their hardcoded `user/appdata`/`user/domains` joins; `reconcileUnraidUpdateStatus`'s call site calls `s.platform.ReconcileContainerUpdateStatus(...)` — move the existing PHP-over-SSH body into `unraid.go`'s implementation verbatim, delete it from `service.go` (keep the surrounding non-fatal error-handling exactly as-is, just move the platform-specific body).
- [ ] **Step 5: Wire into `main.go`** — detect the platform once at startup (after config load, before `Service` construction) and pass it in.
- [ ] **Step 6: Run all four tests, verify PASS. Run the FULL `internal/api` suite** to confirm every existing test that exercised the old hardcoded literals still passes unchanged (they should, since `Unraid{}` reproduces them exactly) — this is the single most important regression gate in Phase A.
- [ ] **Step 7: `go build/vet/gofmt/golangci-lint`; `go test ./...`; commit** (`feat(platform): introduce a Platform adapter, replacing hardcoded Unraid literals with an unraid/generic seam`).

### Task 6: Feature-gate the remaining Unraid-only extras

**Files:**
- Modify: `internal/api/dashplugin.go` — gate install/remove behind `platform.Kind() == platform.KindUnraid`
- Modify: `internal/api/service.go` — `sendUnraidNotify` call sites gated the same way
- No change needed to `internal/template/*` (already fail-soft, confirmed in the design spec's audit) or `internal/virshcli/nvram.go`'s `defaultOVMFVars` (Phase A ships containers only; VMs stay Unraid-only until Phase B)

**Interfaces:** No signature changes — each gated call site becomes `if s.platform.Kind() == platform.KindUnraid { ...existing call... }`, matching the existing best-effort/non-fatal error handling (these already log-and-continue on failure; on `generic`, skip the attempt entirely rather than let it fail and log noise every run).

- [ ] **Step 1: Write a failing test** asserting `sendUnraidNotify`'s call site is a no-op (returns without attempting the SSH command) when `s.platform.Kind() != platform.KindUnraid` — use a fake SSH conn that fails the test if `Run`/`ReadFile`/`WriteFile` is called.
- [ ] **Step 2: Run it, verify it fails.**
- [ ] **Step 3: Add the guard** at each call site (`sendUnraidNotify` in `service.go`, `digest.go`, `receiver_watch.go` per the design spec's audit) and in `dashplugin.go`'s install/remove entry points.
- [ ] **Step 4: Run it, verify it passes.** Run the full `internal/api` suite to confirm Unraid-platform behavior (the guard's `true` branch) is unchanged.
- [ ] **Step 5: `go build/vet/gofmt`; `go test ./internal/api/...`; commit** (`feat(platform): skip Unraid-only notify/dashplugin steps entirely on generic hosts`).

### Task 7: "Host system config" preset fileset (generic/TrueNAS flash-domain analogue)

**Files:**
- Modify: `internal/backup/files_orchestrator.go` or wherever named-fileset presets/defaults are seeded (check `internal/store/migrate.go` for how default filesets, if any, are currently seeded — files domain today has no presets, this adds the first one)
- Modify: `web/src/pages/Files.tsx` (or wherever the Files domain UI lists/creates filesets) — surface the preset as a one-click "Add preset: Host system config" action, generic-platform only
- Test: whichever backend test file covers fileset creation

**Interfaces:** No new backend type — this reuses the existing `files` domain's named-fileset mechanism entirely (`internal/backup/files_orchestrator.go`'s existing `FileSet{Name, Path, Excludes}` shape). "Preset" here means a pre-filled creation form / one API convenience call, not a new domain or new store type.

- [ ] **Step 1: Write a failing test** for a new small helper, e.g. `defaultHostConfigFileSet(platform.Kind) (name, path string, excludes []string)`, asserting it returns something sensible (e.g. `/etc` with `excludes: []` as a starting point — keep this conservative and editable, not a claim of completeness) on `generic`/`truenas` and returns `(_, _, nil, false)` (not offered) on `unraid`, since Unraid already has the flash domain for this purpose.
- [ ] **Step 2: Run it, verify it fails.**
- [ ] **Step 3: Add the helper** and wire a one-click "Add preset" affordance into the Files page (frontend) that calls the existing create-fileset endpoint with the helper's output, shown only when `settings.platform !== "unraid"` (surface `platform.Kind()` through the existing settings/status payload if it isn't already exposed — check `handlers.go`'s `settingsView`/status struct first).
- [ ] **Step 4: Run it, verify it passes.**
- [ ] **Step 5: `go build/vet`; `npx tsc --noEmit`; `go test`; commit** (`feat(files): offer a "Host system config" preset fileset on generic/TrueNAS platforms`).

### Task 8: Generic `docker-compose.yml` delivery artifact

**Files:**
- Create: `deploy/docker-compose.generic.yml` (or match whatever top-level convention the repo already uses for deploy artifacts — check for an existing `deploy/` or similar directory first)
- Modify: `docs/getting-started.md` (English source; do NOT touch the 25 translated copies for this — it's a new deployment-path doc, not a new user-facing key, follow whatever precedent exists for platform-specific getting-started content, or add a new top-level doc page if none does)

**Interfaces:** N/A — this is a deployment artifact + docs, not code.

- [ ] **Step 1:** Write `docker-compose.yml` covering: the image (`ghcr.io/junkerderprovinz/bombvault:latest` or the Docker Hub mirror, match whatever the CA template currently points at), `hostname: bombvault` (pin — required for stable restic repo locks, per the design spec), `/config` volume, `/var/run/docker.sock:/var/run/docker.sock`, an identity-bind host data mount (`./data:/host/user` documented as "replace `./data` with your actual data root, matching the container path"), `APP_KEY` env var with a comment on generating one (`openssl rand -hex 32`), `restart: unless-stopped`, and (commented out, opt-in) the `extra_hosts: ["host.docker.internal:host-gateway"]` + `LIBVIRT_*` block for VM support with a comment explaining it's optional and needs a libvirtd host reachable over SSH.
- [ ] **Step 2:** Add a short "Generic Docker host" section to the getting-started doc pointing at the compose file, stating plainly what does NOT apply (flash domain, Unraid-specific notify) per the design spec's Non-goals framing.
- [ ] **Step 3: Commit** (`docs: generic Docker host deployment via docker-compose.yml`).

## Phase A — Live Verification & Ship

- [ ] Live-verify on Bottich: clone `feature/platform-generic-docker` fresh, build a throwaway image, run ONE isolated test instance with `PLATFORM=generic` and NO Unraid-shaped mounts (plain identity-bind host data dir, a couple of test containers using a mix of bind mounts under a non-"appdata" path, named volumes, and compose labels) — confirm each container's data is correctly discovered and backed up, confirm a second isolated Unraid-flavored instance (`PLATFORM=unraid` or auto-detected) still behaves byte-identically to before this phase. Clean up fully afterward.
- [ ] Open PR, `gh pr checks --watch`, ask before merge (per this repo's own-repo-PR-merge convention), merge, sync local `main`, delete branch.
- [ ] Update the vault BombVault.md "Offene Punkte" entry for direction 4/4 with Phase A's PR number and status.

---

# Phase B — TrueNAS Scale specifics (starts after Phase A merges)

### Task 9: `truenas` Platform implementation + libvirt socket override

**Files:**
- Create: `internal/platform/truenas.go`
- Modify: `internal/platform/detect.go` — TrueNAS is explicit-only (`PLATFORM=truenas`), per Task 5's documented limitation; this task does not add auto-detection, it adds the implementation `PLATFORM=truenas` selects
- Modify: `internal/config/config.go` — add `LibvirtURI string` (env `LIBVIRT_URI`, default `""` meaning "build from LIBVIRT_HOST/USER/PORT as today")
- Modify: `internal/sshconn/sshconn.go:67-70` (`VirshURI`) — when `LibvirtURI` is explicitly set, use it verbatim instead of building the `qemu+ssh://...` string
- Test: `internal/platform/platform_test.go`, `internal/sshconn/sshconn_test.go`

**Interfaces:**
- `truenas.go`: `TrueNAS{}` embeds `Generic{}` (containers behave identically to generic) and overrides nothing on the `Platform` interface itself — the VM-specific behavior (Tasks 10-12) is NOT part of the `Platform` interface (it's VM-domain-specific, not appdata/restore-dest-specific), so this file may end up being a thin type alias initially; do not add interface methods speculatively — only add what Tasks 10-12 actually need once written.
- `sshconn.VirshURI()`: when `c.explicitURI != ""`, return it directly; otherwise build as today. Wire `LibvirtURI` from config into `sshconn.New(...)`'s existing constructor (add a parameter or a `WithExplicitURI` option — match whatever style `sshconn.New`'s existing parameter list already uses).

- [ ] **Step 1: Write a failing test** asserting `VirshURI()` returns the configured `LIBVIRT_URI` verbatim when set (e.g. `qemu+ssh://root@truenas.local/system?socket=/run/truenas_libvirt/libvirt-sock`), and falls back to today's built string when unset.
- [ ] **Step 2: Run it, verify it fails.**
- [ ] **Step 3: Implement** the `explicitURI` field/option on `sshconn.Conn` and the config plumbing.
- [ ] **Step 4: Run it, verify it passes.** Run the full `internal/sshconn` suite to confirm the default (unset) path is unchanged.
- [ ] **Step 5: `go build/vet/gofmt`; `go test ./internal/sshconn/... ./internal/platform/...`; commit** (`feat(vms): LIBVIRT_URI override for non-standard libvirt sockets (TrueNAS Scale)`).
- [ ] **Step 6:** Document in `docs/vm-backup-ssh-setup.md` (English source) the TrueNAS-specific setup: `LIBVIRT_URI=qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock`, the requirement to enable "Log In As Root with Password" (or a sudo-capable admin + API key) since root SSH is disabled by default since TrueNAS 24.10, and the caveat that direct `virsh` state changes are invisible to TrueNAS's middleware and may be reconciled/overwritten — same risk class as Unraid's own VM Manager, sourced from TrueNAS's public middleware docs, not verified live (no test instance available).

### Task 10: zvol-aware VM disk backup path

This is the largest, least-certain task in the plan — it needs a concrete mechanism, reasoned from TrueNAS's documented architecture, but UNVERIFIED against a real TrueNAS box. Flag this clearly wherever it's mentioned (PR description, vault note) as needing live-hardware validation before it's trusted.

**Files:**
- Modify: `internal/virshcli/virshcli.go` — disk-path parsing already exists (`ParseDomain`/`domainXML`, per the design spec's audit); extend it to detect a `<disk>` whose `<source dev="...">` (block device) rather than `<source file="...">` is used — this is the existing libvirt-XML-level signal that a disk is a zvol/raw block device, not a qcow2 file.
- Create: `internal/virshcli/zvol.go` — snapshot-and-stream helpers
- Modify: `internal/backup/vm_orchestrator.go` — branch on disk kind (file vs block device) when building the restic backup source list
- Test: `internal/virshcli/zvol_test.go` (unit-testable argv/command construction only — cannot integration-test against a real zvol without TrueNAS hardware; say so in the test file's package doc)

**Interfaces:**
- `ParseDomain`'s existing disk struct gains a `IsBlockDevice bool` field (true when `<source dev="...">` was used instead of `<source file="...">`).
- New: `func ZFSSnapshotArgs(dataset, snapName string) []string` returning `["zfs", "snapshot", dataset+"@"+snapName]`, executed over the existing SSH transport (`sshconn.Conn.Run`, the same mechanism already used for `virsh`/NVRAM commands) — mirrors how `virshcli` already shells commands over SSH rather than requiring a local zfs toolchain in the container.
- New: `func ZFSSnapshotDestroyArgs(dataset, snapName string) []string` for cleanup after the backup completes (a snapshot is a live consistency point, not the backup itself — always clean it up, success or failure, via a `defer`).
- The zvol's dataset path (`<pool>/<dataset>` form, needed for the `zfs` command) must be derived from the block device path in the domain XML (typically `/dev/zvol/<pool>/<dataset>`) — add `func zvolDatasetFromDevPath(devPath string) (string, bool)` parsing that convention, with a clear error when the path doesn't match it (do not guess).
- Backup mechanism: after snapshotting, stream the snapshot's block content over SSH (`ssh ... "zfs send <dataset>@<snap>"`, piped into restic via stdin — mirror the existing pattern `internal/restic/restic.go`'s `DumpZip` already uses for streaming content into a restic backup without a local file) rather than attempting a `dd`-based raw block copy, since `zfs send` is the ZFS-native, documented way to get a stable point-in-time byte stream off a dataset/zvol without mounting or converting it.
- Restore mirrors this: `zfs receive` on the target dataset, streamed from restic's restore output — this is a NEW restore path, not a reuse of the existing file-restore code, since the destination is a raw dataset, not a directory of files. Design this restore path defensively: since receiving into an EXISTING dataset can destroy data, always receive into a freshly-named dataset (e.g. `<pool>/<dataset>-bombvault-restore-<timestamp>`) and require an explicit follow-up step (documented, not automated) to rename/promote it over the original — do not auto-overwrite a live zvol.

- [ ] **Step 1: Write failing unit tests** for `zvolDatasetFromDevPath` (valid `/dev/zvol/pool/dataset` parses; a file-backed path or malformed dev path returns `false`), `ZFSSnapshotArgs`/`ZFSSnapshotDestroyArgs` (exact argv), and `ParseDomain` correctly setting `IsBlockDevice` for a `<source dev="...">` fixture vs `false` for the existing `<source file="...">` fixture.
- [ ] **Step 2: Run them, verify FAIL.**
- [ ] **Step 3: Implement** each piece as specified above.
- [ ] **Step 4: Run them, verify PASS.** Run the full `internal/virshcli` suite to confirm existing (file-backed, Unraid) VM disk parsing is unchanged.
- [ ] **Step 5:** Wire the branch into `vm_orchestrator.go`: when a domain's disk `IsBlockDevice`, take the snapshot → stream → restic-backup → destroy-snapshot path instead of the existing file-copy path; leave file-backed disks completely untouched (same code path as today).
- [ ] **Step 6: `go build/vet/gofmt/golangci-lint`; `go test ./internal/virshcli/... ./internal/backup/...`; commit** (`feat(vms): zvol-aware backup path for block-device-backed VM disks (TrueNAS)`).
- [ ] **Step 7:** Add a prominent note to the Phase B PR description and to `docs/vm-backup-ssh-setup.md`'s TrueNAS section: this path is reasoned from TrueNAS's public ZFS/libvirt documentation and unit-tested for argv correctness, but has NOT been exercised against a real TrueNAS box — recommend a manual verification pass (a real TrueNAS Scale instance, a small test VM, backup + restore-to-a-fresh-dataset + boot check) before this ships in a tagged release, and track that as a follow-up.

### Task 11: NVRAM/TPM capture and restore for TrueNAS VMs

**Files:**
- Modify: `internal/virshcli/nvram.go` — extend NVRAM handling to also read/write TPM state; add TrueNAS's fixed paths as recognized locations
- Modify: `internal/api/service.go` (the NVRAM backup/restore call sites, `:5817-5826` backup read and `:6125-6143` restore write-back per the design spec's audit) — generalize to also carry TPM state through the same best-effort, non-fatal SSH read/write pattern

**Interfaces:**
- The existing NVRAM bytes are stored inline in the definition/DB (`NVRAMHostPath`, `NVRAMBytes` fields, per the design spec's audit) — add parallel `TPMHostPath`, `TPMStateBytes` fields to the same struct, following the exact same "best-effort, logged, non-fatal" read/write pattern already used for NVRAM (do not make TPM state a hard requirement — a VM without vTPM has none, and a failed TPM read/write should degrade the same way a failed NVRAM one does today).
- TrueNAS's fixed paths (`/var/db/system/vm/nvram/{id}_{name}_VARS.fd`, `/var/db/system/vm/tpm/{id}_{name}_tpm_state`) are NOT hardcoded as a fallback the way Unraid's `defaultOVMFVars` is — TrueNAS's domain XML already carries the real NVRAM path via `<os><nvram>`, and this task should confirm (from the sourced TrueNAS documentation, or live once hardware is available) whether the TPM device path is similarly discoverable from the domain XML (`<tpm>` element) rather than needing a guessed/reconstructed path — prefer parsing it from the XML if the element exists, and only fall back to the documented fixed-path convention if the XML doesn't carry it.

- [ ] **Step 1: Write a failing test** for parsing a `<tpm>` element out of a domain XML fixture (mirroring how `<os><nvram>` is already parsed) and for the new `TPMHostPath`/`TPMStateBytes` fields round-tripping through backup/restore the same way NVRAM fields do today.
- [ ] **Step 2: Run it, verify it fails.**
- [ ] **Step 3: Implement** the XML parsing addition and the backup/restore call-site generalization, mirroring the existing NVRAM code path exactly (same non-fatal error handling, same "logged and never affects the backup/restore outcome" contract).
- [ ] **Step 4: Run it, verify it passes.** Run the full `internal/virshcli` and relevant `internal/api` suites to confirm NVRAM-only (Unraid) behavior is unaffected — TPM handling should be purely additive.
- [ ] **Step 5: `go build/vet/gofmt`; `go test`; commit** (`feat(vms): capture/restore TPM state alongside NVRAM for vTPM-enabled guests`).
- [ ] **Step 6:** Same "unverified against real hardware" flag as Task 10 — add to the Phase B PR description.

### Task 12: VM domain-name version handling

**Files:**
- Modify: `internal/virshcli/virshcli.go` — wherever a VM is looked up or matched by name (check `List`/`Inspect`/domain-lookup call sites)
- Test: `internal/virshcli/virshcli_test.go`

**Interfaces:** Add `func normalizeDomainName(raw string) (friendlyName string, isVersioned26Style bool)` — TrueNAS 25.10's libvirt domain name is `{id}_{name}` (e.g. `1_debian`); TrueNAS 26's is the VM's UUID with the friendly name in `<title>` instead. Detect the `{digits}_{rest}` pattern for the 25.10 style; for the UUID style, read `<title>` from the parsed domain XML as the friendly name instead of `<name>`. This function is TrueNAS-specific in purpose but harmless on Unraid (Unraid domain names never match either pattern, so both checks simply fall through to using the raw name as-is, unchanged from today).

- [ ] **Step 1: Write failing tests** for: an Unraid-style plain name passes through unchanged; a `{id}_{name}` TrueNAS-25.10-style name extracts the friendly name; a UUID-style name (with a `<title>` element present in the domain XML) resolves the friendly name from `<title>`.
- [ ] **Step 2: Run them, verify FAIL.**
- [ ] **Step 3: Implement** `normalizeDomainName` and wire it into the domain-lookup/listing call sites, keeping the raw libvirt name as the identifier used for actual `virsh` commands (never the friendly name) and using the normalized friendly name only for display/matching against BombVault's own stored VM records.
- [ ] **Step 4: Run them, verify PASS.** Run the full `internal/virshcli` suite to confirm Unraid VM listing/lookup is unaffected.
- [ ] **Step 5: `go build/vet/gofmt`; `go test ./internal/virshcli/...`; commit** (`feat(vms): handle TrueNAS's versioned domain-naming schemes (id_name vs UUID+title)`).

### Task 13: TrueNAS Apps community-train catalog submission

**Files:**
- Create: `truenas-apps/app.yaml`
- Create: `truenas-apps/ix_values.yaml`
- Create: `truenas-apps/questions.yaml`
- Create: `truenas-apps/templates/docker-compose.yaml` (Jinja2)
- Create: `truenas-apps/README.md`
- Create: `truenas-apps/templates/test_values/default.yaml` (per `github.com/truenas/apps`'s contribution requirements)

This is a submission to an EXTERNAL repository (`github.com/truenas/apps`, community train), not a change to BombVault's own runtime. Build and validate it in this repo first (so it's reviewable and versioned alongside the release it targets), then open the actual PR against `truenas/apps` as a separate, explicit step — do not silently open an external PR without a checkpoint, per this project's own-repo-vs-foreign-repo distinction (this counts as foreign-repo territory even though BombVault owns the content, since it's merged by TrueNAS's maintainers under their process).

- [ ] **Step 1:** Write `app.yaml` (title, description, icon reference, `app_version` matching BombVault's release version, `lib_version` pinned to the current `truenas/apps` library version referenced in the design spec's research, `run_as_context` for the required UID/GID).
- [ ] **Step 2:** Write `ix_values.yaml` (image reference — the same GHCR/Docker Hub image Phase A/B already ship — and any static constants the compose template needs).
- [ ] **Step 3:** Write `questions.yaml` — the TrueNAS UI form schema, covering (at minimum) `APP_KEY`, the data-root host-path picker, an optional VM/libvirt-SSH settings block matching Task 9's `LIBVIRT_URI`/`LIBVIRT_HOST`/`LIBVIRT_SSH_*` env vars, mirroring the Unraid CA template's `<Config>` field set field-for-field where a TrueNAS equivalent exists.
- [ ] **Step 4:** Write `templates/docker-compose.yaml` (Jinja2) — the TrueNAS-rendered equivalent of Task 8's generic compose file, using `add_docker_socket()` (read-write, per the design spec's finding that this is the first-party-supported pattern) instead of a raw bind, and TrueNAS's `ix_volumes`/host-path conventions instead of a plain bind for `/config`.
- [ ] **Step 5:** Write `README.md` (what BombVault is, linking back to the main repo) and a minimal `test_values/default.yaml` covering the required fields for `truenas/apps`'s own CI (`ci.py --render-only`) to render the template without errors — run that CI script locally if it's vendorable/runnable without a live TrueNAS instance (check `truenas/apps/.github/scripts/ci.py`'s requirements first; if it needs a live box, document that this step is unverified and flag it the same way as Tasks 10-11).
- [ ] **Step 6: Commit** these files to BombVault's own repo under `truenas-apps/` (`docs: TrueNAS Apps catalog submission source (app.yaml/questions.yaml/compose template)`).
- [ ] **Step 7 (separate, explicit, only after Phase B's code is merged and tagged):** Fork `truenas/apps`, copy `truenas-apps/*` into the correct `ix-dev/community/bombvault/` location per their contribution doc, open the PR — this step needs its own go-ahead, do not do it automatically as part of finishing this task list.

## Phase B — Live Verification & Ship

- [ ] Flag prominently (PR description + vault note) that Tasks 10 and 11 (zvol backup, NVRAM/TPM) are unverified against real TrueNAS hardware — no test instance is available. Recommend acquiring or renting TrueNAS Scale test access before tagging a release that claims TrueNAS VM support works, not just "should work per the docs."
- [ ] Everything else (containers on TrueNAS, the `LIBVIRT_URI` override itself, domain-name handling) can be reasoned/tested at the unit level but should still get a live pass if/when a test instance becomes available.
- [ ] Open PR, `gh pr checks --watch`, ask before merge, merge, sync local `main`, delete branch.
- [ ] Update the vault BombVault.md entry: mark direction 4/4 complete, note the TrueNAS-hardware-verification gap explicitly as an open follow-up rather than a closed item.
- [ ] Only after this: revisit the originally-planned full BombVault code review (Opus + Ultracode), now that all four v8.0.0 directions are shipped.

---

## Self-Review

- Spec coverage: §3 (Task 1-4), §4/Platform interface (Task 5), Unraid-only feature gating (Task 6), flash-domain analogue (Task 7), delivery (Task 8) — all of Phase A's spec sections covered. §5 TrueNAS specifics: libvirt socket (Task 9), zvol backup (Task 10), NVRAM/TPM (Task 11), domain naming (Task 12), catalog (Task 13) — all covered.
- Every task that touches existing Unraid-serving code (Tasks 2, 3, 4, 5, 6) pins current Unraid behavior with a regression test BEFORE adding new behavior, per the Global Constraints.
- No task silently changes a default that would affect an existing Unraid installation — `Unraid{}`'s `Platform` methods are required to return byte-identical values to today's hardcoded literals, tested directly.
- Tasks 10/11 (the two genuinely uncertain pieces) are explicitly flagged as unverified-against-hardware in both the task steps and the Phase B ship checklist — this is a deliberate, stated gap, not a silently-dropped one, matching the design spec's own §8 testing-plan admission that TrueNAS live verification isn't possible on Bottich.
- Task 13 (external catalog PR) is scoped to "prepare the files in this repo" with the actual external submission as a separate, explicitly-gated step — matches this project's foreign-repo-vs-own-repo handling conventions even though the content originates here.
