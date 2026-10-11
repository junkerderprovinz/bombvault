import { Badge } from "../../components/Badge";
import { Button } from "../../components/Button";
import { Note } from "../../components/entry/Note";
import { IconTipButton } from "../../components/IconTipButton";
import { InfoBubble } from "../../components/InfoBubble";
import { IconCopy, IconTrash } from "../../components/navGlyphs";
import { SettingRow } from "../../components/storage/rows";
import { disableWidgetToken, generateWidgetToken, getDashboardPlugin, installDashboardPlugin, removeDashboardPlugin } from "../../lib/api";
import { hueVars } from "../../lib/appearance";
import { copyText } from "../../lib/clipboard";
import { useT } from "../../lib/i18n";
import { useConfirm } from "../../lib/useConfirm";
import { useToast } from "../../lib/toast";
import { PageSentence } from "./SidebarTile";
import { Card, SubCard } from "./shared";
import { type CSSProperties, useEffect, useState } from "react";

const DASH_PLUGIN_PLG_URL =
  "https://raw.githubusercontent.com/junkerderprovinz/bombvault-widget/main/plugin/bombvaultwidget.plg";

const DASH_PLUGIN_REPO_URL = "https://github.com/junkerderprovinz/bombvault-widget";

type T = ReturnType<typeof useT>["t"];

type DashPluginStatus =
  | { kind: "loading" }
  | { kind: "noSsh" }
  | { kind: "absent" }
  | { kind: "installed"; version: string }
  // The status check itself failed; install and remove failures go to runErr.
  | { kind: "error"; message: string };

/** useDashPlugin is the state of the Unraid dashboard plugin and the calls
 *  that install and remove it. */
function useDashPlugin(t: T) {
  const { push } = useToast();
  // Whether the tile is installed is a lasting fact, so it is shown inline
  // rather than in a toast.
  const [status, setStatus] = useState<DashPluginStatus>({ kind: "loading" });
  const [busy, setBusy] = useState<"idle" | "install" | "remove">("idle");
  // A failed install or remove is kept apart from status: an "error" status
  // would replace the controls in the same render, so the button the shake
  // targets would never paint.
  const [runErr, setRunErr] = useState<{ message: string; output?: string } | null>(null);
  const [shake, setShake] = useState<Record<string, number>>({});

  function refresh() {
    getDashboardPlugin()
      .then((r) => {
        if (!r.ok) {
          setStatus({ kind: "error", message: r.error ?? t("settings.error") });
        } else if (!r.sshConfigured) {
          setStatus({ kind: "noSsh" });
        } else if (r.installed) {
          setStatus({ kind: "installed", version: r.version ?? "" });
        } else {
          setStatus({ kind: "absent" });
        }
      })
      .catch((err) => {
        setStatus({ kind: "error", message: err instanceof Error ? err.message : t("settings.error") });
      });
  }

  // The status is checked once, when the card mounts.
  useEffect(refresh, []); // eslint-disable-line react-hooks/exhaustive-deps

  async function run(op: "install" | "remove") {
    setBusy(op);
    setRunErr(null);
    try {
      const r = await (op === "install" ? installDashboardPlugin() : removeDashboardPlugin());
      if (r.ok) {
        push(op === "install" ? t("settings.dashTileInstallOk") : t("settings.dashTileRemoveOk"), "success");
        refresh();
      } else {
        const message = r.error ?? t("settings.error");
        setRunErr({ message, output: r.output });
        push(message, "fail");
        setShake((sh) => ({ ...sh, [op]: (sh[op] ?? 0) + 1 }));
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : t("settings.error");
      setRunErr({ message });
      push(message, "fail");
      setShake((sh) => ({ ...sh, [op]: (sh[op] ?? 0) + 1 }));
    } finally {
      setBusy("idle");
    }
  }

  async function copyUrl() {
    if (await copyText(DASH_PLUGIN_PLG_URL)) push(t("common.copied"), "success");
    else push(t("vm.ssh.copyFailed"), "fail");
  }

  function retry() {
    setStatus({ kind: "loading" });
    refresh();
  }

  return { status, busy, runErr, shake, run, copyUrl, retry };
}

function RunOutput({ output }: { output?: string }) {
  if (!output) return null;
  return (
    <pre className="overflow-x-auto whitespace-pre-wrap rounded-control bg-carbon-background p-2 text-caption leading-snug text-carbon-text">
      {output}
    </pre>
  );
}

function PluginUrl({ t, onCopy }: { t: T; onCopy: () => void }) {
  return (
    <div className="flex items-start gap-2 max-md:flex-col">
      <code dir="ltr" className="flex-1 break-all rounded-control bg-carbon-surface2 p-2 text-start text-xs text-carbon-text">
        {DASH_PLUGIN_PLG_URL}
      </code>
      <Button label={t("common.copy")} labelKey="common.copy" glyph={<IconCopy />} onClick={onCopy} />
    </div>
  );
}

/** UnraidTileSection is the plugin's state with its install and remove
 *  buttons, as the Apps page shows it under the widget's card. */
export function UnraidTileSection({
  t,
  hueIndex,
}: {
  t: T;
  /** The enclosing card's palette position, so these buttons share its hue. */
  hueIndex?: number;
}) {
  const { status, busy, runErr, shake, run, copyUrl, retry } = useDashPlugin(t);

  return (
    <div className="flex flex-col gap-3 border-t border-carbon-border pt-4">
      <h3 className="flex items-center gap-1.5 text-xs font-semibold text-carbon-textSub uppercase tracking-widest">
        {t("settings.dashTile")}
        <InfoBubble tip={t("settings.dashTileHint")} />
      </h3>

      {status.kind === "loading" && <span className="text-xs text-carbon-textMuted">{t("settings.dashTileChecking")}</span>}

      {status.kind === "noSsh" && (
        <div className="flex flex-col gap-2">
          <p className="text-xs text-carbon-textSub">{t("settings.dashTileNoSsh")}</p>
          <PluginUrl t={t} onCopy={() => void copyUrl()} />
          <p className="text-xs text-carbon-textMuted">{t("settings.dashTileCa")}</p>
        </div>
      )}

      {status.kind === "absent" && (
        <div className="flex flex-col gap-2">
          <span className="text-sm text-carbon-text">{t("settings.dashTileNotInstalled")}</span>
          {/* What Install does and where the code lives, before anyone clicks it. */}
          <p className="text-xs text-carbon-textMuted">{t("settings.dashTileConfirm")}</p>
          <Badge
            as="a"
            href={DASH_PLUGIN_REPO_URL}
            target="_blank"
            rel="noopener noreferrer"
            tone="neutral"
            size="small"
            className="self-start"
          >
            {t("settings.dashTileRepo")} →
          </Badge>
          <Button
            key={shake.install || 0}
            label={t("settings.dashTileInstall")}
            labelKey="settings.dashTileInstall"
            tone="accent"
            onClick={() => void run("install")}
            disabled={busy !== "idle"}
            hueIndex={hueIndex}
            busy={busy === "install"}
            title={busy === "install" ? t("settings.dashTileInstalling") : undefined}
            className={`self-start${shake.install ? " glim-shake" : ""}`}
          />
          <RunOutput output={runErr?.output} />
        </div>
      )}

      {status.kind === "installed" && (
        <div className="flex flex-col gap-2">
          <span className="text-sm text-statusOk">
            ✓{" "}
            {status.version
              ? t("settings.dashTileInstalled").replace("{version}", status.version)
              : t("settings.dashTileInstalledNoV")}
          </span>
          <p className="text-xs text-carbon-textMuted">{t("settings.dashTileInstalledHint")}</p>
          <Button
            key={shake.remove || 0}
            label={t("settings.dashTileRemove")}
            labelKey="settings.dashTileRemove"
            tone="accent"
            glyph={<IconTrash />}
            onClick={() => void run("remove")}
            disabled={busy !== "idle"}
            hueIndex={hueIndex}
            busy={busy === "remove"}
            title={busy === "remove" ? t("settings.dashTileRemoving") : undefined}
            className={`self-start${shake.remove ? " glim-shake" : ""}`}
          />
          <RunOutput output={runErr?.output} />
        </div>
      )}

      {status.kind === "error" && (
        <div className="flex flex-col gap-2">
          <span className="text-xs text-statusFail wrap-break-word">✗ {status.message}</span>
          <Button label={t("whatsnew.retry")} labelKey="whatsnew.retry" onClick={retry} className="self-start" />
        </div>
      )}
    </div>
  );
}

/** DashTileRow is the plugin as one row of the widget card. */
function DashTileRow({ t }: { t: T }) {
  const { status, busy, runErr, shake, run, copyUrl, retry } = useDashPlugin(t);
  const { confirm, confirmDialog } = useConfirm();

  async function install() {
    if (await confirm(t("settings.dashTileConfirm"), { confirmKey: "settings.dashTileInstall" })) void run("install");
  }

  const note =
    status.kind === "installed"
      ? status.version
        ? t("settings.dashTileInstalled").replace("{version}", status.version)
        : t("settings.dashTileInstalledNoV")
      : status.kind === "absent"
        ? t("settings.dashTileNotInstalled")
        : status.kind === "error"
          ? status.message
          : undefined;

  return (
    <>
      <SettingRow label={t("settings.dashTile")} hint={t("settings.dashTileHint")} note={note}>
        {status.kind === "loading" && <Badge tone="neutral" size="small">{t("settings.dashTileChecking")}</Badge>}
        {status.kind === "noSsh" && <Badge tone="neutral" size="small">{t("settings.dashTileNoSshShort")}</Badge>}
        {status.kind === "absent" && (
          <Button
            key={shake.install || 0}
            label={t("settings.dashTileInstall")}
            labelKey="settings.dashTileInstall"
            onClick={() => void install()}
            disabled={busy !== "idle"}
            busy={busy === "install"}
            title={busy === "install" ? t("settings.dashTileInstalling") : undefined}
            className={shake.install ? "glim-shake" : ""}
          />
        )}
        {status.kind === "installed" && (
          <Button
            key={shake.remove || 0}
            label={t("settings.dashTileRemove")}
            labelKey="settings.dashTileRemove"
            glyph={<IconTrash />}
            onClick={() => void run("remove")}
            disabled={busy !== "idle"}
            busy={busy === "remove"}
            title={busy === "remove" ? t("settings.dashTileRemoving") : undefined}
            className={shake.remove ? "glim-shake" : ""}
          />
        )}
        {status.kind === "error" && <Button label={t("whatsnew.retry")} labelKey="whatsnew.retry" onClick={retry} />}
      </SettingRow>
      {status.kind === "noSsh" && (
        <div className="flex flex-col gap-2 py-3">
          <Note>{t("settings.dashTileNoSsh")}</Note>
          <PluginUrl t={t} onCopy={() => void copyUrl()} />
        </div>
      )}
      <RunOutput output={runErr?.output} />
      {confirmDialog}
    </>
  );
}

export function DashboardWidgetCard({
  t,
  tokenSet,
  onTokenSet,
  hueIndex,
}: {
  t: T;
  tokenSet: boolean;
  onTokenSet: (set: boolean) => void;
  hueIndex?: number;
}) {
  const { push } = useToast();
  const [token, setToken] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  // One shake key per action. Generate and Regenerate share "generate", since
  // only one of them is on screen at a time.
  const [shake, setShake] = useState<Record<string, number>>({});
  function bumpShake(key: string) {
    setShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
  }

  const widgetUrl = token ? `${window.location.origin}/widget?token=${token}` : null;

  async function handleGenerate() {
    setBusy(true);
    try {
      const r = await generateWidgetToken();
      if (r.ok && r.token) {
        setToken(r.token);
        onTokenSet(true);
      } else {
        push(r.error ?? t("settings.error"), "fail");
        bumpShake("generate");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      bumpShake("generate");
    } finally {
      setBusy(false);
    }
  }

  async function handleDisable() {
    setBusy(true);
    try {
      const r = await disableWidgetToken();
      if (r.ok) {
        setToken(null);
        onTokenSet(false);
      } else {
        push(r.error ?? t("settings.error"), "fail");
        bumpShake("disable");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      bumpShake("disable");
    } finally {
      setBusy(false);
    }
  }

  async function handleCopy() {
    if (!widgetUrl) return;
    if (await copyText(widgetUrl)) push(t("common.copied"), "success");
    else push(t("vm.ssh.copyFailed"), "fail");
  }

  const hueStyle = hueIndex !== undefined ? (hueVars(hueIndex) as CSSProperties) : undefined;

  return (
    <Card title={t("settings.widget")} hint={t("settings.widgetHint")} hueIndex={hueIndex}>
      <div className="flex flex-col divide-y divide-carbon-border/60">
        <SettingRow
          label={t("settings.widgetToken")}
          hint={t("settings.widgetAccess")}
          note={tokenSet ? (token ? undefined : t("settings.widgetUrlOnce")) : t("settings.widgetNoToken")}
        >
          {tokenSet ? (
            <>
              <Button
                key={`g${shake.generate ?? 0}`}
                label={t("settings.widgetRegenerate")}
                labelKey="settings.widgetRegenerate"
                onClick={() => void handleGenerate()}
                disabled={busy}
                className={shake.generate ? "glim-shake" : ""}
              />
              <Button
                key={`d${shake.disable ?? 0}`}
                label={t("settings.widgetDisable")}
                labelKey="settings.widgetDisable"
                onClick={() => void handleDisable()}
                disabled={busy}
                className={shake.disable ? "glim-shake" : ""}
              />
            </>
          ) : (
            <Button
              key={`g${shake.generate ?? 0}`}
              label={t("settings.widgetGenerate")}
              labelKey="settings.widgetGenerate"
              onClick={() => void handleGenerate()}
              disabled={busy}
              className={shake.generate ? "glim-shake" : ""}
            />
          )}
        </SettingRow>
        {widgetUrl && (
          <div className="flex flex-col gap-3 py-3">
            <div className="flex items-start gap-2">
              <code dir="ltr" aria-label={t("settings.widgetUrl")} className="flex-1 break-all rounded-control bg-carbon-surface2 p-2 text-start text-xs text-carbon-text">
                {widgetUrl}
              </code>
              <IconTipButton
                onClick={() => void handleCopy()}
                tip={t("common.copy")}
                className="glim-hue inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-pill bg-accent text-accentContrast transition-colors hover:bg-accentHover"
                style={hueStyle}
              >
                <IconCopy />
              </IconTipButton>
            </div>
            <iframe
              src={widgetUrl}
              title={t("settings.widgetPreview")}
              className="h-[300px] w-full max-w-[560px] rounded-card bg-carbon-surface2"
            />
          </div>
        )}
        <DashTileRow t={t} />
      </div>
      <SubCard title={t("settings.widgetSetup")}>
        <ol className="flex list-decimal flex-col gap-1.5 ps-5 text-sm text-carbon-text">
          <li>{t("settings.widgetStep1")}</li>
          <li>{t("settings.widgetStep2")}</li>
          <li>{t("settings.widgetStep3")}</li>
        </ol>
        <div className="mt-2 flex flex-col gap-1">
          <p className="text-xs text-carbon-textMuted">{t("settings.widgetMissing")}</p>
          <PageSentence text="settings.widgetCa" page="settings.tab.apps" to="/settings/apps" />
        </div>
      </SubCard>
    </Card>
  );
}
