import type { SaveState } from "./shared";
import { Card } from "../settings/shared";
import { InfoBubble } from "../../components/InfoBubble";
import { RevealInput } from "../../components/RevealInput";
import { getCloud, setCloud } from "../../lib/api";
import { useEffect, useRef, useState } from "react";
import { useReveal } from "../../lib/useReveal";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { pushSaveWarnings } from "../../lib/placementCodes";
import { SelectField } from "../../components/SelectField";
import { STORAGE_CLASSES } from "../../lib/storageClasses";

// CloudCard stores the encrypted credentials for off-site restic backends (S3
// and restic REST). Secrets are write-only: blank on load, and a blank save
// keeps the stored value. Field labels are restic's own env var names.
export function CloudCard({
  t,
  hueIndex,
  nested,
}: {
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
  /** Passed through to Card; Recovery's step 3 sets it to render this card
   *  inside its own step card. */
  nested?: boolean;
}) {
  const { push } = useToast();
  const [c, setC] = useState({ s3KeyId: "", s3Secret: "", s3Region: "", restUser: "", restPassword: "", s3StorageClass: "" });
  const [secretSet, setSecretSet] = useState(false);
  const [pwSet, setPwSet] = useState(false);
  const [, setState] = useState<SaveState>("idle");
  const revealS3Secret = useReveal();
  const revealRestPassword = useReveal();
  // Fields save themselves after a pause in typing. The card keeps its own
  // debounce because it has no access to the settings page's.
  const debounceTimers = useRef<Record<string, ReturnType<typeof setTimeout>>>({});
  function debounced(key: string, run: () => void) {
    const existing = debounceTimers.current[key];
    if (existing) clearTimeout(existing);
    debounceTimers.current[key] = setTimeout(run, 800);
  }

  // loaded gates persistPatch: setCloud replaces the whole record, so a save
  // before a successful read would blank the stored key id, region, REST user
  // and storage class.
  const [loaded, setLoaded] = useState(false);
  const [loadErr, setLoadErr] = useState(false);
  function refresh() {
    getCloud()
      .then((r) => {
        if (r.ok) {
          setC((p) => ({ ...p, s3KeyId: r.s3KeyId ?? "", s3Region: r.s3Region ?? "", restUser: r.restUser ?? "", s3StorageClass: r.s3StorageClass ?? "" }));
          setSecretSet(!!r.s3SecretSet);
          setPwSet(!!r.restPasswordSet);
          setLoaded(true);
          setLoadErr(false);
        } else {
          setLoadErr(true);
        }
      })
      .catch(() => setLoadErr(true));
  }
  useEffect(refresh, []);

  // persistPatch merges one field onto the `c` captured when the save was
  // scheduled and posts the whole object, since setCloud has no partial form.
  //
  // A saved secret's input keeps its text; only the "set" flag follows the
  // save. Clearing it would let the debounce fire during a pause mid-secret,
  // wipe the field under the cursor, and save the rest of the secret as a
  // fragment over the real credential.
  async function persistPatch(patch: Partial<typeof c>) {
    if (!loaded) {
      push(t("settings.notLoadedNoSave"), "fail");
      return;
    }
    setState("saving");
    const merged = { ...c, ...patch };
    try {
      const r = await setCloud(merged);
      if (r.ok) {
        setState("idle");
        if (patch.s3Secret) setSecretSet(true);
        if (patch.restPassword) setPwSet(true);
        push(t("settings.saved"), "success");
        pushSaveWarnings(push, t, r.warnings);
      } else {
        setState("idle");
        push(r.error ?? t("settings.error"), "fail");
      }
    } catch (err) {
      setState("idle");
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    }
  }

  function set<K extends keyof typeof c>(k: K, v: string) {
    setC((p) => ({ ...p, [k]: v }));
    debounced(String(k), () => void persistPatch({ [k]: v } as Partial<typeof c>));
  }

  // A select changes once per pick rather than per keystroke, so it saves
  // right away.
  function setImmediate<K extends keyof typeof c>(k: K, v: string) {
    setC((p) => ({ ...p, [k]: v }));
    void persistPatch({ [k]: v } as Partial<typeof c>);
  }

  const inputCls =
    "rounded-control bg-carbon-surface3 text-carbon-text text-sm font-mono px-3 py-1.5 glim-field-focus-well";
  const fieldCls = "flex flex-col gap-1 text-xs font-mono text-carbon-textSub";

  return (
    <Card title={t("cloud.title")} hueIndex={hueIndex} nested={nested}>
      {/* The card refuses to save until the read succeeds, so a failed read
          has to be visible. */}
      {loadErr && <span className="text-xs text-statusFail">{t("settings.notLoadedNoSave")}</span>}
      {/* Plain text rather than an info bubble: it is the only place that
          lists the remote URL prefixes (s3:, rest:, sftp:) the
          Backup Path fields accept, and people copy from it. */}
      <p className="text-xs text-carbon-textMuted -mt-1">{t("cloud.hint")}</p>

      <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
        <span className="text-xs font-semibold text-carbon-textSub">Amazon S3</span>
        <label className={fieldCls}>AWS_ACCESS_KEY_ID
          <input value={c.s3KeyId} onChange={(e) => set("s3KeyId", e.target.value)} spellCheck={false} dir="ltr" className={`${inputCls} text-start`} /></label>
        <label className={fieldCls}>AWS_SECRET_ACCESS_KEY
          <RevealInput {...revealS3Secret} value={c.s3Secret} onChange={(e) => set("s3Secret", e.target.value)} spellCheck={false}
            placeholder={secretSet ? t("cloud.secretSet") : ""} wrapperClassName="w-full" className={inputCls} /></label>
        <label className={fieldCls}>AWS_DEFAULT_REGION
          <input value={c.s3Region} onChange={(e) => set("s3Region", e.target.value)} spellCheck={false} placeholder="us-east-1" className={inputCls} /></label>
        <label className={fieldCls}>
          <span className="flex items-center gap-1">
            {t("cloud.storageClass.label")}
            <InfoBubble tip={t("cloud.storageClass.hint")} />
          </span>
          <SelectField
            value={c.s3StorageClass}
            onChange={(v) => setImmediate("s3StorageClass", v)}
            label={t("cloud.storageClass.label")}
            options={[
              { value: "", label: t("cloud.storageClass.default") },
              ...STORAGE_CLASSES.map((sc) => ({ value: sc, label: sc })),
            ]}
            className={inputCls}
          /></label>
      </div>

      <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
        <span className="text-xs font-semibold text-carbon-textSub">restic REST server</span>
        <label className={fieldCls}>RESTIC_REST_USERNAME
          <input value={c.restUser} onChange={(e) => set("restUser", e.target.value)} spellCheck={false} dir="ltr" className={`${inputCls} text-start`} /></label>
        <label className={fieldCls}>RESTIC_REST_PASSWORD
          <RevealInput {...revealRestPassword} value={c.restPassword} onChange={(e) => set("restPassword", e.target.value)} spellCheck={false}
            placeholder={pwSet ? t("cloud.secretSet") : ""} wrapperClassName="w-full" className={inputCls} /></label>
      </div>
    </Card>
  );
}
