import { useId, useState } from "react";
import type { SaveState } from "./shared";
import { Button } from "../../components/Button";
import { IconEye, IconEyeOff } from "../../components/glyphs";
import { InfoBubble } from "../../components/InfoBubble";
import { RcloneRemoteForm } from "../../components/RcloneRemoteForm";
import { Rows, SettingRow } from "../../components/storage/rows";
import { Card } from "../settings/shared";
import { setRclone } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { useRcloneRemotes } from "../../lib/useRcloneRemotes";

export function RcloneCard({
  t,
  hueIndex,
  nested,
}: {
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
  /** Passed straight through to Card, see its own `nested` doc. Set by
   *  Recovery's step 3, which renders this Card inside its own step card. */
  nested?: boolean;
}) {
  const { push } = useToast();
  const uid = useId();
  const { remotes, refresh } = useRcloneRemotes();
  const [conf, setConf] = useState("");
  const [pasting, setPasting] = useState(false);
  const [state, setState] = useState<SaveState>("idle");
  // A failed save shakes the Save button, as every failed action does.
  const [shake, setShake] = useState(0);

  async function handleSave() {
    setState("saving");
    try {
      const r = await setRclone(conf);
      if (r.ok) {
        setState("idle");
        setConf("");
        refresh();
        push(t("settings.saved"), "success");
      } else {
        setState("idle");
        push(r.error ?? t("settings.error"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      setState("idle");
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      setShake((n) => n + 1);
    }
  }

  return (
    <Card title={t("rclone.title")} hint={`${t("rclone.hint")} ${t("rclone.pathHint")}`} hueIndex={hueIndex} nested={nested}>
      {/* The form comes first: it covers the two backends asked for most, and
          a paste box on top taught every new user that a config file is the
          normal way in. */}
      <div className="flex flex-col gap-2">
        <div className="flex items-center gap-1.5 text-xs font-semibold text-carbon-textSub uppercase tracking-widest">
          {t("rcloneRemote.heading")}
          <InfoBubble tip={t("rcloneRemote.hint")} />
        </div>
        <RcloneRemoteForm t={t} onAdded={() => refresh()} />
      </div>

      <Rows>
        {remotes.length > 0 && (
          <SettingRow label={t("rclone.configured")}>
            <span dir="ltr" className="font-mono text-sm text-carbon-text text-start">
              {remotes.join(", ")}
            </span>
          </SettingRow>
        )}
        <SettingRow label={t("rclone.paste")} newId="settings-rclone-compact">
          <Button
            label={t(pasting ? "dashboard.hideCard" : "dashboard.showCard")}
            labelKey={pasting ? "dashboard.hideCard" : "dashboard.showCard"}
            glyph={pasting ? <IconEyeOff /> : <IconEye />}
            ariaExpanded={pasting}
            ariaControls={`${uid}-paste`}
            onClick={() => setPasting((was) => !was)}
          />
        </SettingRow>
      </Rows>
      {pasting && (
        <div id={`${uid}-paste`} className="flex flex-col gap-3">
          <textarea
            value={conf}
            onChange={(e) => setConf(e.target.value)}
            spellCheck={false}
            rows={6}
            aria-label={t("rclone.paste")}
            placeholder={"[b2]\ntype = b2\naccount = ...\nkey = ..."}
            dir="ltr"
            className="rounded-control bg-carbon-surface2 text-carbon-text text-xs font-mono px-3 py-2 glim-field-focus text-start"
          />
          <Button
            key={shake || 0}
            label={t("rclone.save")}
            labelKey="rclone.save"
            tone="accent"
            onClick={() => void handleSave()}
            disabled={state === "saving" || conf.trim() === ""}
            busy={state === "saving"}
            title={state === "saving" ? t("auth.saving") : undefined}
            className={shake ? "glim-shake self-end" : "self-end"}
          />
        </div>
      )}
    </Card>
  );
}
