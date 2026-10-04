import { useState } from "react";
import { copyText } from "../lib/clipboard";
import { useT } from "../lib/i18n";
import { useToast } from "../lib/toast";
import { Button } from "./Button";

/**
 * A monospace block with a copy button. copyText() falls back to execCommand,
 * because the Clipboard API alone does nothing on a plain HTTP origin.
 */
export function CopyBlock({ text }: { text: string }) {
  const { t } = useT();
  const { push } = useToast();
  const [shake, setShake] = useState(0);
  async function copy() {
    if (await copyText(text)) {
      push(t("common.copied"), "success");
    } else {
      push(t("vm.ssh.copyFailed"), "fail");
      setShake((n) => n + 1);
    }
  }
  return (
    <div className="flex items-start gap-2">
      <pre
        dir="ltr"
        className="flex-1 overflow-x-auto rounded-control bg-carbon-background p-2 text-caption leading-snug text-carbon-text whitespace-pre text-start"
      >
        {text}
      </pre>
      <Button
        key={shake}
        label={t("common.copy")}
        labelKey="common.copy"
        tone="neutral"
        onClick={() => void copy()}
        className={`shrink-0${shake ? " glim-shake" : ""}`}
      />
    </div>
  );
}
