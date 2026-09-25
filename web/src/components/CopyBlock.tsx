import { useState } from "react";
import { Button } from "./Button";
import { copyText } from "../lib/clipboard";
import type { useT } from "../lib/i18n";
import { useToast } from "../lib/toast";

type T = ReturnType<typeof useT>["t"];

// CopyBlock is a monospace <pre> with a copy button. copyText() is used
// because the Clipboard API alone silently does nothing on a plain HTTP
// origin.
export function CopyBlock({ text, t }: { text: string; t: T }) {
  const { push } = useToast();
  const [shake, setShake] = useState(0);
  async function copy() {
    if (await copyText(text)) {
      push(t("common.copied"), "success");
    } else {
      // Both the Clipboard API and the execCommand fallback failed, which is
      // worth telling even in quiet mode.
      push(t("vm.ssh.copyFailed"), "fail");
      setShake((n) => n + 1);
    }
  }
  return (
    <div className="flex items-start gap-2">
      <pre className="flex-1 overflow-x-auto rounded-control bg-carbon-background p-2 text-caption leading-snug text-carbon-text whitespace-pre">
        {text}
      </pre>
      <Button
        key={shake}
        label={t("common.copy")}
        labelKey="common.copy"
        tone="neutral"
        onClick={() => void copy()}
        className={`shrink-0 rounded-pill px-3 py-2 text-xs text-carbon-text${
          shake ? " glim-shake" : ""
        }`}
      />
    </div>
  );
}
