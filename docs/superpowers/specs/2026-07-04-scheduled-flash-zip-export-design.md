# Scheduled flash ZIP export — design

**Source:** GitHub issue #28 (BaukeZwart): *"I would love to be able to schedule a flash backup as zip. That way
if things go horribly wrong and the server can't boot I can have easy access to the backup."* The user syncs
the zips off-server with Syncthing.

**Branch:** `feat/flash-zip-export` (off `main` == v4.2.0). **Stack:** Go backend + embedded React/Vite/TS SPA.

**Goal:** after a successful flash backup, automatically write the just-created snapshot out as a plain `.zip`
file to a user-chosen folder — so it drops straight into a Syncthing (or any) folder without a manual download.

## Mechanism

The flash "restore" is already an on-demand zip stream (`Service.DownloadFlashZip` → `restic.DumpZip(ctx, repo,
snapshotID, subfolder, w io.Writer, m)`, `service.go`). This feature reuses `DumpZip` writing to a **file**
instead of an HTTP response, triggered automatically after each successful flash backup.

- **Trigger (inline, non-fatal):** at the end of `Service.BackupFlash` (`service.go:4028`), after the backup
  succeeds and stats/offsite run, call a new `exportFlashZip(ctx, settings, sum.SnapshotID, mode, repo)`.
  It runs for BOTH scheduled and manual flash backups (any successful one). A failure is **logged, never
  fails the backup** — the restic snapshot is the source of truth; the zip is a convenience copy.
- **Destination:** a configured folder, `settings.FlashZipExportPath` — a relative subpath resolved under the
  host mount root exactly like the backup paths (`resolveRepo`/`paths.Resolve`, so traversal/absolute paths
  are rejected). The user points it at their Syncthing folder.
- **Atomic write:** dump to `<dir>/.flash-export.tmp.zip`, then `os.Rename` to the final name — Syncthing (or
  a reader) never sees a half-written zip. On any dump error, remove the temp file and return the error
  (logged by the caller).
- **Filename / retention** (`settings.FlashZipExportKeep int`, the user's "both via a toggle" choice):
  - `Keep == 0` → a single **`flash-latest.zip`**, overwritten each time (smallest footprint; one always-current
    file for Syncthing).
  - `Keep > 0` → a timestamped **`flash-<YYYYMMDD-HHMMSS>.zip`**; after writing, prune the export folder's
    `flash-<timestamp>.zip` files (regex-matched, so unrelated files are never touched) down to the newest
    `Keep`, deleting older ones. Keeps a rolling history so one bad backup can't overwrite the last good zip.

## Settings (new)

`internal/store/settings.go` + migrations (next free version after v49 → **v50–v52**):
- `FlashZipExportEnabled bool` (`flash_zip_export_enabled INTEGER NOT NULL DEFAULT 0`)
- `FlashZipExportPath string` (`flash_zip_export_path TEXT NOT NULL DEFAULT ''`)
- `FlashZipExportKeep int` (`flash_zip_export_keep INTEGER NOT NULL DEFAULT 0`)

Wired through `GetSettings`/`UpdateSettings` and the settings JSON DTO (`handlers.go` `settingsView` GET+PUT),
mirroring the existing flash fields.

## Frontend

On the Flash page (`web/src/pages/Flash.tsx`) — or the flash settings block, matching where the other flash
options live — add an **"Export a zip after each flash backup"** toggle that reveals a folder-path field
(`FolderBrowser`, bound to `flashZipExportPath`) and a **"Keep history"** toggle → a "keep last N" number
field (`flashZipExportKeep`, 0 = keep only the latest). Persist via the existing `getSettings`/`putSettings`.
Copy explains it lands as a plain `.zip` at that path for off-server sync. `Settings` DTO type in `api.ts`
gains the three `flashZipExport*` fields. i18n keys in `i18n.ts` (en+de) + all 24 locale files.

## Error handling

- Export path missing/unwritable → logged, backup still succeeds (surface the last export error in the flash
  status/UI is out of scope for v1; a log line is enough).
- Dump failure → temp file removed, error logged, backup unaffected.
- Prune failure → logged, non-fatal.

## Testing

- **Go:** `exportFlashZip` writes the zip atomically (temp then rename); `Keep==0` overwrites `flash-latest.zip`;
  `Keep==N` writes a timestamped file and prunes older `flash-*.zip` to N (and does NOT delete non-matching
  files); a dump error leaves no temp file and returns an error. Use a fake engine whose `DumpZip` writes
  known bytes to the writer. Settings round-trip preserves the three new fields.
- **Frontend gate:** `tsc --noEmit` + `npm run build`.

## Out of scope (YAGNI)

- A separate export *schedule* decoupled from the flash backup (piggybacking the flash backup is simpler and
  matches the request).
- Exporting other domains as zips (flash is the bootable-USB special case the user needs).
- Encrypting/compressing beyond what `restic dump`'s zip already does.

## File map

- **Modify (backend):** `internal/store/settings.go`, `internal/store/migrate.go` (v50–v52),
  `internal/api/service.go` (`exportFlashZip` + call in `BackupFlash` + `flashZipExportDir` resolution),
  `internal/api/handlers.go` (settings DTO).
- **Modify (frontend):** `web/src/pages/Flash.tsx`, `web/src/lib/api.ts` (`Settings` fields),
  `web/src/lib/i18n.ts` + `web/src/lib/locales/*.ts` (24).
- **Create:** this spec + the implementation plan.
