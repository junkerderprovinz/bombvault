import { Badge } from "../../components/Badge";
import { Button } from "../../components/Button";
import { IconTipButton } from "../../components/IconTipButton";
import { InfoBubble } from "../../components/InfoBubble";
import { RevealInput } from "../../components/RevealInput";
import { IconCopy, IconTrash } from "../../components/navGlyphs";
import { disableWidgetToken, generateWidgetToken, getDashboardPlugin, installDashboardPlugin, removeDashboardPlugin } from "../../lib/api";
import { hueVars } from "../../lib/appearance";
import { copyText } from "../../lib/clipboard";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { useReveal } from "../../lib/useReveal";
import { Card } from "./shared";
import { type CSSProperties, useEffect, useState } from "react";

const DASH_PLUGIN_PLG_URL =
  "https://raw.githubusercontent.com/junkerderprovinz/bombvault-widget/main/plugin/bombvaultwidget.plg";

const DASH_PLUGIN_REPO_URL = "https://github.com/junkerderprovinz/bombvault-widget";

type DashPluginStatus =
  | { kind: "loading" }
  | { kind: "noSsh" }
  | { kind: "absent" }
  | { kind: "installed"; version: string }
  // The status check itself failed; install and remove failures go to runErr.
  | { kind: "error"; message: string };

export function UnraidTileSection({
  t,
  hueIndex,
}: {
  t: ReturnType<typeof useT>["t"];
  /** The enclosing card's palette position, so these buttons share its hue. */
  hueIndex?: number;
}) {
  const { push } = useToast();
  const hueOn = hueIndex !== undefined;
  // Whether the tile is installed is a lasting fact, so it is shown inline
  // rather than in a toast.
  const [status, setStatus] = useState<DashPluginStatus>({ kind: "loading" });
  const [busy, setBusy] = useState<"idle" | "install" | "remove">("idle");
  // runErr keeps a failed install or remove apart from status. Setting status
  // to "error" would replace the section with the Retry branch in the same
  // render, so the button the shake targets would never paint. The "error"
  // status is for a failed status check, where the install state is unknown.
  const [runErr, setRunErr] = useState<{ message: string; output?: string } | null>(null);
  const [shake, setShake] = useState<Record<string, number>>({});
  function bumpShake(key: string) {
    setShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
  }

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
        setStatus({
          kind: "error",
          message: err instanceof Error ? err.message : t("settings.error"),
        });
      });
  }

  useEffect(refresh, []); // eslint-disable-line react-hooks/exhaustive-deps -- status check on card mount only

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
        bumpShake(op);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : t("settings.error");
      setRunErr({ message });
      push(message, "fail");
      bumpShake(op);
    } finally {
      setBusy("idle");
    }
  }

  async function handleCopyUrl() {
    if (await copyText(DASH_PLUGIN_PLG_URL)) {
      push(t("common.copied"), "success");
    } else {
      push(t("vm.ssh.copyFailed"), "fail");
    }
  }

  return (
    <div className="flex flex-col gap-3 border-t border-carbon-border pt-4">
      <h3 className="flex items-center gap-1.5 text-xs font-semibold text-carbon-textSub uppercase tracking-widest">
        {t("settings.dashTile")}
        <InfoBubble tip={t("settings.dashTileHint")} />
      </h3>

      {status.kind === "loading" && (
        <span className="text-xs text-carbon-textMuted">{t("settings.dashTileChecking")}</span>
      )}

      {status.kind === "noSsh" && (
        <div className="flex flex-col gap-2">
          <p className="text-xs text-carbon-textSub">{t("settings.dashTileNoSsh")}</p>
          <div className="flex items-start gap-2">
            <code className="flex-1 break-all rounded-control bg-carbon-surface2 p-2 text-xs text-carbon-text">
              {DASH_PLUGIN_PLG_URL}
            </code>
            <Button
              label={t("common.copy")}
              labelKey="common.copy"
              tone="accent"
              onClick={() => void handleCopyUrl()}
              className={`shrink-0 rounded-pill bg-accent px-3 py-2 text-xs font-medium text-accentContrast${hueOn ? " glim-hue" : ""}`}
            />
          </div>
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
          {/* The toast carries the message; command output needs a place that
              outlasts it. */}
          {runErr?.output && (
            <pre className="overflow-x-auto rounded-control bg-carbon-background p-2 text-caption leading-snug text-carbon-text whitespace-pre-wrap">
              {runErr.output}
            </pre>
          )}
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
            // No red of its own: the confirmation dialog guards the removal.
            tone="accent"
            glyph={<IconTrash />}
            onClick={() => void run("remove")}
            disabled={busy !== "idle"}
            hueIndex={hueIndex}
            busy={busy === "remove"}
            title={busy === "remove" ? t("settings.dashTileRemoving") : undefined}
            className={`self-start${shake.remove ? " glim-shake" : ""}`}
          />
          {runErr?.output && (
            <pre className="overflow-x-auto rounded-control bg-carbon-background p-2 text-caption leading-snug text-carbon-text whitespace-pre-wrap">
              {runErr.output}
            </pre>
          )}
        </div>
      )}

      {status.kind === "error" && (
        <div className="flex flex-col gap-2">
          <span className="text-xs text-statusFail wrap-break-word">✗ {status.message}</span>
          <Button
            label={t("whatsnew.retry")}
            labelKey="whatsnew.retry"
            tone="neutral"
            onClick={() => {
              setStatus({ kind: "loading" });
              refresh();
            }}
            className={`self-start rounded-pill px-3 py-2 text-xs text-carbon-text${hueOn ? " glim-hue" : ""}`}
          />
        </div>
      )}
    </div>
  );
}

export function DashboardWidgetCard({
  t,
  tokenSet,
  onTokenSet,
  hueIndex,
}: {
  t: ReturnType<typeof useT>["t"];
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
  const reveal = useReveal();

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
    if (await copyText(widgetUrl)) {
      push(t("common.copied"), "success");
    } else {
      push(t("vm.ssh.copyFailed"), "fail");
    }
  }

  const hueOn = hueIndex !== undefined;
  const hueStyle = hueOn ? (hueVars(hueIndex) as CSSProperties) : undefined;

  return (
    <Card title={t("settings.widget")} hint={t("settings.widgetHint")} hueIndex={hueIndex}>
      <ul className="list-disc ps-5 text-xs text-carbon-textSub flex flex-col gap-1">
        <li>{t("settings.widgetHow")}</li>
        <li>{t("settings.widgetAccess")}</li>
        <li>{t("settings.widgetEnglish")}</li>
      </ul>

      {tokenSet ? (
        <div className="flex flex-col gap-1.5">
          <span className="text-xs text-carbon-textSub">{t("settings.widgetToken")}</span>
          {/* Show-once secret: the field holds only a freshly generated token,
              and a stored one shows the placeholder. The actions sit on their
              own line below the field, per the reveal-eye rule in
              design-language.md. */}
          <RevealInput
            {...reveal}
            readOnly
            value={token ?? ""}
            placeholder={token ? "" : t("cloud.secretSet")}
            wrapperClassName="w-full"
            className="rounded-control bg-carbon-surface2 text-carbon-text text-sm font-mono px-3 py-1.5 glim-field-focus"
          />
          <div className="flex items-center gap-2">
            <Button
              label={t("settings.widgetRegenerate")}
              labelKey="settings.widgetRegenerate"
              tone="neutral"
              onClick={() => void handleGenerate()}
              disabled={busy}
              className={`shrink-0 rounded-pill px-3 py-2 text-xs text-carbon-text disabled:opacity-50${
                shake.generate ? " glim-shake" : ""
              }${hueOn ? " glim-hue" : ""}`}
            />
            <Button
              label={t("settings.widgetDisable")}
              labelKey="settings.widgetDisable"
              tone="neutral"
              onClick={() => void handleDisable()}
              disabled={busy}
              className={`shrink-0 rounded-pill px-3 py-2 text-xs text-carbon-text disabled:opacity-50${
                shake.disable ? " glim-shake" : ""
              }${hueOn ? " glim-hue" : ""}`}
            />
          </div>
        </div>
      ) : (
        <Button
          label={t("settings.widgetGenerate")}
          labelKey="settings.widgetGenerate"
          tone="accent"
          onClick={() => void handleGenerate()}
          disabled={busy}
          className={`self-start rounded-pill bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${
            shake.generate ? " glim-shake" : ""
          }${hueOn ? " glim-hue" : ""}`}
        />
      )}

      {tokenSet && !token && (
        <p className="text-xs text-carbon-textMuted">{t("settings.widgetUrlOnce")}</p>
      )}
      {widgetUrl && (
        <>
          <div className="flex flex-col gap-1">
            <span className="text-xs text-carbon-textSub">{t("settings.widgetUrl")}</span>
            <div className="flex items-start gap-2">
              <code className="flex-1 break-all rounded-control bg-carbon-surface2 p-2 text-xs text-carbon-text">
                {widgetUrl}
              </code>
              {/* h-8 w-8 is Badge's square icon size, written out because this
                  renders IconTipButton directly. The accent fill marks copying
                  the URL as the primary action here. */}
              <IconTipButton
                onClick={() => void handleCopy()}
                tip={t("common.copy")}
                className={`shrink-0 inline-flex items-center justify-center rounded-pill bg-accent h-8 w-8 text-accentContrast hover:opacity-90 transition-opacity${hueOn ? " glim-hue" : ""}`}
                style={hueStyle}
              >
                <IconCopy />
              </IconTipButton>
            </div>
          </div>
          <div className="flex flex-col gap-1">
            <span className="text-xs text-carbon-textSub">{t("settings.widgetPreview")}</span>
            <iframe
              src={widgetUrl}
              title={t("settings.widgetPreview")}
              className="w-full max-w-[560px] h-[300px] rounded-card bg-carbon-surface2"
            />
          </div>
        </>
      )}

      {/* The companion Unraid dashboard tile plugin, installed over SSH. */}
      <UnraidTileSection t={t} hueIndex={hueIndex} />
    </Card>
  );
}
