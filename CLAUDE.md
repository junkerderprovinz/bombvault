# BombVault — Repo Guide

Backup and full disaster recovery for Unraid (Docker containers, KVM VMs, USB flash, appdata/config). Go backend + React/Vite SPA, restic as the storage engine, shipped as a multi-arch Docker image.

## Layout

- `cmd/bombvault/` — entrypoint, wiring, scheduler.
- `internal/api/` — HTTP service + all domain logic (`service.go` is large; backup/restore/retention/offsite live here).
- `internal/backup/` — per-domain orchestrators (container/vm/flash/files/config); each sets restic tags.
- `internal/restic/` — restic argv builders + engine (`restic.go`); args are unit-tested in `restic_args_test.go`.
- `internal/store/` — SQLite settings/state. `internal/releasenotes/` — release notes embedded into the binary.
- `web/` — React + Vite + TypeScript SPA; built output is embedded via `web/embed.go`.

## Build / test / lint (run before every push)

```sh
go build ./...            # compile
go vet ./...
gofmt -l .                # must print nothing (CI + the pre-push hook fail on output)
golangci-lint run ./...   # CI gate
go test ./...             # needs restic >= 0.17 on PATH (apt's is too old)
# Frontend (only when web/ changed):
cd web && npm ci && npm run build   # tsc --noEmit && vite build; commit web/dist
hadolint Dockerfile
```

`just check` runs the Go chain in one go (see `justfile`). The global pre-push hook already runs gofmt + hadolint + gitleaks.

## Release (NEVER tag without explicit approval)

1. Write release notes to **both** `.github/release-notes/vX.Y.Z.md` **and** `internal/releasenotes/notes/vX.Y.Z.md` — an embed-sync test fails if the copy is missing/different.
2. Commit, push, wait for Lint + Test + Build Docker Image to go green.
3. `git tag vX.Y.Z && git push origin vX.Y.Z` → the tag build publishes `:X.Y.Z / :X.Y / :X / :latest` to GHCR + Docker Hub.
4. `gh release create vX.Y.Z --title "vX.Y.Z" --notes-file .github/release-notes/vX.Y.Z.md` (title = version only).
5. Verify all Docker Hub tags return 200, then reply to any fixed issue **once** and close it.

Version SemVer is 3-digit. Main builds stamp `v<lastTag>+main.<sha>` into the binary; tag builds use the tag.

## CI gates

- **Lint** — `go build`, `go vet`, golangci-lint.
- **Test** — `go test ./...` (installs restic first).
- **Build Docker Image** — amd64 boot smoke test (must serve `/api/health`, tini must be PID 1) → multi-arch build+push with SBOM + provenance attestations → non-blocking Trivy CVE scan (SARIF to the Security tab).

## Conventions / gotchas

- Retention is **identity-stable** (per item tag, ungrouped) — do not reintroduce `--group-by paths` (issue #91). VM live snapshots carry an extra `live` tag, so never `--group-by tags` globally.
- restic hash mismatch on stable input = bad RAM, not a code bug.
- Off-site immutable/append-only repos are never pruned from this box.
- Async cleanup can flake on Linux CI — wait for the goroutine. Cancelled exec returns `*ExitError`, remap via `ctx.Err()`.
- No real user data / IPs in the repo. This repo is and has always been public.
