# BombVault

[BombVault](https://github.com/junkerderprovinz/bombvault) is a self-hosted backup and full disaster-recovery tool for Docker containers and KVM/libvirt VMs, powered by [restic](https://restic.net). It runs as a single container with a web UI: back up appdata and VM disks, then restore with one click — containers and VMs are automatically re-created, not just their files copied back.

Incremental, deduplicated and encrypted by default, with off-site replication (SMB/NFS/S3/rclone/SSH), immutable/append-only off-site copies with tamper verification, per-source retention, file-level restore, restore-verification drills, scheduling, and pre/post-backup hooks — all configured in the WebUI.

BombVault needs the Docker socket (root-equivalent) to stop/start/recreate containers around backup and restore, and a real host path covering the other apps' persistent data it backs up (**Host Data** below). VM backup is optional and talks to libvirt over SSH — no libvirt mount required.

See the [main README](https://github.com/junkerderprovinz/bombvault/blob/main/README.md) for full documentation, and [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) for VM backup setup, including the TrueNAS Scale-specific `LIBVIRT_URI` this catalog form asks for.

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
- **`bombvault.self_container_name`'s empty default** (in `questions.yaml`)
  is a deliberate, honestly-caveated choice, not an oversight: BombVault's
  self-detection normally resolves its own container by inspecting its own
  hostname via Docker, but TrueNAS's compose deployment may give the
  actual Docker container a different real name than what the Apps UI
  shows. This is unverified without a live TrueNAS box — see the
  field's own description and `templates/docker-compose.yaml`'s comments.
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
- **VM backup on TrueNAS Scale** carries the same caveat already documented
  in the main repo's `docs/vm-backup-ssh-setup.md`: reasoned from public
  TrueNAS documentation, not exercised against real TrueNAS Scale
  hardware.
- **Default WebUI ports** (`questions.yaml`'s `network.web_port`/`http_port`
  defaults: 3443/3000) sit outside the 30000+ range most real catalog apps
  default to — deliberately left as BombVault's own real, documented ports
  (matching the Dockerfile's `EXPOSE` and the Unraid CA template) rather
  than shifted into that range for convention's sake; the real
  `port_validation.py` run against all 424 real apps found zero conflicts
  on either port, so there's no correctness reason to change them.

Once these are resolved (and Phase B's TrueNAS code on this branch is
merged and tagged), the actual submission — forking `truenas/apps`,
copying this directory's contents to `ix-dev/community/bombvault/` there,
and opening the PR — is a separate, later, explicitly human-gated step.
