// FleetSettingsCard manages this instance's identity in the Fleet view: the
// display name reported to peers, and the status token (GET /api/fleet/status)
// that lets other instances poll this one. The token is a show-once secret,
// like the widget token.
import { Button } from "../../components/Button";
import { RevealInput } from "../../components/RevealInput";
import { IconClose } from "../../components/Sidebar";
// Neither Sidebar nor navGlyphs exports IconRefresh.
import { IconRefresh } from "../../components/glyphs";
import { Settings, disableFleetToken, generateFleetToken } from "../../lib/api";
import { copyText } from "../../lib/clipboard";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { useReveal } from "../../lib/useReveal";
import { Card, type SaveState } from "./shared";
import { useRef, useState } from "react";

export function FleetSettingsCard({
  t,
  settings,
  setSettings,
  save,
  tokenSet,
  onTokenSet,
  hueIndex,
}: {
  t: ReturnType<typeof useT>["t"];
  settings: Settings;
  setSettings: React.Dispatch<React.SetStateAction<Settings | null>>;
  save: (
    patch: Partial<Settings>,
    setSaveState: (s: SaveState) => void,
    setSaveError: (e: string | null) => void
  ) => Promise<boolean>;
  tokenSet: boolean;
  onTokenSet: (set: boolean) => void;
  hueIndex?: number;
}) {
  const { push } = useToast();
  // save() reports the outcome in a toast, so only the setters are used.
  const [, setNameSaveState] = useState<SaveState>("idle");
  const [, setNameSaveError] = useState<string | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  // One shake key per action. Generate and Regenerate share "generate", since
  // only one of them is on screen at a time.
  const [shake, setShake] = useState<Record<string, number>>({});
  function bumpShake(key: string) {
    setShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
  }
  const reveal = useReveal();
  // The instance name saves itself after a pause in typing. Only the save
  // prop crosses over from the settings page, so the card keeps its own
  // debounce.
  const debounceTimers = useRef<Record<string, ReturnType<typeof setTimeout>>>({});
  function debounced(key: string, run: () => void) {
    const existing = debounceTimers.current[key];
    if (existing) clearTimeout(existing);
    debounceTimers.current[key] = setTimeout(run, 800);
  }

  async function handleGenerate() {
    setBusy(true);
    try {
      const r = await generateFleetToken();
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
      const r = await disableFleetToken();
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

  // copyText falls back to execCommand outside a secure context.
  async function handleCopy() {
    if (!token) return;
    if (await copyText(token)) {
      push(t("common.copied"), "success");
    } else {
      push(t("vm.ssh.copyFailed"), "fail");
    }
  }

  const hueOn = hueIndex !== undefined;

  return (
    <Card title={t("settings.fleet")} hint={t("settings.fleetHint")} hueIndex={hueIndex}>
      <div className="flex flex-col gap-1.5">
        <label className="text-xs text-carbon-textSub">{t("settings.instanceName")}</label>
        <div className="flex items-center gap-2">
          <input
            type="text"
            value={settings.instanceName}
            onChange={(e) => {
              const v = e.target.value;
              setSettings((prev) => (prev ? { ...prev, instanceName: v } : prev));
              debounced("instanceName", () => void save({ instanceName: v }, setNameSaveState, setNameSaveError));
            }}
            spellCheck={false}
            autoComplete="off"
            placeholder="tower"
            className="flex-1 min-w-0 rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus"
          />
        </div>
      </div>

      <ul className="list-disc ps-5 text-xs text-carbon-textSub flex flex-col gap-1">
        <li>{t("settings.fleetHow")}</li>
        <li>{t("settings.fleetAccess")}</li>
      </ul>

      {tokenSet ? (
        <div className="flex flex-col gap-1.5">
          <span className="text-xs text-carbon-textSub">{t("settings.fleetToken")}</span>
          {/* Actions on their own line below the field, per the reveal-eye
              rule. */}
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
              label={t("settings.fleetRegenerate")}
              labelKey="settings.fleetRegenerate"
              // Not IconSync: its dense ring paints 1.7 times the area of the
              // thin cross beside it and looks bigger at the same size.
              glyph={<IconRefresh />}
              // Regenerating is what this row is for. It never competes with
              // the generate button, which shows only while there is no token.
              tone="accent"
              onClick={() => void handleGenerate()}
              disabled={busy}
              // No text colour: the accent tone brings its own ink.
              className={`shrink-0 rounded-pill px-3 py-2 text-xs disabled:opacity-50${
                shake.generate ? " glim-shake" : ""
              }${hueOn ? " glim-hue" : ""}`}
              hueIndex={hueIndex}
            />
            <Button
              label={t("settings.fleetDisable")}
              labelKey="settings.fleetDisable"
              // Disabling revokes access at once, so it should not look like
              // the button beside it that hands out a new key.
              tone="danger"
              // The same cross as every other dismissal.
              glyph={<IconClose />}
              onClick={() => void handleDisable()}
              disabled={busy}
              // No text colour: the danger tone brings its own ink.
              className={`shrink-0 rounded-pill px-3 py-2 text-xs disabled:opacity-50${
                shake.disable ? " glim-shake" : ""
              }${hueOn ? " glim-hue" : ""}`}
              hueIndex={hueIndex}
            />
          </div>
        </div>
      ) : (
        <Button
          label={t("settings.fleetGenerate")}
          labelKey="settings.fleetGenerate"
          tone="accent"
          onClick={() => void handleGenerate()}
          disabled={busy}
          className={`self-start rounded-pill bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${
            shake.generate ? " glim-shake" : ""
          }${hueOn ? " glim-hue" : ""}`}
          hueIndex={hueIndex}
        />
      )}

      {tokenSet && !token && (
        <p className="text-xs text-carbon-textMuted">{t("settings.fleetTokenOnce")}</p>
      )}
      {token && (
        <div className="flex flex-col gap-1">
          <span className="text-xs text-carbon-textSub">{t("settings.fleetTokenPasteHint")}</span>
          <div className="flex items-start gap-2">
            <code className="flex-1 break-all rounded-control bg-carbon-surface2 p-2 text-xs text-carbon-text">
              {token}
            </code>
            <Button
              label={t("common.copy")}
              labelKey="common.copy"
              tone="accent"
              onClick={() => void handleCopy()}
              className={`shrink-0 rounded-pill bg-accent px-3 py-2 text-xs font-medium text-accentContrast${hueOn ? " glim-hue" : ""}`}
              hueIndex={hueIndex}
            />
          </div>
          <p className="text-caption text-carbon-textMuted">
            {t("settings.fleetUrlHint").replace("{url}", window.location.origin)}
          </p>
        </div>
      )}
    </Card>
  );
}
