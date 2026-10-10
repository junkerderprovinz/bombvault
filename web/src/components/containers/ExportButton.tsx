import { useState } from "react";
import { exportContainer } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { IconDownload } from "../Sidebar";
import { Button } from "../Button";
import { groupStage } from "../../lib/controls";
import { useToast } from "../../lib/toast";

type T = ReturnType<typeof useT>["t"];

// ExportButton writes a plain, unencrypted tar and XML copy of the container
// (the folders restic backs up plus the Unraid template) into a browsable
// folder next to the repo. The result stays inline under the button: it is the
// destination path or the raw error, which the user may want to read or copy
// after the failure toast has gone.
export function ExportButton({ name, t }: { name: string; t: T }) {
  const [state, setState] = useState<"idle" | "pending" | "done" | "error">("idle");
  const [msg, setMsg] = useState<string | null>(null);
  const { push } = useToast();
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
      <Button
        key={shake}
        label={t("export.button")}
        labelKey="export.button"
        glyph={<IconDownload />}
        tone="accent"
        // Same width as the backup button beside it: both derive it from the
        // same two labels, so they match in every language.
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
