// CloudCard, lifted out of Settings.tsx ([337]).
//
// A MOVE, not a rewrite: the component is byte-identical to what stood
// in Settings.tsx, and it was already module-level and prop-driven, so
// nothing crosses a new seam. See that file's own note for why the cut
// stops here rather than continuing into SettingsPage itself.
import type { SaveState } from "./shared";
import { Card } from "../settings/shared";
import { InfoBubble } from "../../components/InfoBubble";
import { RevealInput } from "../../components/RevealInput";
import { getCloud, setCloud } from "../../lib/api";
import { useEffect, useRef, useState } from "react";
import { useReveal } from "../../lib/useReveal";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";

// CloudCard stores credentials for off-site restic backends (S3 + restic REST),
// kept encrypted. Secrets are write-only: blank on load, blank-on-save keeps the
// stored value. Field labels are restic's actual env var names (self-documenting).
export function CloudCard({
  t,
  hueIndex,
  nested,
}: {
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
  /** Passed straight through to Card — see its own `nested` doc. Set by
   *  Recovery's step 3, which renders this Card inside its own step card. */
  nested?: boolean;
}) {
  const { push } = useToast();
  const [c, setC] = useState({ s3KeyId: "", s3Secret: "", s3Region: "", restUser: "", restPassword: "", s3StorageClass: "" });
  const [secretSet, setSecretSet] = useState(false);
  const [pwSet, setPwSet] = useState(false);
  // No SaveBar/button reads this back anymore post-conversion (see
  // persistPatch below) — only the setter is needed, same "only the setters
  // are needed" shape as Settings.tsx's own setDomSaveState.
  const [, setState] = useState<SaveState>("idle");
  const revealS3Secret = useReveal();
  const revealRestPassword = useReveal();
  // Full-page Speichern-Button sweep (jdp, live review, emphatic: "Die
  // Speicher-Buttons sollen in allen Tabs weg. Überall soll es automatisch
  // speichern."): unlike RcloneCard right above (kept as a genuine exception
  // — see that Card's own header comment), every field here already
  // round-trips a real persisted value (getCloud below), the two secrets
  // included via the exact same "blank = keep the stored one" contract
  // Settings.tsx's own metricsToken/exportAgeRecipients fields already use
  // safely — there is no "draft not meant to take effect" shape to protect
  // here, just plain settings fields that happen to live in this Card's own
  // local state instead of the page-wide `settings` object. Same local-
  // debounce mechanism as FleetSettingsCard's own instanceName conversion
  // (this Card has no access to SettingsPage's shared debouncedSave either).
  const debounceTimers = useRef<Record<string, ReturnType<typeof setTimeout>>>({});
  function debounced(key: string, run: () => void) {
    const existing = debounceTimers.current[key];
    if (existing) clearTimeout(existing);
    debounceTimers.current[key] = setTimeout(run, 800);
  }

  // loaded gates persistPatch below, exactly like OffsiteWizard's cloudLoaded
  // and for the same reason: setCloud is a FULL REPLACE, so posting before a
  // successful read had filled `c` would send this card's empty initial state
  // as the new truth and blank the stored AWS key id, region, REST user and
  // storage class. The read used to swallow every failure silently
  // (.catch(() => undefined), nothing set, nothing shown), so a single failed
  // GET plus one dropdown click was enough, and the toast said "saved".
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

  // persistPatch merges one field's freshly-typed value onto the CURRENT `c`
  // snapshot (closed over at the point the caller scheduled it, same
  // "correct as long as one field is edited at a time" reasoning
  // debouncedSave's own callers rely on elsewhere in this file) and POSTs the
  // whole object — setCloud has no partial-patch form, unlike SettingsPage's
  // own save().
  //
  // A saved secret's FIELD IS LEFT ALONE. Only the "…Set" flag follows the
  // save, so a blank field still shows the "already set" placeholder after a
  // refresh(). Emptying the input here — which is what the deleted manual Save
  // button did, correctly, because by then the user had finished typing — turns
  // an auto-saving field into a secret shredder: the debounce fires 800 ms into
  // any pause mid-secret, the input is wiped under the cursor, the REST of the
  // secret is typed into an empty field, and that fragment is saved over the
  // real credential. Nothing shows an error; the backend's "blank = keep the
  // stored one" contract has no way to tell a fragment from a whole key. The
  // field keeps its text until the card is remounted, and every later save just
  // re-sends the same value.
  async function persistPatch(patch: Partial<typeof c>) {
    // Never write a full replace built on a state that was never read.
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
      } else {
        setState("idle");
        push(r.error ?? t("settings.error"), "fail");
      }
    } catch (err) {
      setState("idle");
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    }
  }

  // set — continuously-typed fields (the four text inputs): optimistic local
  // update + debounce, same shape as every other free-text field on this
  // page.
  function set<K extends keyof typeof c>(k: K, v: string) {
    setC((p) => ({ ...p, [k]: v }));
    debounced(String(k), () => void persistPatch({ [k]: v } as Partial<typeof c>));
  }

  // A <select> fires once per discrete pick, not per keystroke — same
  // "single discrete choice, not continuous typing" reasoning
  // autoSaveScheduleField's own header comment gives for immediate (not
  // debounced) saves, so this one saves right away instead.
  function setImmediate<K extends keyof typeof c>(k: K, v: string) {
    setC((p) => ({ ...p, [k]: v }));
    void persistPatch({ [k]: v } as Partial<typeof c>);
  }

  const inputCls =
    "rounded-control bg-carbon-surface3 text-carbon-text text-sm font-mono px-3 py-1.5 bv-field-focus-well";
  const fieldCls = "flex flex-col gap-1 text-xs font-mono text-carbon-textSub";

  return (
    <Card title={t("cloud.title")} hueIndex={hueIndex} nested={nested}>
      {/* A failed read used to be invisible. It has to be on screen, because
          the card refuses to save until it succeeds. */}
      {loadErr && <span className="text-xs text-statusFail">{t("settings.notLoadedNoSave")}</span>}
      {/* GlimStone follow-up pass (Phase 2 Task 4's remainder): stays permanent
          text, NOT bubbled — it is the only complete reference for all four
          remote-URL prefixes this card's credentials unlock (s3:/rest:/b2:/
          sftp:), used on a DIFFERENT tab's Backup Path fields. Those fields'
          own placeholder only ever shows two of the four (s3:/rest:), so this
          paragraph is the sole place b2: and sftp: are documented at all —
          exactly the "exact path syntax they need to copy correctly" carve-out
          the task spec names, same reasoning as RcloneCard's own pathHint. */}
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
          <select value={c.s3StorageClass} onChange={(e) => setImmediate("s3StorageClass", e.target.value)} className={inputCls}>
            <option value="">{t("cloud.storageClass.default")}</option>
            <option value="STANDARD">STANDARD</option>
            <option value="STANDARD_IA">STANDARD_IA</option>
            <option value="ONEZONE_IA">ONEZONE_IA</option>
            <option value="INTELLIGENT_TIERING">INTELLIGENT_TIERING</option>
            <option value="GLACIER_IR">GLACIER_IR</option>
          </select></label>
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
