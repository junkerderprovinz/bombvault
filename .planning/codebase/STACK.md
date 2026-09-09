# Technology Stack

**Analysis Date:** 2026-09-09

## Languages

**Primary:**
- **Go 1.25.0** declared in `go.mod` (`go 1.25.0`, module `github.com/junkerderprovinz/bombvault`) — backend, entrypoint `cmd/bombvault/main.go`. CI (`setup-go` in `.github/workflows/lint.yml`) and the Docker build stage (`golang:1.26-bookworm`) both run **Go 1.26**; the `go` directive is a floor, so this builds, but the manifest lags the toolchain (see CONCERNS.md). To bump Go, change all three places together: `go.mod`, `.github/workflows/lint.yml` `go-version`, and the `Dockerfile` build-stage image + digest.
- **TypeScript ^7.0.2** (native TS 7 compiler, tsgo) — the SPA in `web/`. `npm run build` = `tsc --noEmit && vite build`. typescript-eslint runs against a side-by-side TS 6.0.3 copy in `web/lint-ts/` (TS 7 has no JS compiler API); do not delete `web/lint-ts/`.

**Secondary:**
- **Node 24** — SPA build stage (`node:24-slim`, digest-pinned in `Dockerfile`); CI web job pins `node-version: '24'`.
- **Python 3.x** — MkDocs docs site only (`.github/workflows/docs.yml` installs `docs-requirements.txt`).

## Runtime

**Environment:**
- Single static Go binary (`CGO_ENABLED=0`, enabled by the pure-Go `modernc.org/sqlite` driver). `tini` is PID 1 with `-g` (forwards signals to the whole process group, reaping orphaned restic→rclone grandchildren — issue #35).
- External binaries shipped in the image: **restic 0.17.3** and **rclone 1.74.2**, downloaded from upstream releases with pinned SHA256 `ARG`s (Debian's apt packages are too old). `RESTIC_VERSION`/`RCLONE_VERSION` and their SHA256 args must be bumped in the same change or `sha256sum -c` fails the build.
- The Go test suite requires **restic >= 0.17 on PATH** (`--insecure-no-password`); apt's is too old — CI installs 0.17.3.

**Package Manager:**
- Go modules (`go.mod`, `go.sum`).
- npm for the SPA (`web/package-lock.json` committed).

## Frameworks

**Core:**
- **No web framework** — `net/http` stdlib only. `http.NewServeMux()` with Go 1.22+ method+path patterns, middleware as plain `http.Handler` wrappers, hand-rolled SSE (`internal/api/sse.go`), Prometheus text format written by hand (`internal/api/metrics.go`). No chi/gin/echo/httprouter, no prometheus client dep.
- **React ^19.2.7** + react-dom ^19.2.7 — SPA (`web/src/main.tsx`, StrictMode, `createRoot`).
- **react-router-dom ^7.18.1** — classic `<BrowserRouter>/<Routes>/<Route>` API only (`web/src/app/router.tsx`); data-router/loader APIs are NOT used.
- **Tailwind CSS ^4.3.3** via `@tailwindcss/postcss` + postcss ^8.5.19 — design tokens as CSS variables in `web/src/index.css` (`carbon-*`, `accent*`, `statusOk/Fail/Warn/Neutral`), dark mode via `[data-theme="dark"]` custom variant.

**Testing:**
- Go: stdlib `go test` (no testify/ginkgo).
- Web: vitest ^4.1.10, @testing-library/react ^16.3.2, jsdom ^30.0.1.

**Build/Dev:**
- Vite ^8.1.5 + @vitejs/plugin-react ^6.0.3 (`web/vite.config.ts`; dev proxy `/api` → `https://localhost:3443`).
- **just** — task runner (`justfile`; `just check` = the pre-push Go chain).
- golangci-lint (`.golangci.yml`, v2 schema: errcheck, govet, staticcheck, ineffassign, unused, gosec), gofmt, go vet, hadolint, gitleaks (`.gitleaks.toml`).
- eslint ^10.8.1 + typescript-eslint ^8.65.0 + eslint-plugin-react-hooks ^7.1.1 + local plugin `bombvault-lint-ts` (`web/lint-rules/`, 8 `bombvault/*` house rules).
- Docker buildx — multi-arch amd64+arm64; Go/web stages cross-compile via `TARGETOS`/`TARGETARCH`, no QEMU for the compile.
- Renovate (`renovate.json`) — weekly; pins Action and Docker base-image digests; groups Go minor/patch.

## Key Dependencies

**Critical (from `go.mod` — the manifest of record for the whole repo):**
- `modernc.org/sqlite v1.56.0` — CGO-free SQLite (`internal/store`); what makes the binary fully static and the multi-arch build toolchain-free.
- `github.com/robfig/cron/v3 v3.0.1` — scheduler engine (`internal/schedule`), one cron per domain.
- `github.com/docker/docker v28.5.2+incompatible` + `github.com/docker/go-connections v0.8.1` + `github.com/containerd/errdefs` — Docker SDK over the mounted `docker.sock` (no docker CLI).
- `filippo.io/age v1.3.1` + `golang.org/x/crypto` — export encryption (`internal/ageseal`) and crypto (`internal/secret`: AES-256-GCM, Argon2id, hand-written TOTP).

**Web runtime deps (deliberately minimal):**
- `flag-icons` (language switcher), `qrcode-generator` (TOTP QR in `web/src/components/QRCode.tsx`). No UI kit, no react-query/SWR, no Redux/Zustand, no CSS-in-JS.

**Infrastructure:**
- restic (external process) — the storage engine; the ONLY code touching it is `internal/restic`.
- rclone — spawned by restic as a grandchild for `rclone:` off-site repos; no direct API use.

## Configuration

**Environment:**
- Process config from env vars via `config.LoadFromEnv()` (`internal/config/config.go`): `PORT` (3000), `HTTPS_PORT` (3443), `TZ` (scheduler timezone), `DATA_DIR` (=/config), `APP_KEY`, `PLATFORM`, `HOST_SOURCE_ROOT`/`HOST_MOUNT_ROOT`, `BACKUP_MAX_HOURS`, `HTTP_ONLY`, `LIBVIRT_*`.
- Defaults set in the `Dockerfile` `ENV` block; user-facing docs in `deploy/docker-compose.generic.yml`.
- Database file: `<DataDir>/bombvault.sqlite` (derived at `internal/config/config.go:104`).
- Secrets are NOT env-var-borne at runtime: `APP_KEY` unlocks AES-256-GCM blobs stored in SQLite (see INTEGRATIONS.md).

**Build:**
- `Dockerfile` — 3 stages (web → build → runtime), digest-pinned bases, `# syntax=docker/dockerfile:1@sha256:...` header, `SHELL ... pipefail`, checksum-pinned restic/rclone, `HEALTHCHECK` invoking `bombvault healthcheck` itself, tini entrypoint.
- Version stamping: `-ldflags "-s -w -X ...internal/api.Version=${VERSION}"`; CI computes `v<tag>` for tags and `v<lastTag>+<branch>.<sha>` otherwise (`internal/releasenotes` parses this string).

## Platform Requirements

**Development:**
- Go 1.26, Node 24, restic >= 0.17 on PATH for `go test ./...`, golangci-lint, hadolint, just (recipes use sh — Git Bash on Windows).
- Windows dev caveat: POSIX-only tests (`internal/restic/proc_test.go`, cancel/stream self-exec tests) skip on Windows — a reduced suite runs locally.
- Frontend: `cd web && npm ci && npm run build` before `go build` after any `web/` change (the binary embeds `web/dist`; a stale build embeds the stale SPA).

**Production:**
- Multi-arch (amd64+arm64) Docker image published to GHCR (+ Docker Hub mirror); targets Unraid, TrueNAS Scale, and generic Docker hosts (`internal/platform` detection, `PLATFORM` env override).
- Requires: writable `/config` volume (fail-fast check in `cmd/bombvault/main.go`), Docker socket (root-equivalent — documented warning), optional SSH to the libvirt host (`qemu+ssh://`, key generated on first run).
- restic cache must live under the persistent DataDir (critical for off-site repos, issue #95).

---

*Stack analysis: 2026-09-09*
