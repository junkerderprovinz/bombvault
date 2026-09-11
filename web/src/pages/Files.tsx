// ---------------------------------------------------------------------------
// Files page (#62) — first-class file-set backups ("point BombVault at any
// folder"). Modeled on VMs.tsx, the closest per-item domain page: one card per
// file set with an include-in-schedule switch, a fire-and-watch backup button
// (progress key "files:<name>"), and an expandable Backups panel whose restore
// control offers "original location" (confirm-gated, in place) vs "to a folder"
// (non-destructive extract via FolderBrowser). Add/edit runs in a dialog with a
// FolderBrowser path picker and an excludes textarea (one pattern per line).
// ---------------------------------------------------------------------------

import { useEffect, useId, useRef, useState, type CSSProperties, type ReactNode } from "react";
import { createPortal } from "react-dom";
import {
  listFileSets,
  createFileSet,
  patchFileSet,
  deleteFileSet,
  deleteFileSetBackups,
  backupFileSet,
  backupFilesAll,
  fileSetSnapshots,
  restoreFileSet,
  listSnapshotFilesFileSet,
  restoreFileSetFiles,
  discoverFiles,
  deleteSnapshot,
  getSettings,
  getFileSetPreset,
} from "../lib/api";
import type { BrowseResponse, FileSetView, Snapshot, FileEntry, FileSetPresetResponse } from "../lib/api";
import { applyToggle, browseRelToHost, splitFlatSet, toFlatList } from "../lib/selectionTree";
import { SelectionTree } from "../components/SelectionTree";
import { SourceToggle, type RepoSource } from "../components/SourceToggle";
import { PAGE_SHELL } from "../lib/pageShell";
import { OffsiteIndicator } from "../components/OffsiteIndicator";
import { EffectiveScheduleLine } from "../components/EffectiveScheduleLine";
import { FolderBrowser } from "../components/FolderBrowser";
import { DEFAULT_RESTORE_FOLDER } from "../components/RestorePanel";
import { SnapshotFileTree } from "../components/SnapshotFileTree";
import { BackupCancelButton } from "../components/BackupCancelButton";
import { ProgressBar } from "../components/ProgressBar";
import { RecentRunsList } from "../components/RecentRunsList";
import { RestoreProgress } from "../components/restore/RestoreProgress";
import { EmptyStateIcon } from "../components/EmptyStateIcon";
import { IconBackupNow, IconFiles, IconPencil, IconTrash } from "../components/Sidebar";
import { BULK_HUE } from "../lib/bulkHue";
import { useT } from "../lib/i18n";
import { Advanced, useAdvanced } from "../lib/advanced";
import { useProgress, anyActive, busyPhraseKey } from "../lib/progress";
import { useBackupWatch } from "../lib/backupWatch";
import { loadErrorMessage } from "../lib/errors";
import { useConfirm } from "../lib/useConfirm";
import { hueVars, rainbowAt } from "../lib/appearance";
import { Selector, type SelectorItem } from "../components/Selector";
import { useRainbow } from "../lib/useRainbow";
import { Badge } from "../components/Badge";
import { Button } from "../components/Button";
import { InfoBubble } from "../components/InfoBubble";
// ToggleRow, not the bare Toggle: FileSetEnabledToggle renders the shared row
// (label + switch) rather than a naked switch its caller labels by hand — the
// same import components/IncludeToggle.tsx already uses for the Container
// tab's copy of that control.
import { ToggleRow } from "./settings/shared";
import { CheckDraw } from "../components/CheckDraw";
import { useToast } from "../lib/toast";
import { IconRestore } from "../components/Sidebar";

type T = ReturnType<typeof useT>["t"];

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function formatTs(unix: number | null | undefined): string {
  if (!unix) return "—";
  return new Date(unix * 1000).toLocaleString();
}

// ---------------------------------------------------------------------------
// Include-in-schedule toggle (mirrors VMIncludeToggle, PATCHes {enabled})
// ---------------------------------------------------------------------------

function FileSetEnabledToggle({ id, initial }: { id: string; initial: boolean }) {
  const { t } = useT();
  const { push } = useToast();
  const [enabled, setEnabled] = useState(initial);
  const [busy, setBusy] = useState(false);
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): a
  // failed toggle toasts AND shakes, same mechanism as ToggleRow's shakeNonce.
  const [shake, setShake] = useState(0);

  // Re-seed when the parent passes a fresh value (rows are keyed by id and do
  // not remount, so a list reload must reach the toggle).
  useEffect(() => setEnabled(initial), [initial]);

  async function handleChange(next: boolean) {
    setBusy(true);
    try {
      const res = await patchFileSet(id, { enabled: next });
      if (res.ok) {
        setEnabled(next);
      } else {
        push(res.error ?? t("schedule.updateFailed"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("schedule.updateFailed"), "fail");
      setShake((n) => n + 1);
    } finally {
      setBusy(false);
    }
  }

  // Renders through the SAME shared ToggleRow every other row-shaped toggle in
  // the app already uses — the identical conversion components/IncludeToggle.tsx
  // (the Container tab's copy of this exact control) already went through after
  // jdp's live review: "Die Toggles ... bitte gleich anordnen und der Text
  // gleich formatieren. Der Text soll immer ganz links stehen und der Toggle
  // ganz rechts sein."
  //
  // That round only touched the Container tab, so THIS copy and VMs.tsx's
  // VMIncludeToggle were left as the mirror image of the shape they were meant
  // to match: a bare `hideLabel` Toggle whose caller hand-rolled a `<label
  // className="flex items-center gap-2">` around it — switch FIRST, text
  // SECOND, `text-xs text-carbon-textSub` — against ToggleRow's own
  // text-first/switch-last, `text-sm text-carbon-text`. All three tabs render
  // the SAME string (files.enabled and containers.includeInSchedule are
  // byte-identical in every locale: "Include in schedule" / "Im Zeitplan
  // einschließen"), so the drift was visible as the same label laid out two
  // different ways on two adjacent tabs.
  //
  // Fixed at the shared mechanism rather than by hand-matching classes: this
  // now IS ToggleRow, so it cannot drift from the Container tab's copy again.
  // The visible label keeps this file's own `files.enabled` key (the string
  // the call site was already displaying) rather than silently switching to
  // the containers.* key the bare Toggle happened to use for its aria-label.
  return (
    <ToggleRow
      label={t("files.enabled")}
      checked={enabled}
      onChange={(next) => void handleChange(next)}
      disabled={busy}
      shakeNonce={shake}
    />
  );
}

// ---------------------------------------------------------------------------
// Backup button (fire-and-watch, mirrors VMBackupButton)
// ---------------------------------------------------------------------------

// GlimStone follow-up pass (v8.0.0) audit note: the state.phase "success"/
// "error" result below is deliberately NOT migrated to a toast, unlike this
// file's other flash sites (FileSetEnabledToggle/FileSetDialog above). Exact
// same reasoning as Containers.tsx's BackupButton / VMs.tsx's VMBackupButton:
// it's driven by the SHARED lib/backupWatch.ts useBackupWatch hook (kind
// defaults to "backup" here, which already self-clears after 4s —
// SUCCESS_CLEAR_MS, effectively already toast-like), but the identical state
// shape also backs RESTORE outcomes elsewhere, which are explicitly STICKY BY
// DESIGN. Splitting that shared, cross-file state machine's rendering by kind
// is a hook-level architecture change, not the local flash-swap this pass
// does everywhere else — left as its own deliberate follow-up.
//
// WHOLE-AREA SWEEP (icon-badge round for this tab, alongside jdp's two named
// buttons above): the TRIGGER is now a square icon badge, while everything the
// audit note above describes stays exactly as it was. This is the same control
// jdp already had converted on the two other domains that own one —
// components/BackupButton.tsx (Containers, "Jetzt sichern und Export sollen
// quadratische Badges mit Glyph sein") and Flash.tsx — and it was the only
// thing left in a FileSetRow card standing as a text pill beside two 32px glyph
// tiles. Badge.tsx's own header is explicit about why that matters: a user sees
// one card, not a set of independently-reasonable controls.
//   Note this conversion does NOT drag the toast migration with it. Containers'
// BackupButton had to move its terminal states to toasts because it lives in a
// card corner with no room beneath it; this one sits in its own
// `flex flex-col` column with the result lines stacked below the trigger — the
// exact shape Containers.tsx's ExportButton keeps for its own sticky result —
// so the deliberate decision recorded above survives untouched. The column
// flips from `items-start` to `items-end` so the 32px tile lines up with the
// card's right edge (its parent is already `items-end`) instead of anchoring a
// wide text block whose left edge it would otherwise inherit.
function FileSetBackupButton({
  set,
  t,
  onBackedUp,
  running,
}: {
  set: FileSetView;
  t: T;
  onBackedUp?: () => void;
  /** "Something is running" signal (anyActive): busy-guards this backup while
   *  another op runs, but never for its OWN in-flight backup (isPending). */
  running?: { active: boolean; phase?: string };
}) {
  const { state, fire, isPending } = useBackupWatch({
    progressKey: `files:${set.name}`,
    start: () => backupFileSet(set.id),
    matchRun: (r) => r.domain === "files" && r.target === set.name,
    onDone: onBackedUp,
  });
  const blockedByOther = !!running?.active && !isPending;
  // A path-less discovered set has nothing to back up until a folder is set
  // (the server would refuse anyway) — restore-to-folder still works below.
  const noPath = set.path === "";

  // One tip, resolved in the same priority order BackupButton.tsx uses, plus
  // this domain's own extra refusal reason (a discovered set with no folder
  // yet). The label the visible text used to carry is the last fallback — an
  // icon-only trigger's tooltip has to say what the button DOES when nothing
  // is blocking it.
  // #178: stable name, exceptional states as tooltip only.
  const stateTip = noPath
    ? t("files.noPathHint")
    : isPending
      ? t("common.backingUp")
      : blockedByOther
        ? t(busyPhraseKey(running?.phase))
        : undefined;

  return (
    <div className="flex flex-col gap-1 items-end">
      <Button
        label={t("containers.backupNow")}
        labelKey="containers.backupNow"
        glyph={<IconBackupNow />}
        tone="accent"
        onClick={() => void fire()}
        disabled={isPending || blockedByOther || noPath}
        busy={isPending}
        title={stateTip}
      />
      {/* The "something else is running" note does NOT live here any more
          (jdp, 2026-09-11: "eine sicherung läuft... text bitte über den text
          von letztes backup"). It hung directly under the badge corner, which
          put a status line at the top of the card and the fact it qualifies -
          the last backup - at the bottom. FileSetRow renders it above that
          line now.

          Worth noting what the sibling does, because it is the reason this was
          drift rather than a choice: components/BackupButton.tsx, the container
          card's own, renders NO visible note at all and puts the same phrase in
          `title`, per #178's "the button's name is stable, only exceptional
          states get a tooltip". This button keeps its tooltip too (`stateTip`
          above), so the phrase is in both places for the same reason it is on
          the container card - the text below is the card talking, the tooltip
          is the button talking. */}
      {state.phase === "success" && (
        <span className="inline-flex items-center gap-1 text-xs text-statusOk">
          <CheckDraw />
          {t("common.done")}
          {state.snapshotId && (
            <span dir="ltr" className="font-mono ms-1 text-start text-carbon-textMuted">
              {state.snapshotId.slice(0, 8)}
            </span>
          )}
        </span>
      )}
      {state.phase === "error" && (
        <span className="text-xs text-statusFail max-w-[18rem] wrap-break-word text-end">
          {state.message}
        </span>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Selective restore — tick individual files/folders from a snapshot and restore
// just those into a chosen folder (#65). Mirrors the container SnapshotFileBrowser,
// reusing the shared SnapshotFileTree; scoped to the file-set routes and always
// non-destructive (into a folder), so no in-place confirm is needed here.
// ---------------------------------------------------------------------------

function FileSetFileBrowser({
  set,
  snapshotId,
  source,
  hostMountRoot,
  restoreFolder,
  otherActive,
  t,
}: {
  set: FileSetView;
  snapshotId: string;
  source: RepoSource;
  hostMountRoot: string;
  restoreFolder: string;
  otherActive: { active: boolean; phase?: string };
  t: T;
}) {
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState("");
  const [selected, setSelected] = useState<Set<string>>(new Set());
  // Same #69 fix as FileSetRestoreControl: seed from the global default instead
  // of an empty string that only ever showed the FolderBrowser's placeholder.
  const [folder, setFolder] = useState(restoreFolder);
  const [restoredTarget, setRestoredTarget] = useState("");

  const progressKey = `files:${set.name}`;
  // The SAME ref instance flows to useBackupWatch AND (via RestoreProgress) to the
  // cancel button — see FileSetRestoreControl / RestoreAction. Never split it.
  const cancelledRef = useRef(false);
  const { state, fire, reset, isPending } = useBackupWatch({
    progressKey,
    kind: "restore",
    matchRun: (r) => r.domain === "files" && r.target === set.name,
    cancelledRef,
    start: async () => {
      const res = await restoreFileSetFiles(set.id, snapshotId, [...selected], folder.trim(), true, source);
      if (res.ok) setRestoredTarget(res.target ?? "");
      return res;
    },
  });
  const prog = useProgress()[progressKey];
  const blockedByOther = otherActive.active && !isPending;

  useEffect(() => {
    setLoading(true);
    listSnapshotFilesFileSet(set.id, snapshotId, source)
      .then((res) => {
        // #129 — show the server's own reason (e.g. a stale repo lock) when it
        // sent one; the generic message is only for a plain network failure.
        if (res.ok) setFiles(res.files ?? []);
        else setError(loadErrorMessage(res, t("files.loadFailed")));
      })
      .catch(() => setError(t("files.loadFailed")))
      .finally(() => setLoading(false));
  }, [set.id, snapshotId, source, t]);

  // A new selection / target clears any prior result banner so it can't linger
  // over a fresh, unrun choice.
  function toggle(p: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(p)) next.delete(p);
      else next.add(p);
      return next;
    });
    reset();
  }
  function pickFolder(v: string) {
    setFolder(v);
    reset();
  }

  function handleRestoreSelected() {
    if (selected.size === 0 || !folder.trim()) return;
    void fire();
  }

  const count = selected.size;

  return (
    <div className="mt-1 rounded-card bg-carbon-background p-2 flex flex-col gap-2">
      <p className="text-caption text-carbon-textMuted">{t("files.selectHint")}</p>
      <SnapshotFileTree
        files={files}
        loading={loading}
        error={error}
        filter={filter}
        onFilterChange={setFilter}
        selected={selected}
        onToggle={toggle}
        t={t}
      />

      {/* Target folder + restore-selected action — shown once something is ticked. */}
      {count > 0 && (
        <div className="border-t border-carbon-border pt-2 flex flex-col gap-2">
          <FolderBrowser
            label={t("restore.targetPath")}
            value={folder}
            hostMountRoot={hostMountRoot}
            onChange={pickFolder}
          />
          <div className="flex items-center gap-2">
            <Button
              label={t("files.restoreSelected").replace("{n}", String(count))}
              labelKey="files.restoreSelected"
              glyph={<IconRestore />}
              tone="accent"
              onClick={handleRestoreSelected}
              disabled={isPending || blockedByOther || !folder.trim()}
              busy={isPending}
              title={isPending ? t("common.restoring") : undefined}
              className="shrink-0"
            />
            {blockedByOther && (
              <span className="text-caption text-carbon-textMuted">{t(busyPhraseKey(otherActive.phase))}</span>
            )}
          </div>
          <RestoreProgress
            state={state}
            isPending={isPending}
            prog={prog}
            cancelKey={progressKey}
            inPlace={false}
            name={set.name}
            cancelledRef={cancelledRef}
            successMessage={
              restoredTarget
                ? t("restore.restoredTo").replace("{path}", restoredTarget)
                : t("files.restoreComplete")
            }
            t={t}
          />
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Restore control — "original location" (confirm, in place) vs "to a folder" vs
// "select files" (selective, #65)
// ---------------------------------------------------------------------------

type RestoreDest = "original" | "folder" | "select";

function FileSetRestoreControl({
  set,
  snapshotId,
  source,
  hostMountRoot,
  restoreFolder,
  otherActive,
  t,
  trailing,
}: {
  set: FileSetView;
  snapshotId: string;
  source: RepoSource;
  hostMountRoot: string;
  restoreFolder: string;
  otherActive: { active: boolean; phase?: string };
  t: T;
  /** Rendered at the end of the destination row, after the Restore button.
   *
   *  The snapshot's delete badge, in practice: everything a snapshot DOES sits
   *  in one row (jdp, 2026-09-11: "der löschen button in die gleiche zeile der
   *  anderen buttons"). A prop rather than the caller wrapping this component,
   *  because the row it joins is this component's own flex line - a wrapper
   *  outside it would land on the next one. */
  trailing?: ReactNode;
}) {
  // A path-less discovered set can only restore into a chosen folder — the
  // server refuses an in-place restore when it doesn't know the original path.
  const noPath = set.path === "";
  const [dest, setDest] = useState<RestoreDest>(noPath ? "folder" : "original");
  // "Select files" (the #65 selective restore) is an advanced option; basic
  // mode keeps the whole-set original / to-folder pair. Read directly (rather
  // than staying inside an <Advanced> JSX wrapper) because the destination
  // Selector below needs to decide, in JS, whether "select" belongs in its
  // items array at all — Selector renders a flat items list, not children a
  // wrapper component could conditionally swallow.
  const { advanced } = useAdvanced();
  // Seeded from the operator's global "Default restore folder" setting, exactly
  // like the container restore panel — was hardcoded to "" (#69), which left the
  // FolderBrowser showing only its generic placeholder example text instead of
  // a real usable default.
  const [targetPath, setTargetPath] = useState(restoreFolder);

  const progressKey = `files:${set.name}`;
  // The SAME ref instance flows to useBackupWatch AND (via RestoreProgress) to
  // RestoreCancelButton — see RestoreAction's header note. Never split it.
  const cancelledRef = useRef(false);
  const { state, fire, reset, isPending } = useBackupWatch({
    progressKey,
    kind: "restore",
    matchRun: (r) => r.domain === "files" && r.target === set.name,
    cancelledRef,
    start: () =>
      restoreFileSet(set.id, snapshotId, true, dest === "folder" ? targetPath : "", source),
  });
  const prog = useProgress()[progressKey];
  const blockedByOther = otherActive.active && !isPending;
  const { confirm, confirmDialog } = useConfirm();

  // A stale success/error banner would misdescribe a different destination —
  // clear it when the choice changes (no-op while a restore is in flight).
  useEffect(() => reset(), [dest, targetPath, reset]);

  async function handleRestore() {
    if (dest === "original" && !(await confirm(t("files.restoreOriginalConfirm")))) return;
    if (dest === "folder" && targetPath.trim() === "") return;
    void fire();
  }

  // Destination choice, on the shared Selector component (GlimStone
  // form-engine Phase 2, Task 3). "select" only enters the items array in
  // advanced mode — see the `advanced` comment above.
  const destItems: SelectorItem[] = [
    { id: "original", label: t("files.restoreOriginal"), disabled: noPath, title: noPath ? t("files.noPathHint") : undefined },
    { id: "folder", label: t("files.restoreToFolder") },
    ...(advanced ? [{ id: "select", label: t("files.restoreSelectFiles") }] : []),
  ];

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-2 flex-wrap">
        {/* label reuses the existing "Restore" string rather than a new
            aria-only i18n key: this strip's own three item labels already
            describe the actual choice ("Restore to original location" /
            "Restore to a folder" / "Select files"), and this repo's i18n
            convention (see lib/i18n.ts's 26-locale parity test) requires a
            brand-new key to land in every locale in the same pass — not
            worth doing for a screen-reader-only group name that "Restore"
            already names clearly enough in context. */}
        {/* buttonHeight, because the Restore button sits in this same row
            (jdp, 2026-09-11, on this exact strip: "soll das nicht besser ein
            horizontaler selektor sein oder zumindest alle buttons gleiche
            höhe?"). It already was a horizontal selector, so the answer is the
            second half. Measured on the running build before changing
            anything: these three segments came out 24px and the button beside
            them 32px, which is why the square glyph-mode button read as too
            tall - it was the only control in the row at the height the house
            gives a button. */}
        <Selector
          items={destItems}
          label={t("snapshots.restore")}
          select="one"
          active={dest}
          buttonHeight
          onChange={(id) => setDest(id as RestoreDest)}
          disabled={isPending}
        />
        {/* The whole-set restore button + its own picker/progress; the selective
            mode renders its own controls below (FileSetFileBrowser). */}
        {dest !== "select" && (
          <Button
            label={t("snapshots.restore")}
            labelKey="snapshots.restore"
            tone="accent"
            onClick={() => void handleRestore()}
            disabled={isPending || blockedByOther || (dest === "folder" && targetPath.trim() === "")}
            busy={isPending}
            title={isPending ? t("common.restoring") : undefined}
            className="shrink-0"
          />
        )}
        {blockedByOther && dest !== "select" && (
          <span className="text-caption text-carbon-textMuted shrink-0">
            {t(busyPhraseKey(otherActive.phase))}
          </span>
        )}
        {/* `ms-auto` on the wrapper, so whatever the caller puts here is pushed
            to the row's far end (jdp, 2026-09-11: "Der löschen button in den
            Backupzeilen soll ganz rechts sein"). Sitting straight after the
            Restore button, the delete badge read as a third step in the same
            sequence; at the far edge it reads as what it is - the one control
            on the row that is not part of restoring. Applied here rather than
            by the caller because `ms-auto` only means anything inside THIS
            flex row. */}
        {trailing && <span className="ms-auto flex items-center">{trailing}</span>}
      </div>
      {/* Target folder picker for the non-destructive whole-set extract */}
      {dest === "folder" && (
        <FolderBrowser
          label={t("restore.targetPath")}
          value={targetPath}
          hostMountRoot={hostMountRoot}
          onChange={setTargetPath}
        />
      )}
      {dest !== "select" && (
        <RestoreProgress
          state={state}
          isPending={isPending}
          prog={prog}
          cancelKey={progressKey}
          inPlace={dest === "original"}
          name={set.name}
          cancelledRef={cancelledRef}
          successMessage={t("files.restoreComplete")}
          t={t}
        />
      )}
      {/* Selective restore: tick files/folders and restore just those to a folder. */}
      {dest === "select" && (
        <FileSetFileBrowser
          set={set}
          snapshotId={snapshotId}
          source={source}
          hostMountRoot={hostMountRoot}
          restoreFolder={restoreFolder}
          otherActive={otherActive}
          t={t}
        />
      )}
      {confirmDialog}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Snapshot row + Backups panel (mirror VMSnapshotRow / VMRestorePanel)
// ---------------------------------------------------------------------------

function FileSetSnapshotRow({
  snap,
  set,
  source,
  hostMountRoot,
  restoreFolder,
  onDeleted,
  t,
}: {
  snap: Snapshot;
  set: FileSetView;
  source: RepoSource;
  hostMountRoot: string;
  restoreFolder: string;
  onDeleted: () => void;
  t: T;
}) {
  const progressMap = useProgress();
  const running = anyActive(progressMap);
  // Delete is guarded only against THIS set's own in-flight backup/restore, not
  // any global activity (mirrors the VM panel's rationale).
  const busy = progressMap[`files:${set.name}`]?.active ?? false;
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
      const res = await deleteSnapshot("files", snap.id, source);
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

  return (
    // py-1.5, not the py-2.5 this row used to carry — the identical trade
    // components/RestorePanel.tsx's own snapshot row already made, and for the
    // identical reason: its delete control grew from a ~24px text button to the
    // app's one 32px square icon badge, and trimming 4px of padding per side
    // keeps the collapsed row at the 44px it measured before. A bigger badge in
    // a list of unchanged density, rather than a list that grew. Config.tsx's
    // ConfigSnapshotRow carries the same pairing.
    <div className="flex flex-col gap-1 py-1.5 border-b border-carbon-border last:border-0">
      {/* Identity FIRST, then when it was taken, on the line under it (jdp,
          2026-09-11: "datum und uhrzeit unter die backup kennung"). They were
          side by side in one row with the delete button, which made a snapshot
          read as three unrelated columns; stacked, the date is plainly a
          property OF the id above it. The id keeps its own mono face and the
          date the muted caption tone, so the two stay told apart without a
          fixed column width holding them.

          The `w-20` on the id is gone with the row it was measured for, and so
          is the `ps-24` indent that used to align the action row under it. */}
      <div className="flex flex-col">
        <span dir="ltr" className="font-mono text-start text-carbon-text text-xs">
          {snap.id.slice(0, 8)}
        </span>
        <span className="text-carbon-textMuted text-xs">
          {new Date(snap.time).toLocaleString()}
          {snap.tags && snap.tags.length > 0 && (
            <span className="hidden sm:inline">{` · ${snap.tags.join(", ")}`}</span>
          )}
        </span>
      </div>
        {/* Whole-area sweep finding, not part of jdp's two named buttons: this
            per-snapshot delete was the LAST surviving copy of the exact defect
            already fixed in components/RestorePanel.tsx and pages/Config.tsx —
            a plain text button whose only colour was a bespoke
            `hover:bg-statusFailBg hover:text-statusFail` red flash, sitting
            inside a card every other control of which is hue-integrated. jdp's
            wording when he reported it there ("Der Löschen-Badge ist auch
            anders eingefärbt, soll nicht so sein, ganz normal in die Farbmodi
            integrieren") applies here verbatim; leaving this one behind would
            have meant the Files tab's own remove badge above is hue-integrated
            while the delete one panel down is not.
              Same recipe as its two already-corrected siblings and as the
            remove badge above: square, `size="icon"` (32px), `tone="active"`,
            no `hueIndex` — this panel renders inside FileSetRow's own
            `.glim-hue` card, so the cascade paints it in this set's rainbow
            position. No red, no grey: the meaning is carried by IconTrash, by
            the tip, and by the confirm dialog handleDelete already opens
            (t("snapshots.deleteConfirm")), which is untouched. The "…"
            in-flight label has nowhere to live on an icon-only badge, so
            `deleting` shows as `disabled`, exactly like RestorePanel's. */}
      {/* The delete badge moved DOWN into the action row (jdp, 2026-09-11:
          "der löschen button in die gleiche zeile der anderen buttons"). It
          used to sit alone at the far end of the identity line, which put the
          one destructive control on this row as far as possible from the two
          controls it belongs with, and left it as the only reason that line
          was a flex row at all. Everything a snapshot DOES is now in one row;
          the line above only says which snapshot. */}
      <div>
        <FileSetRestoreControl
          set={set}
          snapshotId={snap.id}
          source={source}
          hostMountRoot={hostMountRoot}
          restoreFolder={restoreFolder}
          otherActive={running}
          t={t}
          trailing={
            <Button
              key={shake}
              label={t("snapshots.delete")}
              labelKey="snapshots.delete"
              glyph={<IconTrash />}
              tone="accent"
              onClick={() => void handleDelete()}
              disabled={deleting || busy}
              className={`shrink-0${shake ? " glim-shake" : ""}`}
            />
          }
        />
      </div>
      {confirmDialog}
    </div>
  );
}

function FileSetRestorePanel({
  set,
  hostMountRoot,
  restoreFolder,
  t,
  onSetsChanged,
  trailing,
}: {
  set: FileSetView;
  hostMountRoot: string;
  restoreFolder: string;
  t: T;
  /** Delete-all forgets the whole set — the parent must reload the list. */
  onSetsChanged: () => void;
  /** Always-visible summary shown at the far end of the trigger's own row,
   *  never inside the panel it opens.
   *
   *  This is the container card's shape, adopted here (jdp, 2026-09-11: "kannst
   *  du die buttons und toggle in den ordner cards genauso anordnen wie in den
   *  container cards?"). There, "Letztes Backup: …" shares the disclosure
   *  row rather than occupying the card's top-right corner, and the corner
   *  carries the action badges instead. A prop rather than a second row,
   *  because the trigger row already exists and this text is one line of
   *  summary, not a section of its own. */
  trailing?: ReactNode;
}) {
  const [open, setOpen] = useState(false);
  const [source, setSource] = useState<RepoSource>("local");
  const [snapshots, setSnapshots] = useState<Snapshot[]>([]);
  const [loading, setLoading] = useState(false);
  // Section-load error (list failed to load) — NOT migrated to a toast
  // (GlimStone follow-up pass, v8.0.0 audit note): it replaces the whole
  // snapshot-list content area, the same "the section failed to load"
  // structural condition as Files()'s own page-level `error`, not a one-shot
  // button-click confirmation. handleDeleteAll's own one-shot failure below
  // is the bug fix — it used to share this exact state slot (see comment
  // there).
  const [error, setError] = useState<string | null>(null);

  const [reloadTick, setReloadTick] = useState(0);
  const [deletingAll, setDeletingAll] = useState(false);
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): shake
  // the "Delete all" control on a failed delete, alongside the toast below.
  const [shakeDeleteAll, setShakeDeleteAll] = useState(0);

  useEffect(() => {
    if (!open) return;
    setLoading(true);
    setError(null);
    fileSetSnapshots(set.id, source)
      .then((res) => {
        if (res.ok) setSnapshots(res.snapshots ?? []);
        else setError(res.error ?? t("common.loadBackupsFailed"));
      })
      .catch(() => setError(t("common.loadBackupsFailed")))
      .finally(() => setLoading(false));
  }, [open, set.id, source, reloadTick]); // eslint-disable-line react-hooks/exhaustive-deps -- t() is only read to build a failure message; re-fetching on a language switch would be a wasted round-trip

  // BUG FIX (GlimStone follow-up pass, v8.0.0): "Delete all" is a one-shot
  // action failure — it used to be routed through the section-load `error`
  // above via setError(), but the failure branch below also bumps
  // reloadTick to refresh the (still-existing) snapshot list, which re-fires
  // the load effect above and clears `error` again almost immediately
  // (setError(null) at the top of that effect) — so a delete-all failure was
  // already near-invisible before this fix, the EXACT same dead-error-
  // display bug 43c6b49 found and fixed in VMs.tsx's
  // VMRestorePanel.handleDeleteAll. A toast survives that reload.
  async function handleDeleteAll() {
    // TODO(#follow-up): richer stake-detail copy ("N snapshots, X GB") belongs
    // here once it ships (deferred — new interpolated i18n keys across all 25
    // non-English locales, out of scope for this window.confirm() → dialog
    // mechanism swap, form-engine Task 7). This is the highest-value site for
    // it: an irreversible bulk delete of every backup this set has. Same
    // flagged follow-up as Containers.tsx's deleteBackupsConfirm and
    // VMs.tsx's deleteAllConfirm.
    if (!(await confirm(t("files.deleteBackupsConfirm")))) return;
    setDeletingAll(true);
    deleteFileSetBackups(set.id)
      .then((res) => {
        if (!res.ok) {
          push(res.error ?? t("common.deleteBackupsFailed"), "fail");
          setShakeDeleteAll((n) => n + 1);
          setReloadTick((n) => n + 1);
          return;
        }
        // The set itself was forgotten along with its snapshots — reload the
        // whole list so the card disappears instead of going stale.
        onSetsChanged();
      })
      .catch(() => {
        push(t("common.deleteBackupsFailed"), "fail");
        setShakeDeleteAll((n) => n + 1);
        setReloadTick((n) => n + 1);
      })
      .finally(() => setDeletingAll(false));
  }

  return (
    <div className="mt-1">
      {/* Trigger left, summary flush right — the container card's own
          disclosure row, class for class (`flex items-center gap-2 flex-wrap`
          plus `ms-auto shrink-0` on the text), so the two cards line up
          instead of each arranging the same two things its own way. */}
      <div className="flex items-center gap-2 flex-wrap">
        <Button
          label={t("snapshots.title")}
          labelKey="snapshots.title"
          tone="neutral"
          onClick={() => setOpen((prev) => !prev)}
        />
        {trailing}
      </div>

      {open && (
        <div className="mt-2 rounded-card bg-carbon-background px-3 py-1">
          {/* `source.hint` moved from a permanent `text-caption` <p> under
              this row onto the "Quelle" label as an InfoBubble — rule 8's
              "read once, costs vertical space forever" case, the same
              conversion Flash.tsx got in 63f53d5 and the other three copies
              (components/RestorePanel.tsx, pages/Config.tsx, pages/VMs.tsx)
              get in this same pass. This was the fourth and last of them.
                Moving it also fixes the same latent mismatch VMs.tsx's
              identical row had: the label + SourceToggle are wrapped in
              <Advanced>, but the <p> explaining what choosing a source DOES
              sat outside it, so basic mode rendered a hint about a control it
              wasn't showing. As part of the label it now appears exactly when
              the toggle does.
                The old outer `flex flex-col gap-1` wrapper is gone with the
              <p> (one child left); its `py-2 border-b` moves onto this row,
              so the row's own box is unchanged. */}
          <div className="flex items-center gap-2 py-2 border-b border-carbon-border">
              {/* Source (Local / Off-site) toggle is advanced; basic mode uses local. */}
              <Advanced>
                <span className="flex items-center gap-1 text-xs text-carbon-textMuted">
                  {t("source.label")}
                  <InfoBubble tip={t("source.hint")} />
                </span>
                <SourceToggle source={source} onChange={setSource} disabled={loading} domain="files" />
              </Advanced>
              {/* Delete-all acts on the LOCAL repo (and forgets the set), so it
                  is only offered while the local source is shown. */}
              {source === "local" && snapshots.length > 0 && (
                // NO bespoke red. The comment that used to sit here claimed
                // this badge was "already correctly fault-red per 'the
                // destructive control is always the fault colour'" — that
                // rule was REVERSED, and this call site never heard about it.
                // The standing rule is the opposite (jdp: "Der Löschen-Badge
                // ist auch anders eingefärbt, soll nicht so sein"; "Keine
                // Sonderfarbe für den Entfernen-Badge"), and commit d336e532
                // swept eight controls onto it — but it found them by
                // grepping for `statusFail` CLASSES, so this badge, carrying
                // the identical red through Badge's own `tone` prop, was
                // invisible to that sweep and kept it.
                //   `tone="neutral"` now: the same secondary chip its
                // siblings use, and the same neutral chrome Containers.tsx's
                // own "Alle Backups löschen" took in that sweep. Nothing
                // becomes ambiguous — the label still says "Alle löschen"
                // verbatim and handleDeleteAll still routes through the
                // shared confirm dialog. `glim-shake` survives: behaviour,
                // not colour. bombvault/no-status-color-on-control now fails
                // the build if this comes back.
                // A BUTTON, at the ordinary button size, with the glyph its own
                // key already earns (jdp, 2026-09-11: "der alle backups löschen
                // button in der ordner card soll auch normale button größe inkl
                // glyph sein"). It was a `size="small"` Badge, which is the one
                // shape this action may not have: Containers.tsx's identical
                // "Alle Backups löschen" is a full Button and says why in its
                // own comment - a labelled action that also carries an
                // in-flight label, not a row-action glyph pair. Two cards
                // offering the same destructive action in two different sizes
                // is exactly the drift that comment was written to stop.
                //
                // The glyph is not passed: `labelKey` is `snapshots.deleteAll`
                // and glyphFor's `/\.(delete|remove)/` rule resolves the trash
                // for it, the same way the container card gets its own. Passing
                // one here would be a second opinion about a symbol the table
                // already owns.
                //
                // The label is STABLE and the in-flight wording moves to
                // `title`, which is Button's own documented contract: a label
                // that changes to "Wird gelöscht…" mid-action resizes the
                // control at the one moment somebody is watching it. `busy`
                // carries the spinner instead. Same three props as the
                // container card, in the same order.
                <Button
                  key={shakeDeleteAll}
                  label={t("snapshots.deleteAll")}
                  labelKey="snapshots.deleteAll"
                  tone="neutral"
                  onClick={() => void handleDeleteAll()}
                  disabled={deletingAll || loading}
                  busy={deletingAll}
                  title={deletingAll ? t("snapshots.deletingAll") : undefined}
                  className={`ms-auto${shakeDeleteAll ? " glim-shake" : ""}`}
                />
              )}
          </div>
          <RecentRunsList name={set.name} domain="files" t={t} />
          {loading && (
            <p className="py-3 text-xs text-carbon-textMuted">{t("common.loadingBackups")}</p>
          )}
          {error && <p className="py-3 text-xs text-statusFail">{error}</p>}
          {!loading && !error && snapshots.length === 0 && (
            <p className="py-3 text-xs text-carbon-textMuted">{t("snapshots.none")}</p>
          )}
          {!loading &&
            snapshots.map((snap) => (
              <FileSetSnapshotRow
                key={snap.id}
                snap={snap}
                set={set}
                source={source}
                hostMountRoot={hostMountRoot}
                restoreFolder={restoreFolder}
                onDeleted={() => setReloadTick((n) => n + 1)}
                t={t}
              />
            ))}
        </div>
      )}
      {confirmDialog}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Add / edit dialog
// ---------------------------------------------------------------------------

// Exported for this page's dom harness (the FoldersEditor precedent — the
// harness renders the dialog against the mocked api client, not a whole page).
export function FileSetDialog({
  initial,
  presetSeed,
  hostMountRoot,
  t,
  onClose,
  onSaved,
}: {
  /** null = create a new set; a view = edit that set. */
  initial: FileSetView | null;
  /** Pre-fill values for a NEW set opened via "Add preset: Host system
   *  config" (#134 — the files domain's flash-domain analogue on
   *  generic/TrueNAS). Ignored when `initial` is set (editing an existing
   *  set never seeds from a preset). Still just a starting point — every
   *  field stays fully editable before Save, same as a blank create. */
  presetSeed: { name: string; path: string; excludes: string[] } | null;
  hostMountRoot: string;
  t: T;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { push } = useToast();
  const [name, setName] = useState(initial?.name ?? presetSeed?.name ?? "");
  const [path, setPath] = useState(initial?.path ?? presetSeed?.path ?? "");
  const [excludesText, setExcludesText] = useState(
    (initial?.excludes ?? presetSeed?.excludes ?? []).join("\n")
  );
  const [enabled, setEnabled] = useState(initial?.enabled ?? true);
  const [saving, setSaving] = useState(false);
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): shake
  // the Save button alongside the toast on a failed save.
  const [shake, setShake] = useState(0);

  const canSave = name.trim() !== "" && path.trim() !== "" && !saving;

  // GlimStone follow-up pass (v8.0.0): the "error" flash below is now a toast
  // — same shape as Settings.tsx's CloudCredSetsCard.save() (a dialog editor
  // that closes on success via onSaved(), so a toast is the only outcome
  // notice left, success or failure).
  async function handleSave() {
    if (!canSave) return;
    setSaving(true);
    const excludes = excludesText
      .split("\n")
      .map((line) => line.trim())
      .filter((line) => line !== "");
    try {
      const res = initial
        ? await patchFileSet(initial.id, { name: name.trim(), path: path.trim(), excludes, enabled })
        : await createFileSet({ name: name.trim(), path: path.trim(), excludes, enabled });
      if (res.ok) {
        push(t("settings.saved"), "success");
        onSaved();
      } else {
        push(res.error ?? t("settings.error"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      setShake((n) => n + 1);
    } finally {
      setSaving(false);
    }
  }

  // Portal to <body> so the fixed overlay can never be trapped by an ancestor's
  // CSS transform (belt-and-braces with the glim-page-in keyframe fix, #62).
  //
  // GlimStone follow-up pass (jdp live review: "wird das Fenster zu weit oben
  // eingeblendet, dort sitzt der Cardtitelbadge nicht richtig"): was
  // `items-start` (top-anchored, only the backdrop's own `p-4` = 16px above
  // the relative shell) — for THIS dialog's actual short, single-screen
  // content that left the heading Badge's own -11px notch poking up to just
  // ~5px below the literal browser-viewport edge (measured live), reading as
  // a flat rectangle jammed into the corner rather than a notch with any
  // breathing room. `items-center` is the same fix ConfirmDialog.tsx/
  // WhatsNewDialog.tsx/ErrorDetailPanel.tsx already use for their own
  // tone="heading" notch — safe here for the identical reason theirs is
  // safe: the visible box below is capped at `max-h-[90vh]`, strictly under
  // the 100vh flex container, so a centred item's top offset is always
  // positive (never negative/off-screen) regardless of content height —
  // short content (like this one) gets comfortable margin on all sides,
  // and content that grows toward the 90vh cap still centres safely with
  // `overflow-y-auto` on the backdrop covering the rest. The three sites this
  // comment used to flag as "still owed" — Receiver.tsx's ReceiverDialog and
  // Fleet.tsx's own two `items-start` dialogs, the identical copy-pasted
  // shell — are now converted too (whole-app sweep), so every dialog backdrop
  // in this app is `items-center` and there is no remaining copy to find.
  return createPortal(
    <div
      className="glim-modal-backdrop fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4"
      onClick={onClose}
    >
      {/* GlimStone follow-up pass ("half-overlap card notch"): non-scrolling
          `relative` shell wraps the scrollable dialog box — see
          Receiver.tsx's ReceiverDialog for the identical split and why. */}
      <div className="relative w-full max-w-lg">
      {/* `px-5` matches the box's `p-5` so the heading notch lands where a Card's does ([542]) — see FolderBrowser.tsx for why the notch has no offset of its own. */}
      <h2 className="flex items-center px-5">
        <Badge tone="heading" size="heading" wrap>{initial ? t("files.editSet") : t("files.addSet")}</Badge>
      </h2>
      <div
        role="dialog"
        aria-modal="true"
        aria-label={initial ? t("files.editSet") : t("files.addSet")}
        onClick={(e) => e.stopPropagation()}
        className="w-full max-h-[90vh] overflow-y-auto rounded-card bg-carbon-surface p-5 flex flex-col gap-4 shadow-2xl"
      >
        {/* Name — feeds the restic tag, so the server validates it strictly. */}
        <div className="flex flex-col gap-1.5">
          <label className="text-xs text-carbon-textSub">{t("files.name")}</label>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            spellCheck={false}
            autoComplete="off"
            placeholder="documents"
            className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus"
          />
        </div>

        {/* Source folder (relative subpath under the host mount root) */}
        <div className="flex flex-col gap-1.5">
          <FolderBrowser
            inDialog
            label={t("files.path")}
            value={path}
            hostMountRoot={hostMountRoot}
            onChange={setPath}
          />
          {/* A3 disclosure (04-02's PATCH-time clear rule): saving a changed
              path clears the ticked sub-folder selection server-side, so the
              consequence is named HERE, before it happens. Unconditional by
              design — it states the consequence, not a condition, so it fires
              whether or not a selection is stored (and for a create, where
              none can exist yet). The tree-editing surface itself lives on the
              card (FileSetFoldersEditor), never in this dialog. */}
          <p className="text-caption text-carbon-textMuted">{t("files.pathChangeHint")}</p>
          <p className="text-caption text-carbon-textMuted">{t("files.pathHint")}</p>
        </div>

        {/* Exclude patterns, one per line */}
        <div className="flex flex-col gap-1.5">
          <label className="text-xs text-carbon-textSub">{t("files.excludes")}</label>
          <textarea
            value={excludesText}
            onChange={(e) => setExcludesText(e.target.value)}
            spellCheck={false}
            rows={4}
            placeholder={"*.tmp\ncache/"}
            dir="ltr"
            className="rounded-control bg-carbon-surface2 text-carbon-text text-sm font-mono px-3 py-1.5 glim-field-focus text-start"
          />
          <p className="text-caption text-carbon-textMuted">{t("files.excludesHint")}</p>
        </div>

        {/* Include in schedule */}
        {/* ToggleRow, not a bare Toggle ([544]). A bare Toggle sets its label
            immediately beside the switch; every setting row in this app puts the
            words at the start and the switch at the end, which is what ToggleRow
            renders (`flex items-start justify-between`). jdp: "der toggle soll
            rechtsbuendig sein, der text linksbuendig". Fixed at the shared
            component rather than by hand-matching classes here, the same reason
            IncludeToggle.tsx gives for its own switch to ToggleRow. */}
        <ToggleRow checked={enabled} onChange={setEnabled} label={t("files.enabled")} />

        <div className="flex items-center justify-end gap-2 pt-1">
          <Button
            label={t("files.cancel")}
          labelKey="files.cancel"
            tone="neutral"
            onClick={onClose}
            disabled={saving}
          />
          <Button
            key={shake}
            label={t("settings.save")}
            labelKey="settings.save"
            tone="accent"
            onClick={() => void handleSave()}
            disabled={!canSave}
            busy={saving}
            title={saving ? t("common.saving") : undefined}
            className={shake ? "glim-shake" : ""}
          />
        </div>
      </div>
      </div>
    </div>,
    document.body,
  );
}

// ---------------------------------------------------------------------------
// Choose folders — the Files-page selection tree (Phase 4, INTEG-02)
// ---------------------------------------------------------------------------

// The Phase 2 SelectionTree remounted over ONE set root, per the UI-SPEC reuse
// contract (items 1-9): a row-level disclosure on the set card — never inside
// FileSetDialog, whose full-set PATCHes would race the live-selection queue —
// one synthetic root (the set's resolved host path), lazy children through
// GET /api/browse with hostMountRoot as the browse prefix, and NO container
// surfaces: no CACHEDIR switch (optional props skip the sub-row), no custom
// rows, no dest←source arrow, no Reset (the files domain has no
// auto-detection fallback; D-06's exit is Delete folder set, named by the refusal
// copy).
//
// The genuinely new decision is the NULL mirror seed (UI-SPEC item 9): a set
// whose selected_paths is NULL — never touched by this tree — is seeded with a
// SYNTHETIC include of its root, so the root renders CHECKED and the preview
// reads "1 path": exactly what the legacy argv [SourceDir] covers. Rendering
// it unchecked would contradict both the trust posture and D-06's own logic.
// The seed lives in client state only; nothing PATCHes until the first
// toggle, so the column stays NULL (legacy argv pinned by plan 01's tests).
//
// The D-07 exclusions audit list needs no Files-side code either: the per-root
// review disclosure (rootExclusions over the mirror — folders.exclusions
// count + relative mono muted rows, collapsed on every reopen, rendered
// identically for active and dormant roots, zero interactive controls) is
// INSIDE SelectionTree since Phase 3 and lights up for this synthetic root the
// moment the mirror holds exclusions. Restating it here would be a second
// audit surface — the exact duplication the phase's zero-second-
// implementation lock forbids; the Files.tree.dom pins lock this page's
// rendering of it instead.
//
// Exported for this page's dom harness (the FoldersEditor precedent — the
// harness renders the editor against the mocked api client, not a whole card).

/** What one queued file-set PATCH was initiated for (Containers.tsx Pattern 4,
 *  narrowed to the files editor's single owed class — there are no caches or
 *  reset descriptors here). `pre`/`sent` carry the initiating mutation's
 *  mirror effect so a FAILURE can revert by set-difference inverse
 *  (revertFrom below) instead of a captured snapshot — a snapshot would also
 *  undo newer toggles stacked behind the failed one (Pitfall 5). `node` is
 *  the row a failure blames: its shake replays and (for a coded
 *  empty-selection refusal) the inline warn line routes under it. */
interface FileSetSaveDesc {
  node: string;
  pre: { includes: ReadonlySet<string>; exclusions: ReadonlySet<string> };
  sent: { includes: ReadonlySet<string>; exclusions: ReadonlySet<string> };
}

/** Remount key for FileSetFoldersEditor (review CR-01): set id + anchor +
 *  selection PRESENCE. The editor seeds its (includes, exclusions) mirror from
 *  the mount-time `set` prop and never re-syncs, so an edited `set` prop alone
 *  reaches nothing: after a folder edit through FileSetDialog — which fires the
 *  server-side A3 clear (selected_paths = NULL) — the still-mounted editor kept
 *  the OLD anchor's mirror, whose next full-list PATCH was either atomically
 *  refused against the new root (the editor wedged until a page reload) or,
 *  when the path moved deeper, silently resurrected entries the clear had just
 *  deleted. Keying the element on this string remounts the editor on every
 *  anchor/selection-presence change and reseeds from the fresh (post-clear)
 *  view — NULL reseeds to the synthetic root include (UI-SPEC item 9).
 *  Content-only changes of a PRESENT selection deliberately keep the same key:
 *  a list refetch must not reset the editor's expansion memory or browse
 *  cache, nor tear down the serialized save queue while a PATCH is in flight
 *  (Pattern 4). */
export function fileSetEditorKey(set: FileSetView): string {
  return `${set.id}:${set.path}:${set.selectedPaths ? "set" : "null"}`;
}

export function FileSetFoldersEditor({
  set,
  hostMountRoot,
  t,
}: {
  set: FileSetView;
  hostMountRoot: string;
  t: T;
}) {
  const regionId = useId();
  const [open, setOpen] = useState(false);
  const noPath = set.path === "";
  // The set's resolved host path, cleaned on both segments (browseRelToHost is
  // the exact inverse of the browse prefix swap the tree performs below).
  const root = noPath ? "" : browseRelToHost(set.path, hostMountRoot);
  // The (includes, exclusions) mirror in host path space — splitFlatSet over
  // the stored selection, with the NULL case seeded as a synthetic include of
  // the root (UI-SPEC item 9; see the block comment above). Seeded once per
  // editor instance — these seed inputs are read exactly here, never
  // reactively. Reseeding is the CALL SITE's job (review CR-01): the element
  // is keyed by fileSetEditorKey(set) (id + anchor + selection presence), so a
  // folder edit through FileSetDialog — which fires the server-side A3 clear —
  // remounts this component and re-runs this seed against the fresh
  // (post-clear) view, instead of the mounted editor keeping the old anchor's
  // mirror (wedged PATCHes against the new root, or silent resurrection of
  // cleared entries).
  const seed = splitFlatSet(noPath ? [] : (set.selectedPaths ?? [root]));
  const [includes, setIncludes] = useState<Set<string>>(seed.includes);
  const [exclusions, setExclusions] = useState<Set<string>>(seed.exclusions);
  // The mirror the queue reads through a ref (Containers.tsx Pattern 4): the
  // ref the save pipeline reads and the state the tree renders can never
  // drift apart, because every mutation lands in this one helper.
  const mirrorRef = useRef<{ inc: Set<string>; exc: Set<string> }>({ inc: seed.includes, exc: seed.exclusions });
  // Per-row busy/shake maps keyed by HOST path, exactly like FoldersEditor's;
  // blockedPath carries the row whose last toggle was refused (client or
  // server D-06) so SelectionTree renders the inline warn line under it.
  const [rowBusy, setRowBusy] = useState<Record<string, boolean>>({});
  const [rowShake, setRowShake] = useState<Record<string, number>>({});
  const [blockedPath, setBlockedPath] = useState<string | null>(null);
  // Editor-lifetime listings cache (Phase 2 research, Pitfall 3): survives the
  // disclosure closing (this component stays mounted above its null return),
  // dies with the page.
  const browseCache = useRef(new Map<string, Promise<BrowseResponse>>());
  const { push } = useToast();
  // The one-deep serialized PATCH queue (T-04-11; Containers.tsx Pattern 4
  // narrowed to ONE owed class): while an attempt is in flight a further
  // toggle just updates the mirror and marks the queue dirty; when the
  // attempt resolves, a single drain sends the LATEST full flat list. A slow
  // or failing save can therefore never clobber a newer toggle, and a burst
  // collapses to one draining request.
  const queueRef = useRef<{ inFlight: boolean; dirty: boolean }>({ inFlight: false, dirty: false });
  const pendingDescRef = useRef<FileSetSaveDesc | null>(null);
  // Rows whose toggles are folded into the next attempt's body (an attempt
  // always carries the live mirror); each row's busy flag clears exactly when
  // the attempt that acknowledges its effect settles.
  const pendingRowsRef = useRef<Set<string>>(new Set());

  function applyMirror(inc: Set<string>, exc: Set<string>): void {
    mirrorRef.current = { inc, exc };
    setIncludes(inc);
    setExclusions(exc);
  }

  // Queue entry point — called AFTER the mirror has been updated (the exact
  // FoldersEditor shape, minus the reset-stickiness: the files editor has no
  // reset class).
  function scheduleSave(desc: FileSetSaveDesc): void {
    if (queueRef.current.inFlight) {
      queueRef.current.dirty = true;
      pendingDescRef.current = desc; // latest desc wins, matching the drain
      return;
    }
    pendingDescRef.current = desc;
    void attemptSave();
  }

  // One save attempt. The body is composed at attempt start from the LIVE
  // mirror — never from desc.sent, which is already stale when later
  // mutations stacked behind it. Every file-set selection PATCH the editor
  // sends comes through here as a single fetchJSON call, so no two saves are
  // ever concurrent (maxConcurrentPatches === 1, pinned in the harness).
  async function attemptSave(): Promise<void> {
    queueRef.current.inFlight = true;
    const rows = [...pendingRowsRef.current];
    pendingRowsRef.current.clear();
    const desc = pendingDescRef.current;
    pendingDescRef.current = null;
    try {
      const live = mirrorRef.current;
      const r = await patchFileSet(set.id, { selectedPaths: toFlatList(live.inc, live.exc) });
      if (r.ok) {
        // The live-save house shape (FoldersEditor's paths class): the saved
        // toast announces each acknowledged pick.
        push(t("folders.saved"), "success");
      } else {
        // Server error text VERBATIM, coded envelope or not. A coded
        // "empty-selection" refusal additionally routes to the same inline
        // warn line the client-side block uses (defense-in-depth: unreachable
        // while the client block below exists, honored when it fires — a
        // concurrent writer or a stale anchor can still produce it).
        push(r.error ?? t("settings.error"), "fail");
        if (desc) {
          if (r.code === "empty-selection") setBlockedPath(desc.node);
          revertFrom(desc);
        }
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      if (desc) revertFrom(desc);
    } finally {
      queueRef.current.inFlight = false;
      setRowBusy((b) => {
        const n = { ...b };
        for (const p of rows) n[p] = false;
        return n;
      });
      if (queueRef.current.dirty) {
        queueRef.current.dirty = false;
        if (pendingDescRef.current) void attemptSave();
      }
    }
  }

  // Failure revert: un-apply the failed mutation's DELTA onto the live
  // mirror — delete what it added, re-add what it removed. applyToggle's
  // inverse computed as set differences, so a toggle that happened AFTER the
  // failed one (but before its save resolved) survives untouched; restoring a
  // captured pre-mutation snapshot here is the exact Pitfall 5 bug.
  function revertFrom(desc: FileSetSaveDesc): void {
    const live = mirrorRef.current;
    const inc = new Set(live.inc);
    const exc = new Set(live.exc);
    for (const p of desc.sent.includes) if (!desc.pre.includes.has(p)) inc.delete(p);
    for (const p of desc.pre.includes) if (!desc.sent.includes.has(p)) inc.add(p);
    for (const p of desc.sent.exclusions) if (!desc.pre.exclusions.has(p)) exc.delete(p);
    for (const p of desc.pre.exclusions) if (!desc.sent.exclusions.has(p)) exc.add(p);
    applyMirror(inc, exc);
    setRowShake((s) => ({ ...s, [desc.node]: (s[desc.node] ?? 0) + 1 }));
  }

  // One tree checkbox toggle — optimistic reducer apply over the LIVE mirror
  // (the ref, not the state closure), then the queue. The shared applyToggle
  // is the ONLY mutation path (T-02-10; no second selection implementation
  // exists here). D-06's client half runs BEFORE anything else: a toggle that
  // would leave ZERO includes for the set never PATCHes — the refusal copy
  // orients to Delete folder set (the files domain has no Reset and no
  // auto-detection fallback to return to).
  function onToggle(hostPath: string): void {
    const pre = { includes: mirrorRef.current.inc, exclusions: mirrorRef.current.exc };
    const next = applyToggle(hostPath, pre.includes, pre.exclusions);
    // A reducer no-op must never become a save (review CR-01 discipline).
    const unchanged =
      next.includes.size === pre.includes.size &&
      [...next.includes].every((p) => pre.includes.has(p)) &&
      next.exclusions.size === pre.exclusions.size &&
      [...next.exclusions].every((p) => pre.exclusions.has(p));
    if (unchanged) return;
    if (next.includes.size === 0) {
      setBlockedPath(hostPath);
      setRowShake((s) => ({ ...s, [hostPath]: (s[hostPath] ?? 0) + 1 }));
      return;
    }
    setBlockedPath(null);
    applyMirror(next.includes, next.exclusions);
    setRowBusy((b) => ({ ...b, [hostPath]: true }));
    pendingRowsRef.current.add(hostPath);
    scheduleSave({
      node: hostPath,
      pre,
      sent: { includes: next.includes, exclusions: next.exclusions },
    });
  }

  // A no-Path set has nothing to present — the card's files.noPathHint line
  // stands in (D-02). Guarded here AND at the FileSetRow call site, so the
  // disclosure can never render for a set the tree cannot represent, even if
  // a future caller forgets the gate. After the hooks (rules of hooks).
  if (noPath) return null;

  return (
    // UI-SPEC item 1 placement: below the Backups/Restore disclosure,
    // separated by a top border; full-width text button, rotating chevron,
    // aria-expanded + aria-controls to the region it reveals.
    <div className="border-t border-carbon-border pt-3">
      <button
        type="button"
        aria-expanded={open}
        aria-controls={regionId}
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-center gap-2 py-1 text-start text-sm text-carbon-textSub"
      >
        <span className="w-4 shrink-0 flex items-center justify-center" aria-hidden="true">
          <svg
            width="10"
            height="10"
            viewBox="0 0 12 12"
            fill="none"
            className={`transition-transform ${open ? "rotate-90" : "rtl:rotate-180"}`}
          >
            <path fill="currentColor" d="M4 1.3 8.5 6 4 10.7Z" />
          </svg>
        </span>
        {t("files.foldersToggle")}
      </button>
      {open && (
        <div id={regionId} role="region" aria-label={t("folders.title")} className="mt-2 flex flex-col gap-2">
          <p className="text-xs text-carbon-textMuted">{t("files.foldersHint")}</p>
          {/* One synthetic root, per UI-SPEC item 2: source = the resolved
              host path, dest = "" (the tree then labels the row with the bare
              path — no dest←source arrow), reachable always. The preview count
              line on that root row renders from rootIncludeCount inside
              SelectionTree — the exact maximal-include membership the next
              backup hands restic (T-04-10: never a DOM or loaded-children
              count; pinned against toFlatList membership in the harness). */}
          <SelectionTree
            mounts={[{ source: root, dest: "", selected: true, isAppdata: false, reachable: true }]}
            customPaths={[]}
            includes={includes}
            exclusions={exclusions}
            hostSourceRoot={hostMountRoot}
            containerName={`fileset-${set.id}`}
            browseCache={browseCache.current}
            onToggle={onToggle}
            // customPaths is always [] so the remove chip can never render;
            // the prop stays required on SelectionTreeProps (container shape).
            onRemoveCustom={() => {}}
            busyPaths={new Set(Object.keys(rowBusy).filter((k) => rowBusy[k]))}
            shakeCounts={rowShake}
            blockedPath={blockedPath}
            // D-06 copy routing: the refusal orients to THIS card's exit
            // ("Delete folder set"), not the folders page's "Reset" (UI-SPEC copy
            // table; the files domain has no Reset and no auto-detect).
            blockedMessage={t("files.emptySelectionBlocked")}
          />
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// File-set row
// ---------------------------------------------------------------------------

export function FileSetRow({
  set,
  hostMountRoot,
  restoreFolder,
  t,
  onRefresh,
  onEdit,
  index,
}: {
  set: FileSetView;
  hostMountRoot: string;
  restoreFolder: string;
  t: T;
  onRefresh: () => void;
  onEdit: () => void;
  /** Position in the rendered list — the rainbow palette position (GlimStone
   *  form-engine Phase 2, Task 2). Assigned by LIST INDEX, never a hash of
   *  `set.id`/name — see the caller below. */
  index: number;
}) {
  const progressMap = useProgress();
  const progress = progressMap[`files:${set.name}`];
  const running = anyActive(progressMap);
  const [removing, setRemoving] = useState(false);
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): a
  // failed action toasts AND shakes its button.
  const [shake, setShake] = useState(0);

  const noPath = set.path === "";
  const pathMissing = !noPath && !set.pathExists;

  async function handleRemove() {
    if (!(await confirm(t("files.deleteSetConfirm")))) return;
    setRemoving(true);
    try {
      const res = await deleteFileSet(set.id);
      if (res.ok) onRefresh();
      else {
        push(res.error ?? t("common.removeFailed"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.removeFailed"), "fail");
      setShake((n) => n + 1);
    } finally {
      setRemoving(false);
    }
  }

  return (
    <div
      style={{ ...hueVars(rainbowAt(index)), "--row-i": String(index) } as CSSProperties}
      // glim-tint washes the card (trap #2 — without it this card shows
      // almost no colour at rest); glim-active while THIS set's own
      // backup/restore is actively running — mirrors ContainerRow/VMRow.
      // glim-stagger-row (GlimStone motion-engine animation 3) — see
      // ContainerRow's identical comment.
      className={`relative overflow-hidden bg-carbon-surface rounded-card p-4 flex flex-col gap-3 glim-hue glim-stagger-row ${
        progress?.active ? "glim-active" : ""
      }`}
    >
      {/* Top row: name + chips, path, last backup */}
      <div className="flex items-start gap-3 flex-wrap">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="font-semibold text-carbon-text text-sm truncate">
              {set.name}
            </span>
            {set.excludes.length > 0 && (
              // GlimStone completeness sweep: was a hand-rolled span byte-
              // identical to Badge's own tone="neutral" medium-stage classes
              // (bg-carbon-surface2/text-carbon-textSub, rounded-control) —
              // exactly the drift Badge.tsx exists to prevent. `wrap` matches
              // this straggler's original un-clipped, content-grows sizing
              // (no fixed height, just px-2/py-0.5) more closely than the
              // default fixed-height stage would.
              <Badge tone="neutral" wrap>
                {t("files.excludesCount").replace("{n}", String(set.excludes.length))}
              </Badge>
            )}
            {/* Source-folder problems, loudest first: no folder at all (discovered
                set), then folder configured but missing on disk. */}
            {noPath && (
              <Badge tone="warn" wrap title={t("files.noPathHint")}>
                {t("files.noPath")}
              </Badge>
            )}
            {pathMissing && (
              <Badge tone="fail" wrap>
                {t("files.pathMissing")}
              </Badge>
            )}
          </div>
          {!noPath && (
            <p dir="ltr" className="mt-1 text-xs font-mono text-carbon-textMuted truncate text-start">
              {hostMountRoot}/{set.path}
            </p>
          )}
          {noPath && (
            <p className="mt-1 text-xs text-carbon-textMuted">{t("files.noPathHint")}</p>
          )}
        </div>

        {/* Action badges, top-right — the corner "Letztes Backup" used to
            occupy (jdp, 2026-09-11: "kannst du die buttons und toggle in den
            ordner cards genauso anordnen wie in den container cards?"). That is
            exactly the move the container card already made, in the same words
            from the same reviewer ("Jetzt sichern und Export sollen
            quadratische Badges mit Glyph sein, die sollen rechts oben in der
            Ecke sein wo jetzt Letztes Backup steht"), and this card was the one
            left behind: it kept the badges scattered along a middle row while
            the corner held text.

            The date is not lost, it moves down beside the Backups trigger, the
            same place the container card keeps its own `lastBackupText`. One
            fact, one place.

            Same `ms-auto flex items-start gap-1.5 shrink-0` wrapper, and the
            same gap-1.5 between adjacent 32px tiles that every icon-badge pair
            in this app uses. Backup first because it is the thing somebody
            comes to the card to do; edit and remove follow. */}
        <div className="ms-auto flex items-start gap-1.5 shrink-0">
          <FileSetBackupButton set={set} t={t} onBackedUp={onRefresh} running={running} />
          {/* Just the verb (jdp, 2026-09-11: "Odner-Set bearbeiten soll nur
              Bearbeiten heißen, Ordner-Set löschen nur Löschen"). These two sit
              INSIDE the card of the set they act on, so naming the set again on
              the badge repeats what the heading two lines up already says - and
              in reactive mode that repetition is the whole word that appears
              under the pointer.

              New `common.edit` / `common.delete` rather than editing
              files.editSet: that key is ALSO the edit dialog's own heading
              (see FileSetDialog), where "Bearbeiten" alone would stop saying
              what is being edited. Two call sites, two jobs, two keys.

              The words are not new translations - they are lifted from what
              the app already says for these verbs (offsite.targets.edit and
              snapshots.delete), so the house key cannot drift from the rest.
              `common.delete` still matches glyphFor's `/\.(delete|remove)/`,
              so the trash resolves the same way. */}
          <Button
            label={t("common.edit")}
            labelKey="common.edit"
            glyph={<IconPencil />}
            tone="accent"
            title={t("files.editSet")}
            onClick={onEdit}
          />
          <Button
            key={shake}
            label={t("common.delete")}
            labelKey="common.delete"
            glyph={<IconTrash />}
            tone="accent"
            title={t("files.deleteSet")}
            onClick={() => void handleRemove()}
            disabled={removing}
            className={shake ? "glim-shake" : ""}
          />
        </div>
      </div>

      {/* Actions row — the schedule toggle, flush right and nothing else.
          The container card's own equivalent row is a single `ms-auto flex
          flex-col items-end` stack of toggle rows, and it says why: once the
          action badges moved into the top-right corner, this row was free to
          become one flush-right column. The folder card now has the same two
          moves behind it, so it gets the same row.

          The edit and remove badges that used to sit here are in that corner
          now, next to the backup badge. They are actions, not settings, and a
          row that mixed a switch with two action tiles read as one group of
          four unrelated controls. */}
      <div className="flex items-start">
        <div className="ms-auto flex flex-col items-end gap-2">
          {/* No wrapping `<label>`/`<span>` anymore: FileSetEnabledToggle now
              renders the full ToggleRow itself (label included, text-first),
              the identical shape Containers.tsx's IncludeToggle call site
              already uses — see that component's own comment. */}
          <FileSetEnabledToggle id={set.id} initial={set.enabled} />
        </div>
      </div>

      {/* #199: the consequence of the toggle right ABOVE, in words. This tab is
          where a set is created and where the include switch is flipped, so it
          is the second place a reader can walk away with the wrong belief about
          whether the folder is protected. Same component, same server-computed
          sentence as the Schedules card.

          It sat between the badge corner and the toggle until now, which put a
          sentence between the switch and the badges it belongs under. jdp asked
          for the toggle directly beneath the buttons (2026-09-11: "der toggle
          im zeitplan einschließen bitte direkt unter die oberen buttons"), so
          the line moves below the switch it describes. It reads better there
          anyway: a consequence after the control, not before it. */}
      <EffectiveScheduleLine effective={set.effectiveSchedule} />

      {/* Backups / Restore disclosure, with the last-backup date on its own
          row — the container card's shape. See `trailing`'s own doc. */}
      <FileSetRestorePanel
        set={set}
        hostMountRoot={hostMountRoot}
        restoreFolder={restoreFolder}
        t={t}
        onSetsChanged={onRefresh}
        trailing={
          // A column, so the running note sits ON TOP of the date it qualifies
          // (jdp, 2026-09-11). The two belong together: one says the card is
          // busy right now, the other says when it last was not. Only ever one
          // line tall at rest - the note renders only while something else is
          // actually running, and `text-end` keeps both flush with the card's
          // right edge whether or not it is there.
          <span className="ms-auto shrink-0 flex flex-col items-end text-xs whitespace-nowrap">
            {running.active && !progress?.active && (
              <span className="text-carbon-textMuted">{t(busyPhraseKey(running.phase))}</span>
            )}
            <span className="text-carbon-textMuted">
              {`${t("containers.lastBackup")}: ${
                set.lastBackup ? formatTs(set.lastBackup) : t("containers.never")
              }`}
            </span>
          </span>
        }
      />

      {/* Choose folders — the Phase 2 SelectionTree over this set's own root
          (Phase 4, INTEG-02; UI-SPEC item 1: below the restore disclosure,
          separated by the editor's own top border). A no-Path set renders no
          disclosure — the files.noPathHint line in the header stands in.
          Keyed by fileSetEditorKey (review CR-01): the editor seeds its mirror
          from mount-time props only, so an anchor or selection-presence change
          (a dialog path edit and its server-side A3 clear) must remount it. */}
      {!noPath && (
        <FileSetFoldersEditor key={fileSetEditorKey(set)} set={set} hostMountRoot={hostMountRoot} t={t} />
      )}

      {/* Live backup/restore progress, pinned to the card's bottom edge */}
      {progress && (
        <ProgressBar
          percent={progress.percent}
          active={progress.active}
          label={progress.phase === "restore" ? t("common.restoring") : t("common.backingUp")}
        />
      )}
      {/* Stop a backup that is running (#200). Beside the bar that shows it,
          because that bar is the only place this card admits something is
          happening at all, and a control for stopping a thing belongs where the
          thing is visible.
            Only while a BACKUP is actually running: the restore has its own
          cancel inside the Backups panel above, with its own confirmation about
          a half-restored target, and two cancel buttons on one card that mean
          different things is worse than none. `progress.active` gates it so a
          finished run's last frame does not leave a button that can only ever
          answer "nothing to cancel". */}
      {progress && progress.active && progress.phase !== "restore" && (
        <div className="flex justify-end">
          <BackupCancelButton cancelKey={`files:${set.name}`} name={set.name} t={t} />
        </div>
      )}
      {confirmDialog}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Files page
// ---------------------------------------------------------------------------

export function Files() {
  const { t } = useT();
  const { push } = useToast();
  // One subscription for the whole list rather than one per row — see
  // Containers.tsx's identical call for the same reasoning.
  useRainbow();
  // Broader "something is running" signal: any backup/restore/replication in
  // flight disables the bulk start buttons + shows a hint.
  const running = anyActive(useProgress());
  const [sets, setSets] = useState<FileSetView[]>([]);
  const [hostMountRoot, setHostMountRoot] = useState("/host/user");
  const [restoreFolder, setRestoreFolder] = useState(DEFAULT_RESTORE_FOLDER);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  // null = closed; "new" = create dialog; a view = edit dialog for that set.
  const [dialog, setDialog] = useState<"new" | FileSetView | null>(null);
  // Pre-fill values for the create dialog when opened via "Add preset: Host
  // system config" (null for a plain "Add folder set"). Only meaningful while
  // dialog === "new"; cleared alongside it.
  const [presetSeed, setPresetSeed] = useState<{
    name: string;
    path: string;
    excludes: string[];
  } | null>(null);
  // The "Host system config" preset suggestion for the current platform
  // (#134, files domain's flash-domain analogue). null until loaded or on a
  // failed fetch — either way the preset button stays hidden, never a
  // half-working affordance.
  const [preset, setPreset] = useState<FileSetPresetResponse | null>(null);
  const [discovering, setDiscovering] = useState(false);
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): shake
  // the Discover button on a failed discover, alongside its existing toast —
  // same mechanism as Containers.tsx's/VMs.tsx's identical shakeDiscover.
  const [shakeDiscover, setShakeDiscover] = useState(0);
  const [backupAllBusy, setBackupAllBusy] = useState(false);
  // Same shake-on-failure treatment for the "back up all" batch start — mirrors
  // Containers.tsx's backupSelected/shakeBackupSelected.
  const [shakeBackupAll, setShakeBackupAll] = useState(0);

  function loadSets() {
    return listFileSets()
      .then((res) => {
        if (res.ok) {
          setSets(res.fileSets ?? []);
          // Clear any stale banner from a previous failed load — a later success
          // must not leave "Failed to load file sets" up while the UI works.
          setError(null);
        } else setError(res.error ?? t("files.loadSetsFailed"));
      })
      .catch(() => setError(t("files.loadSetsFailed")));
  }

  useEffect(() => {
    // Gate the loading flag on BOTH fetches (Promise.all, not two independent
    // .finally()s): the file-set restore controls below seed their target-folder
    // state from restoreFolder ONCE at mount (React only reads a useState
    // initialiser the first render), so if they mounted before this settings
    // fetch resolved they'd permanently miss the real default and fall back to
    // the generic placeholder example instead — same class of bug as #69.
    const sets = loadSets();
    const settings = getSettings()
      .then((res) => {
        if (res.hostMountRoot) setHostMountRoot(res.hostMountRoot);
        if (res.settings?.restoreFolder) setRestoreFolder(res.settings.restoreFolder);
      })
      .catch(() => undefined);
    // Independent of the two fetches above: a failed/slow preset lookup must
    // never block the page (loading gate stays on sets+settings only) — the
    // preset button just stays hidden until it resolves.
    void getFileSetPreset()
      .then((res) => {
        if (res.ok) setPreset(res);
      })
      .catch(() => undefined);
    void Promise.all([sets, settings]).finally(() => setLoading(false));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps -- t() is only read to build a failure message; re-fetching on a language switch would be a wasted round-trip

  /** Opens the create dialog pre-filled with the "Host system config" preset
   *  (still fully editable — Save persists through the SAME create-file-set
   *  endpoint as a blank "Add folder set"). No-op until the preset has
   *  loaded and is offered for this platform. */
  function handleAddPreset() {
    if (!preset?.offered) return;
    setPresetSeed({ name: preset.name, path: preset.path, excludes: preset.excludes });
    setDialog("new");
  }

  /** Opens a blank create dialog — used by both "Add folder set" entry
   *  points so a stale preset seed from a previous open can never leak in. */
  function handleAddBlank() {
    setPresetSeed(null);
    setDialog("new");
  }

  // GlimStone follow-up pass (v8.0.0): the "+N" / error note never
  // auto-cleared (it stuck around next to the Discover button until the next
  // click) — now a toast, mirroring Containers.tsx's/VMs.tsx's identical
  // handleDiscover.
  async function handleDiscover() {
    setDiscovering(true);
    try {
      const res = await discoverFiles();
      if (res.ok) {
        push(`+${res.discovered ?? 0}`, "success");
        await loadSets();
      } else {
        push(res.error ?? t("common.discoverFailed"), "fail");
        setShakeDiscover((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.discoverFailed"), "fail");
      setShakeDiscover((n) => n + 1);
    } finally {
      setDiscovering(false);
    }
  }

  // "Back up all now" fires the SERVER-SIDE batch (batch:files) for every
  // enabled set that has a source folder; per-set progress shows on the cards.
  const backupableIds = sets.filter((s) => s.enabled && s.path !== "").map((s) => s.id);

  // jdp live review ("Ordnerset hinzufügen Button rechts oben kann weg, der
  // ist redundant"): the empty-state Card below already carries its own
  // prominent "Add folder set" (+ "Add preset", where offered) CTA, so
  // showing the identical pair a second time in the top-right actions bar
  // was pure duplication — confirmed both call the exact same handlers
  // (handleAddPreset/handleAddBlank). Gate the top-right pair on NOT being in
  // that empty state; once a set exists the empty-state Card stops rendering
  // and the top-right pair is the page's only entry point again, so "Add" is
  // never unreachable. Mirrors the loading/error/empty guard already used
  // for the empty-state block itself below.
  const showEmptyState = !loading && !error && sets.length === 0;

  // GlimStone follow-up pass (v8.0.0): same "+N"/error note migrated off a
  // stuck local span onto a toast — mirrors Containers.tsx's backupSelected
  // (push + shakeBackupSelected).
  async function handleBackupAll() {
    setBackupAllBusy(true);
    try {
      const res = await backupFilesAll(backupableIds);
      if (res.ok) {
        push(t("containers.batchStarted"), "success");
      } else {
        push(res.error ?? t("settings.error"), "fail");
        setShakeBackupAll((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      setShakeBackupAll((n) => n + 1);
    } finally {
      setBackupAllBusy(false);
    }
  }

  return (
    // PAGE_SHELL (jdp live-review, "Können wir die nicht überall gleich breit
    // machen?"): was `gap-6 max-w-5xl` — the third page carrying that same
    // off-standard pair, alongside Containers and VMs. Flat 40px, same
    // reasoning as Containers: this page's "Alle jetzt sichern" action row is
    // a sibling of the heading, not part of it. See lib/pageShell.ts.
    <div className={PAGE_SHELL}>
      {/* Page heading + Discover (disaster-recovery) + Add actions */}
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-2xl font-semibold text-carbon-text">{t("files.title")}</h1>
          <p className="mt-1 text-sm text-carbon-textSub">{t("files.subtitle")}</p>
          <div className="mt-2"><OffsiteIndicator domain="files" /></div>
        </div>
        <div className="flex items-center gap-2 shrink-0 flex-wrap">
          <Button
            key={shakeDiscover}
            label={t("containers.discover")}
            labelKey="containers.discover"
            tone="neutral"
            onClick={() => void handleDiscover()}
            disabled={discovering}
            busy={discovering}
            title={t("files.discoverHint")}
            className={shakeDiscover ? "glim-shake" : ""}
          />
          {/* Generic/TrueNAS-only one-click starting point (#134): Unraid
              already has the dedicated flash domain for host-level config, so
              preset stays null (never offered) there. Also hidden in the
              empty state — see showEmptyState's own comment above. */}
          {!showEmptyState && preset?.offered && (
            <Button
              label={t("files.addPreset")}
              labelKey="files.addPreset"
              tone="neutral"
              onClick={handleAddPreset}
              title={t("files.addPresetHint")}
            />
          )}
          {!showEmptyState && (
            <Button
              label={t("files.addSet")}
          labelKey="files.addSet"
              tone="accent"
              onClick={handleAddBlank}
            />
          )}
        </div>
      </div>

      {loading && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      )}
      {error && <p className="text-sm text-statusFail">{error}</p>}

      {/* Empty state — the "no separate file-backup tool needed" pitch.
          GlimStone follow-up pass (jdp live review: "Die Card im Ordner-Tab
          hat keine Cardtitelbadge mit dem Infotext der in der Card steht"):
          this card had no heading at all — just the icon, the permanent
          pitch paragraph, and the buttons — the one Card-shaped box on this
          page that never got the tone="heading" notch every other Card in
          the app carries. `relative glim-notch-card` (no separate inner
          overflow-hidden box needed — unlike Config.tsx's backupTitle split,
          this card was never `overflow-hidden` to begin with, so a single
          div can host both the badge's positioned ancestor and the visible
          surface, matching Config.tsx's own snapshotsTitle card). The old
          permanent `<p>{t("files.empty")}</p>` reads once and then costs
          vertical space forever (rule 8) — moved verbatim onto the new
          heading Badge as an `onAccent` InfoBubble instead, same content,
          zero new i18n keys for the body (mirrors Flash.tsx's
          backupTitle/restoreNote pass). hueIndex={0}: the only tone="heading"
          notch on this page's own body (the dialog's h2 badge deliberately
          carries no hueIndex, same as every other dialog title in the app),
          and mutually exclusive with FileSetRow's OWN rainbowAt(index) tint
          (this card only renders while the list is empty, i.e. never
          alongside a single FileSetRow), so there is no position to collide
          with.
          insetStart={6} (GlimStone follow-up pass, jdp: "Files/Ordner-Tab:
          Cardtitelbadge falsch platziert" — a SECOND, distinct root-cause
          mechanism from the split-notch one Badge.tsx's own `insetStart` doc
          otherwise documents: this card's `relative` ancestor and its p-6
          padded content ARE the same single div (no structural split), so
          the static-position fallback should already be correct here — but
          this parent is ALSO `text-center flex flex-col items-center`
          (centering the icon/button below), and the `<h2>` above them has NO
          in-flow content of its own once its only child (the notch Badge)
          becomes `position: absolute` — an h2 with nothing left in flow
          collapses to a 0×0 box, which `items-center` then centers
          horizontally in the card rather than stretching to the padding
          edge. The static position then resolves against that zero-width,
          CENTERED h2, landing the badge at the card's horizontal centre
          (measured live: 488px right of the p-6 content edge) instead of
          flush with it — confirmed identical on Fleet.tsx's and
          Receiver.tsx's own empty-state Cards, which share this exact
          `text-center items-center` recipe. `insetStart={6}` sidesteps the
          collapsed-h2 quirk entirely: it's a real `start-6` CSS offset
          resolved against the outer `relative` box directly, so it doesn't
          care what the h2 collapsed to. */}
      {showEmptyState && (
        // `.glim-hue` added (rainbow-mode completeness sweep, jdp live
        // review: "Es sind nicht alle Buttons in den Regenbogen-Modus
        // eingepflegt"): `glim-notch-card` alone only wires the reactive-mode
        // hover reveal on the Badge's own notch, never --accent/--focus-ring
        // itself, so the "Add set"/"Add from preset" buttons below stayed
        // flat regardless of rainbow. Same hueIndex={0} the Badge already
        // uses (Fleet.tsx's/Receiver.tsx's own identical fix, same
        // reasoning).
        <div
          className="relative glim-notch-card glim-hue bg-carbon-surface rounded-card p-6 text-center flex flex-col items-center gap-3"
          style={hueVars(rainbowAt(0)) as CSSProperties}
        >
          <h2 className="flex items-center">
            <Badge tone="heading" size="heading" wrap hueIndex={0} insetStart={6}>
              {t("files.setsTitle")}
              <InfoBubble tip={t("files.empty")} onAccent />
            </Badge>
          </h2>
          <EmptyStateIcon icon={IconFiles} />
          <div className="flex items-center gap-2 flex-wrap justify-center">
            {preset?.offered && (
              <Button
                label={t("files.addPreset")}
                labelKey="files.addPreset"
                tone="neutral"
                onClick={handleAddPreset}
                title={t("files.addPresetHint")}
              />
            )}
            <Button
              label={t("files.addSet")}
          labelKey="files.addSet"
              tone="accent"
              onClick={handleAddBlank}
            />
          </div>
        </div>
      )}

      {/* Bulk "back up all" bar */}
      {!loading && sets.length > 0 && (
        <div className="flex items-center gap-3 flex-wrap">
          <Button
            key={shakeBackupAll}
            label={t("files.backupAll")}
            labelKey="files.backupAll"
            hueIndex={BULK_HUE.backup}
            tone="accent"
            onClick={() => void handleBackupAll()}
            disabled={backupAllBusy || running.active || backupableIds.length === 0}
            className={shakeBackupAll ? "glim-shake" : ""}
          />
          {!backupAllBusy && running.active && (
            <span className="text-xs text-carbon-textMuted">
              {t(busyPhraseKey(running.phase))}
            </span>
          )}
        </div>
      )}

      {/* File-set cards */}
      {!loading && sets.length > 0 && (
        <div className="flex flex-col gap-3 glim-content-fade">
          {sets.map((s, i) => (
            <FileSetRow
              key={s.id}
              set={s}
              hostMountRoot={hostMountRoot}
              restoreFolder={restoreFolder}
              t={t}
              onRefresh={() => void loadSets()}
              onEdit={() => setDialog(s)}
              index={i}
            />
          ))}
        </div>
      )}

      {/* Add / edit dialog */}
      {dialog !== null && (
        <FileSetDialog
          initial={dialog === "new" ? null : dialog}
          presetSeed={dialog === "new" ? presetSeed : null}
          hostMountRoot={hostMountRoot}
          t={t}
          onClose={() => {
            setDialog(null);
            setPresetSeed(null);
          }}
          onSaved={() => {
            setDialog(null);
            setPresetSeed(null);
            void loadSets();
          }}
        />
      )}
    </div>
  );
}
