
// toDraft turns a secret-blanked CloudCredSetInfo (from GET) into an editable
// CloudCredSet with blank secret fields — sending it back with those fields
// blank is exactly what makes the backend's keep-prior-if-blank merge (matched
// by id) preserve the real stored secret, so an untouched set's key/password
// survives a save that only edited a DIFFERENT set in the same list.
import { Button } from "../../components/Button";
import { RevealInput } from "../../components/RevealInput";
import { setCloudCredSets, type CloudCredSet, type CloudCredSetInfo } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { credSetsChanged, useCloudCredSets } from "../../lib/useCloudCredSets";
import { useReveal } from "../../lib/useReveal";
import { randomId } from "../../lib/uuid";
import { Card, type SaveState } from "./shared";
import { useState } from "react";

function toDraft(s: CloudCredSetInfo): CloudCredSet {
  return { id: s.id, name: s.name, s3KeyId: s.s3KeyId, s3Secret: "", s3Region: s.s3Region, restUser: s.restUser, restPassword: "", s3StorageClass: s.s3StorageClass };
}
// CloudCredSetsCard, lifted out of Settings.tsx ([337]).
//
// A move, not a rewrite: the component and its comments are unchanged.

// CloudCredSetsCard manages ADDITIONAL named credential sets (#141 stage 2):
// lets an off-site target (OffsiteTargetsSection's editor) opt into its OWN
// S3/restic-REST credentials instead of sharing the single set CloudCard
// manages above — e.g. two S3 endpoints (Hetzner + a local Garage/MinIO) that
// need different keys. Same write-only-secret contract as CloudCard; the
// whole list round-trips through setCloudCredSets (replace-all), which is why
// every save resends every set (via toDraft — see its own comment for why
// that is safe for the sets NOT being edited).
//
// GENUINE EXCEPTION to the full-page Speichern-Button sweep (jdp, live
// review, emphatic: "Die Speicher-Buttons sollen in allen Tabs weg... Nur
// dort sollen Speicher-Buttons bleiben, wo es unbedingt sein muss."): the
// `editing` form's own Save button stays, unlike every plain settings field
// converted elsewhere in this file. This is exactly the "multi-step DRAFT of
// something not meant to take effect until deliberately applied" shape the
// sweep's own criteria calls out: `editing` is a scratch draft (openNew()
// mints one with a random id that exists NOWHERE in `sets` yet), sitting
// beside an explicit "Close" button that discards it — auto-saving per
// keystroke would create a half-filled, visibly-listed credential set the
// instant the FIRST character of its name is typed, and would silently
// defeat Close's own "discard my edits" contract (the whole point of a
// cancel affordance). Every other row-level action here (Edit/Remove) is
// already immediate, unbatched CRUD against the same API — only this one
// scratch-draft form keeps a manual commit step.
export function CloudCredSetsCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  const { push } = useToast();
  // Shared with every off-site target's credential picker (#173) — see
  // useCloudCredSets. This card is the only editor of the list, so it is also
  // the only thing that announces a change, and it reads the result back
  // through the same hook: its own rows and the pickers can no longer disagree.
  const sets = useCloudCredSets();
  const [editing, setEditing] = useState<CloudCredSet | null>(null);
  const [state, setState] = useState<SaveState>("idle");
  const [confirmRemove, setConfirmRemove] = useState<string | null>(null);
  const [removingId, setRemovingId] = useState<string | null>(null);
  // GlimStone standing rule (jdp, live review, emphatic — "Wenn etwas
  // fehlschlägt soll der Toggle/Button kurz zittern. Systemweit!!"): save()
  // and remove() below already push a fail toast but never bumped a shake
  // nonce, leaving the failure feedback toast-only instead of the toast+
  // shake pairing this session's own system-wide shake sweep (commit
  // b48fe30) established everywhere else — that commit's own file list did
  // not include Settings.tsx. Same per-key nonce shape as IntegrityCard's
  // own `shake` state (see its `bumpShake` doc comment); "save" for the
  // single shared editor panel, and `remove:${id}` per row (there can be
  // MULTIPLE saved cred sets rendered at once, unlike the single-editor
  // Save button, so a shared "remove" key would incorrectly shake every
  // row's button when only one row's delete actually failed — same
  // per-row-id keying IntegrityCard's own domain actions use).
  const [shake, setShake] = useState<Record<string, number>>({});
  function bumpShake(key: string) {
    setShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
  }
  const revealS3Secret = useReveal();
  const revealRestPassword = useReveal();

  function openNew() {
    // randomId(), not crypto.randomUUID() — the latter is secure-context-only
    // and BombVault ships a documented plain-HTTP mode, where it is undefined
    // and this click would throw instead of opening the editor (see lib/uuid.ts).
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

  // GlimStone follow-up pass (v8.0.0): the "saved"/"error" flash below is now
  // a toast (save() previously had no success notice at all, since closeEditor()
  // removed the form before it could show one — push() fixes that too). remove()'s
  // failure used to set `msg` with nothing left mounted to render it (the editor
  // closes on remove, and `state` never becomes "error" from remove() alone) — a
  // latent dead branch this migration also resolves, now that both routes push
  // the same way.
  async function save() {
    if (!editing) return;
    setState("saving");
    const isNew = !sets.some((s) => s.id === editing.id);
    const rest = sets.filter((s) => s.id !== editing.id).map(toDraft);
    const next = isNew ? [...rest, editing] : [...rest, editing];
    try {
      const r = await setCloudCredSets(next);
      if (r.ok) {
        setState("idle");
        closeEditor();
        credSetsChanged();
        push(t("settings.saved"), "success");
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
    "rounded-control bg-carbon-surface3 text-carbon-text text-sm font-mono px-3 py-1.5 bv-field-focus-well";
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
            {/* NO bespoke red on either state (whole-app sweep). The confirm
                state was `bg-statusFailBg`/`text-statusFail` and the resting
                state carried `text-statusFail` ink on a neutral fill — the
                one control in this Card with a colour of its own. Standing
                rule: a destructive action gets no special red treatment (jdp:
                "Keine Sonderfarbe fuer den Entfernen-Badge"). Both states now
                use the same neutral secondary chrome as the "Bearbeiten"
                button beside them.
                  Stays a TEXT button for the same reason Fleet's and
                Receiver's remove pairs do: the two-click inline confirm needs
                a label to flip ("Entfernen" → "Entfernen bestätigen" → "Wird
                entfernt…"), which an icon-only badge has nowhere to put. */}
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
          {/* `bg-carbon-surface3` — was `bg-carbon-surface3/40`, a 40%-alpha
              wash of the surface3 token and the ONLY place in web/src that
              ever used it (every other nested panel on this page, e.g. this
              same Card's own outer `bg-carbon-surface2` a moment ago, uses a
              plain solid surface token). A translucent wash over the Card's
              own solid surface2 background just reads as a slightly darker
              grey, not as "surface3" in any theme — the exact "translucent
              wash reads as darkened, not as the intended colour" failure
              this app's own design language already flags elsewhere. Solid
              surface3 is the correct next step up from this panel's
              surface2 parent, same nesting depth as every sibling panel. */}
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
              <select value={editing.s3StorageClass} onChange={(e) => setField("s3StorageClass", e.target.value)} className={inputCls}>
                <option value="">{t("cloud.storageClass.default")}</option>
                <option value="STANDARD">STANDARD</option>
                <option value="STANDARD_IA">STANDARD_IA</option>
                <option value="ONEZONE_IA">ONEZONE_IA</option>
                <option value="INTELLIGENT_TIERING">INTELLIGENT_TIERING</option>
                <option value="GLACIER_IR">GLACIER_IR</option>
              </select></label>
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
            <Button
              label={t("common.close")}
          labelKey="common.close"
              tone="neutral"
              onClick={closeEditor}
            />
          </div>
        </div>
      ) : (
        <Button
          label={t("cloud.credSets.add")}
          labelKey="cloud.credSets.add"
          // Accent ([288]). This branch renders only when the list is empty, so
          // it is the card's single call to action rather than one control
          // among several.
          tone="accent"
          onClick={openNew}
          className="self-start"
        />
      )}
    </Card>
  );
}
