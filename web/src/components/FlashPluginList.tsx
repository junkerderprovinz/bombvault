import { useEffect, useRef, useState } from "react";
import { listFlashPlugins, restoreFlashPlugin, type FlashPlugin } from "../lib/api";
import { useBackupWatch } from "../lib/backupWatch";
import { humanBytes } from "../lib/forecast";
import { tLtr } from "../lib/ltrFragments";
import type { useT } from "../lib/i18n";
import { useToast } from "../lib/toast";
import { useConfirm } from "../lib/useConfirm";
import { Button } from "./Button";
import { InfoBubble } from "./InfoBubble";

type T = ReturnType<typeof useT>["t"];

function PluginRow({ plugin, snapshotId, source, t }: { plugin: FlashPlugin; snapshotId: string; source: string; t: T }) {
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [shake, setShake] = useState(0);
  const { state, fire, isPending } = useBackupWatch({
    progressKey: "flash",
    kind: "restore",
    start: () => restoreFlashPlugin(snapshotId, plugin.name, source),
    matchRun: (r) => r.domain === "flash",
  });
  const seen = useRef(state.phase);

  useEffect(() => {
    if (state.phase === seen.current) return;
    seen.current = state.phase;
    if (state.phase === "success") {
      push(tLtr(t, "flash.pluginRestored").split("{name}").join(plugin.name), "success");
    } else if (state.phase === "error") {
      push(state.message, "fail");
      setShake((n) => n + 1);
    }
  }, [state, push, t, plugin.name]);

  async function handleRestore() {
    const ok = await confirm(t("flash.pluginRestoreConfirm").split("{name}").join(plugin.name), {
      confirmKey: "flash.pluginRestore",
    });
    if (ok) void fire();
  }

  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1 py-1 text-xs">
      {/* On a phone the name keeps a line of its own, so the row's figures
          and button cannot squeeze it to nothing. */}
      <span dir="ltr" className="min-w-0 flex-1 basis-40 truncate text-start font-mono text-carbon-text">{plugin.name}</span>
      <div className="ms-auto flex items-center gap-3">
        <span dir="ltr" className="text-carbon-textSub">{plugin.version}</span>
        <span className="w-16 text-end text-carbon-textSub">{humanBytes(plugin.size)}</span>
        <Button
          key={shake}
          label={t("flash.pluginRestore")}
          labelKey="flash.pluginRestore"
          tone="accent"
          onClick={() => void handleRestore()}
          disabled={isPending}
          busy={isPending}
          className={`shrink-0${shake ? " glim-shake" : ""}`}
        />
      </div>
      {confirmDialog}
    </div>
  );
}

// FlashPluginList shows the plugins one flash backup holds, each with its own
// restore into the live flash.
export function FlashPluginList({ snapshotId, source, t }: { snapshotId: string; source: string; t: T }) {
  const [plugins, setPlugins] = useState<FlashPlugin[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let current = true;
    listFlashPlugins(snapshotId, source)
      .then((r) => {
        if (!current) return;
        if (r.ok) setPlugins(r.plugins ?? []);
        else setError(r.error ?? t("flash.pluginsFailed"));
      })
      .catch((err: unknown) => {
        if (current) setError(err instanceof Error ? err.message : t("flash.pluginsFailed"));
      });
    return () => {
      current = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [snapshotId, source]);

  return (
    <div className="ms-4 flex flex-col rounded-control bg-carbon-surface2 px-3 py-2">
      <p className="flex items-center gap-1 text-xs text-carbon-textSub">
        {t("flash.pluginsTitle")}
        <InfoBubble tip={t("flash.pluginsHint")} />
      </p>
      {error && <p className="text-xs text-statusFail">{error}</p>}
      {!error && plugins === null && <p className="text-xs text-carbon-textMuted">{t("dashboard.checking")}</p>}
      {!error && plugins?.length === 0 && <p className="text-xs text-carbon-textMuted">{t("flash.pluginsNone")}</p>}
      {plugins?.map((p) => (
        <PluginRow key={p.name} plugin={p} snapshotId={snapshotId} source={source} t={t} />
      ))}
    </div>
  );
}
