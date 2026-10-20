import { useId, useState } from "react";
import { testSignIn, type CloudCredSet, type CloudCredSetInfo, type SignInCheck } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { offsiteVerdict } from "../../lib/offsiteVerdict";
import { formatList } from "../../lib/placement";
import { pushSaveWarnings } from "../../lib/placementCodes";
import { STORAGE_CLASSES } from "../../lib/storageClasses";
import { useToast } from "../../lib/toast";
import { credSetDraft, credSetLabel, saveCredSet } from "../../lib/useCloudCredSets";
import { useReveal } from "../../lib/useReveal";
import { useTestVerdict } from "../../lib/useTestVerdict";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { RevealInput } from "../RevealInput";
import { SelectField } from "../SelectField";
import { TestButton, VerdictLine, verdictGlyph, verdictKey } from "../TestButton";
import { Rows, SettingRow } from "./rows";

const INPUT = "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus";
const FIELD = `${INPUT} w-72 max-w-full`;
const HEAD = "text-xs font-semibold uppercase tracking-widest text-carbon-textSub";

/** One location's line of a test's answer. */
function CheckRow({ check }: { check: SignInCheck }) {
  const { t } = useT();
  const verdict = offsiteVerdict({ ok: true, reachable: check.reachable, initialized: check.initialized, error: check.error || undefined }, t);
  return (
    <SettingRow as="li" label={check.name} note={verdict.ok ? verdict.note : verdict.reason}>
      <Badge tone={verdict.ok ? "ok" : "fail"} size="small">
        <span className="flex [&_svg]:h-[1em] [&_svg]:w-[1em]">{verdictGlyph(verdict)}</span>
        {t(verdictKey(verdict, "connection"))}
      </Badge>
    </SettingRow>
  );
}

/**
 * SignInWindow edits a saved sign-in for every storage location that uses it.
 * Test signs in to each of them with the values as typed; Save does not wait
 * for a test. `sets` is the whole list, which every save resends.
 */
export function SignInWindow({ set, sets, onClose }: { set: CloudCredSetInfo; sets: CloudCredSetInfo[]; onClose: () => void }) {
  const { t, lang } = useT();
  const uid = useId();
  const { push } = useToast();
  const [draft, setDraft] = useState(() => credSetDraft(set));
  const [checks, setChecks] = useState<SignInCheck[]>([]);
  const [saving, setSaving] = useState(false);
  const [shake, setShake] = useState(0);
  const test = useTestVerdict(draft, t("offsite.testFailed"));
  const revealSecret = useReveal();
  const revealPassword = useReveal();
  const users = set.usedBy ?? [];
  // A sign-in holds the keys of one kind as a rule. One that holds none yet
  // offers both.
  const s3 = !!set.s3KeyId || !set.restUser;
  const rest = !!set.restUser || !set.s3KeyId;
  const edit = <K extends keyof CloudCredSet>(key: K, value: CloudCredSet[K]) => setDraft((d) => ({ ...d, [key]: value }));

  function runTest() {
    void test.run(async () => {
      const res = await testSignIn(draft);
      if (!res.ok) return { ok: false, reason: res.error ?? t("offsite.testFailed") };
      const found = res.locations ?? [];
      setChecks(found);
      return { ok: found.every((check) => check.reachable) };
    });
  }

  async function save() {
    if (!draft.name.trim()) {
      push(t("storage.creds.nameFirst"), "fail");
      setShake((n) => n + 1);
      return;
    }
    setSaving(true);
    const res = await saveCredSet(sets, draft).catch((e: unknown) => ({
      ok: false,
      error: e instanceof Error ? e.message : undefined,
      warnings: undefined,
    }));
    setSaving(false);
    if (!res.ok) {
      push(res.error ?? t("settings.error"), "fail");
      setShake((n) => n + 1);
      return;
    }
    push(t("settings.saved"), "success");
    pushSaveWarnings(push, t, res.warnings);
    onClose();
  }

  const s3Rows = (
    <Rows>
      <SettingRow label={t("dest.field.access_key_id")} htmlFor={`${uid}-key`}>
        <input id={`${uid}-key`} value={draft.s3KeyId} onChange={(e) => edit("s3KeyId", e.target.value)} spellCheck={false} dir="ltr" className={`${FIELD} text-start`} />
      </SettingRow>
      <SettingRow label={t("dest.field.secret_access_key")} htmlFor={`${uid}-secret`}>
        <RevealInput
          {...revealSecret}
          id={`${uid}-secret`}
          value={draft.s3Secret}
          onChange={(e) => edit("s3Secret", e.target.value)}
          spellCheck={false}
          autoComplete="new-password"
          placeholder={set.s3SecretSet ? t("cloud.secretSet") : ""}
          wrapperClassName="w-72 max-w-full"
          className={INPUT}
        />
      </SettingRow>
      <SettingRow label={t("dest.field.region")} htmlFor={`${uid}-region`}>
        <input id={`${uid}-region`} value={draft.s3Region} onChange={(e) => edit("s3Region", e.target.value)} spellCheck={false} dir="ltr" placeholder="us-east-1" className={`${FIELD} text-start`} />
      </SettingRow>
      <SettingRow label={t("cloud.storageClass.label")} hint={t("cloud.storageClass.hint")} htmlFor={`${uid}-class`}>
        <SelectField
          id={`${uid}-class`}
          value={draft.s3StorageClass}
          onChange={(v) => edit("s3StorageClass", v)}
          label={t("cloud.storageClass.label")}
          options={[{ value: "", label: t("cloud.storageClass.default") }, ...STORAGE_CLASSES.map((sc) => ({ value: sc, label: sc }))]}
          className={INPUT}
        />
      </SettingRow>
    </Rows>
  );

  const restRows = (
    <Rows>
      <SettingRow label={t("dest.field.user")} htmlFor={`${uid}-user`}>
        <input id={`${uid}-user`} value={draft.restUser} onChange={(e) => edit("restUser", e.target.value)} spellCheck={false} dir="ltr" className={`${FIELD} text-start`} />
      </SettingRow>
      <SettingRow label={t("dest.field.pass")} htmlFor={`${uid}-pass`}>
        <RevealInput
          {...revealPassword}
          id={`${uid}-pass`}
          value={draft.restPassword}
          onChange={(e) => edit("restPassword", e.target.value)}
          spellCheck={false}
          autoComplete="new-password"
          placeholder={set.restPasswordSet ? t("cloud.secretSet") : ""}
          wrapperClassName="w-72 max-w-full"
          className={INPUT}
        />
      </SettingRow>
    </Rows>
  );

  return (
    <InstanceWindow
      title={credSetLabel(t, set)}
      hint={t("signIns.windowHint")}
      facts={
        <span className="text-xs wrap-break-word">
          {users.length > 0 ? t("signIns.usedBy").replace("{list}", () => formatList(lang, users.map((u) => u.name))) : t("signIns.unused")}
        </span>
      }
      onClose={onClose}
      footer={
        <>
          <Button label={t("common.cancel")} labelKey="common.cancel" tone="neutral" onClick={onClose} />
          <TestButton
            label={t("signIns.test")}
            labelKey="signIns.test"
            test={test}
            disabled={users.length === 0}
            hint={users.length === 0 ? t("signIns.testUnused") : undefined}
            onClick={runTest}
          />
          <Button
            key={shake}
            label={t("settings.save")}
            labelKey="settings.save"
            tone="accent"
            onClick={() => void save()}
            disabled={saving}
            busy={saving}
            className={shake ? "glim-shake" : ""}
          />
        </>
      }
    >
      <Rows>
        <SettingRow label={t("cloud.credSets.name")} htmlFor={`${uid}-name`}>
          <input id={`${uid}-name`} value={draft.name} onChange={(e) => edit("name", e.target.value)} className={FIELD} />
        </SettingRow>
      </Rows>
      {s3 && rest ? (
        <>
          <span className={HEAD}>Amazon S3</span>
          {s3Rows}
          <span className={HEAD}>restic REST server</span>
          {restRows}
        </>
      ) : s3 ? (
        s3Rows
      ) : (
        restRows
      )}
      {test.verdict && checks.length > 0 && (
        <Rows as="ul">
          {checks.map((check) => (
            <CheckRow key={check.id} check={check} />
          ))}
        </Rows>
      )}
      <VerdictLine verdict={test.verdict} />
    </InstanceWindow>
  );
}
