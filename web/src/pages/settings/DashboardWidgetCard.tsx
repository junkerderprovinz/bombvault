import { Button } from "../../components/Button";
import { IconTipButton } from "../../components/IconTipButton";
import { IconCopy } from "../../components/navGlyphs";
import { Rows, SettingRow } from "../../components/storage/rows";
import { disableWidgetToken, generateWidgetToken } from "../../lib/api";
import { hueVars } from "../../lib/appearance";
import { copyText } from "../../lib/clipboard";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { Card, SubCard } from "./shared";
import { type CSSProperties, useState } from "react";

type T = ReturnType<typeof useT>["t"];

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
      <Rows>
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
        <SettingRow label={t("settings.dashTile")} note={t("settings.dashTileHint")} />
      </Rows>
      <SubCard title={t("settings.widgetSetup")}>
        <ol className="flex list-decimal flex-col gap-1.5 ps-5 text-sm text-carbon-text">
          <li>{t("settings.widgetStep1")}</li>
          <li>{t("settings.widgetStep2")}</li>
          <li>{t("settings.widgetStep3")}</li>
        </ol>
      </SubCard>
    </Card>
  );
}
