import { useEffect, useRef, useState } from "react";
import { backupNow } from "../lib/api";
import { useBackupWatch } from "../lib/backupWatch";
import { busyPhraseKey } from "../lib/progress";
import type { useT } from "../lib/i18n";
import { Button } from "./Button";
import { groupStage } from "../lib/controls";
import { IconBackupNow } from "./Sidebar";
import { useToast } from "../lib/toast";

type T = ReturnType<typeof useT>["t"];

interface BackupButtonProps {
  name: string;
  t: T;
  /** Called after a successful backup so the caller can refresh (e.g. last-backup time). */
  onBackedUp?: () => void;
  /** "Something is running" signal (anyActive): disables this backup with a
   *  friendly hint while another op runs — but never for its OWN in-flight
   *  backup (that is isPending, handled below). */
  running?: { active: boolean; phase?: string };
}

// Square icon-only badge (Containers.tsx Task 2, jdp live-review: "Jetzt
// sichern und Export sollen quadratische Badges mit Glyph sein... rechts
// oben in der Ecke"). Was a full-width text button with FOUR different
// permanently-inline states living below it (pending spinner+label,
// success checkmark+snapshot id, a stateless-container "config only" note,
// a red error message, a neutral "skipped" note) — there is no room for any
// of that next to a small square glyph in the row's top-right corner, so
// every TERMINAL state (success/error/skipped) now surfaces as a toast
// instead, matching the "failed action toasts AND shakes its button"
// standing rule this exact file's ExportButton/HooksEditor save() already
// follow. Only the PENDING state stays genuinely inline — swapped for the
// glyph itself (a spinner replacing the icon while running), since a badge
// has no separate space to put a spinner NEXT TO its own glyph.
// `size="icon"` (Badge.tsx, h-8/w-8 = 32px) — the app's ONE square-icon-badge
// size, not a number derived here. This badge used to be 28px, measured
// against its own pre-conversion self, which was correct per-control and
// wrong per-card: sitting in the same Container card as the 32px Lokal/
// Offsite pair and the (then) 24px restore/delete pair, it was one of three
// badge sizes a user saw at once. jdp reported that twice; the fix was to
// delete the per-role sizes entirely. Do not re-measure this button against
// its neighbours and "improve" the number — see Badge.tsx's own "ONE SIZE
// FOR SQUARE ICON BADGES" block for why that reasoning is the defect.
export function BackupButton({ name, t, onBackedUp, running }: BackupButtonProps) {
  // Fire-and-watch: the server runs the backup detached and answers immediately,
  // so we watch the "container:<name>" progress + recorded run for the outcome
  // instead of awaiting (which would die if we back up the proxy the UI runs
  // through). See useBackupWatch.
  const { state, fire, isPending } = useBackupWatch({
    progressKey: `container:${name}`,
    start: () => backupNow(name),
    matchRun: (r) => r.domain === "container" && r.target === name,
    onDone: onBackedUp,
  });
  const blockedByOther = !!running?.active && !isPending;
  const { push } = useToast();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): a
  // failed action toasts AND shakes its button.
  const [shake, setShake] = useState(0);
  // Tracks the last phase already reported, so this effect toasts exactly
  // once per NEW terminal transition — state.phase can only ever start at
  // "idle" (useBackupWatch never begins mid-run), so this never fires on
  // mount, only on a real fire()-driven change.
  const seenPhase = useRef(state.phase);

  useEffect(() => {
    if (state.phase === seenPhase.current) return;
    seenPhase.current = state.phase;
    if (state.phase === "success") {
      // No snapshot id ⇒ a stateless container with no data folders (the
      // definition/template is still captured for recreate) — say so
      // instead of a bare, opaque "Done".
      push(
        state.snapshotId ? `${t("common.done")} · ${state.snapshotId.slice(0, 8)}` : t("backup.configOnly"),
        "success"
      );
    } else if (state.phase === "error") {
      push(state.message, "fail");
      setShake((n) => n + 1);
    } else if (state.phase === "skipped") {
      // Neutral terminal: the container is gone, so the backup was skipped
      // (not failed) — "warn" severity, the same neutral-not-red tone this
      // file's own batch actions already use for a partial/non-failure result.
      push(`↷ ${t("containers.notInstalledTitle")}`, "warn");
    }
  }, [state, push, t]);

  // #178: the button's NAME is stable; only the exceptional states get a
  // tooltip. A label that changed to "Backing up…" would resize the control
  // mid-action, which the width stages exist to prevent.
  const stateTip = isPending
    ? t("common.backingUp")
    : blockedByOther
      ? t(busyPhraseKey(running?.phase))
      : undefined;

  return (
    <Button
      key={shake}
      label={t("containers.backupNow")}
      labelKey="containers.backupNow"
      glyph={<IconBackupNow />}
      tone="accent"
      // Shares a width with the Export button it sits beside in every card
      // (jdp: "die buttons jetzt sichern und Export sollen gleich breit
      // sein"). Both compute it from the SAME two labels rather than one
      // handing a number to the other, so they agree in all 42 languages and
      // keep agreeing when either word is retranslated.
      stage={groupStage([t("containers.backupNow"), t("export.button")])}
      onClick={() => void fire()}
      disabled={isPending || blockedByOther}
      busy={isPending}
      title={stateTip}
      className={shake ? "glim-shake" : ""}
    />
  );
}
