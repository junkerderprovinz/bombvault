import { useEffect, useRef, useState, type CSSProperties } from "react";
import { hueVars, rainbowAt } from "../lib/appearance";
import { backupConfigNow, getSettings, putSettings } from "../lib/api";
import type { Settings } from "../lib/api";
import { useT } from "../lib/i18n";
import { PAGE_SHELL } from "../lib/pageShell";
import { BackupCancelButton } from "../components/BackupCancelButton";
import { ProgressBar } from "../components/ProgressBar";
import { useProgress, anyActive, busyPhraseKey } from "../lib/progress";
import { useBackupWatch } from "../lib/backupWatch";
import { PlacementFlow } from "../components/placement/PlacementFlow";
import { Timeline } from "../components/timeline/Timeline";
import { ToggleRow } from "./settings/shared";
import { useToast } from "../lib/toast";
import { Badge } from "../components/Badge";
import { Button } from "../components/Button";
import { InfoBubble } from "../components/InfoBubble";
import { IconBackupNow } from "../components/Sidebar";
import { tLtr } from "../lib/ltrFragments";

type T = ReturnType<typeof useT>["t"];

// ---------------------------------------------------------------------------
// Backup button — fire-and-watch, mirroring the flash domain (see useBackupWatch:
// the config backup runs detached on the server and the POST returns immediately,
// so we watch the "config" progress + recorded run for the outcome).
//
// Square icon-only badge, flush right in the card (jdp, live review: "Tab
// Selbst-Backup: in der 'Einstellungen jetzt sichern' Card der 'Einstellungen
// jetzt sichern' Button soll ein quadratischer Badge mit Speichern-Glyph sein
// und ganz rechts angeordnet sein"). Was a full-width `bg-accent px-4 py-1.5
// text-sm` text button with THREE permanently-inline states stacked below it
// (blocked-elsewhere hint, a CheckDraw success line carrying the snapshot id,
// a red error message).
//
// This is the SAME conversion Flash.tsx's FlashBackupButton already received
// in 63f53d5, mirrored rather than re-invented — same `IconBackupNow` glyph,
// same `shape="square" size="icon" tone="active"` recipe, same `tip` priority
// order (pending → blocked-by-other → label), same `flex justify-end` wrapper
// at the call site, and the same terminal-state migration: success and error
// become TOASTS, because a square badge has no room for inline text. An error
// additionally shakes the badge, per the system-wide "a failed action toasts
// AND shakes its button" rule.
//
// That toast migration supersedes this comment's own earlier v8.0.0 audit
// note, which deferred it on the grounds that useBackupWatch's state shape
// also backs RESTORE outcomes elsewhere (sticky by design, kind="restore" —
// see the hook's SUCCESS_CLEAR_MS comment). That reasoning still correctly
// blocks changing the HOOK, which is untouched here. But rendering
// state.phase as a toast is a per-component decision, not a hook change:
// Containers.tsx's BackupButton and then Flash.tsx's both proved it — same
// hook, zero hook changes, just a different render for kind="backup"'s
// already-self-clearing 4s terminal states. VMs.tsx's VMBackupButton is the
// one remaining full-width text button of this family and is NOT touched
// here; jdp's ask named this card, and unlike the source.hint sweep in this
// same pass that one is a different card layout (a per-VM row control), not
// another copy of this exact card.
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
  // A backup/restore/replication elsewhere blocks a new config backup.
  const blockedByOther = externallyBusy && !isPending;
  const { push } = useToast();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): a
  // failed action toasts AND shakes its button.
  const [shake, setShake] = useState(0);
  // Tracks the last phase already reported, so this effect toasts exactly
  // once per NEW terminal transition — same guard as Flash.tsx's
  // FlashBackupButton (state.phase can only ever start at "idle", so this
  // never fires on mount, only on a real fire()-driven change).
  const seenPhase = useRef(state.phase);

  useEffect(() => {
    if (state.phase === seenPhase.current) return;
    seenPhase.current = state.phase;
    if (state.phase === "success") {
      push(
        state.snapshotId ? `${t("settings.saved")} · ${state.snapshotId.slice(0, 8)}` : t("settings.saved"),
        "success"
      );
    } else if (state.phase === "error") {
      push(state.message, "fail");
      setShake((n) => n + 1);
    }
  }, [state, push, t]);

  // #178: stable name, exceptional states as tooltip only.
  const stateTip = isPending
    ? t("config.backingUp")
    : blockedByOther
      ? t(busyPhraseKey(busyPhase))
      : undefined;

  return (
    <Button
      key={shake}
      label={t("config.backupNow")}
      labelKey="config.backupNow"
      glyph={<IconBackupNow />}
      tone="accent"
      onClick={() => void fire()}
      disabled={isPending || blockedByOther}
      busy={isPending}
      title={stateTip}
      className={shake ? "glim-shake" : ""}
    />
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
  // Only the setter survives: with the Save button gone there is no control
  // left to disable while a write is in flight, but the state still guards
  // against overlapping writes and keeps the shape of every other autosaving
  // card in the app.
  const [, setSaveState] = useState<SaveState>("idle");

  async function persist(enabled: boolean) {
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
        push(latest.error ?? t("config.loadSettingsFailed"), "fail");
        return;
      }
      // configOffsite/configOffsiteImmutable are deliberately NOT merged in
      // any more: since #176 self-backup has a full off-site card in Settings ›
      // Off-site like every other domain, and that card owns them. Sending this
      // page's mount-time snapshot would re-assert a stale repo URL and undo an
      // edit made there, exactly the way the schedules used to be clobbered
      // before they moved out for the same reason.
      // configPath left out for the same reason as the two above (#182): the
      // path row in Settings › Paths and storage owns it now, and re-asserting
      // this page's snapshot would undo a location set there.
      const merged: Settings = {
        ...latest.settings,
        configEnabled: enabled,
      };
      const res = await putSettings(merged);
      if (res.ok) {
        setSaveState("idle");
        // "fail"/"warn" toasts always surface even in quiet mode; "success"
        // is the routine, suppressible case (design-language.md "Toasts").
        push(t("settings.saved"), "success");
      } else {
        setSaveState("idle");
        push(res.error ?? t("common.saveFailed"), "fail");
      }
    } catch (err) {
      setSaveState("idle");
      push(err instanceof Error ? err.message : t("common.saveFailed"), "fail");
    }
  }

  return (
    // `glim-notch-card` (jdp, live-review — see Settings.tsx's Card() for the
    // full reasoning): lets this card's own hueIndex'd heading notch reveal
    // its colour in reactive rainbow mode on hover/focus anywhere in the
    // card, not just its own tiny badge glyph.
    //
    // `.glim-hue` ALSO added (rainbow-mode completeness sweep, jdp live
    // review: "Es sind nicht alle Buttons in den Regenbogen-Modus
    // eingepflegt"): `glim-notch-card` alone never redefines
    // --accent/--focus-ring, only the reactive-mode hover reveal — so the
    // Save button below stayed the flat theme accent regardless of rainbow.
    // Same hueIndex prop the Badge already uses (StepCard.tsx's/
    // Dashboard.tsx Card()'s own identical fix, same mechanism: custom
    // properties cascade to every descendant once redefined once here).
    <div
      className={`relative glim-notch-card bg-carbon-surface rounded-card p-5 flex flex-col gap-4${
        hueIndex !== undefined ? " glim-hue" : ""
      }`}
      style={hueIndex !== undefined ? (hueVars(rainbowAt(hueIndex)) as CSSProperties) : undefined}
    >
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

      {/* Rule 8, "explanations live in a bubble, not on the page": these were
          the app's LAST two `description` captions, on the same card whose own
          heading text the sweep did convert six lines above. A permanent grey
          paragraph is read once and costs vertical space forever. */}
      {/* Saves itself (#182, manilx: "switching the setting autosaves ... here i
          need to select save button"). Every other toggle in the app persists
          on the spot; this card kept a Save button because it used to own three
          text fields, and once those moved out a lone toggle behind a button
          was the only one of its kind left. */}
      <ToggleRow
        label={t("config.enabled")}
        hint={tLtr(t, "config.enabledHint")}
        checked={settings.configEnabled}
        onChange={(v) => {
          setSettings((prev) => ({ ...prev, configEnabled: v }));
          void persist(v);
        }}
      />

      {/* The backup location moved out too (#182, manilx: "Can't set
          credentials here"). It was a plain text field, while Settings › Paths
          and storage has had the same value as a full path row for a long
          time: local or remote, and for a remote one the safety dialog that
          holds bandwidth limits, append-only and, since #182, the credential
          set. Someone pointing self-backup at an S3 bucket from THIS field
          therefore had nowhere to say which keys it should use.
          Its caption was wrong for that case as well, promising a "relative
          subpath under the host mount root" for a value that had just been
          given an s3: URL.
          Keeping both would also have repeated the schedules/off-site mistake:
          two editors for one setting, with this page's mount-time snapshot
          able to overwrite the other one. */}
      <p className="text-xs text-carbon-textMuted">{t("config.pathMoved")}</p>

      {/* The self-backup + off-site cadences moved to Settings › Schedules (the
          single schedule owner), and since #176 the off-site repo and its
          append-only flag moved to Settings › Off-site, where self-backup now
          has the same card every other domain has: a setup wizard, a connection
          test, replicate-now, extra destinations and per-destination
          credentials. Only the enable toggle lives here now. */}
      <p className="text-xs text-carbon-textMuted">{t("config.offsiteMoved")}</p>
      <PlacementFlow domain="config" />

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
  const [reloadTick, setReloadTick] = useState(0);
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

  return (
    // PAGE_SHELL (jdp live-review: "Im Tab Selbst-Backup und Flash sind die
    // Cards schmaler. Können wir die nicht überall gleich breit machen?").
    // This page — "Selbst-Backup" — is one of the two he named: it was
    // max-w-3xl (768px) against 1024px on five pages and 1152px on Dashboard,
    // the narrowest in the app. The gap here was already the correct 40px
    // from an earlier round; only the width changes. See lib/pageShell.ts for
    // the full before/after measurement table and why 1152px won.
    //   The heading is a single bare `<h1>+<p>` div with no tab-strip or
    // indicator row that needs a tighter gap of its own, so the one flat
    // PAGE_SHELL gap governs every gap on the page (heading→Card 1, 1→2,
    // 2→3) — no nested sub-wrapper needed the way Dashboard/Settings have.
    <div className={PAGE_SHELL}>
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
      {/* `.glim-hue` added (rainbow-mode completeness sweep, jdp live review:
          "Es sind nicht alle Buttons in den Regenbogen-Modus eingepflegt"):
          `glim-notch-card` alone never redefines --accent/--focus-ring, only
          the reactive-mode hover reveal — so ConfigBackupButton's own
          bg-accent button below stayed flat regardless of rainbow. Same
          hueIndex={1} the Badge already uses; the inner overflow-hidden box
          inherits it too via ordinary CSS custom-property cascade. */}
      <div className="relative glim-notch-card glim-hue" style={hueVars(rainbowAt(1)) as CSSProperties}>
        <h2 className="flex items-center">
          <Badge tone="heading" size="heading" wrap hueIndex={1} insetStart={5}>
            {t("config.backupTitle")}
            <InfoBubble tip={tLtr(t, "config.backupHint")} onAccent />
          </Badge>
        </h2>
        <div className="relative overflow-hidden bg-carbon-surface rounded-card p-5 flex flex-col gap-4">
          {/* jdp live-review: "der Button ... ganz rechts angeordnet" — the
              badge is the row's only content, right-aligned via justify-end.
              This app's established "push to the row's far edge" idiom is
              `ms-auto` on the badge itself when it shares a row with a leading
              sibling (see Containers.tsx's BackupButton/ExportButton row), but
              there is no leading sibling here, so justify-end on the row gets
              the identical flush-right result with nothing to push away from —
              byte-identical to how Flash.tsx's own backup card does it. */}
          <div className="flex justify-end">
            <ConfigBackupButton
              t={t}
              onBackedUp={() => setReloadTick((n) => n + 1)}
              externallyBusy={running.active}
              busyPhase={running.phase}
            />
          </div>

          {/* Stop a backup that is running (#200), gated exactly as on the
              Folders page: not on a RESTORE, which has its own control with its
              own warning, and only while the run is active. The server has
              accepted this key all along; only the button was missing. */}
          {progress && progress.active && progress.phase !== "restore" && (
            <div className="flex justify-end">
              <BackupCancelButton cancelKey={"config"} name={t("nav.config")} t={t} />
            </div>
          )}

          {/* Live backup/restore progress, pinned to the card's bottom edge */}
          {progress && (
            <ProgressBar percent={progress.percent} active={progress.active} />
          )}
        </div>
      </div>

      {/* Snapshots card — list + delete; restoring settings lives in Recovery.
          `glim-notch-card`: see Settings.tsx's Card() for the reasoning.
          `.glim-hue` added (rainbow-mode completeness sweep, jdp live
          review): same hueIndex={2} the Badge already uses. Timeline's own
          delete button inherits it via the ordinary custom-property cascade. */}
      <div
        className="relative glim-notch-card glim-hue bg-carbon-surface rounded-card p-5 flex flex-col gap-4"
        style={hueVars(rainbowAt(2)) as CSSProperties}
      >
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

        <div className="rounded-card bg-carbon-background px-3 py-1">
          <Timeline
            key={reloadTick}
            domain="config"
            itemKey="config"
            itemName={t("config.snapshotsTitle")}
            open
            renderActions={() => null}
          />
        </div>
      </div>
    </div>
  );
}
