import { Button } from "../../components/Button";
import { SelectField } from "../../components/SelectField";
import { STORAGE_CLASSES } from "../../lib/storageClasses";
import { RevealInput } from "../../components/RevealInput";
import { setCloudCredSets, type CloudCredSet, type CloudCredSetInfo } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { pushSaveWarnings } from "../../lib/placementCodes";
import { credSetsChanged, useCloudCredSets } from "../../lib/useCloudCredSets";
import { useReveal } from "../../lib/useReveal";
import { randomId } from "../../lib/uuid";
import { Card, type SaveState } from "./shared";
import { useState } from "react";

// toDraft blanks the secrets of a stored set. The backend keeps a stored
// secret when the posted one is blank (matched by id), so resending untouched
// sets this way preserves their keys.
function toDraft(s: CloudCredSetInfo): CloudCredSet {
  return { id: s.id, name: s.name, s3KeyId: s.s3KeyId, s3Secret: "", s3Region: s.s3Region, restUser: s.restUser, restPassword: "", s3StorageClass: s.s3StorageClass };
}

// CloudCredSetsCard manages additional named credential sets, so an off-site
// target can use its own S3 or REST credentials instead of the shared set from
// CloudCard, for example two S3 endpoints that need different keys. The list
// is saved as a whole, which is why every save resends every set.
//
// Unlike the rest of the settings page the editor has a Save button: it holds
// a draft that Close discards, and saving on every keystroke would list a
// half-filled set as soon as the first letter of its name was typed.
export function CloudCredSetsCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  const { push } = useToast();
  // Shared with every off-site target's credential picker. This card is the
  // only editor, so it announces changes and reads them back through the same
  // hook, which keeps its rows and the pickers in agreement.
  const sets = useCloudCredSets();
  const [editing, setEditing] = useState<CloudCredSet | null>(null);
  const [state, setState] = useState<SaveState>("idle");
  const [confirmRemove, setConfirmRemove] = useState<string | null>(null);
  const [removingId, setRemovingId] = useState<string | null>(null);
  // A failed action shakes the button that caused it. Removal is keyed per
  // row, since several remove buttons can be on screen at once.
  const [shake, setShake] = useState<Record<string, number>>({});
  function bumpShake(key: string) {
    setShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
  }
  const revealS3Secret = useReveal();
  const revealRestPassword = useReveal();

  function openNew() {
    // Not crypto.randomUUID(): it is undefined outside a secure context, and
    // BombVault supports plain HTTP.
    setEditing({ id: randomId(), name: "", s3KeyId: "", s3Secret: "", s3Region: "", restUser: "", restPassword: "", s3StorageClass: "" });
    setState("idle");
  }
  function openEdit(s: CloudCredSetInfo) {
    setEditing(toDraft(s));
    setState("idle");
  }
  function closeEditor() {
    setEditing(null);
    setState("idle");
  }
  function setField<K extends keyof CloudCredSet>(k: K, v: CloudCredSet[K]) {
    setEditing((p) => (p ? { ...p, [k]: v } : p));
  }

  async function save() {
    if (!editing) return;
    setState("saving");
    const rest = sets.filter((s) => s.id !== editing.id).map(toDraft);
    const next = [...rest, editing];
    try {
      const r = await setCloudCredSets(next);
      if (r.ok) {
        setState("idle");
        closeEditor();
        credSetsChanged();
        push(t("settings.saved"), "success");
        pushSaveWarnings(push, t, r.warnings);
      } else {
        setState("idle");
        push(r.error ?? t("settings.error"), "fail");
        bumpShake("save");
      }
    } catch (err) {
      setState("idle");
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      bumpShake("save");
    }
  }

  async function remove(id: string) {
    setRemovingId(id);
    try {
      const next = sets.filter((s) => s.id !== id).map(toDraft);
      const r = await setCloudCredSets(next);
      if (r.ok) {
        credSetsChanged();
      } else {
        push(r.error ?? t("settings.error"), "fail");
        bumpShake(`remove:${id}`);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      bumpShake(`remove:${id}`);
    } finally {
      setRemovingId(null);
      setConfirmRemove(null);
    }
  }

  const inputCls =
    "rounded-control bg-carbon-surface3 text-carbon-text text-sm font-mono px-3 py-1.5 glim-field-focus-well";
  const fieldCls = "flex flex-col gap-1 text-xs font-mono text-carbon-textSub";

  return (
    <Card title={t("cloud.credSets.title")} hint={t("cloud.credSets.hint")} hueIndex={hueIndex}>
      {sets.length === 0 && !editing && (
        <span className="text-xs text-carbon-textMuted">{t("cloud.credSets.none")}</span>
      )}

      {sets.map((s) => (
        <div key={s.id} className="flex items-start justify-between gap-3 rounded-card bg-carbon-surface2 p-3">
          <div className="flex min-w-0 flex-col gap-1">
            <span className="text-sm text-carbon-text truncate">{s.name}</span>
            <span dir="ltr" className="text-xs text-carbon-textMuted font-mono break-all text-start">
              {s.s3KeyId || s.restUser || "—"}
            </span>
          </div>
          <div className="flex shrink-0 items-start gap-2">
            <Button
              label={t("offsite.targets.edit")}
              labelKey="offsite.targets.edit"
              tone="neutral"
              onClick={() => openEdit(s)}
            />
            {/* A destructive action gets no red of its own in this app. It is
                a text button because the two-click confirm needs a label to
                change, which an icon badge has nowhere to put. */}
            {confirmRemove === s.id ? (
              <Button
                key={shake[`remove:${s.id}`] || 0}
                label={t("offsite.targets.confirmRemove")}
                labelKey="offsite.targets.confirmRemove"
                tone="neutral"
                onClick={() => void remove(s.id)}
                disabled={removingId === s.id}
                busy={removingId === s.id}
                title={removingId === s.id ? t("offsite.targets.removing") : undefined}
                className={shake[`remove:${s.id}`] ? "glim-shake" : ""}
              />
            ) : (
              <Button
                label={t("offsite.targets.remove")}
                labelKey="offsite.targets.remove"
                tone="subtle"
                onClick={() => setConfirmRemove(s.id)}
                className={`rounded-control px-2.5 py-1 text-xs text-carbon-text${
                  shake[`remove:${s.id}`] ? " glim-shake" : ""
                }`}
              />
            )}
          </div>
        </div>
      ))}

      {editing ? (
        <div className="flex flex-col gap-3 rounded-card bg-carbon-surface2 p-3">
          <label className={fieldCls}>{t("cloud.credSets.name")}
            <input value={editing.name} onChange={(e) => setField("name", e.target.value)} className={inputCls} /></label>
          <div className="flex flex-col gap-2 rounded-card bg-carbon-surface3 p-3">
            <span className="text-xs font-semibold text-carbon-textSub">Amazon S3</span>
            <label className={fieldCls}>AWS_ACCESS_KEY_ID
              <input value={editing.s3KeyId} onChange={(e) => setField("s3KeyId", e.target.value)} spellCheck={false} dir="ltr" className={`${inputCls} text-start`} /></label>
            <label className={fieldCls}>AWS_SECRET_ACCESS_KEY
              <RevealInput {...revealS3Secret} value={editing.s3Secret} onChange={(e) => setField("s3Secret", e.target.value)} spellCheck={false}
                placeholder={sets.find((s) => s.id === editing.id)?.s3SecretSet ? t("cloud.secretSet") : ""} wrapperClassName="w-full" className={inputCls} /></label>
            <label className={fieldCls}>AWS_DEFAULT_REGION
              <input value={editing.s3Region} onChange={(e) => setField("s3Region", e.target.value)} spellCheck={false} placeholder="us-east-1" className={inputCls} /></label>
            <label className={fieldCls}>{t("cloud.storageClass.label")}
              <SelectField
                value={editing.s3StorageClass}
                onChange={(v) => setField("s3StorageClass", v)}
                label={t("cloud.storageClass.label")}
                options={[
                  { value: "", label: t("cloud.storageClass.default") },
                  ...STORAGE_CLASSES.map((sc) => ({ value: sc, label: sc })),
                ]}
                className={inputCls}
              /></label>
          </div>
          <div className="flex flex-col gap-2 rounded-card bg-carbon-surface3 p-3">
            <span className="text-xs font-semibold text-carbon-textSub">restic REST server</span>
            <label className={fieldCls}>RESTIC_REST_USERNAME
              <input value={editing.restUser} onChange={(e) => setField("restUser", e.target.value)} spellCheck={false} dir="ltr" className={`${inputCls} text-start`} /></label>
            <label className={fieldCls}>RESTIC_REST_PASSWORD
              <RevealInput {...revealRestPassword} value={editing.restPassword} onChange={(e) => setField("restPassword", e.target.value)} spellCheck={false}
                placeholder={sets.find((s) => s.id === editing.id)?.restPasswordSet ? t("cloud.secretSet") : ""} wrapperClassName="w-full" className={inputCls} /></label>
          </div>
          <div className="flex items-center gap-3">
            <Button
              label={t("common.close")}
              labelKey="common.close"
              tone="neutral"
              onClick={closeEditor}
            />
            <Button
              key={shake.save || 0}
              label={t("settings.save")}
              labelKey="settings.save"
              tone="accent"
              onClick={() => void save()}
              disabled={state === "saving"}
              busy={state === "saving"}
              title={state === "saving" ? t("auth.saving") : undefined}
              className={shake.save ? "glim-shake" : ""}
            />
          </div>
        </div>
      ) : (
        <Button
          label={t("cloud.credSets.add")}
          labelKey="cloud.credSets.add"
          // Accent: with no editor open this is the card's one primary action.
          tone="accent"
          onClick={openNew}
          className="self-start"
        />
      )}
    </Card>
  );
}
