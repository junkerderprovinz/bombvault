# Foreign-Repo Restore Design (Issue #61)

**Goal:** restore a container/VM (and, with the files domain, file sets) from
ANOTHER BombVault instance's repo — e.g. move a container from server A to
server B — without touching the local domain/repo settings. Today manilx must
temporarily repoint his own repo settings and revert them afterwards (#61).

**Decision (user, 2026-07-14):** lives inside the existing **Recovery page** as an
"other repo" mode, no separate Migrate pane.

## Concept

A **one-time, non-persisted repo session**:

1. **Connect:** user enters the foreign repo location + credentials. Support the
   same storage forms the app already understands (local/mounted path first and
   foremost — manilx mounts server A's share on server B; reuse the existing
   repo-location + key input widgets from Recovery attach). Validation opens the
   repo read-only and lists snapshots; nothing is written to settings.
2. **Browse:** snapshots grouped per domain/item exactly like Discover does for a
   bare repo (reuse the Discover machinery — it already reconstructs entries from
   storage).
3. **Restore:** pick an item snapshot and run the EXISTING restore paths
   (container recreation from stored defs, VM define + disk restore, file-set
   restore into a chosen directory) with streaming progress. The restored object
   becomes a normal local container/VM afterwards.

## Hard guards

- **Read-only against the foreign repo:** never prune/forget/write; no retention
  applies; the session must not leak into `Settings` (in-memory only, gone after
  the session/page ends).
- The existing Recovery attach flow OVERWRITES the local domain repo settings —
  this mode must NOT reuse that persistence path. Guard with a test that settings
  are byte-identical after a foreign-repo session.
- Restores collide like any restore: existing container names/VM names prompt the
  same conflict handling the normal restore uses (no silent overwrite).
- Auth: all new endpoints behind the authGate; every endpoint the frontend calls
  is listed in the proxy allowlist (parity rule).

## API sketch

- `POST /api/foreign/open` { location, key } → session id + snapshot inventory
  (domain → items → snapshots). Session state in memory with TTL; no disk writes.
- `POST /api/foreign/restore` { session, domain, item, snapshot, options } →
  async run id, watched via the existing runs/progress infrastructure.
- Session close/expiry endpoint or implicit TTL — keep it simple.

## UI

- Recovery page gains a clearly separated card/mode: "Restore from another
  BombVault repo" with the 3 steps above (connect → browse → restore), using the
  existing wizard/card widgets and progress components.
- Copy makes the semantics explicit: "reads the other repo, changes nothing over
  there, and leaves your own backup settings untouched."
- All new strings translated in ALL 26 locales, parity check green.

## Non-goals (this release)

- No live server-to-server federation/talking instances (ptmorris1's larger idea
  in #61 — explicitly out of scope, mention in the eventual issue reply).
- No off-site cloud targets for the foreign session beyond what the existing
  repo-location widget supports.
