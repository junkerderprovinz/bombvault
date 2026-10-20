import { useState } from "react";
import { setRclone } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { Button } from "../Button";
import { IconBack, IconSave } from "../glyphs";
import { IconClose } from "../navGlyphs";
import { Rows, SettingRow } from "./rows";
import { StepCard } from "./StepCard";

/**
 * OwnRcloneWindow is the choice in Add storage location for a service no tile
 * covers: remotes pasted from an rclone.conf, saved into BombVault's own one.
 * A saved remote is then used like any other address.
 */
export function OwnRcloneWindow({
  remotes,
  onSaved,
  onBack,
  onClose,
}: {
  /** The remotes BombVault's rclone.conf already holds. */
  remotes: string[];
  onSaved: () => void;
  onBack: () => void;
  onClose: () => void;
}) {
  const { t } = useT();
  const { push } = useToast();
  const [conf, setConf] = useState("");
  const [saving, setSaving] = useState(false);
  const [shake, setShake] = useState(0);

  async function save() {
    setSaving(true);
    try {
      const r = await setRclone(conf);
      if (r.ok) {
        setConf("");
        onSaved();
        push(t("settings.saved"), "success");
      } else {
        push(r.error ?? t("settings.error"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      setShake((n) => n + 1);
    } finally {
      setSaving(false);
    }
  }

  return (
    <InstanceWindow
      title={t("storage.add.title")}
      hint={t("storage.listHint")}
      onClose={onClose}
      footer={
        <>
          <Button label={t("common.cancel")} labelKey="common.cancel" glyph={<IconClose />} tone="neutral" onClick={onClose} />
          <Button label={t("common.back")} labelKey="common.back" glyph={<IconBack />} tone="neutral" onClick={onBack} />
          <Button
            key={shake}
            label={t("rclone.save")}
            labelKey="rclone.save"
            glyph={<IconSave />}
            tone="accent"
            busy={saving}
            disabled={saving || conf.trim() === ""}
            onClick={() => void save()}
            className={shake ? "glim-shake" : ""}
          />
        </>
      }
    >
      <div className="flex flex-col gap-4">
        <StepCard title={t("storage.add.ownRclone")} tip={t("storage.add.ownRcloneTip")}>
          <textarea
            value={conf}
            onChange={(e) => setConf(e.target.value)}
            spellCheck={false}
            rows={8}
            aria-label={t("rclone.paste")}
            placeholder={"[b2]\ntype = b2\naccount = ...\nkey = ..."}
            dir="ltr"
            className="rounded-control bg-carbon-surface text-carbon-text text-xs font-mono px-3 py-2 glim-field-focus text-start"
          />
        </StepCard>
        {remotes.length > 0 && (
          <StepCard title={t("rclone.configured")} tip={t("storage.add.ownRcloneUse")}>
            <Rows as="ul">
              {remotes.map((remote) => (
                <SettingRow
                  key={remote}
                  as="li"
                  label={remote}
                  name={
                    <span dir="ltr" className="min-w-0 break-all text-start font-mono">
                      {remote}
                    </span>
                  }
                />
              ))}
            </Rows>
          </StepCard>
        )}
      </div>
    </InstanceWindow>
  );
}
