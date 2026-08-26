import { useEffect, useRef, useState, type CSSProperties } from "react";
import { hueVars, rainbowAt } from "../lib/appearance";
import { backupFlashNow, listFlashSnapshots, flashDownloadURL, deleteSnapshot } from "../lib/api";
import type { Snapshot } from "../lib/api";
import { useT } from "../lib/i18n";
import { PAGE_SHELL } from "../lib/pageShell";
import { ProgressBar } from "../components/ProgressBar";
import { useProgress, anyActive, busyPhraseKey } from "../lib/progress";
import { useBackupWatch } from "../lib/backupWatch";
import { SourceToggle, type RepoSource } from "../components/SourceToggle";
import { OffsiteIndicator } from "../components/OffsiteIndicator";
import { useConfirm } from "../lib/useConfirm";
import { Badge } from "../components/Badge";
import { useToast } from "../lib/toast";
import { FlashZipExportCard } from "./Settings";
import { InfoBubble } from "../components/InfoBubble";
import { IconBackupNow, IconDownload, IconTrash } from "../components/Sidebar";
import { tLtr } from "../lib/ltrFragments";

type T = ReturnType<typeof useT>["t"];

// ---------------------------------------------------------------------------
// Backup button
// ---------------------------------------------------------------------------

// Square icon-only badge, flush right in the card (jdp live-review: "In der
// Flash-sichern-Card soll der 'Flash jetzt sichern'-Button ein quadratischer
// Badge mit Glyph sein, der ganz rechts in der Card platziert ist"). Was a
// full-width `bg-accent px-4 py-1.5 text-sm` button with THREE permanently-
// inline states living below it (pending spinner+label, success
// checkmark+snapshot id, a red error message) — the identical shape
// Containers.tsx's own BackupButton (that file's per-container "Jetzt
// sichern" conversion) already solved: there is no room for any of that
// next to a small square glyph, so every TERMINAL state (success/error) now
// surfaces as a toast instead, matching the "failed action toasts AND
// shakes its button" standing rule this same file's FlashSnapshotRow delete
// button already follows. Only the PENDING state stays inline — swapped for
// the glyph itself (a spinner replacing the icon while running), same as
// Containers.tsx's BackupButton.
//
// `size="icon"` (Badge.tsx, h-8/w-8 = 32px): reused verbatim from
// Containers.tsx's BackupButton rather than re-measured against THIS
// button's own prior self — per the standing size-token rule ("check whether
// an established token for the EXACT role already exists... rather than
// independently re-measuring your own local context"), this is the same
// role: a domain's "back up now" action, icon-only, the same IconBackupNow
// glyph. Re-deriving a number from the old text button's own footprint here
// would be exactly the "each individually well-fitted to its own [former]
// neighbour" trap the standing rule names — every square icon badge in this
// app renders at the identical 32px, not several close-but-different numbers
// that each looked fine in isolation. (This comment used to read "h-7/w-7 =
// 28px", written when `icon` was one of THREE icon-badge stages; that
// role-based split was reported broken by jdp twice and removed, and `icon`
// is now the app's single 32px stage — see Badge.tsx's "ONE SIZE FOR SQUARE
// ICON BADGES" block. The prop never changed, only the token behind it.)
//
// GlimStone follow-up pass (v8.0.0) audit note, UPDATED: an earlier version
// of this comment deferred migrating state.phase's success/error rendering
// to a toast, reasoning that useBackupWatch's shared state shape also backs
// Config.tsx's ConfigBackupButton / VMs.tsx's VMBackupButton / restore
// outcomes elsewhere (which stay STICKY BY DESIGN, kind="restore" — see the
// hook's own SUCCESS_CLEAR_MS comment). That reasoning still correctly
// blocks changing the HOOK itself — untouched here, same as always. But
// rendering state.phase as a toast instead of inline text is a per-
// component, per-file decision (Containers.tsx's BackupButton already
// proved this: same hook, zero hook changes, just a different render for
// kind="backup"'s already-self-clearing 4s terminal states) — not the
// hook-level architecture change the old comment worried about. This file
// now makes that same local rendering choice, for this jdp-requested badge
// conversion specifically. Config.tsx's ConfigBackupButton and VMs.tsx's
// VMBackupButton are UNCHANGED, still full-width text buttons — out of
// scope for this pass, not attempted here.
function FlashBackupButton({
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
  // Fire-and-watch (see useBackupWatch): the flash backup runs detached on the
  // server and the POST returns immediately, so we watch the "flash" progress +
  // recorded run for the outcome instead of awaiting the whole backup.
  const { state, fire, isPending } = useBackupWatch({
    progressKey: "flash",
    start: () => backupFlashNow(),
    matchRun: (r) => r.domain === "flash",
    onDone: onBackedUp,
  });
  // A backup/restore/replication elsewhere blocks a new flash backup.
  const blockedByOther = externallyBusy && !isPending;
  const { push } = useToast();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): a
  // failed action toasts AND shakes its button.
  const [shake, setShake] = useState(0);
  // Tracks the last phase already reported, so this effect toasts exactly
  // once per NEW terminal transition — same guard as Containers.tsx's
  // BackupButton (state.phase can only ever start at "idle", so this never
  // fires on mount, only on a real fire()-driven change).
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

  const tip = isPending
    ? t("flash.backingUp")
    : blockedByOther
      ? t(busyPhraseKey(busyPhase))
      : t("flash.backupNow");

  return (
    <Badge
      key={shake}
      as="button"
      shape="square"
      tone="active"
      size="icon"
      tip={tip}
      onClick={() => void fire()}
      disabled={isPending || blockedByOther}
      className={shake ? "glim-shake" : undefined}
    >
      {isPending ? (
        <span
          className="h-3 w-3 rounded-full border-2 border-t-transparent animate-spin inline-block"
          style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
        />
      ) : (
        <IconBackupNow />
      )}
    </Badge>
  );
}

// ---------------------------------------------------------------------------
// Snapshot row (zip download restore)
// ---------------------------------------------------------------------------

// How long the download button shows a "preparing" spinner after a click,
// bridging the gap before the browser's own download indicator takes over
// (see handleDownload). Not a real progress signal, just a best-effort
// window: the server can't send a single byte until it has finished
// dumping + recompressing the whole snapshot server-side (see
// dumpFlashZipCompat in the Go backend), which for a multi-GB flash backup
// routinely takes longer than a moment. A generous fixed timeout beats no
// feedback at all; there is no browser event to key off instead.
const DOWNLOAD_PREPARING_MS = 20_000;

function FlashSnapshotRow({ snap, source, onDeleted, t }: { snap: Snapshot; source: RepoSource; onDeleted: () => void; t: T }) {
  const [deleting, setDeleting] = useState(false);
  const [preparing, setPreparing] = useState(false);
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): a
  // failed delete toasts AND shakes the delete button.
  const [shake, setShake] = useState(0);

  async function handleDelete() {
    if (!(await confirm(t("snapshots.deleteConfirm")))) return;
    setDeleting(true);
    try {
      const res = await deleteSnapshot("flash", snap.id, source);
      if (res.ok) onDeleted();
      else {
        push(res.error ?? t("common.deleteFailed"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.deleteFailed"), "fail");
      setShake((n) => n + 1);
    } finally {
      setDeleting(false);
    }
  }

  // Native <a download>, not fetch()+Blob: the browser's own download
  // manager then owns progress/completion, so it survives this row
  // unmounting on tab switch (React was silently discarding the old fetch
  // loop's progress state on remount, not the download itself). Trades away
  // the pre-stream JSON-error surfacing that fetch() gives downloadRecoveryKit
  // in lib/api.ts — acceptable here since a flash zip is far larger and
  // failures are rare.
  function handleDownload() {
    setPreparing(true);
    setTimeout(() => setPreparing(false), DOWNLOAD_PREPARING_MS);
    const a = document.createElement("a");
    a.href = flashDownloadURL(snap.id, source);
    a.download = `flash-${snap.id.slice(0, 8)}.zip`;
    document.body.appendChild(a);
    a.click();
    a.remove();
  }

  return (
    // py-1.5, not the py-2.5 this row used to carry — the identical trade
    // components/RestorePanel.tsx's SnapshotRow, pages/Config.tsx's
    // ConfigSnapshotRow and pages/Files.tsx's FileSetSnapshotRow each already
    // made, and for the identical reason: this row's controls grew from ~24px
    // text buttons to the app's one 32px square icon badge, and trimming 4px
    // of padding per side keeps the row at exactly the 44px it measured
    // before. A bigger badge in a list of unchanged density, rather than a
    // list that grew. This was the FOURTH and last copy of that row.
    <div className="flex flex-col gap-1 py-1.5 border-b border-carbon-border last:border-0">
      <div className="flex items-center gap-3 text-sm">
        <span dir="ltr" className="font-mono text-start text-carbon-text text-xs w-20 shrink-0">{snap.id.slice(0, 8)}</span>
        <span className="text-carbon-textMuted text-xs flex-1">
          {new Date(snap.time).toLocaleString()}
        </span>
        {/* Download + Delete as square icon badges (jdp, live review:
            "Flash-Tab, Backups-Card: die Buttons Download und Löschen sind
            Text-Buttons, sollen aber quadratische Badges mit Glyphen sein
            (inkl. Farbmodi, keine extra Färbung für Löschen)"). This row was
            the LAST unconverted copy of a pattern already fixed in
            components/RestorePanel.tsx, pages/Config.tsx's ConfigSnapshotRow
            and pages/Files.tsx's FileSetSnapshotRow — not a regression, a
            spot those three passes each missed.
              Both take the same recipe as those three siblings: shape="square"
            size="icon" (32px, the app's ONE square-icon-badge stage — see
            Badge.tsx's "ONE SIZE FOR SQUARE ICON BADGES" block), tone="active",
            and NO hueIndex — the Restore card that owns this list already
            carries `.glim-hue` with hueIndex={1}, so the ordinary custom-
            property cascade paints both badges in that card's own rainbow
            position. Each gets a `tip` carrying the label the glyph replaced,
            per the standing "an icon-only badge gets colour-engine
            integration AND a tooltip" rule.
              Glyphs are reused verbatim, no new drawings: IconDownload (the
            same glyph Containers.tsx's ExportButton already uses for "hand me
            this artefact as a file") and IconTrash (Config/Files/Settings'
            own remove glyph).
              The delete badge gets NO special colour treatment — not the
            `hover:bg-statusFailBg hover:text-statusFail` red flash it used to
            carry, and not a grey-neutral exemption either (neutral is one of
            the tones Badge keeps OUT of the rainbow, which would leave it flat
            grey beside a hued sibling — the exact "anders eingefärbt" defect
            jdp reported on RestorePanel's delete). Its meaning is carried by
            IconTrash, by its tip, and by the useConfirm dialog handleDelete
            already opens, which is untouched.
              Both in-flight labels ("…" for delete, the inline spinner+text
            for download) have nowhere to live on an icon-only badge: delete
            shows as `disabled` exactly like its three siblings, and download
            keeps its spinner by swapping the GLYPH for it, the same trade
            FlashBackupButton above and Containers' ExportButton already
            make. */}
        <Badge
          as="button"
          shape="square"
          size="icon"
          tone="active"
          tip={t("flash.download")}
          onClick={handleDownload}
          disabled={preparing}
          className="shrink-0"
        >
          {preparing ? (
            <span
              className="h-3 w-3 rounded-full border-2 border-t-transparent animate-spin inline-block"
              style={{ borderColor: "currentColor", borderTopColor: "transparent" }}
            />
          ) : (
            <IconDownload />
          )}
        </Badge>
        <Badge
          key={shake}
          as="button"
          shape="square"
          size="icon"
          tone="active"
          tip={t("snapshots.delete")}
          onClick={() => void handleDelete()}
          disabled={deleting || preparing}
          className={`shrink-0${shake ? " glim-shake" : ""}`}
        >
          <IconTrash />
        </Badge>
      </div>
      {confirmDialog}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Flash page
// ---------------------------------------------------------------------------

export function Flash() {
  const { t } = useT();
  const [source, setSource] = useState<RepoSource>("local");
  const [snapshots, setSnapshots] = useState<Snapshot[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const progressMap = useProgress();
  const progress = progressMap["flash"];
  // Any backup/restore/replication in flight (any domain) disables the flash
  // backup button + shows a hint, instead of relying on the 409 round-trip.
  const running = anyActive(progressMap);

  function load() {
    setError(null);
    return listFlashSnapshots(source)
      .then((res) => {
        if (res.ok) setSnapshots(res.snapshots ?? []);
        else setError(res.error ?? t("flash.loadBackupsFailed"));
      })
      .catch((err: unknown) =>
        setError(err instanceof Error ? err.message : t("flash.loadBackupsFailed"))
      );
  }

  useEffect(() => {
    setLoading(true);
    void load().finally(() => setLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [source]);

  return (
    // PAGE_SHELL (jdp live-review: "Im Tab Selbst-Backup und Flash sind die
    // Cards schmaler. Können wir die nicht überall gleich breit machen?").
    // This is the second page he named, and it was the worst case in the app:
    // `gap-6 max-w-3xl`, i.e. the NARROWEST width (768px) on the OLD 24px
    // rhythm — both values off-standard at once. See lib/pageShell.ts.
    //   The OffsiteIndicator sits inside the heading div (not as a sibling),
    // so the flat 40px shell gap governs heading→Card 1 and every Card→Card
    // gap alike; nothing here needs a nested tighter group.
    <div className={PAGE_SHELL}>
      {/* Page heading */}
      <div>
        <h1 className="text-2xl font-semibold text-carbon-text">{t("flash.title")}</h1>
        <p className="mt-1 text-sm text-carbon-textSub">{tLtr(t, "flash.subtitle")}</p>
        <div className="mt-2"><OffsiteIndicator domain="flash" /></div>
      </div>

      {/* Backup card. GlimStone follow-up pass ("half-overlap card notch"):
          split into an outer structural `relative` div (hosting the heading
          Badge, now `position: absolute`) + this same inner
          `relative overflow-hidden` div (unchanged, still the box
          ProgressBar.tsx documents clipping itself to) — see Config.tsx's
          identical backup-card split and Badge.tsx's badgeClassName
          comment. */}
      {/* `glim-notch-card` on this OUTER div, not the inner overflow-hidden
          box: the badge itself lives here (see the split's own comment
          above), so this is the element that has to be the hover/focus zone
          for index.css's card-wide reactive-hover rule — see Settings.tsx's
          Card() for the full reasoning. */}
      {/* GlimStone follow-up pass (jdp, live review, root-mechanism fix
          replacing this file's own earlier `ps-5`-on-the-h2 patch): the
          outer div is deliberately UNPADDED (that's what makes its top edge
          pixel-identical to the inner box's, for the badge's `top-0`
          reference — see the split comment above), which leaves Badge.tsx's
          own CSS static-position fallback (no left/right set) measuring the
          badge's horizontal position against THIS outer div's bare edge, not
          the inner p-5 box's content edge — the same bug this page ONCE
          fixed by adding `ps-5` to the `<h2>` alone (a real fix, but a
          per-call-site padding patch a future edit to this div could
          silently un-fix again). Replaced with `insetStart={5}` on the Badge
          itself: an explicit, self-documenting override at the ONE place
          that actually knows the inner box's own padding number, rather than
          a second h2-level class faking the same value — see Badge.tsx's own
          `insetStart` doc for the full mechanism and its other real call
          sites (Config.tsx's identical Backup Card, Dashboard.tsx's Card()
          and SummaryCell(), all independently hit the identical mismatch). */}
      {/* `.glim-hue` added (rainbow-mode completeness sweep, jdp live review:
          "Es sind nicht alle Buttons in den Regenbogen-Modus eingepflegt"):
          `glim-notch-card` alone never redefines --accent/--focus-ring, only
          the reactive-mode hover reveal — so FlashBackupButton's own
          bg-accent button below stayed flat regardless of rainbow. Same
          hueIndex={0} the Badge already uses; the inner box inherits it via
          ordinary CSS custom-property cascade. */}
      <div className="relative glim-notch-card glim-hue" style={hueVars(rainbowAt(0)) as CSSProperties}>
        {/* Task 5 (rule 11): heading is now a filled Badge, not bare eyebrow text.
            jdp live-review ("Infotexte in eine i Infobubble"): the permanent
            `<p>` explaining what this backup captures used to sit under the
            heading, read once and then costing vertical space forever — the
            exact "Fließtext unter jedem Bedienelement" rule 8 exists to fold
            away. Moved onto the heading Badge itself as an InfoBubble
            (`onAccent`: this badge is a full solid accent fill, same reasoning
            as Settings.tsx's Card() own hint bubble), same content
            (`flash.backupHint`), zero new i18n keys. */}
        <h2 className="flex items-center">
          <Badge tone="heading" size="heading" wrap hueIndex={0} insetStart={5}>
            {t("flash.backupTitle")}
            <InfoBubble tip={tLtr(t, "flash.backupHint")} onAccent />
          </Badge>
        </h2>
        <div className="relative overflow-hidden bg-carbon-surface rounded-card p-5 flex flex-col gap-4">
          {/* jdp live-review: "der Button ganz rechts in der Card platziert"
              — the badge is the row's only content, right-aligned via
              justify-end (this app's established "push to the row's far
              edge" idiom is `ms-auto` on the badge itself when it shares a
              row with a leading sibling — see Containers.tsx's own
              BackupButton/ExportButton row — but there is no leading sibling
              here, so justify-end on the row achieves the identical flush-
              right result with nothing to push away from). */}
          <div className="flex justify-end">
            <FlashBackupButton
              t={t}
              onBackedUp={() => void load()}
              externallyBusy={running.active}
              busyPhase={running.phase}
            />
          </div>

          {/* Live backup/restore progress, pinned to the card's bottom edge */}
          {progress && (
            <ProgressBar percent={progress.percent} active={progress.active} />
          )}
        </div>
      </div>

      {/* Restore card. `glim-notch-card`: see Settings.tsx's Card() for the
          reasoning. `.glim-hue`: same hueIndex={1} the Badge already uses,
          which FlashSnapshotRow's Download/Delete badges inherit via the
          ordinary custom-property cascade — that is why neither passes a
          hueIndex of its own.
            This comment used to end "…no per-row change needed", written
          during the rainbow-completeness sweep and describing the two row
          controls as though they were already hue-integrated badges. They
          were not: Download was a hard `bg-accent` text button and Delete a
          text button whose only colour was a bespoke `hover:bg-statusFailBg`
          red, so the cascade reached exactly one of the two and the claim
          read as done work. Corrected here in the same pass that actually
          converted them — a comment describing an intended end state as
          fact is how this row survived three earlier sweeps unnoticed. */}
      <div
        className="relative glim-notch-card glim-hue bg-carbon-surface rounded-card p-5 flex flex-col gap-4"
        style={hueVars(rainbowAt(1)) as CSSProperties}
      >
        {/* Safe-restore explainer — jdp live-review ("Infotexte in eine i
            Infobubble"): this used to be a permanent bg-statusNeutralBg
            banner (Task 7 had already folded its COLOUR from the old fifth
            "info" hue into neutral, but kept the banner FORM — pure
            informational prose, not a live status/state readout, so it's
            exactly rule 8's "read once, costs vertical space forever" case).
            Same content (`flash.restoreNote`), now an InfoBubble on the
            heading Badge instead, matching FlashZipExportCard's own
            already-bubbled hint below on this same page. */}
        <h2 className="flex items-center">
          <Badge tone="heading" size="heading" wrap hueIndex={1}>
            {t("snapshots.title")}
            <InfoBubble tip={tLtr(t, "flash.restoreNote")} onAccent />
          </Badge>
        </h2>

        {/* jdp gave this exact text for an InfoBubble here ("Restore und
            Löschen wirken nur auf die gewählte Quelle — ein lokales Backup
            zu löschen rührt die Offsite-Kopie nie an und umgekehrt."). It
            turns out to be byte-identical to the EXISTING `source.hint`
            i18n key already used at this same call site — as a permanent
            <p> caption below the row, precisely the rule-8 pattern an
            InfoBubble exists to replace — and at four other call sites
            app-wide with the identical label+SourceToggle-row-then-<p>
            shape. Those were components/RestorePanel.tsx (the Containers
            tab's per-container panel — note it lives in components/, not
            Containers.tsx, which is why grepping pages/ alone missed it),
            pages/Config.tsx, pages/VMs.tsx and pages/Files.tsx. This comment
            used to end "the other four sites are UNCHANGED... a follow-up for
            a future round"; that follow-up has since been done, and all four
            now render the identical InfoBubble-on-the-label form this call
            site pioneered. Zero new i18n keys — `source.hint` is already
            translated in all 42 locales. */}
        <div className="flex items-center gap-2">
          <span className="flex items-center gap-1 text-xs text-carbon-textMuted">
            {t("source.label")}
            <InfoBubble tip={t("source.hint")} />
          </span>
          <SourceToggle source={source} onChange={setSource} disabled={loading} domain="flash" />
        </div>

        {loading && <p className="text-xs text-carbon-textMuted">{t("dashboard.checking")}</p>}
        {error && <p className="text-xs text-statusFail">{error}</p>}
        {!loading && !error && snapshots.length === 0 && (
          <p className="text-xs text-carbon-textMuted">{t("flash.none")}</p>
        )}
        {!loading && snapshots.length > 0 && (
          <div className="rounded-card bg-carbon-background px-3 py-1">
            {snapshots.map((snap) => (
              <FlashSnapshotRow key={snap.id} snap={snap} source={source} onDeleted={() => void load()} t={t} />
            ))}
          </div>
        )}
      </div>

      {/* Flash-ZIP-Export card — MOVED here from Settings' Storage tab (jdp,
          two live-review messages in sequence, the second superseding the
          first: "trenn bitte flash zip export und den rest wieder in zwei
          separate cards", then "soll die flash zip export toggle nicht
          einfach in den flash tab? macht doch mehr sinn"). This setting is
          entirely about THIS domain's own backup behaviour (an extra plain
          .zip written after every flash backup), so it now lives on this
          page instead of a generic Settings card — see FlashZipExportCard's
          own header comment in Settings.tsx for the full move rationale and
          why it's self-contained (fetches/persists its own settings slice
          rather than reaching into SettingsPage's state, which doesn't exist
          here). `hueIndex={2}`: this page hand-numbers its own Card notches
          (0 = Backup, 1 = Restore, both above) rather than using
          Settings.tsx's `nextHue()` counter, which is scoped to that file's
          own component — the next literal in the same sequence. */}
      <FlashZipExportCard t={t} hueIndex={2} />
    </div>
  );
}
