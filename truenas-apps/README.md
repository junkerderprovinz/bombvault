# BombVault

[BombVault](https://github.com/junkerderprovinz/bombvault) backs up Docker containers and KVM/libvirt VMs with [restic](https://restic.net), and restores them by recreating the container or VM rather than only copying its files back. Incremental, deduplicated and encrypted, with off-site replication, retention, file-level restore and scheduling, all from a web UI.

Two things to know before installing. It needs the Docker socket, which is root-equivalent access, because it stops and recreates containers around backup and restore. And it needs a real host path holding the data you want backed up: TrueNAS does not allow mounting `/mnt` itself, and apps left on the default ixVolume storage keep their data under `/mnt/.ix-apps`, which cannot be mounted either, so those apps' data is out of reach while apps configured with host-path storage are fully covered.

VM backup is optional and talks to libvirt over SSH. On TrueNAS it needs `LIBVIRT_URI` to name the socket at `/run/truenas_libvirt/libvirt-sock`, since the default URI does not work there. See [the VM backup setup guide](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

Support: [GitHub issues](https://github.com/junkerderprovinz/bombvault/issues).

---

## Maintenance notes (not part of the catalog-facing description above)

This `truenas-apps/` directory is **prep work for a future submission to
[truenas/apps](https://github.com/truenas/apps)** — it has not been
submitted there. Before an actual PR can be opened, the following need a
human decision or a real TrueNAS/Docker environment to verify (see the
commit message / task report this directory was introduced in for the full
detail):

- **`app.yaml`'s `lib_version_hash`** is left empty per convention — the
  real repo's `.github/scripts/ci.py` computes and fills it in
  automatically on first run there; it cannot be meaningfully pre-computed
  outside that repo.
- **`app.yaml`'s `app_version` / `ix_values.yaml`'s image `tag`** track the
  BombVault release these files were prepared against (`8.0.0`, the bare
  image tag, not the `v`-prefixed git tag). Bump both on every BombVault
  release meant to reach this catalog listing. This is an ongoing
  commitment, not a one-time value, so do not treat "8.0.0" as fixed.
  Nothing enforces it: the two fields drifted apart once already, when
  `app_version` went to 8.0.0 and the image `tag` stayed on 7.11.1.
- **`app.yaml`'s `version`** is the chart's own version, not BombVault's.
  It stays at `1.0.0` until the app is actually listed, because a bump
  would claim a published predecessor that never existed. Once the app is
  in the catalog, bump it alongside the two fields above.
- **`app.yaml`'s maintainer entry** is `name: truenas` / `email: dev@truenas.com`
  / `url: https://www.truenas.com/`, matching every real catalog app read
  for this file and CONTRIBUTIONS.md's explicit "TrueNAS is the only
  maintainer for now" — NOT a junkerderprovinz contact address. If that
  convention ever changes upstream, update this to match.
- **`item.yaml` was missing until 2026-08-28.** Every real app in
  `ix-dev/community/` ships one (categories, `icon_url`, `screenshots`,
  tags) alongside `app.yaml`, and this directory did not have it — found by
  listing a real app's contents rather than trusting the prepared set. Its
  `icon_url` follows the catalog convention and points at
  `media.sys.truenas.net`, which is where the reviewer will host the icon;
  it is therefore a forward reference that only resolves once the icon is
  uploaded, exactly like every other app's.
- **Icon and screenshots**: `app.yaml`'s `icon` currently points at
  `raw.githubusercontent.com` (BombVault's own repo asset) and
  `screenshots: []` is empty. Real catalog apps host BOTH on
  `media.sys.truenas.net`, uploaded by the reviewer at submission time —
  per CONTRIBUTIONS.md, both the icon and any screenshot URLs are meant to
  be supplied in the PR description for the reviewer to upload to that
  CDN, not inlined here pre-submission. The `raw.githubusercontent.com`
  icon URL here is a working placeholder (the same one the Unraid CA
  template uses) that renders correctly until that re-hosting happens, but
  whoever does the actual Step-7 submission should expect the reviewer to
  replace it, and should offer the repo's existing real screenshots at
  `.github/assets/screenshots/*.png` (dashboard, recovery, containers,
  settings) in the PR description for the same treatment.
- **`bombvault.self_container_name` is REQUIRED on TrueNAS — measured, no
  longer a caveat (2026-08-27).** BombVault resolves its own container by
  inspecting its own hostname via Docker. On a live TrueNAS SCALE 25.10 box
  an app deploys as compose project `ix-<app name>`, so a service without an
  explicit `container_name` becomes `ix-<app name>-<service>-1` — which is
  never what the hostname says. Worse, two probes on the same box showed that
  with a pinned hostname BombVault identified a **different** container that
  merely happened to carry that name **as itself**: it would refuse to back
  that stranger up, and a self-restart would restart it. So the field is not
  a nicety, and its description says to set it. The empty default stays,
  because guessing a wrong name is worse than an unset one that fails
  loudly.
- **Rendering**: NOT run through the real repo's actual
  `.github/scripts/ci.py --render-only` / `--wait` / full deploy — that
  script hard-requires the `docker` CLI even for `--render-only` (checked
  at startup) and does its real rendering inside
  `docker run ghcr.io/truenas/apps_validation:latest apps_render_app render`,
  a separate packaged tool this repo doesn't ship in source form; no
  Docker is available in this environment. What WAS done instead: the
  real vendored `library/2.3.11/*.py` render engine (Container, Storage,
  Healthcheck, Environment, Ports, Portals, Render — unmodified upstream
  code) was imported directly and used to actually render
  `templates/docker-compose.yaml` against these files' own test values.
  This is an approximation of the official tool, not identical to it (no
  questions.yaml pydantic schema validation; the one `ix_volume` storage
  entry was resolved to its test host path by hand instead of replicating
  that translation step, which lives in the private `apps_render_app`
  tool, not in `library/`) — but it exercises the real library calls this
  template makes and produced a real, successful compose render, which
  was inspected directly and confirmed to show: `cap_add` with the five
  capabilities `add_caps()` declares; `healthcheck.start_period: 40s` /
  `retries: 3` (matching the Dockerfile's real HEALTHCHECK, not the
  library's 15s/5 defaults); `user: 0:0` and an x-notes security summary
  reading "User: root / Group: root"; and the Host Data volume mounted
  with `source` = the real host path and `target: /host/user` (confirming
  the split-root description above, not an identity bind).
- **VM backup on TrueNAS Scale is now verified on real hardware
  (2026-08-27), with one gap named.** On a live TrueNAS SCALE 25.10 box the
  whole zvol chain was exercised against a zvol attached to a RUNNING VM:
  the domain XML carries the `/dev/zvol/<pool>/<dataset>` path verbatim
  (libvirt does not resolve it to the `/dev/zdN` node, which is what the
  parser depends on), `zfs snapshot` works on a disk in active use, and
  `zfs send` → `restic backup --stdin` → dump → `zfs receive` round-trips
  byte-identically. What is still NOT verified is a full restore driven by
  BombVault's own Go orchestration, and throughput on a large zvol — the
  test zvol was sparse. See the main repo's `docs/vm-backup-ssh-setup.md`
  for the measurements.
- **`LIBVIRT_URI` is mandatory on TrueNAS, also now confirmed.** TrueNAS
  runs libvirtd on the non-standard socket
  `/run/truenas_libvirt/libvirt-sock`; the stock `qemu:///system` URI fails
  there outright with "Failed to connect socket to
  '/var/run/libvirt/libvirt-sock'". `questions.yaml` already asks for the
  URI, so nothing changes in these files — but reviewers and users should
  know the field is required rather than optional for VM backup.
- **Default WebUI ports** (`questions.yaml`'s `network.web_port`/`http_port`
  defaults: 3443/3000) sit outside the 30000+ range most real catalog apps
  default to — deliberately left as BombVault's own real, documented ports
  (matching the Dockerfile's `EXPOSE` and the Unraid CA template) rather
  than shifted into that range for convention's sake; the real
  `port_validation.py` run against all 424 real apps found zero conflicts
  on either port, so there's no correctness reason to change them.

**Where this stands (2026-08-27).** The two blockers that were about the
software itself are gone: the TrueNAS platform code shipped in v8.0.0, and
the hardware-verification gap is closed for the zvol path. What is left
before a PR is process, not engineering:

1. The icon and the four screenshots in `.github/assets/screenshots/` are
   offered in the PR description, for the reviewer to re-host on
   `media.sys.truenas.net`.
2. `lib_version_hash` fills itself in on the first CI run in that repo.
3. `app_version` and the image `tag` must match the release being listed.
   `TestTrueNASCatalogTracksLatestRelease` now enforces that against the
   newest file in `.github/release-notes/`, so the drift that happened once
   between 7.11.1 and 8.0.0 cannot recur silently.

The submission itself — forking `truenas/apps`, copying this directory's
contents to `ix-dev/community/bombvault/` there, and opening the PR —
remains a separate, explicitly human-gated step. It has NOT been done.
