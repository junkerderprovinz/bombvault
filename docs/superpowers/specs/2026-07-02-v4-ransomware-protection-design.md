# v4 Sub-Project 1: Ransomware Protection (Immutable Off-site) — Design

> Part of the v4.0.0 wave (features: ransomware protection, platform expansion,
> new backup domains, onboarding). Built on `feat/v4`; released only when the
> whole wave works. Background and code-grounded analysis live in the project
> vault ("v4.0.0 Feature-Exploration", chapter 2 + verified research); this spec
> records the decisions and requirements.

## Goal

Backups that survive a full compromise of the Unraid box. Ransomware running on
the host owns BombVault's credentials (`APP_KEY` is in the container env), so
today it can delete the local repo **and** the off-site repo (`restic forget
--prune` or raw deletes with the same push credentials). BombVault v4 makes the
off-site repo **append-only** (guided setup), **verifies** that protection with
an active tamper test, **proves** restorability from off-site with a real
sandbox-restore drill, and **shows** the protection level on the dashboard.

Principle: BombVault never *enforces* immutability — the far side does
(rest-server). BombVault orchestrates, verifies (with a visible test age), and
never shows green on configuration claims alone.

## Threat model (what this does and does not cover)

Covered: an attacker with full control of the Unraid box (including BombVault's
credentials) cannot delete or overwrite existing off-site snapshots; degraded
protection (far side reconfigured) is detected and alerted; restorability from
off-site is periodically proven.

Not covered (documented honestly in the UI/docs): reading/exfiltrating the
off-site data (box credentials still allow reads), disk-filling writes (growth
budget alarm = detection, not prevention), slow source-data encryption
replicating as valid snapshots (mitigated by long far-side retention), and a
compromise of the storage box itself. The local repo on the compromised box is
written off by design.

## Decisions (open questions from the exploration, resolved)

1. **Tamper-test depth:** Stage 1 ships in the core (authenticated DELETE
   against provably non-existent object IDs; side-effect-free). Stage 2 (real
   canary object with save/restore journal) is a later optional wave, opt-in.
2. **Advertised prune path:** far-side prune cron (generated snippet) is the
   recommended standard; maintenance window (temporary non-append-only
   instance, credentials never persisted, mandatory tamper re-test afterwards)
   is the documented alternative; "grow + budget alarm" is the honest default
   until the user picks one. Generated snippets include `--keep-within` and a
   snapshot-count anomaly note (documented retention-policy timestamp attack).
3. **S3:** supported as a repo, but marked "configured, not verified" (visible
   scorecard malus + manual-confirm checkbox) until the optional Mini-SigV4
   verification wave (bucket versioning + deny-DeleteObjectVersion check).
   Native S3 Object Lock is NOT pursued (rejected upstream by restic).
   `rclone serve restic --append-only` is NOT recommended by the wizard (open
   upstream retry bug); a warning appears if an rclone-served repo is toggled
   immutable.
4. **Scorecard:** a per-domain checklist with age-stamped facts and a
   red/amber/green aggregate — no numeric score. Aggregate + key facts exported
   to `/metrics` as gauges.
5. **DR-drill target:** restores into a marker-guarded, auto-cleaned sandbox
   folder under the default restore folder. The drilled container is
   user-selectable; default = the most recently successfully backed-up
   container. VMs are exempt from routine DR drills (subset check only; the
   scorecard shows "partially verified" for VMs, never a false green).
6. **Tamper-test cadence:** its own schedule (default weekly), plus a mandatory
   run when the immutable toggle is switched on and a manual "Test now" button.
   Not coupled to every replication.

## Scope

**In:** per-domain immutable flag for off-site repos; guided rest-server
deployment (docker run + compose snippet with generated htpasswd credentials);
connection test endpoint; tamper test (stage 1) with history + notifications on
protection loss; prune-skip + off-site delete/prune refusal for immutable
repos; retention-strategy UX (three explicit options + snippets); replication
history (persisted runs); off-site growth tracking + budget alarm; DR-drill v2
(kind="dr", off-site sandbox restore + verification + cleanup, schedulable);
"Ransomware protection" dashboard card; Prometheus gauges; i18n × 26.

**Out:** pull-based backups; S3 Object Lock; per-key privilege separation
(doesn't exist in restic); anomaly detection on backup contents; protecting the
local repo.

## Architecture

### Data model (forward-only migrations)

```sql
-- flags + budget + tamper schedule (one column per migration, house pattern)
ALTER TABLE settings ADD COLUMN containers_offsite_immutable INTEGER NOT NULL DEFAULT 0;
ALTER TABLE settings ADD COLUMN vms_offsite_immutable        INTEGER NOT NULL DEFAULT 0;
ALTER TABLE settings ADD COLUMN flash_offsite_immutable      INTEGER NOT NULL DEFAULT 0;
ALTER TABLE settings ADD COLUMN offsite_growth_budget_gb     INTEGER NOT NULL DEFAULT 0; -- 0 = off
ALTER TABLE settings ADD COLUMN tamper_test_schedule TEXT NOT NULL DEFAULT 'weekly Sun 04:30';
ALTER TABLE settings ADD COLUMN dr_drill_target      TEXT NOT NULL DEFAULT '';           -- '' = auto (most recent backup)

CREATE TABLE tamper_tests (
  domain TEXT NOT NULL, at INTEGER NOT NULL,
  protected INTEGER NOT NULL,          -- 1 = delete was refused
  detail TEXT NOT NULL DEFAULT ''      -- scrubbed status/error
);
CREATE TABLE offsite_runs (
  domain TEXT NOT NULL, started_at INTEGER NOT NULL, finished_at INTEGER,
  ok INTEGER NOT NULL DEFAULT 0, error TEXT NOT NULL DEFAULT ''
);
ALTER TABLE restore_drills ADD COLUMN kind TEXT NOT NULL DEFAULT 'subset'; -- 'subset' | 'dr'
```

### Backend behaviour

- `offsiteImmutableFor(domain, settings)`: with the flag set, `copyToOffsite`
  skips the off-site `ForgetPolicy` block (log: "retention is enforced
  far-side"); off-site `DeleteSnapshot`/`Prune` are refused with a clear error;
  `Unlock` stays allowed (rest-server permits lock deletion in append-only —
  operationally required). `handlePutSettings` warns (not fails) when immutable
  + off-site retention are both set.
- **Connection test** `POST /api/offsite/{domain}/test`: resolve repo, probe
  reachable/initialized/encryption-match (reuses `EnsureRepo`'s probe path;
  modelled on `POST /api/vm/ssh/test`).
- **Tamper test** `POST /api/offsite/{domain}/tamper-test` (stage 1): raw
  authenticated HTTP `DELETE` against `<repo>/data/<random-64-hex>` and
  `<repo>/snapshots/<random-hex>` using `net/http` + the stored REST
  credentials. 403/405 → protected; **404 → NOT protected** (server would have
  deleted); 2xx → alarm. Never touches real repo objects. Results persist to
  `tamper_tests`; a protected→unprotected flip notifies via all configured
  channels. Only REST-protocol repos are stage-1 testable; others report
  "unverifiable" honestly. A CI integration matrix (rest-server × rclone-serve,
  each with/without append-only) pins the status-code interpretation.
- **Deployment snippet generator**: for the wizard — `docker run` + compose
  snippet for `restic/rest-server` with `--append-only --private-repos`, a
  generated user + bcrypt htpasswd line (plaintext shown once), volume + port
  placeholders (generic IPs only).
- **Replication history**: `copyToOffsite`/`ReplicateOffsite` write begin /
  end / ok / scrubbed-error to `offsite_runs` (restic copy has no
  machine-readable progress — duration + outcome only).
- **Growth tracking**: after each successful replication,
  `CollectStatsAsync(domain, "offsite")` (method exists) populates the existing
  `repo_stats` time series; budget check on sampling; breach → notification.
  (The REST protocol cannot see the far side's free space — only own growth.)
- **DR-drill v2**: `RunRestoreDrill` gains `kind`. `dr` = domain lock → newest
  snapshot of the drill target from the OFF-SITE repo → restore via the
  existing `RestoreInclude` machinery into
  `<RestoreFolder>/bombvault-drill-<domain>-<ts>` (containment via
  `paths.Resolve`; a marker file gates the cleanup so nothing else is ever
  deleted) → verify file count + bytes against `restic ls`/`stats
  --mode restore-size` → cleanup → record in `restore_drills(kind='dr')`;
  failure notifies. Scheduling: the drill job becomes configurable
  (domain × source × kind) instead of hardcoded "local"; flash always full-DR,
  containers = drill target, VMs subset-only. Off-site bandwidth limits apply.
- **Scorecard**: extend `DomainStatusEntry` (no second round-trip) with
  `offsiteConfigured`, `offsiteImmutable`, `lastTamperTest/lastTamperOK`,
  `lastReplication/lastReplicationOK`, `lastDRDrill/lastDRDrillOK`. Aggregate:
  red when off-site missing or tamper test failed/stale; amber when
  replication/drill overdue (reuse the rpoStatus doubling logic); else green.
  New gauges: `bombvault_offsite_immutable{domain}`,
  `bombvault_tamper_test_ok{domain}`,
  `bombvault_offsite_last_replication_timestamp_seconds{domain}`.

### Frontend

- Settings → Off-site card: per-domain "Set up…" wizard (backend choice →
  rest-server snippet + copy → URL/credentials + "Test connection" → immutable
  toggle + immediate tamper test with explicit verdict) + retention-strategy
  chooser (three options, snippets, warnings). rclone-served repos get the
  known-bug warning on the immutable toggle; S3 gets the "unverified —
  confirm manually" checkbox.
- Dashboard: "Ransomware protection" card next to the protection card —
  age-stamped checklist per domain + aggregate chip; every red row deep-links
  to the owning settings section. Growth graph + budget in Advanced mode.
- Verify section: DR-drill controls (run now, schedule, drill-target picker),
  results with the "last proven restorable from off-site" badge.
- All new keys in all 26 locales in the same waves.

## Waves & gates

1. **Immutable foundation (M):** migrations, flag plumbing, prune-skip,
   off-site delete/prune refusal, settings toggle + warnings, connection test.
   *Gate:* replication against a real `rest-server --append-only` runs clean
   (no prune attempts in the log); connection test discriminates
   reachable/uninitialized/auth-failure.
2. **Guided deployment + tamper test (M):** snippet generator, tamper endpoint
   + history + notifications, scheduled test.
   *Gate:* CI matrix (rest-server / rclone-serve × append-only on/off) returns
   the correct verdict in all four cells.
3. **Scorecard + history + growth (M):** offsite_runs, DomainStatus extension,
   dashboard card, off-site stats sampling, budget alarm, gauges, i18n.
   *Gate:* reconfigure the test server → card flips red after the next test +
   notification fires.
4. **DR-drill v2 (M):** kind parameter, sandbox restore + verification +
   marker-guarded cleanup, scheduling, badge/scorecard wiring.
   *Gate:* full drill green for containers + flash against a real off-site
   repo; VM subset behaviour documented.
5. **Optional (M–L, may slip past v4.0.0):** retention snippets automation,
   tamper stage 2 (canary + journal), S3 verification (Mini-SigV4).

Box gates on Bottich before the v4 release: wizard → rest-server on the QNAP/
NAS → tamper test verdict correct in both server modes; DR drill green from
off-site; protection-loss notification observed end-to-end.

## Risks

The guarantee lives on the far side (hence age-stamped verification, never
flag-green); append-only protects against deletion only (reads/writes remain);
rest-server's append-only is global (the "privileged key" story is a
two-instance/maintenance-window story); support surface grows toward the far
side (snippets can age with upstream); DR drills cost real bandwidth/scratch
space (limits apply, VMs exempt).
