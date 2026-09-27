import { useCallback, useEffect, useState } from "react";
import { Button } from "../Button";
import { InfoBubble } from "../InfoBubble";
import { RcloneRemoteForm } from "../RcloneRemoteForm";
import { getRclone, setRclone } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";

/** RcloneConfig is BombVault's rclone config inside the rclone form. It saves
 *  on its own and at once, because the config serves every rclone place, not
 *  only the one this window may add. The SMB and WebDAV form comes first: the
 *  paste box covers any of rclone's backends, the form the two asked for most,
 *  which nobody should have to write an INI section for. */
export function RcloneConfig({ onRemotes }: { onRemotes: (remotes: string[]) => void }) {
  const { t } = useT();
  const { push } = useToast();
  const [conf, setConf] = useState("");
  const [saving, setSaving] = useState(false);
  const [shake, setShake] = useState(0);

  const load = useCallback(async () => {
    try {
      const res = await getRclone();
      onRemotes(res.ok ? (res.remotes ?? []) : []);
    } catch {
      onRemotes([]);
    }
  }, [onRemotes]);

  useEffect(() => {
    void load();
  }, [load]);

  async function save() {
    setSaving(true);
    try {
      const res = await setRclone(conf);
      if (res.ok) {
        setConf("");
        push(t("places.rclone.saved"), "success");
        await load();
        return;
      }
      push(res.error ?? t("common.actionFailed"), "fail");
      setShake((n) => n + 1);
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.actionFailed"), "fail");
      setShake((n) => n + 1);
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-2">
        <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
          {t("rcloneRemote.heading")}
          <InfoBubble tip={t("rcloneRemote.hint")} />
        </span>
        <RcloneRemoteForm t={t} onAdded={() => void load()} />
      </div>
      <div className="flex flex-col gap-1.5">
        <span className="flex items-center gap-1 text-xs text-carbon-textSub">
          <label htmlFor="place-rclone-config">{t("places.rclone.config")}</label>
          <InfoBubble tip={t("places.rclone.configHint")} />
        </span>
        <textarea
          id="place-rclone-config"
          value={conf}
          onChange={(e) => setConf(e.target.value)}
          spellCheck={false}
          rows={5}
          placeholder={"[b2]\ntype = b2\naccount = ...\nkey = ..."}
          dir="ltr"
          className="rounded-control bg-carbon-surface2 px-3 py-2 font-mono text-xs text-carbon-text text-start glim-field-focus"
        />
        <Button
          key={shake}
          label={t("places.rclone.save")}
          labelKey="places.rclone.save"
          tone="neutral"
          busy={saving}
          disabled={saving || conf.trim() === ""}
          onClick={() => void save()}
          className={`self-start${shake ? " glim-shake" : ""}`}
        />
      </div>
    </div>
  );
}
