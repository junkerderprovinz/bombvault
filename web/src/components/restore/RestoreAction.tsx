// ---------------------------------------------------------------------------
// RestoreAction — the shared in-place restore control.
//
// Containers, VMs, and Recovery each hand-rolled the SAME in-place restore
// mechanics: a useBackupWatch(kind:'restore') fire-and-watch cycle, an optional
// confirm gate, an optional "leave stopped" toggle, the accent restore trigger
// (spinner while pending + a busy hint when another op blocks it), and the
// <RestoreProgress> banner underneath. This owns that one control so the three
// call sites stop diverging.
//
// The trigger has TWO shapes and exactly one behaviour: a text button (the
// default — a restore FORM's submit, under a destination picker and a confirm)
// or, with `iconBadge`, the app's square 32px icon badge flush at the row's far
// edge (a per-item LIST ROW action). Both are built from the same
// `triggerDisabled` expression and the same `handleRestore`, a few lines above
// where they are rendered, so the two shapes cannot drift into two different
// notions of "can this restore run" — see `iconBadge`'s own doc below.
//
// It is deliberately delete-agnostic and list-agnostic: the caller owns the row
// chrome (snapshot id / time / tags), the snapshot list + Source toggle, and the
// delete button (delete uses the PLURAL-domain deleteSnapshot — never crossed
// here; this watch's matchRun stays SINGULAR "container"/"vm" or it never
// resolves). RestoreAction only drives the one restore.
//
// cancelledRef is load-bearing: this component owns the single ref instance,
// hands it to useBackupWatch (whose no-run fallback reads it to report a neutral
// "cancelled" instead of a phantom green "success"), and forwards the SAME
// instance to RestoreProgress → RestoreCancelButton (which sets it true on a
// successful cancel). They must all share one ref.
// ---------------------------------------------------------------------------

import { useRef, useState, type ReactNode } from "react";
import { restore, restoreVM } from "../../lib/api";
import type { useT } from "../../lib/i18n";
import { useBackupWatch } from "../../lib/backupWatch";
import { useProgress, busyPhraseKey } from "../../lib/progress";
import { RestoreProgress } from "./RestoreProgress";
import type { RepoSource } from "../SourceToggle";
import { Button } from "../Button";
import { IconRestore } from "../Sidebar";
import { useConfirm } from "../../lib/useConfirm";

import { Toggle } from "../Toggle";
type T = ReturnType<typeof useT>["t"];

interface RestoreActionProps {
  /** Restore domain — drives the progressKey `${domain}:${name}`, the matchRun
   *  domain string (SINGULAR: a plural typo makes the watch never resolve), and
   *  the choice of restore() vs restoreVM(). */
  domain: "container" | "vm";
  /** Target container/VM identifier — the ONLY value restore()/restoreVM()
   *  and the progressKey/matchRun below may use. For containers this is
   *  always the container name (no display/identifier split there). For VMs
   *  the caller MUST pass the raw libvirt name (VM.libvirtName), never the
   *  display VM.name — see VM.libvirtName's doc comment (lib/api.ts). */
  name: string;
  /** Human-readable name substituted into the in-place cancel warning.
   *  Defaults to `name`. Callers whose domain has a display/identifier split
   *  (VMs on TrueNAS) should pass the display name here while `name` above
   *  stays the raw identifier. */
  displayName?: string;
  /** Snapshot to restore — a snapshot id or the literal "latest". */
  snapshotId: string;
  /** Repo to restore from; undefined => the backend-default repo (Recovery). */
  source?: RepoSource;
  /** "Something else is running" signal (anyActive) — busy-guards this restore.
   *  Recovery wraps its plain boolean as { active }. */
  otherActive: { active: boolean; phase?: string };
  /** Sticky success-banner text (already localized by the caller). */
  successMessage: string;
  /** Gate the restore behind an explicit confirm checkbox. Default true.
   *  Recovery's per-row action passes false because the checkbox does not fit
   *  in a single-line row action, and pairs it with `confirmMessage` instead —
   *  it does NOT mean the restore is unguarded. */
  requireConfirm?: boolean;
  /** Ask this question in a modal before firing, the same way "Restore all"
   *  and every other destructive action in the app does (useConfirm). Already
   *  localized by the caller.
   *
   *  It exists because `requireConfirm={false}` used to mean genuinely
   *  unguarded: Recovery's card-5 rows passed it on the strength of a prop doc
   *  claiming "its own stepper gates the whole flow", and that stepper gates
   *  nothing. One click on an unlabelled glyph badge started an in-place
   *  restore over live appdata or VM disks, while the "Restore all" button in
   *  the SAME card asked first. A row action needs a modal rather than the
   *  checkbox, so it gets one here instead of a second fire() path at the call
   *  site — nothing that decides WHETHER a restore runs may live twice. */
  confirmMessage?: string;
  /** Offer the "recreate but leave stopped" checkbox. Default true;
   *  Recovery passes false. */
  showLeaveStopped?: boolean;
  /** Force leaveStopped on regardless of the checkbox — Recovery restores every
   *  target left-stopped, then you start them from the tabs. Default false. */
  forceLeaveStopped?: boolean;
  /** Show the "another op is running" phrase beside a blocked button. Default
   *  true; Recovery passes false. */
  showBusyHint?: boolean;
  /** Forwarded to RestoreProgress — gates the started / bgHint lines. Default
   *  true; Recovery passes false. */
  showStartedHint?: boolean;
  /** Button label when idle. Default t("snapshots.restore"). In `iconBadge`
   *  mode the glyph replaces this text on screen and it becomes the badge's
   *  hover tooltip + accessible name instead — it is never dropped. */
  label?: string;
  /** Render the trigger as this app's square 32px icon badge (IconRestore
   *  glyph, `tip` carrying `label`) pushed flush to the row's far edge, instead
   *  of a text button sitting at the row's start.
   *
   *  jdp, live review: "Card 5: die ganzen Wiederherstellen-Buttons sollen
   *  quadratische Badges mit Glyphen sein und ganz rechts platziert sein." The
   *  recipe is the app's standard one, copied from the already-converted
   *  RestorePanel/VMs/Flash/Config row actions rather than re-derived:
   *  shape="square" size="icon" (32px, Badge.tsx's ONE square-icon-badge
   *  stage), tone="active", and NO hueIndex — every list row that renders one
   *  of these already carries `.glim-hue` with its own rainbow position, so the
   *  custom-property cascade paints the badge in that row's colour. Passing a
   *  hueIndex here would override the row and break the sequence.
   *
   *  `ms-auto`, not `ml-auto`: under dir="rtl" the row's far edge is its left
   *  one and the badge has to follow it. Same idiom, and the same resulting
   *  right edge, as the "Restore all" button an earlier round pushed to the far
   *  edge of this same card.
   *
   *  Left OFF for the two call sites that submit a restore FORM (RestorePanel's
   *  and VMs' expanded panels, where this control sits under a destination
   *  picker and a confirm): those are not per-item row actions, and both are
   *  already reached through an icon badge of their own. */
  iconBadge?: boolean;
  /** Row content rendered at the START of the trigger's own flex row. Lets a
   *  list row put its name/timestamp on the SAME line as an `iconBadge`
   *  trigger, which is what "flush right in its row" requires — without this
   *  the badge lands on a line of its own under the row it belongs to. */
  leading?: ReactNode;
  t: T;
}

export function RestoreAction({
  domain,
  name,
  displayName,
  snapshotId,
  source,
  otherActive,
  successMessage,
  requireConfirm = true,
  confirmMessage,
  showLeaveStopped = true,
  forceLeaveStopped = false,
  showBusyHint = true,
  showStartedHint = true,
  label,
  iconBadge = false,
  leading,
  t,
}: RestoreActionProps) {
  const [confirmed, setConfirmed] = useState(false);
  const { confirm, confirmDialog } = useConfirm();
  // leaveStopped overrides the captured run-state so an in-place restore
  // recreates the target without starting it (rebuild a stack member by member).
  const [leaveStopped, setLeaveStopped] = useState(false);

  const progressKey = `${domain}:${name}`;
  // The SAME ref instance flows to useBackupWatch AND (via RestoreProgress) to
  // RestoreCancelButton — see the header note. Never split it.
  const cancelledRef = useRef(false);
  const { state, fire, isPending } = useBackupWatch({
    progressKey,
    kind: "restore",
    matchRun: (r) => r.domain === domain && r.target === name,
    cancelledRef,
    start: () =>
      domain === "container"
        ? restore(name, snapshotId, true, source, forceLeaveStopped || leaveStopped)
        : restoreVM(name, snapshotId, true, source, forceLeaveStopped || leaveStopped),
  });
  const prog = useProgress()[progressKey];
  // Busy-guard: block a new restore while any OTHER backup/restore/replication
  // runs (this item's own in-flight op is covered by isPending, never blocked).
  const blockedByOther = otherActive.active && !isPending;
  const done = state.phase === "success";

  async function handleRestore() {
    if (requireConfirm && !confirmed) return;
    // The modal is the row action's stand-in for the checkbox, so it gates the
    // SAME single fire() rather than adding a second path to it.
    if (confirmMessage && !(await confirm(confirmMessage))) return;
    void fire();
  }

  // ONE disabled expression and ONE click handler for both trigger shapes
  // below — the whole point of `iconBadge` is that a row action and a form
  // submit reach the identical fire()/useBackupWatch cycle, so nothing that
  // decides WHETHER a restore runs may be written twice.
  const triggerDisabled = (requireConfirm && !confirmed) || isPending || blockedByOther || done;
  // Rendered once, placed differently: in `iconBadge` mode the badge is the
  // row's last child (that is what `ms-auto` pushes to the far edge), so the
  // busy phrase has to come BEFORE it rather than trailing it.
  const busyHint =
    showBusyHint && blockedByOther ? (
      <span className="text-caption text-carbon-textMuted shrink-0">{t(busyPhraseKey(otherActive.phase))}</span>
    ) : null;

  // The caller's own `label` wins when it passes one, exactly as before; it is
  // a per-site NAME (e.g. "Restore this snapshot"), not a state, so it is safe
  // as the width-bearing label. The spinner is now the component's own `busy`
  // rather than a hand-rolled conditional child (#178, [201]).
  const trigger = iconBadge ? (
    <Button
      label={label ?? t("snapshots.restore")}
      labelKey="snapshots.restore"
      glyph={<IconRestore />}
      tone="accent"
      onClick={() => void handleRestore()}
      disabled={triggerDisabled}
      busy={isPending}
      className="ms-auto shrink-0"
    />
  ) : (
    <Button
      label={t("common.restoring")}
      labelKey="common.restoring"
      tone="accent"
      onClick={() => void handleRestore()}
      disabled={triggerDisabled}
      busy={isPending}
      className="shrink-0"
    />
  );

  return (
    <div className="flex flex-col gap-2">
      {confirmDialog}
      <div className="flex items-center gap-3 flex-wrap">
        {leading}
        {requireConfirm && (
          <Toggle
            checked={confirmed}
            onChange={setConfirmed}
            disabled={isPending || done}
            label={t("common.confirm")}
            className="shrink-0"
          />
        )}
        {iconBadge ? (
          <>
            {busyHint}
            {trigger}
          </>
        ) : (
          <>
            {trigger}
            {busyHint}
          </>
        )}
      </div>
      {/* Leave stopped: recreate/restore but don't start (rebuild a stack in order). */}
      {showLeaveStopped && (
        <Toggle
          checked={leaveStopped}
          onChange={setLeaveStopped}
          disabled={isPending || done}
          label={t("restore.leaveStopped")}
        />
      )}
      <RestoreProgress
        state={state}
        isPending={isPending}
        prog={prog}
        cancelKey={progressKey}
        inPlace={true}
        name={displayName ?? name}
        cancelledRef={cancelledRef}
        successMessage={successMessage}
        showStartedHint={showStartedHint}
        t={t}
      />
    </div>
  );
}
