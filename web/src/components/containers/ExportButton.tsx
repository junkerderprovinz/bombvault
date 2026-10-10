import { useState } from "react";
import { exportContainer } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { IconDownload } from "../Sidebar";
import { Button } from "../Button";
import { groupStage } from "../../lib/controls";
import { useToast } from "../../lib/toast";

type T = ReturnType<typeof useT>["t"];

// Container row

// ExportButton writes a plain, tool-free tar+xml copy of the container (the same
// folders restic backs up, plus the Unraid template) into a browsable folder next
// to the repo — restic stays the engine; this is an extra, unencrypted export.
//
// GlimStone follow-up pass (v8.0.0) audit note: the "done"/"error" result below
// is DELIBERATELY left as inline status, not migrated to a toast — it shows the
// actual destination PATH the export landed at (or, on failure, the raw error),
// neither of which auto-dismissed even before this pass. Like
// SettingsPortabilityCard's export/import banners, this is a reference value the
// user needs to actually copy down or read, not a one-shot "it worked" ping a 4s
// toast would cut off mid-read. Still true after the icon-badge
// conversion below — only the TRIGGER became a square glyph badge (jdp:
// "Export soll ein quadratischer Badge mit Glyph sein... rechts oben in der
// Ecke"); this sticky result column renders in the exact same place right
// below it, unchanged, so the copyable path is still there to read once the
// export finishes.
export function ExportButton({ name, t }: { name: string; t: T }) {
  const [state, setState] = useState<"idle" | "pending" | "done" | "error">("idle");
  const [msg, setMsg] = useState<string | null>(null);
  const { push } = useToast();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): a
  // failed action toasts AND shakes its button, layered ON TOP of this
  // button's own pre-existing sticky inline error (kept deliberately — see
  // this component's header comment: the error is a reference value the user
  // may need to read/copy, not a one-shot ping the toast alone would replace).
  const [shake, setShake] = useState(0);

  async function run() {
    setState("pending");
    setMsg(null);
    try {
      const r = await exportContainer(name);
      if (r.ok) {
        setState("done");
        setMsg(r.path ?? null);
      } else {
        setState("error");
        const message = r.error ?? t("settings.error");
        setMsg(message);
        push(message, "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      setState("error");
      const message = err instanceof Error ? err.message : t("settings.error");
      setMsg(message);
      push(message, "fail");
      setShake((n) => n + 1);
    }
  }

  return (
    <div className="flex flex-col items-end gap-1">
      {/* `size="icon"` — the app's one square-icon-badge size (32px); see
          Badge.tsx's "ONE SIZE FOR SQUARE ICON BADGES" block.
            `tone="active"`, NOT the `tone="neutral"` this badge shipped with:
          neutral resolves to a flat `bg-carbon-surface2` grey that takes no
          colour-engine position at all, which left Export as the single grey
          tile in a Container card whose every other badge (Jetzt sichern,
          Lokal/Offsite, Wiederherstellen, Löschen) is hue-integrated — the
          same "anders eingefärbt" defect jdp reported one badge over, on
          RestorePanel's delete. The standing icon-badge rule is that a square
          icon badge gets colour-engine integration and a tooltip
          automatically; neutral here was an unexamined default, not a
          decision. `active` + icon-only resolves to the solid `bg-accent`/
          `text-accentContrast` pair (Badge's own `isIconOnly && tone==="active"`
          branch) and inherits this row's own rainbow position from the
          ambient `.glim-hue` cascade, exactly like BackupButton beside it —
          no `hueIndex` needed. */}
      <Button
        key={shake}
        label={t("export.button")}
        labelKey="export.button"
        glyph={<IconDownload />}
        tone="accent"
        // Same shared stage as BackupButton beside it — see that file.
        stage={groupStage([t("containers.backupNow"), t("export.button")])}
        onClick={() => void run()}
        disabled={state === "pending"}
        busy={state === "pending"}
        className={shake ? "glim-shake" : ""}
      />
      {state === "done" && msg && (
        <span className="text-xs text-statusOk break-all text-end max-w-[18rem]">
          {t("export.exportedTo")} <span dir="ltr" className="text-start">{msg}</span>
        </span>
      )}
      {state === "error" && msg && (
        <span dir="ltr" className="text-xs text-statusFail wrap-break-word text-start max-w-[18rem]">{msg}</span>
      )}
    </div>
  );
}
