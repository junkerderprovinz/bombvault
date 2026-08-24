import { useEffect, useState } from "react";
import {
  backupConfigNow,
  listConfigSnapshots,
  deleteSnapshot,
  getSettings,
  putSettings,
} from "../lib/api";
import type { Snapshot, Settings } from "../lib/api";
import { useT } from "../lib/i18n";
import { ProgressBar } from "../components/ProgressBar";
import { useProgress, anyActive, busyPhraseKey } from "../lib/progress";
import { useBackupWatch } from "../lib/backupWatch";
import { SourceToggle, type RepoSource } from "../components/SourceToggle";
import { ToggleRow } from "./Settings";
import { useConfirm } from "../lib/useConfirm";
import { useToast } from "../lib/toast";
import { Badge } from "../components/Badge";
import { CheckDraw } from "../components/CheckDraw";
import { InfoBubble } from "../components/InfoBubble";

type T = ReturnType<typeof useT>["t"];

// ---------------------------------------------------------------------------
// Backup button — fire-and-watch, mirroring the flash domain (see useBackupWatch:
// the config backup runs detached on the server and the POST returns immediately,
// so we watch the "config" progress + recorded run for the outcome).
//
// GlimStone follow-up pass (v8.0.0) audit note: the state.phase "success"/
// "error" result below is deliberately NOT migrated to a toast, even though
// this file's ConfigSettingsCard (further down) already uses useToast() for
// its own settings save — same shared-hook reasoning as Containers.tsx's
// BackupButton / VMs.tsx's VMBackupButton / Flash.tsx's FlashBackupButton:
// it's driven by lib/backupWatch.ts's useBackupWatch hook (kind defaults to
// "backup", which already self-clears after 4s — SUCCESS_CLEAR_MS,
// effectively already toast-like), but the identical state shape also backs
// RESTORE outcomes elsewhere, which are explicitly STICKY BY DESIGN.
// Splitting that shared, cross-file state machine's rendering by kind is a
// hook-level architecture change, not the local flash-swap this pass does
// everywhere else — left as its own deliberate follow-up.
function ConfigBackupButton({
  t,
  onBackedUp,
  externallyBusy = false,
  busyPhase,
}: {
  t: T;
  onBackedUp: () => void;
  /** True when a backup/restore is running elsewhere (any domain). */
  externallyBusy?: boolean;
  busyPhase?: string;
}) {
  const { state, fire, isPending } = useBackupWatch({
    progressKey: "config",
    start: () => backupConfigNow(),
    matchRun: (r) => r.domain === "config",
    onDone: onBackedUp,
  });

  return (
    <div className="flex flex-col gap-1 items-start">
      <button
        onClick={() => void fire()}
        disabled={isPending || externallyBusy}
        className="inline-flex items-center gap-1.5 rounded-control bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
      >
        {isPending ? (
          <>
            <span
              className="h-3.5 w-3.5 rounded-full border-2 border-t-transparent animate-spin inline-block"
              style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
            />
            {t("config.backingUp")}
          </>
        ) : (
          t("config.backupNow")
        )}
      </button>
      {/* A backup/restore/replication elsewhere blocks a new config backup. */}
      {externallyBusy && !isPending && (
        <span className="text-xs text-carbon-textMuted">
          {t(busyPhraseKey(busyPhase))}
        </span>
      )}
      {state.phase === "success" && (
        <span className="inline-flex items-center gap-1 text-xs text-statusOk">
          <CheckDraw />
          {t("settings.saved")}
          {state.snapshotId && (
            <span dir="ltr" className="font-mono ms-1 text-start text-carbon-textMuted">{state.snapshotId.slice(0, 8)}</span>
          )}
        </span>
      )}
      {state.phase === "error" && (
        <span className="text-xs text-statusFail max-w-md wrap-break-word">{state.message}</span>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Settings card — the config self-backup is configured on its own page (unlike
// flash, whose enable/path/off-site live on the Settings page): one place to say
// "protect BombVault itself". Persists via getSettings/putSettings — the same
// mechanism the rest of the app uses; no new persistence is invented.
// ---------------------------------------------------------------------------

// "saved"/"error" were removed from this type — the toast migration below
// (GlimStone form-engine Task 9) replaced that 3000ms inline-flash outcome
// with a real toast (push(), further down), so setSaveState now only ever
// sets "idle"/"saving" — see the comment on the state declaration itself.
type SaveState = "idle" | "saving";

// jdp live-review ("Infotexte in i Infobubbles"): `hint` used to render as a
// permanent grey <p> under the field — the same "read once, costs vertical
// space forever" case design-language.md rule 8 exists to fold away. Moved
// onto the label itself as an InfoBubble instead, the exact idiom Settings.tsx
// already uses for a labelled field (see e.g. its cloud.storageClass.label
// `<span className="flex items-center gap-1">{label}<InfoBubble .../></span>`
// pair) — reused here rather than inventing a second one for this file's own
// hand-rolled field helper. Zero i18n changes: same keys (config.pathHint/
// config.offsiteHint), only where they render.
function labelledInput(
  label: string,
  value: string,
  onChange: (v: string) => void,
  placeholder: string,
  hint?: string
) {
  return (
    <div className="flex flex-col gap-1">
      <span className="flex items-center gap-1 text-xs text-carbon-textSub">
        {label}
        {hint && <InfoBubble tip={hint} />}
      </span>
      <input
        value={value}
        spellCheck={false}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        dir="ltr"
        className="rounded-control bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text font-mono bv-field-focus text-start"
      />
    </div>
  );
}

function ConfigSettingsCard({
  t,
  settings,
  setSettings,
  hueIndex,
}: {
  t: T;
  settings: Settings;
  setSettings: (updater: (prev: Settings) => Settings) => void;
  /** Rainbow position for this Card's own heading notch — see Settings.tsx's
   *  own `Card`/Badge.tsx's `hueIndex` doc for the full history. This page
   *  has exactly three static, always-in-the-same-order Cards (this one,
   *  the backup Card, the snapshots Card below), a genuine small list, so
   *  each gets its own position rather than the flat single accent. */
  hueIndex?: number;
}) {
  const { push } = useToast();
  // Only "idle"/"saving" are ever set now — the SaveBar success/error pattern
  // (GlimStone form-engine Task 9's other toast candidate, alongside the
  // copy-feedback sites) used to hold "saved"/"error" here for a 3000ms
  // inline-text flash; that completion notice is now a toast instead (push
  // below), so there's no lingering render state left to revert from.
  const [saveState, setSaveState] = useState<SaveState>("idle");
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): shake
  // the Save button alongside the toast on a failed save.
  const [shake, setShake] = useState(0);

  async function handleSave() {
    setSaveState("saving");
    try {
      // Re-fetch the latest settings and merge only the fields THIS card owns,
      // then PUT. Since the self-backup + off-site cadences moved to Settings ›
      // Schedules (the sole schedule owner), a full-object PUT of this page's
      // mount-time snapshot could otherwise re-assert a stale configSchedule/
      // configOffsiteSchedule and silently disable a schedule set elsewhere.
      const latest = await getSettings();
      // Do NOT fall back to the stale mount-time snapshot on a failed re-fetch:
      // the backend returns {ok:false} at HTTP 200 (does not throw), and PUTting
      // the old snapshot would re-assert a stale configSchedule/configOffsiteSchedule
      // now owned by Settings › Schedules, silently reverting a schedule set
      // elsewhere. Abort the save instead.
      if (!latest.ok) {
        setSaveState("idle");
        push(latest.error ?? "Could not load current settings", "fail");
        setShake((n) => n + 1);
        return;
      }
      const merged: Settings = {
        ...latest.settings,
        configEnabled: settings.configEnabled,
        configPath: settings.configPath,
        configOffsite: settings.configOffsite,
        configOffsiteImmutable: settings.configOffsiteImmutable,
      };
      const res = await putSettings(merged);
      if (res.ok) {
        setSaveState("idle");
        // "fail"/"warn" toasts always surface even in quiet mode; "success"
        // is the routine, suppressible case (design-language.md "Toasts").
        push(t("settings.saved"), "success");
      } else {
        setSaveState("idle");
        push(res.error ?? "Save failed", "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      setSaveState("idle");
      push(err instanceof Error ? err.message : "Save failed", "fail");
      setShake((n) => n + 1);
    }
  }

  return (
    // `glim-notch-card` (jdp, live-review — see Settings.tsx's Card() for the
    // full reasoning): lets this card's own hueIndex'd heading notch reveal
    // its colour in reactive rainbow mode on hover/focus anywhere in the
    // card, not just its own tiny badge glyph.
    <div className="relative glim-notch-card bg-carbon-surface rounded-card p-5 flex flex-col gap-4">
      {/* Task 5 (rule 11): same Badge-in-<h2> pattern as Settings.tsx's own
          Card component — this hand-rolled Card equivalent never shared
          Card's component, so it needed its own copy of the conversion.
          jdp live-review ("Infotexte in i Infobubbles"): the permanent <p>
          under the heading (what this whole Card protects) is exactly
          Card's own `hint` case — folded into an InfoBubble on the Badge
          itself, same content (`config.settingsHint`), same `onAccent` this
          badge's solid accent fill needs (see Settings.tsx's Card() and
          Flash.tsx's identical backup-Card fix for the reasoning). */}
      <h2 className="flex items-center">
        <Badge tone="heading" size="heading" wrap hueIndex={hueIndex}>
          {t("config.settingsTitle")}
          <InfoBubble tip={t("config.settingsHint")} onAccent />
        </Badge>
      </h2>

      <ToggleRow
        label={t("config.enabled")}
        description={t("config.enabledHint")}
        checked={settings.configEnabled}
        onChange={(v) => setSettings((prev) => ({ ...prev, configEnabled: v }))}
      />

      {labelledInput(
        t("config.path"),
        settings.configPath,
        (v) => setSettings((prev) => ({ ...prev, configPath: v })),
        "user/bombvault/config",
        t("config.pathHint")
      )}

      {/* The self-backup + off-site cadences moved to Settings › Schedules (the
          single schedule owner). Only path / off-site repo / immutable live here. */}
      {labelledInput(
        t("config.offsite"),
        settings.configOffsite,
        (v) => setSettings((prev) => ({ ...prev, configOffsite: v })),
        "rest:http://host:8000/repo",
        t("config.offsiteHint")
      )}

      <ToggleRow
        label={t("config.immutable")}
        description={t("config.immutableHint")}
        checked={settings.configOffsiteImmutable}
        onChange={(v) => setSettings((prev) => ({ ...prev, configOffsiteImmutable: v }))}
      />

      <div className="flex items-center gap-3 pt-1">
        <button
          key={shake}
          onClick={() => void handleSave()}
          disabled={saveState === "saving"}
          className={`inline-flex items-center gap-2 rounded-control bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${
            shake ? " glim-shake" : ""
          }`}
        >
          {saveState === "saving" ? (
            <>
              <span
                className="h-3.5 w-3.5 rounded-full border-2 border-t-transparent animate-spin"
                style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
              />
              {t("common.saving")}
            </>
          ) : (
            t("settings.save")
          )}
        </button>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Snapshot row — id + timestamp, with a delete affordance (mirrors Flash's
// FlashSnapshotRow, minus the zip download). Delete targets the currently viewed
// repo via `source`; an off-site append-only repo refuses the delete server-side,
// so the returned error is surfaced in `deleteErr` rather than hidden.
// ---------------------------------------------------------------------------

function ConfigSnapshotRow({
  snap,
  source,
  onDeleted,
  t,
}: {
  snap: Snapshot;
  source: RepoSource;
  onDeleted: () => void;
  t: T;
}) {
  const [deleting, setDeleting] = useState(false);
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): a
  // failed delete toasts AND shakes the delete button.
  const [shake, setShake] = useState(0);

  async function handleDelete() {
    if (!(await confirm(t("snapshots.deleteConfirm")))) return;
    setDeleting(true);
    try {
      const res = await deleteSnapshot("config", snap.id, source);
      if (res.ok) onDeleted();
      else {
        push(res.error ?? "Delete failed", "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : "Delete failed", "fail");
      setShake((n) => n + 1);
    } finally {
      setDeleting(false);
    }
  }

  return (
    <div className="flex flex-col gap-1 py-2.5 border-b border-carbon-border last:border-0">
      <div className="flex items-center gap-3 text-sm">
        <span dir="ltr" className="font-mono text-start text-carbon-text text-xs w-20 shrink-0">{snap.id.slice(0, 8)}</span>
        <span className="text-carbon-textMuted text-xs flex-1">
          {new Date(snap.time).toLocaleString()}
        </span>
        <button
          key={shake}
          onClick={() => void handleDelete()}
          disabled={deleting}
          title={t("snapshots.delete")}
          className={`shrink-0 rounded-control px-2 py-1 text-xs text-carbon-textSub hover:bg-statusFailBg hover:text-statusFail transition-colors disabled:opacity-50${
            shake ? " glim-shake" : ""
          }`}
        >
          {deleting ? "…" : t("snapshots.delete")}
        </button>
      </div>
      {confirmDialog}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Config page — BombVault's OWN settings self-backup. Backup + status only; the
// restore flow (which restarts the app to swap the live DB) lives in the Recovery
// tab, so the self-referential restart stays in one place.
// ---------------------------------------------------------------------------

export function Config() {
  const { t } = useT();
  const [settings, setSettings] = useState<Settings | null>(null);
  const [source, setSource] = useState<RepoSource>("local");
  const [snapshots, setSnapshots] = useState<Snapshot[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const progressMap = useProgress();
  const progress = progressMap["config"];
  // Any backup/restore/replication in flight (any domain) disables the config
  // backup button + shows a hint, instead of relying on the 409 round-trip.
  const running = anyActive(progressMap);

  useEffect(() => {
    getSettings()
      .then((res) => {
        if (res.ok) setSettings(res.settings);
      })
      .catch(() => undefined);
  }, []);

  function load() {
    setError(null);
    return listConfigSnapshots(source)
      .then((res) => {
        if (res.ok) setSnapshots(res.snapshots ?? []);
        else setError(res.error ?? "Failed to load config backups");
      })
      .catch((err: unknown) =>
        setError(err instanceof Error ? err.message : "Failed to load config backups")
      );
  }

  useEffect(() => {
    setLoading(true);
    void load().finally(() => setLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [source]);

  return (
    // gap-10 (jdp live-review: "Abstände zwischen den Cards zu klein und
    // erste Card zu weit oben, systemweit gleich machen"): was gap-6 (24px)
    // for EVERY gap on this page, including heading-to-first-card — measured
    // live, that's only 24px, and with the heading-badge's own half-overlap
    // poking 11px into it (Badge.tsx's `-translate-y-1/2` notch), the actual
    // VISIBLE whitespace above each Card was just 13px, on this page only.
    // The rest of the app already settled on gap-10 (40px) as the one
    // Card-to-Card (and heading-to-first-card) rhythm — see Dashboard.tsx's
    // own identical live-review fix ("Die Abstände der Cards passen nicht.
    // Bitte systemweit anpassen!") and Settings.tsx's tab-panels wrapper,
    // both of which measured every OTHER page's gap at 40px before bumping
    // to match. Unlike those two pages, this page's heading is a single bare
    // `<h1>+<p>` div with no tab-strip/indicator row that needs to stay at
    // the tighter 24px — so there's nothing here that needs splitting into a
    // nested gap-6 wrapper the way Dashboard/Settings needed; one flat
    // gap-10 on this single wrapper already gives every gap (heading→Card 1,
    // Card 1→2, Card 2→3) the same corrected 40px DOM gap (29px visible,
    // after the same 11px badge overlap every other gap-10 page also has).
    <div className="flex flex-col gap-10 max-w-3xl">
      {/* Page heading */}
      <div>
        <h1 className="text-2xl font-semibold text-carbon-text">{t("config.title")}</h1>
        <p className="mt-1 text-sm text-carbon-textSub">{t("config.subtitle")}</p>
      </div>

      {/* Settings card */}
      {settings && (
        <ConfigSettingsCard t={t} settings={settings} setSettings={(u) => setSettings((prev) => (prev ? u(prev) : prev))} hueIndex={0} />
      )}

      {/* Backup card. GlimStone follow-up pass ("half-overlap card notch"):
          split into an outer structural `relative` div (hosting the heading
          Badge, now `position: absolute`) + this same inner
          `relative overflow-hidden` div (unchanged, still the box
          ProgressBar.tsx documents clipping itself to) — the inner div's own
          overflow-hidden would otherwise clip the badge's -11px poke above
          it, so the badge needed to move outside that clipping box; see
          Badge.tsx's badgeClassName comment and Dashboard.tsx's Card() for
          the identical split. */}
      {/* `glim-notch-card` on this OUTER div, not the inner overflow-hidden
          box: the badge itself lives here (see the split's own comment
          above), so this is the element that has to be the hover/focus zone
          for index.css's card-wide reactive-hover rule — see Settings.tsx's
          Card() for the full reasoning. Its own bounding box is still
          exactly the visible card (h2 + the inner box beneath it), so this
          doesn't change what "hovering the card" looks like. */}
      {/* GlimStone follow-up pass (jdp, live review, root-mechanism fix
          replacing this file's own earlier `ps-5`-on-the-h2 patch): the
          outer div is deliberately unpadded (see the split comment above),
          which leaves Badge.tsx's own CSS static-position fallback measuring
          the badge's horizontal position against THIS outer div's bare edge
          instead of the inner p-5 box's content edge — the same bug this
          page ONCE fixed by adding `ps-5` to the `<h2>` alone (a real fix,
          but a per-call-site padding patch a future edit to this div could
          silently un-fix again). Replaced with `insetStart={5}` on the Badge
          itself: an explicit, self-documenting override at the ONE place
          that actually knows the inner box's own padding number — see
          Badge.tsx's own `insetStart` doc for the full mechanism and its
          other real call sites (Flash.tsx's identical Backup Card,
          Dashboard.tsx's Card() and SummaryCell(), all independently hit the
          identical mismatch).
          Also folds the permanent backupHint <p> into an InfoBubble on the
          Badge (same "Infotexte in Infobubbles" fix as Flash.tsx's sibling
          backup Card, same content, same onAccent). */}
      <div className="relative glim-notch-card">
        <h2 className="flex items-center">
          <Badge tone="heading" size="heading" wrap hueIndex={1} insetStart={5}>
            {t("config.backupTitle")}
            <InfoBubble tip={t("config.backupHint")} onAccent />
          </Badge>
        </h2>
        <div className="relative overflow-hidden bg-carbon-surface rounded-card p-5 flex flex-col gap-4">
          <ConfigBackupButton
            t={t}
            onBackedUp={() => void load()}
            externallyBusy={running.active}
            busyPhase={running.phase}
          />

          {/* Live backup/restore progress, pinned to the card's bottom edge */}
          {progress && (
            <ProgressBar percent={progress.percent} active={progress.active} />
          )}
        </div>
      </div>

      {/* Snapshots card — list + delete; restoring settings lives in Recovery.
          `glim-notch-card`: see Settings.tsx's Card() for the reasoning. */}
      <div className="relative glim-notch-card bg-carbon-surface rounded-card p-5 flex flex-col gap-4">
        {/* jdp live-review ("Infotexte in i Infobubbles"): this used to be a
            permanent bg-statusNeutralBg banner (Task 7 had already folded
            its COLOUR from the old fifth "info" hue into neutral, but kept
            the banner FORM — pure informational prose, not a live status
            readout, exactly rule 8's "read once, costs vertical space
            forever" case). Same content (`config.snapshotsHint`), now an
            InfoBubble on the heading Badge instead — the identical fix
            Flash.tsx's own Restore card (this page's direct sibling) just
            got for its own restoreNote banner. */}
        <h2 className="flex items-center">
          <Badge tone="heading" size="heading" wrap hueIndex={2}>
            {t("config.snapshotsTitle")}
            <InfoBubble tip={t("config.snapshotsHint")} onAccent />
          </Badge>
        </h2>

        <div className="flex flex-col gap-1">
          <div className="flex items-center gap-2">
            <span className="text-xs text-carbon-textMuted">{t("source.label")}</span>
            <SourceToggle source={source} onChange={setSource} disabled={loading} domain="config" />
          </div>
          <p className="text-caption text-carbon-textMuted">{t("source.hint")}</p>
        </div>

        {loading && <p className="text-xs text-carbon-textMuted">{t("dashboard.checking")}</p>}
        {error && <p className="text-xs text-statusFail">{error}</p>}
        {!loading && !error && snapshots.length === 0 && (
          <p className="text-xs text-carbon-textMuted">{t("config.none")}</p>
        )}
        {!loading && snapshots.length > 0 && (
          <div className="rounded-card bg-carbon-background px-3 py-1">
            {snapshots.map((snap) => (
              <ConfigSnapshotRow
                key={snap.id}
                snap={snap}
                source={source}
                onDeleted={() => void load()}
                t={t}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
