# Scheduled flash ZIP export — implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development. Steps use `- [ ]`.

**Goal:** after a successful flash backup, write the snapshot out as a plain `.zip` to a user-chosen folder
(#28). **Spec:** `docs/superpowers/specs/2026-07-04-scheduled-flash-zip-export-design.md`.

## Global Constraints
- Branch `feat/flash-zip-export` (off `main` == v4.2.0). Sequential implementers.
- Migrations start at **v50** (current max 49). Go gates: `go build ./... && go vet ./...`, `gofmt -l` empty,
  `go test ./... -count=1`, `golangci-lint run ./internal/...` 0. Frontend: `tsc --noEmit` + `npm run build`.
- No AI attribution in commits (controller commits). i18n keys in en+de then all 24 locale files.

---

### Task 1: Settings + migrations + DTO
**Files:** `internal/store/settings.go`, `internal/store/migrate.go`, `internal/api/handlers.go` (settingsView), tests.
**Produces:** `store.Settings.FlashZipExportEnabled bool`, `FlashZipExportPath string`, `FlashZipExportKeep int`.

- [ ] Add the three struct fields (near the other `Flash*` fields) + GetSettings SELECT/Scan (int scan var for the bool) + UpdateSettings SET/Exec, mirroring the flash fields exactly.
- [ ] Append migrations v50–v52 to `migrate.go` (mirror the v44–v49 config block style):
  - v50 `settings_flash_zip_export_enabled`: `ALTER TABLE settings ADD COLUMN flash_zip_export_enabled INTEGER NOT NULL DEFAULT 0;`
  - v51 `settings_flash_zip_export_path`: `ALTER TABLE settings ADD COLUMN flash_zip_export_path TEXT NOT NULL DEFAULT '';`
  - v52 `settings_flash_zip_export_keep`: `ALTER TABLE settings ADD COLUMN flash_zip_export_keep INTEGER NOT NULL DEFAULT 0;`
- [ ] Add the three `flashZipExport*` JSON fields to `settingsView` (handlers.go) + both GET (`toView`) and PUT mappings, mirroring `flashEnabled`/`flashPath`.
- [ ] Test `TestSettingsFlashZipExportRoundTrip` (mirror `TestSettingsConfigFieldsRoundTrip`): set all three, UpdateSettings, GetSettings, assert.
- [ ] Gates + commit: `feat(flash): settings + migrations for scheduled zip export (#28)`.

---

### Task 2: exportFlashZip + BackupFlash hook
**Files:** `internal/api/service.go`, `internal/api/service_test.go` (internal pkg for the unexported fn).
**Consumes:** the three settings (Task 1); `s.engine.DumpZip`; `s.cfg.FlashDir`; the repo/mode already resolved in `BackupFlash`.

- [ ] **flashZipExportDir:** resolve `settings.FlashZipExportPath` under the host mount root using the SAME helper `flashRepoPath`/`configRepoPath` use (grep — likely `s.resolveRepo(...)` / `paths.Resolve(s.cfg.HostMountRoot, path)`; match it so traversal/absolute paths are rejected).
- [ ] Implement `exportFlashZip` + `pruneFlashZips`:
```go
var flashZipRe = regexp.MustCompile(`^flash-\d{8}-\d{6}\.zip$`)

// exportFlashZip writes the just-backed-up flash snapshot to the configured folder
// as a plain .zip, for off-server sync. Non-fatal: any failure is returned to the
// caller (which logs it); it never fails the backup.
func (s *Service) exportFlashZip(ctx context.Context, settings store.Settings, snapshotID string, mode restic.Mode, repo string) error {
	if !settings.FlashZipExportEnabled || settings.FlashZipExportPath == "" {
		return nil
	}
	dir, err := s.flashZipExportDir(settings)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("flash zip export: mkdir: %w", err)
	}
	tmp := filepath.Join(dir, ".flash-export.tmp.zip")
	f, err := os.Create(tmp) //nolint:gosec // G304: dir is an operator-configured path under the host mount root
	if err != nil {
		return fmt.Errorf("flash zip export: create temp: %w", err)
	}
	dumpErr := s.engine.DumpZip(ctx, repo, snapshotID, s.cfg.FlashDir, f, mode)
	closeErr := f.Close()
	if dumpErr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("flash zip export: dump: %w", dumpErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("flash zip export: close temp: %w", closeErr)
	}
	name := "flash-latest.zip"
	if settings.FlashZipExportKeep > 0 {
		name = "flash-" + time.Now().Format("20060102-150405") + ".zip"
	}
	if err := os.Rename(tmp, filepath.Join(dir, name)); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("flash zip export: finalize: %w", err)
	}
	if settings.FlashZipExportKeep > 0 {
		s.pruneFlashZips(dir, settings.FlashZipExportKeep)
	}
	return nil
}

// pruneFlashZips keeps the newest `keep` timestamped flash-*.zip files, deleting
// older ones. Best-effort; only files matching the exact flash-<ts>.zip pattern
// are ever touched (flash-latest.zip and unrelated files are left alone).
func (s *Service) pruneFlashZips(dir string, keep int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var zips []string
	for _, e := range entries {
		if !e.IsDir() && flashZipRe.MatchString(e.Name()) {
			zips = append(zips, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(zips))) // timestamp names sort chronologically → newest first
	if keep >= len(zips) {
		return
	}
	for _, name := range zips[keep:] {
		_ = os.Remove(filepath.Join(dir, name))
	}
}
```
  (If `time.Now().Format` is disallowed by any lint, it isn't — this is normal Go. Add imports `os`, `regexp`, `sort`, `time`, `path/filepath` as needed.)
- [ ] **Hook into BackupFlash** (`service.go:4028`): after `s.maybeCollectStats(ctx, "flash")`, before `return sum, nil`:
```go
	if err := s.exportFlashZip(ctx, settings, sum.SnapshotID, mode, repo); err != nil {
		log.Printf("flash zip export failed (backup is still valid): %v", err)
	}
```
- [ ] Tests (`service_test.go`, use a fake engine whose `DumpZip` writes known bytes):
  - `Keep==0`: after export, `<dir>/flash-latest.zip` exists with the bytes, no temp file remains; a second export overwrites it.
  - `Keep==2`: three exports (vary the timestamp — inject via distinct calls; if `time.Now()` collides at second-resolution, have the test write pre-existing `flash-<ts>.zip` files then call `pruneFlashZips(dir, 2)` directly and assert only the newest 2 remain and a non-matching `keepme.zip` survives).
  - dump error → temp file removed, error returned, no `flash-*.zip` written.
  - disabled or empty path → no-op, nil.
- [ ] Gates + commit: `feat(flash): export snapshot as zip after each flash backup (#28)`.

---

### Task 3: Frontend — Flash page export controls + api.ts
**Files:** `web/src/pages/Flash.tsx`, `web/src/lib/api.ts`, `web/src/lib/i18n.ts` (en+de only).

- [ ] `api.ts`: add `flashZipExportEnabled: boolean`, `flashZipExportPath: string`, `flashZipExportKeep: number` to the `Settings` type (near the flash fields).
- [ ] `Flash.tsx`: add a settings block (persisted via existing `getSettings`/`putSettings`, matching how Flash.tsx or Settings.tsx persists flash options): an **"Export a zip after each flash backup"** toggle → reveals a `FolderBrowser` bound to `flashZipExportPath` + a **"Keep history"** toggle → a number input bound to `flashZipExportKeep` (shown only when keep-history is on; 0 when off = single flash-latest.zip). Copy: it lands as a plain `.zip` at that path for off-server sync (Syncthing etc.). Reuse existing ToggleRow/number-field components; do not invent persistence.
- [ ] i18n: add `flash.zipExport*` keys (title, hint, enable, path, pathHint, keepHistory, keepHistoryHint, keepN, keepNHint, latestNote) to en+de in `i18n.ts`. Grep to avoid collisions.
- [ ] Gate: `tsc --noEmit` + `npm run build` (do not commit web/dist). Commit: `feat(flash): frontend controls for scheduled zip export (#28)`.

---

### Task 4: i18n ×24
**Files:** all `web/src/lib/locales/*.ts`.
- [ ] Add the `flash.zipExport*` key set to all 24 locale files, translated (one writer or disjoint partitions; no nested delegation). Verify each file has the full set exactly once; `tsc` clean; `npm run build` (commit the final bundle: web/dist).
- [ ] Commit: `i18n(flash): zip-export keys in all 24 locale files (#28)`.

## Self-review
- Spec coverage: trigger+dest+atomic+retention → T2; settings → T1; DTO → T1; UI → T3; i18n → T3/T4. Covered.
- Types consistent: `FlashZipExport{Enabled,Path,Keep}` (T1) used by T2; `flashZipExport*` DTO/TS (T1/T3) used by T3.
- No placeholders: exportFlashZip/pruneFlashZips given in full; clones cite the flash analog.

## Handoff
Subagent-driven, sequential, gates per task. After T4: `/code-review` on the branch diff, then release as **v4.3.0** (additive).
