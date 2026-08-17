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
  BombVault release these files were prepared against (`7.11.1`, the bare
  image tag — not the `v`-prefixed git tag). Bump both, plus `app.yaml`'s
  `version`, on every BombVault release meant to reach this catalog
  listing. This is an ongoing commitment, not a one-time value — do not
  treat "7.11.1" as fixed.
- **`app.yaml`'s maintainer email** (`junkerderprovinz@users.noreply.github.com`)
  is GitHub's standard privacy-preserving noreply address for this
  account, used as a real, working, publishable placeholder — confirm it's
  the address actually wanted before submitting (the maintainer may prefer
  the exact numeric-ID noreply form GitHub issues, or a different address
  entirely).
- **Screenshots**: `app.yaml`'s `screenshots: []` is intentionally empty —
  per the real CONTRIBUTIONS.md, screenshot URLs are supplied in the PR
  description for the reviewer to upload to TrueNAS's CDN, not inlined
  here pre-submission. The repo already has real ones at
  `.github/assets/screenshots/*.png` (dashboard, recovery, containers,
  settings) worth offering.
- **`bombvault.self_container_name`'s empty default** (in `questions.yaml`)
  is a deliberate, honestly-caveated choice, not an oversight: BombVault's
  self-detection normally resolves its own container by inspecting its own
  hostname via Docker, but TrueNAS's compose deployment may give the
  actual Docker container a different real name than what the Apps UI
  shows. This is unverified without a live TrueNAS box — see the
  field's own description and `templates/docker-compose.yaml`'s comments.
- **Rendering** was checked with a standalone Jinja2 syntax check only
  (see the task report) — it was NOT run through the real repo's
  `.github/scripts/ci.py --render-only`, `--wait`, or full deploy, so the
  library-specific calls (`add_docker_socket`, `add_storage`,
  `set_hostname`, `healthcheck.set_custom_test`, the manual `host_path`
  IxStorage dict for Host Data, etc.) are grounded in reading the real
  `library/2.3.11/*.py` source, not exercised end to end.
- **VM backup on TrueNAS Scale** carries the same caveat already documented
  in the main repo's `docs/vm-backup-ssh-setup.md`: reasoned from public
  TrueNAS documentation, not exercised against real TrueNAS Scale
  hardware.

Once these are resolved (and Phase B's TrueNAS code on this branch is
merged and tagged), the actual submission — forking `truenas/apps`,
copying this directory's contents to `ix-dev/community/bombvault/` there,
and opening the PR — is a separate, later, explicitly human-gated step.
