import { useEffect, useId, useState } from "react";
import { Badge } from "../../components/Badge";
import { Button } from "../../components/Button";
import { IconStar } from "../../components/glyphs";
import { InfoBubble } from "../../components/InfoBubble";
import { IconCheck } from "../../components/navGlyphs";
import { RevealInput } from "../../components/RevealInput";
import { SelectField } from "../../components/SelectField";
import { Rows, SettingRow } from "../../components/storage/rows";
import { Toggle } from "../../components/Toggle";
import {
  dismissSignInsNote,
  getCloudCredSets,
  makeDefaultSignIn,
  setCloudCredSets,
  type CloudCredSet,
  type CloudCredSetInfo,
  type SignInsNote,
} from "../../lib/api";
import { useT } from "../../lib/i18n";
import { formatList } from "../../lib/placement";
import { pushSaveWarnings } from "../../lib/placementCodes";
import { STORAGE_CLASSES } from "../../lib/storageClasses";
import { DOMAIN_LABEL } from "../../lib/storageLocations";
import { useToast } from "../../lib/toast";
import { credSetDraft, credSetLabel, credSetsChanged, saveCredSet, useCloudCredSets } from "../../lib/useCloudCredSets";
import { useReveal } from "../../lib/useReveal";
import { randomId } from "../../lib/uuid";
import { Card, type SaveState } from "./shared";

/** The note on what the shared cloud credentials became, as one paragraph. */
function noteText(t: ReturnType<typeof useT>["t"], lang: string, note: SignInsNote): string {
  const parts: string[] = [];
  if (note.setName) {
    parts.push(t(note.merged ? "signIns.note.merged" : "signIns.note.created").replace(/\{name\}/g, () => note.setName));
  }
  if (note.duplicates > 0) parts.push(t("signIns.note.duplicates", note.duplicates));
  if (note.linked.length > 0) {
    const list = formatList(lang, note.linked.map((domain) => t(DOMAIN_LABEL[domain])));
    parts.push(t("signIns.note.linked").replace("{list}", () => list));
  }
  return parts.join(" ");
}

/**
 * SignInsCard lists the saved sign-ins: the keys BombVault signs in to S3
 * services and restic REST servers with. A storage location picks one, and
 * the default serves every place that names none. The list is saved as a
 * whole, which is why every save resends every set.
 *
 * Unlike the rest of the settings page the editor has a Save button: it holds
 * a draft that Close discards, and saving on every keystroke would list a
 * half-filled sign-in as soon as the first letter of its name was typed.
 */
export function SignInsCard({ hueIndex }: { hueIndex?: number }) {
  const { t, lang } = useT();
  const uid = useId();
  const { push } = useToast();
  const sets = useCloudCredSets();
  const [note, setNote] = useState<SignInsNote | null>(null);
  const [editing, setEditing] = useState<CloudCredSet | null>(null);
  const [state, setState] = useState<SaveState>("idle");
  const [confirmRemove, setConfirmRemove] = useState<string | null>(null);
  const [removingId, setRemovingId] = useState<string | null>(null);
  const [defaultingId, setDefaultingId] = useState<string | null>(null);
  // A failed action shakes the button that caused it. Removal is keyed per
  // row, since several remove buttons can be on screen at once.
  const [shake, setShake] = useState<Record<string, number>>({});
  const revealS3Secret = useReveal();
  const revealRestPassword = useReveal();
  const stored = editing ? sets.find((s) => s.id === editing.id) : undefined;

  useEffect(() => {
    let alive = true;
    getCloudCredSets()
      .then((res) => {
        if (alive && res.ok) setNote(res.note ?? null);
      })
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, []);

  function bumpShake(key: string) {
    setShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
  }

  function openNew() {
    // Not crypto.randomUUID(): it is undefined outside a secure context, and
    // BombVault supports plain HTTP.
    setEditing({ id: randomId(), name: "", s3KeyId: "", s3Secret: "", s3Region: "", restUser: "", restPassword: "", s3StorageClass: "", default: false });
    setState("idle");
  }
  function openEdit(s: CloudCredSetInfo) {
    setConfirmRemove(null);
    setEditing(editing?.id === s.id ? null : credSetDraft(s));
    setState("idle");
  }
  function setField<K extends keyof CloudCredSet>(k: K, v: CloudCredSet[K]) {
    setEditing((p) => (p ? { ...p, [k]: v } : p));
  }

  function setDefault(on: boolean) {
    if (!on && stored?.default) {
      push(t("signIns.removeDefault"), "fail");
      return;
    }
    setField("default", on);
  }

  async function save() {
    if (!editing) return;
    if (!editing.name.trim()) {
      push(t("storage.creds.nameFirst"), "fail");
      bumpShake("save");
      return;
    }
    setState("saving");
    // The default mark moves: saving one as the default takes it from the rest.
    const others = editing.default ? sets.map((s) => ({ ...s, default: false })) : sets;
    try {
      const r = await saveCredSet(others, editing);
      setState("idle");
      if (!r.ok) {
        push(r.error ?? t("settings.error"), "fail");
        bumpShake("save");
        return;
      }
      setEditing(null);
      push(t("settings.saved"), "success");
      pushSaveWarnings(push, t, r.warnings);
    } catch (err) {
      setState("idle");
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      bumpShake("save");
    }
  }

  async function remove(s: CloudCredSetInfo) {
    if (s.default) {
      push(t("signIns.removeDefault"), "fail");
      bumpShake(`remove:${s.id}`);
      return;
    }
    const users = s.usedBy ?? [];
    if (users.length > 0) {
      const list = formatList(lang, users.map((u) => u.name));
      push(t("signIns.removeUsed").replace("{list}", () => list), "fail");
      bumpShake(`remove:${s.id}`);
      return;
    }
    if (confirmRemove !== s.id) {
      setConfirmRemove(s.id);
      return;
    }
    setRemovingId(s.id);
    try {
      const r = await setCloudCredSets(sets.filter((x) => x.id !== s.id).map(credSetDraft));
      if (r.ok) {
        credSetsChanged();
      } else {
        push(r.error ?? t("settings.error"), "fail");
        bumpShake(`remove:${s.id}`);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      bumpShake(`remove:${s.id}`);
    } finally {
      setRemovingId(null);
      setConfirmRemove(null);
    }
  }

  async function makeDefault(s: CloudCredSetInfo) {
    setDefaultingId(s.id);
    try {
      const r = await makeDefaultSignIn(s.id);
      if (!r.ok) {
        push(r.error ?? t("settings.error"), "fail");
        bumpShake(`default:${s.id}`);
        return;
      }
      credSetsChanged();
      push(t("settings.saved"), "success");
      pushSaveWarnings(push, t, r.warnings);
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      bumpShake(`default:${s.id}`);
    } finally {
      setDefaultingId(null);
    }
  }

  function dismiss() {
    setNote(null);
    void dismissSignInsNote().catch(() => undefined);
  }

  const inputCls = "rounded-control bg-carbon-surface2 text-carbon-text text-sm font-mono px-3 py-1.5 glim-field-focus";
  const textCls = `${inputCls} w-72 max-w-full`;
  const headCls = "text-xs font-semibold uppercase tracking-widest text-carbon-textSub";
  const isNew = editing !== null && !stored;

  const editor = editing && (
    <div className="flex flex-col gap-4 pt-4">
      <Rows>
        <SettingRow label={t("cloud.credSets.name")} htmlFor={`${uid}-name`}>
          <input id={`${uid}-name`} value={editing.name} onChange={(e) => setField("name", e.target.value)} className={textCls} />
        </SettingRow>
        <SettingRow label={t("signIns.defaultToggle")} hint={t("signIns.defaultToggleHint")}>
          <Toggle hideLabel label={t("signIns.defaultToggle")} checked={!!editing.default} onChange={setDefault} />
        </SettingRow>
      </Rows>
      <div className="flex flex-col gap-2">
        <span className={headCls}>Amazon S3</span>
        <Rows>
          <SettingRow label="AWS_ACCESS_KEY_ID" htmlFor={`${uid}-s3-key`}>
            <input id={`${uid}-s3-key`} value={editing.s3KeyId} onChange={(e) => setField("s3KeyId", e.target.value)} spellCheck={false} dir="ltr" className={`${textCls} text-start`} />
          </SettingRow>
          <SettingRow label="AWS_SECRET_ACCESS_KEY" htmlFor={`${uid}-s3-secret`}>
            <RevealInput {...revealS3Secret} id={`${uid}-s3-secret`} value={editing.s3Secret} onChange={(e) => setField("s3Secret", e.target.value)} spellCheck={false}
              placeholder={stored?.s3SecretSet ? t("cloud.secretSet") : ""} wrapperClassName="w-72 max-w-full" className={inputCls} />
          </SettingRow>
          <SettingRow label="AWS_DEFAULT_REGION" htmlFor={`${uid}-s3-region`}>
            <input id={`${uid}-s3-region`} value={editing.s3Region} onChange={(e) => setField("s3Region", e.target.value)} spellCheck={false} placeholder="us-east-1" className={textCls} />
          </SettingRow>
          <SettingRow label={t("cloud.storageClass.label")} hint={t("cloud.storageClass.hint")} htmlFor={`${uid}-s3-class`}>
            <SelectField
              id={`${uid}-s3-class`}
              value={editing.s3StorageClass}
              onChange={(v) => setField("s3StorageClass", v)}
              label={t("cloud.storageClass.label")}
              options={[{ value: "", label: t("cloud.storageClass.default") }, ...STORAGE_CLASSES.map((sc) => ({ value: sc, label: sc }))]}
              className={inputCls}
            />
          </SettingRow>
        </Rows>
      </div>
      <div className="flex flex-col gap-2">
        <span className={headCls}>restic REST server</span>
        <Rows>
          <SettingRow label="RESTIC_REST_USERNAME" htmlFor={`${uid}-rest-user`}>
            <input id={`${uid}-rest-user`} value={editing.restUser} onChange={(e) => setField("restUser", e.target.value)} spellCheck={false} dir="ltr" className={`${textCls} text-start`} />
          </SettingRow>
          <SettingRow label="RESTIC_REST_PASSWORD" htmlFor={`${uid}-rest-password`}>
            <RevealInput {...revealRestPassword} id={`${uid}-rest-password`} value={editing.restPassword} onChange={(e) => setField("restPassword", e.target.value)} spellCheck={false}
              placeholder={stored?.restPasswordSet ? t("cloud.secretSet") : ""} wrapperClassName="w-72 max-w-full" className={inputCls} />
          </SettingRow>
        </Rows>
      </div>
      <div className="flex flex-wrap items-center justify-end gap-3">
        <Button label={t("common.close")} labelKey="common.close" tone="neutral" onClick={() => setEditing(null)} />
        <Button
          key={shake.save || 0}
          label={t("settings.save")}
          labelKey="settings.save"
          tone="accent"
          onClick={() => void save()}
          disabled={state === "saving"}
          busy={state === "saving"}
          className={shake.save ? "glim-shake" : ""}
        />
      </div>
    </div>
  );

  return (
    <Card
      title={t("signIns.title")}
      hint={t("signIns.hint")}
      hueIndex={hueIndex}
      newId="settings-sign-ins"
      foot={
        !isNew && (
          <span data-new="card-foot" className="flex">
            <Button
              label={t("signIns.add")}
              labelKey="signIns.add"
              // Accent: with no editor open this is the card's one primary action.
              tone="accent"
              onClick={openNew}
              className="glim-btn-wrap"
            />
          </span>
        )
      }
    >
      {note && (
        <div role="status" className="flex flex-wrap items-center gap-x-3 gap-y-2 rounded-card bg-statusOkBg px-3 py-2">
          <span className="inline-flex text-statusOk">
            <IconCheck />
          </span>
          <span className="flex min-w-0 flex-1 flex-col gap-0.5 text-sm text-carbon-text">
            <span className="flex items-center gap-1.5 font-semibold">
              {t("signIns.note.title")}
              {noteText(t, lang, note) && <InfoBubble tip={noteText(t, lang, note)} />}
            </span>
            <span className="text-xs text-carbon-textSub">{t("signIns.note.text")}</span>
          </span>
          <Button label={t("signIns.note.ok")} labelKey="signIns.note.ok" tone="neutral" onClick={dismiss} />
        </div>
      )}

      {sets.length === 0 && !editing && <span className="text-xs text-carbon-textMuted">{t("signIns.none")}</span>}

      {sets.length > 0 && (
        <ul className="flex flex-col divide-y divide-carbon-border/60">
          {sets.map((s) => {
            const users = s.usedBy ?? [];
            return (
              <li key={s.id} className="flex flex-col py-3 first:pt-0 last:pb-0">
                {/* On a phone the actions go under the name, which beside them
                    shrinks to a column one character wide. */}
                <div className="flex items-start justify-between gap-3 max-md:flex-col">
                  <div className="flex min-w-0 flex-col gap-1">
                    <span className="flex flex-wrap items-center gap-2 text-sm text-carbon-text">
                      <span className="min-w-0 truncate max-md:whitespace-normal max-md:wrap-anywhere">{credSetLabel(t, s)}</span>
                      {s.s3KeyId && <Badge tone="neutral" size="small">S3</Badge>}
                      {s.restUser && <Badge tone="neutral" size="small">restic REST</Badge>}
                      {s.default && (
                        <Badge tone="ok" size="small">
                          <span className="flex [&_svg]:h-[1em] [&_svg]:w-[1em]">
                            <IconStar />
                          </span>
                          {t("signIns.default")}
                        </Badge>
                      )}
                    </span>
                    {(s.s3KeyId || s.restUser) && (
                      <span dir="ltr" className="break-all text-start font-mono text-xs text-carbon-textMuted">
                        {s.s3KeyId || s.restUser}
                      </span>
                    )}
                    <span className="text-xs text-carbon-textSub wrap-break-word">
                      {users.length > 0
                        ? t("signIns.usedBy").replace("{list}", () => formatList(lang, users.map((u) => u.name)))
                        : t("signIns.unused")}
                    </span>
                    {s.default && <span className="text-xs text-carbon-textSub wrap-break-word">{t("signIns.defaultAlso")}</span>}
                  </div>
                  <div className="flex shrink-0 flex-wrap items-start gap-2">
                    {!s.default && (
                      <span data-new="sign-in-make-default" className="flex">
                        <Button
                          key={shake[`default:${s.id}`] || 0}
                          label={t("signIns.makeDefault")}
                          labelKey="signIns.makeDefault"
                          onClick={() => void makeDefault(s)}
                          disabled={defaultingId !== null}
                          busy={defaultingId === s.id}
                          className={shake[`default:${s.id}`] ? "glim-shake" : ""}
                        />
                      </span>
                    )}
                    <Button
                      label={editing?.id === s.id ? t("common.close") : t("offsite.targets.edit")}
                      labelKey={editing?.id === s.id ? "common.close" : "offsite.targets.edit"}
                      tone="neutral"
                      onClick={() => openEdit(s)}
                    />
                    {/* A destructive action gets no red of its own in this app. */}
                    <Button
                      key={shake[`remove:${s.id}`] || 0}
                      label={t(confirmRemove === s.id ? "offsite.targets.confirmRemove" : "offsite.targets.remove")}
                      labelKey={confirmRemove === s.id ? "offsite.targets.confirmRemove" : "offsite.targets.remove"}
                      tone="neutral"
                      onClick={() => void remove(s)}
                      disabled={removingId === s.id}
                      busy={removingId === s.id}
                      className={shake[`remove:${s.id}`] ? "glim-shake" : ""}
                    />
                  </div>
                </div>
                {editing?.id === s.id && editor}
              </li>
            );
          })}
        </ul>
      )}

      {isNew && editor}
    </Card>
  );
}
