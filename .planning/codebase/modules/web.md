# Module Map: web/ — React + Vite + TypeScript SPA

**Analysis Date:** 2026-09-09

The SPA is BombVault's entire UI. It is built by Vite, embedded into the Go binary via `web/embed.go` (covered by another mapper), and served same-origin by the API server. Scope of this document: `web/` excluding `web/dist/` (generated, placeholder only) and `web/node_modules/`.

## Technology

**Core (`web/package.json`):**

- React `^19.2.7` + react-dom `^19.2.7` — StrictMode on, `createRoot` in `web/src/main.tsx`.
- react-router-dom `^7.18.1` — classic `<BrowserRouter>/<Routes>/<Route>` API (`web/src/app/router.tsx`); the data-router / loader APIs are NOT used.
- TypeScript `^7.0.2` — the native TS 7 compiler (tsgo). `npm run build` = `tsc --noEmit && vite build`.
- Vite `^8.1.5` + `@vitejs/plugin-react` `^6.0.3` — config in `web/vite.config.ts`; dev server proxies `/api` to `https://localhost:3443` (secure: false).
- Tailwind CSS `^4.3.3` via `@tailwindcss/postcss` + postcss `^8.5.19` (`web/postcss.config.js`). All design tokens live in `web/src/index.css` as CSS variables exported into Tailwind's `@theme`: `carbon-*` surfaces, `accent`/`accentContrast`/`accentSoft`/`accentText`, and `statusOk/Fail/Warn/Neutral` token families. Dark mode is a `@custom-variant dark` keyed on `[data-theme="dark"]` (set on `<html>`; NOT the `prefers-color-scheme` media query).
- Runtime deps are minimal: `flag-icons` (language switcher flags), `qrcode-generator` (TOTP setup QR in `web/src/components/QRCode.tsx`). No UI kit, no data-fetching library (no react-query/SWR), no state library (no Redux/Zustand), no CSS-in-JS.

**Testing/lint devDeps:** vitest `^4.1.10`, @testing-library/react `^16.3.2`, @testing-library/dom `^10.4.1`, jsdom `^30.0.1`, eslint `^10.8.1`, typescript-eslint `^8.65.0`, eslint-plugin-react-hooks `^7.1.1`, and a local ESLint plugin `bombvault-lint-ts` → `file:lint-ts` (see Testing).

**TypeScript 7 / typescript-eslint side-by-side shim:** TS 7 has no JS compiler API, so `web/eslint.config.js` registers a module-resolve hook redirecting `typescript` imports made from inside typescript-eslint to a TS 6.0.3 copy in `web/lint-ts/`. Do not delete `web/lint-ts/` — the lint gate breaks without it. Everything else (tsc, vite, vitest) uses TS 7.

**npm scripts (`web/package.json`):** `dev`, `build` (tsc --noEmit && vite build), `typecheck`, `preview`, `lint` (`eslint src`), `test` (`vitest run`).

## Integrations

**Single API client: `web/src/lib/api.ts`** (~3000 lines, the frontend's only backend contract):

- `fetchJSON<T>(path, options)` — base wrapper. Always sends `Content-Type: application/json` (headers merged AFTER `...options` spread so callers can't clobber it). Non-2xx throws `ApiError(status, message)`.
- Every response type mirrors the Go JSON shape exactly (camelCase), documented per endpoint. The universal envelope is `{ ok: boolean; error?: string }` (`OkEnvelope`).
- **Graceful-failure convention:** many endpoints answer HTTP 200 with `{ok:false, error}` — callers must check `res.ok` and surface `res.error` (the server's scrubbed reason), never just "0 results" on failure. Helper: `loadErrorMessage(res, fallback)` in `web/src/lib/errors.ts`.
- Relative `/api/...` paths only — same-origin, session cookie flows automatically. Dev-time only, `web/vite.config.ts` proxies to `https://localhost:3443`.

**Async job pattern (backup/restore/replicate):** POSTs return immediately with `{ok:true, started:true}`; the work runs detached server-side. Outcomes are NEVER read from the POST response — use `useBackupWatch` (`web/src/lib/backupWatch.ts`), which (1) watches the SSE progress entry appear-then-clear and (2) polls `listRuns()` correlating the NEW run by id (snapshot baseline ids before firing — never by client clock, clock-skew bug). Bulk loops use the non-hook `fireAndWaitRun` (same file), which retries "already running" 409s within one shared deadline.

**Live progress (SSE):** `web/src/lib/progress.ts` holds ONE module-level `EventSource("/api/progress")`, ref-counted across subscribers (`useProgress()` returns `ProgressMap` keyed like `"container:plex"`). `anyActive(map)` is the app-wide busy guard — use it to disable start buttons; it deliberately ignores the `"maintenance"` phase.

**Auth (client side):** cookie session only; no tokens in JS. `web/src/app/Layout.tsx` probes `GET /api/auth` on mount and gates the whole app (`loading|pass|blocked` → renders `web/src/pages/Login.tsx` when blocked). Login supports two-step TOTP (`needCode` on `LoginResponse`). `GET /api/auth/totp/setup|confirm`, recovery codes, and password change are all in `api.ts`. Sign-out row lives in `web/src/components/Sidebar.tsx`.

**Write-only secret contract (UI side):** settings GETs return secrets as `""` with a `*Set` boolean flag; a blank field on save keeps the stored value; dedicated Clear flags/DELETE endpoints remove them. Every secret-bearing form (metrics token, widget token, SMTP password, registry auths, cloud creds) follows this — preserve it in new forms.

**Downloads:** fetch-then-blob with `globalThis`-typed browser globals for `exportSettings`/`downloadRecoveryKit` (success vs JSON error distinguished by Content-Disposition/content-type); plain `<a href>` for `flashDownloadURL` and `recoveryKitUrl`.

**Cross-tab / server-driven look sync:** `web/src/lib/displayPrefs.ts` reconciles localStorage (cache) with the server's `/api/display-prefs` (truth, issue #191) and fires `ADOPTED_EVENT` on `window`; `main.tsx` re-applies all look axes on that event.

## Architecture

**Overall:** single-page, client-routed SPA over a same-origin JSON API. No SSR, no server components, no code-splitting by route (only locale chunks are lazy).

**Data flow (per page):**

1. Route renders page component (`web/src/app/router.tsx`).
2. Page `useEffect` calls an `api.ts` function (`web/src/pages/Containers.tsx` is the canonical example) → `useState` for the result.
3. Mutations call another `api.ts` function, then either refetch explicitly, fire `window.dispatchEvent(new Event("bv:settings-changed"))` (Layout reloads settings), or use `useBackupWatch` for detached jobs.
4. Errors land in inline status rows, the toast system, or `res.error` text — there is no global error boundary.

**State management — three mechanisms, no library:**

- **Context providers mounted once** in `main.tsx`/`router.tsx`: `AdvancedProvider` (`web/src/lib/advanced.tsx`, simple/advanced view), `I18nProvider` (`web/src/lib/i18n.ts`), `ToastProvider` (`web/src/lib/toast.tsx`). Also `useConfirm` (`web/src/lib/useConfirm.tsx`) for dialogs.
- **Module-level singletons with subscriber sets:** `progress.ts` (SSE map) is the precedent — shared connection, `listeners: Set`, ref-counted open/close.
- **Window CustomEvents for cross-cutting refresh:** `"bv:settings-changed"` (Layout relists settings), `ADOPTED_EVENT = "bv-display-prefs-adopted"` (re-apply look axes), plus localStorage keys `bv-*` documented in `displayPrefs.ts`.

**Layering:** `pages/` → `components/` → `lib/` (api, i18n, hooks, pure helpers). `lib/api.ts` imports nothing app-specific; nothing imports upward from `lib` into `pages`.

**i18n:** `en` and `de` tables are inline in `web/src/lib/i18n.ts` (en is the source of truth and the always-present fallback); the other 40 locales live as one file each in `web/src/lib/locales/*.ts` and are lazy-loaded per chunk via `import.meta.glob`. Parity/quality/orphan tests guard the tables (see Testing).

**Look/prefs:** `web/src/lib/theme.ts`, `accent.ts`, `appearance.ts` (rainbow/hue), `shape.ts`, `motion.ts`, `controls.ts` (label modes) each own one localStorage axis, applied synchronously before first paint by `main.tsx`; `index.html` contains an inline FOUC-prevention script that must stay in sync with `theme.ts`.

## Structure

```
web/
├── package.json / package-lock.json / .npmrc   # npm project (lockfile committed)
├── index.html               # Vite entry; inline data-theme FOUC script
├── vite.config.ts           # react plugin; /api dev proxy → :3443
├── vitest.config.ts         # node env default; jsdom per-file opt-in
├── tsconfig.json            # strict; paths @/* → ./src/*; excludes *.test.*
├── eslint.config.js         # flat config + TS6 shim + bombvault house rules
├── lint-rules/              # custom ESLint plugin (house UI rules) + README
├── lint-ts/                 # TS 6.0.3 side-package for typescript-eslint
├── postcss.config.js        # @tailwindcss/postcss
├── public/                  # logo.svg / logo-light.svg / logo.png (served at /)
├── dist/                    # Vite output; embedded by web/embed.go
│                            # (only placeholder index.html is committed — see Concerns)
└── src/
    ├── main.tsx             # entry: apply stored prefs → providers → router
    ├── index.css            # ALL design tokens (@theme), Tailwind import
    ├── vite-env.d.ts
    ├── app/
    │   ├── router.tsx       # all routes, providers order: I18n > Toast > Router
    │   └── Layout.tsx       # auth gate, sidebar shell, WhatsNew, settings load
    ├── pages/               # one file per route (PascalCase)
    │   ├── settings/        # Settings tab cards (AboutCard, CloudCard, ...)
    │   └── Dashboard/Containers/VMs/Flash/Config/Files/Receiver/Fleet/
    │       Recovery/Login/Glyphs/Settings.tsx
    ├── components/          # reusable controls (Button, Badge, Toggle, Selector,
    │   ├── restore/         #   DropdownListbox, TimePicker, Sidebar, Toast, ...)
    │   └── recovery/        # feature-scoped subfolders
    └── lib/                 # api.ts + i18n + locales/ + hooks (use*) + pure helpers
```

**Key files:**

- `web/src/lib/api.ts` — every endpoint function + every wire type. THE place to add/extend API calls.
- `web/src/app/router.tsx` — route table; new pages must be registered here.
- `web/src/app/routedPages.test.ts` — fails if a routed page is missing from the page-shell lint rule's coverage.
- `web/src/lib/pageShell.ts` — `PAGE_SHELL` / `PAGE_SHELL_TABBED` constants (one 40px-gap, 1152px-cap root wrapper for every routed page; Settings is the only stated exception).
- `web/src/lib/i18n.ts` — translation tables; add keys to `en` AND `de` inline blocks.
- `web/src/lib/progress.ts`, `web/src/lib/backupWatch.ts` — live progress + async-job watching.

**Where to add new code:**

- **New page:** `web/src/pages/Name.tsx`, root `<div className={PAGE_SHELL}>`, register in `web/src/app/router.tsx`, add nav entry in `web/src/components/Sidebar.tsx`, add `nav.*`/`name.*` keys to `en`+`de` in `i18n.ts`. The `bombvault/page-uses-page-shell` lint rule and `routedPages.test.ts` both enforce conventions here; exceptions must be declared in `web/eslint.config.js`.
- **New reusable component:** `web/src/components/PascalName.tsx` (feature-scoped ones in a subfolder like `components/restore/`). Icon glyphs go in `web/src/components/glyphs.tsx` / `navGlyphs.tsx` (the `/glyphs` contact-sheet page previews them).
- **New hook:** `web/src/lib/useName.ts`.
- **New API endpoint:** extend `web/src/lib/api.ts` — type mirroring the Go JSON exactly + one exported function using `fetchJSON`/`srcParam` for off-site `source=` params.
- **New locale:** drop `web/src/lib/locales/xx.ts` (default-exported `Partial<Translations>`); `import.meta.glob` picks it up, but parity/quality tests (`i18n.parity.test.ts`, `i18n.quality.test.ts`, `i18n.orphans.test.ts`) must pass; register in the offered-languages list in `i18n.ts`.

## Conventions

**Naming:** Components/pages `PascalCase.tsx` (`web/src/components/TimePicker.tsx`); lib helpers and files `camelCase.ts`; hooks prefixed `use*`; test suffixes `.test.ts` (pure) and `.dom.test.tsx` (DOM). Named exports everywhere (default exports only for locale modules and `Recovery.tsx`).

**TypeScript:** `strict` + `noUnusedLocals` + `noUnusedParameters` (`web/tsconfig.json`). Wire types mirror Go JSON field-for-field with a doc comment per field; keep them in sync by hand. Path alias `@/*` exists but the codebase overwhelmingly uses relative imports (`../lib/api`) — follow the relative style. `type T = ReturnType<typeof useT>["t"]` is the standard alias for the translate function.

**Styling:** Tailwind utilities only, built on the semantic tokens from `web/src/index.css` — use `carbon-*`/`status*`/`accent*` token classes, never raw hex or a hard-coded radius on controls. Shared controls carry the `glim-*` engine classes (e.g. `glim-btn-lg` stages in `web/src/components/Button.tsx`). Page roots use `PAGE_SHELL`. Status colors belong on Badges/chips, never on interactive controls (`bombvault/no-status-color-on-control` enforces this).

**Comments:** This codebase's signature convention — long narrative block comments at the top of every nontrivial file explaining WHY, citing issue numbers (`#191`) and reviewer decisions by name, often longer than the code. New code is expected to continue this; if a rule has an exception, the exception is written down at the site (see `pageShell.ts` for the model).

**Error/loading states:** Data loads are plain `useEffect` + `useState` with an inline error/status row; prefer `loadErrorMessage(res, fallback)` so the server's scrubbed reason is shown. Transient feedback uses `useToast` (`web/src/lib/toast.tsx`); confirmations use `useConfirm`. Async jobs surface via `useBackupWatch`'s `BackupWatchState` union (`idle|pending|success|cancelled|skipped|error`) — cancelled/skipped are NEUTRAL terminals, not errors.

**i18n:** every user-visible string goes through `t()` from `useT()` — enforced by the `bombvault/user-message-is-translated` lint rule; em dashes are banned in user text (`no-em-dash-in-user-text`, non-configurable); backend error text is shown verbatim (English by design).

## Testing

**Runner:** vitest `^4.1.10`, `web/vitest.config.ts`. Default environment is **node** (fast, no DOM). A test needing a real DOM opts in with a `// @vitest-environment jsdom` docblock as the FIRST line of that file. Test files are excluded from the tsc program (`tsconfig.json` exclude) — esbuild transpiles them.

**Two test styles (both real, both used):**

1. **Pure node tests (`*.test.ts`, 40 files):** hookless function components are invoked as plain functions and the returned element tree is walked as objects — no jsdom. Model: `web/src/components/Badge.test.ts` (asserts on `className` contents). All pure lib logic (`cron.ts`, `toastEngine.ts`, `forecast.ts`, custom lint rules via `web/src/lib/uiConventions.test.ts`) tests this way.
2. **DOM tests (`*.dom.test.tsx`, 47 files):** `@testing-library/react` `render`/`screen`/`fireEvent` with `cleanup()` + `localStorage.clear()` in `beforeEach`/`afterEach`. Model: `web/src/components/Button.dom.test.tsx`.

**Run commands:** `npm test` (vitest run), `npm run lint` (eslint src — includes the house rules), `npm run build` (typecheck). CI: `.github/workflows/lint.yml` has a `web` job running `npm ci && npm run lint && npm test` on every push.

**Lint gate details (`web/eslint.config.js`):** @eslint/js + typescript-eslint recommended (non-type-checked), `react-hooks/rules-of-hooks` error / `exhaustive-deps` warn, `no-undef` off, `no-unused-vars` with `_` prefix exemptions, `reportUnusedDisableDirectives` error. Plus 8 custom `bombvault/*` rules from `web/lint-rules/` (icon badges need tooltips, one icon-badge size, no status color on controls, controls read engine tokens, user text is translated, no em dashes, pages use PAGE_SHELL) — each rule is itself tested in `web/src/lib/uiConventions.test.ts`. There is NO committed Playwright/e2e suite (mentioned in comments as live-manual verification only).

**Coverage:** no thresholds enforced.

## Concerns

**Monolith files (highest day-to-day risk):**

- `web/src/lib/api.ts` — 3007 lines, every type + endpoint in one file. Adding endpoints grows it unboundedly; merge conflicts are routine. Fix approach when touched: split by domain into `lib/api/` modules re-exported from one index, but coordinate — many files import from `../lib/api` directly.
- `web/src/pages/Settings.tsx` — 5149 lines (though tab cards are being extracted into `web/src/pages/settings/*Card.tsx` — continue that extraction). `web/src/pages/Containers.tsx` — 3298 lines. `web/src/pages/Dashboard.tsx` — 2765 lines. Total non-test src is ~114k LOC across 156 files.

**Stale-dist / embed gotcha:** the repo guide says "commit web/dist", but `D:\code\bombvault\.gitignore` ignores `web/dist/*` except a placeholder `web/dist/index.html` (whose hashed `/assets/...` references are from an old build). The real bundle is built fresh in CI/Docker. If you change `web/` and run only `go build` locally without `npm run build` first, the binary embeds the stale/placeholder SPA. Always run `just web` (`cd web && npm ci && npm run build`) before building the Go binary after frontend changes — and don't "fix" the gitignore to commit hashed assets without an explicit decision.

**Bundle size:** locale chunks are lazy (`i18n.ts`, fixed what was 9/10 of the bundle), but PAGES are not code-split — every route ships in the single main `index-*.js`. With Settings/Containers/Dashboard at multi-thousand lines each, route-level `React.lazy` is the obvious lever if bundle size ever becomes a reported problem.

**Fragile pairs (must change together, enforced only by tests/comments):**

- `web/index.html` inline FOUC script ↔ `web/src/lib/theme.ts` storage key + resolution logic (duplicated deliberately; a drift flashes the wrong theme).
- `web/src/index.css` `--color-statusOffsite` ↔ the hard-coded copy in the Go-side `internal/api/widget.html` (a Go test guards the drift — don't edit one side alone).
- `en`/`de` tables in `i18n.ts` ↔ 40 locale files (parity tests catch missing keys, not bad wording).

**Convention enforcement is load-bearing:** the 8 `bombvault/*` lint rules exist because each was repeatedly broken and user-reported. Breaking them is not style nitpicking — CI fails and past regressions shipped to users. When a rule blocks legitimate work, add a stated exception in `web/eslint.config.js` (the `page-uses-page-shell` options show the expected form), never a disable comment.

**`exhaustive-deps` is only `warn`:** hook-dependency bugs (stale closures in the many `useEffect` data loads) are not gated; the `useBackupWatch`/`progress.ts` ref-mirroring pattern (`xRef.current = x` on every render) is the house workaround — copy it when a callback must read fresh state without re-subscribing.

**No e2e / no route-level error boundary:** a render crash in any page takes down the SPA (no `ErrorBoundary` component exists); long flows (DR restore, Recovery tab restart polling via `waitForAppBack`) are covered only by unit tests on the pure helpers.

---

*Module analysis: 2026-09-09*
