# Healthchecks start ping + lifecycle decoupling — design

**Source:** GitHub issue #27 (ptmorris1). His actual need (clarified in comments): trigger Healthchecks
pings on backup **start** and on **success/failure** when done — not the generic per-container hooks he
originally asked about ("I probably won't do that per container… the notification on failure should be fine").

**Branch:** `feat/healthchecks-start-ping` (off `main` == v4.3.0). **Stack:** Go backend + embedded SPA.

**Goal:** give BombVault's existing Healthchecks.io integration a proper job **lifecycle** — a `/start` ping
when a backup begins, a success ping when it finishes OK, and a `/fail` ping when it fails — so Healthchecks
can measure duration, detect a hung/never-finished backup, and (crucially) stay green on success.

## The two problems

BombVault already pings Healthchecks (`internal/notify/notify.go` `pingHealthchecks`, called from `Send`),
but:

1. **No start ping.** It only pings on *done* (base URL on success, `/fail` on failure). Healthchecks' whole
   value — duration + "did it even start / is it hung" — needs the `/start` ping. This is ptmorris1's
   "ping on backup start."
2. **The success ping is suppressed under `On=failure`** (a latent bug). `Send` returns early on
   `!shouldSend(ev.OK)` (`notify.go:99`), so with the failure-only policy a *successful* backup never pings
   Healthchecks → the check goes red from "no ping received" even though the backup was fine. This is exactly
   ptmorris1's setup ("notification on failure").

## Design — Healthchecks lifecycle is decoupled from the message `On` policy

The `On` policy (`never`|`failure`|`always`) governs the **message channels** (webhook/Matrix/email/Unraid) —
those are notifications a human reads, so "only on failure" is sensible. **Healthchecks is a monitor, not a
message**: it needs the start + success + fail pings to function. So:

- When `HealthchecksURL` is set and `On != "never"`, BombVault pings the **full lifecycle**:
  - `POST/GET <url>/start` when a backup **starts**,
  - `GET <url>` on **success**,
  - `GET <url>/fail` on **failure** —
  regardless of whether `On` is `failure` or `always`.
- The message channels keep honouring `On` exactly as today.
- `On == "never"` still suppresses everything, Healthchecks included.

**Granularity: per item** (matches the existing per-item done ping). Each container/VM/flash/config backup
pings start→result on the configured check. Backups are serialized (`batchActive`/`lockDomain`), so start/done
pairs don't interleave. (A single global check that goes red if any backup fails is exactly the "one alert for
any failure" behaviour ptmorris1 asked about; per-domain checks are a possible future enhancement, out of
scope.)

## Backend changes (`internal/notify/notify.go`)

- **`pingHealthchecks(ctx, client, base, phase string)`** — `phase` ∈ `"start"|"success"|"fail"` maps to
  `base+"/start"` | `base` | `base+"/fail"`. (Replaces the current `ok bool` form.)
- **`Send(ctx, c, ev)`** — restructure:
  ```go
  if c.On == "never" { return }
  // ... ctx timeout + client ...
  // Healthchecks fires on both outcomes regardless of the failure/always policy
  // (it must get the success ping to stay green); the On policy governs only the
  // human message channels below.
  if c.HealthchecksURL != "" {
      phase := "success"; if !ev.OK { phase = "fail" }
      if err := pingHealthchecks(ctx, client, c.HealthchecksURL, phase); err != nil { log ... }
  }
  if !c.shouldSend(ev.OK) { return }
  // webhook / matrix / smtp (unchanged)
  ```
- **`SendStart(ctx, c Config)`** (new, exported) — pings `<HealthchecksURL>/start` when `HealthchecksURL != ""`
  and `c.On != "never"`; best-effort, own timeout/client, logs on error. No message channels (they have no
  "start" concept).
- `SendTest` — unchanged (still pings the base URL as a connectivity test).

## Backend changes (`internal/api/service.go`)

- **`notifyBackupStart(ctx)`** — decode the notify config via the existing `s.NotifyConfig()` and call
  `notify.SendStart(ctx, c)`; best-effort (a start-ping failure never affects the backup). Guard on
  `c.On != "never"` (SendStart also guards, belt-and-suspenders).
- Call `s.notifyBackupStart(ctx)` at the **start** of each backup — `Backup` (per container), `BackupVM`,
  `BackupFlash`, `BackupConfig` — right after settings are loaded and before the restic work begins. The
  existing `notifyBackup(...)` done-call is unchanged (its `Send` now also pings Healthchecks success under
  `On=failure`).

## Frontend

`web/src/pages/Settings.tsx` notifications section: a small helptext under the Healthchecks URL field noting
that Healthchecks pings the start + success/failure lifecycle whenever it's set, independently of the
"notify on" policy (so it stays green on success even with failure-only). One new i18n key
`notify.healthchecksLifecycle` (en+de + all 24 locales).

## Testing (`internal/notify` + `internal/api`)

- `Send` with `On=failure` + `ev.OK=true` and a Healthchecks URL → the base URL IS pinged (success), and no
  message channel fires. (A test HTTP server records the hit path.)
- `Send` with `ev.OK=false` → `/fail` pinged.
- `SendStart` → `/start` pinged when configured + `On != never`; no-op when `On=never` or URL empty.
- `On=never` → `Send`/`SendStart` ping nothing.
- `pingHealthchecks` phase→path mapping (start/success/fail).
- Service: `notifyBackupStart` calls SendStart (fake/httptest); a successful flash/config backup under
  `On=failure` still pings Healthchecks success (integration-ish, or assert via the notify unit tests).

## Out of scope (YAGNI)

- The generic per-container/VM pre/post shell hooks (#27's original title) — the requester retreated from
  them and no one else asked; the notification lifecycle covers his need.
- Per-domain / per-item separate Healthchecks checks (one global check is what he described).
- A start "message" on webhook/Matrix/email (start is a Healthchecks-monitor concept, not a human alert).

## File map

- **Modify (backend):** `internal/notify/notify.go` (pingHealthchecks phase, Send restructure, SendStart),
  `internal/api/service.go` (`notifyBackupStart` + calls in the 4 backup fns).
- **Modify (frontend):** `web/src/pages/Settings.tsx`, `web/src/lib/i18n.ts` + `web/src/lib/locales/*.ts` (24).
- **Create:** this spec + the implementation plan.
