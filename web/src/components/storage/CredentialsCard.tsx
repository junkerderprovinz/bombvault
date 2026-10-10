import { useEffect, useState } from "react";
import { getCloud, type CloudCredSet, type CloudCredSetInfo } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { pushSaveWarnings } from "../../lib/placementCodes";
import { useToast } from "../../lib/toast";
import { credSetDraft, credSetLabel, saveCredSet, useCloudCredSets } from "../../lib/useCloudCredSets";
import { useReveal } from "../../lib/useReveal";
import { randomId } from "../../lib/uuid";
import { Card } from "../../pages/settings/shared";
import { Button } from "../Button";
import { IconAdd } from "../navGlyphs";
import { RevealInput } from "../RevealInput";
import { SelectField } from "../SelectField";
import { Rows, SettingRow } from "./rows";
import type { LocationEdit } from "./useLocationEdit";

type Kind = "s3" | "rest";

const INPUT = "rounded-control bg-carbon-surface3 text-carbon-text text-sm px-3 py-1.5 glim-field-focus-well";
const LABEL = "flex min-w-0 flex-col gap-1 text-xs text-carbon-textSub";

function emptySet(): CloudCredSet {
  return { id: randomId(), name: "", s3KeyId: "", s3Secret: "", s3Region: "", restUser: "", restPassword: "", s3StorageClass: "" };
}

/** What a set signs in as, which is all of it that is ever shown again. */
function identity(kind: Kind, set: { s3KeyId?: string; restUser?: string }): string {
  return (kind === "s3" ? set.s3KeyId : set.restUser) ?? "";
}

function CredSetForm({
  kind,
  initial,
  stored,
  sets,
  onSaved,
  onCancel,
}: {
  kind: Kind;
  initial: CloudCredSet;
  /** The set as the server holds it, absent for a new one. */
  stored?: CloudCredSetInfo;
  sets: CloudCredSetInfo[];
  /** Given for a new set: it can be dropped again, and once saved the
   *  location has to be pointed at it. */
  onSaved?: (set: CloudCredSet) => void;
  onCancel?: () => void;
}) {
  const { t } = useT();
  const { push } = useToast();
  const reveal = useReveal();
  const [draft, setDraft] = useState(initial);
  const [saving, setSaving] = useState(false);
  const [shake, setShake] = useState(0);
  const set = <K extends keyof CloudCredSet>(key: K, value: CloudCredSet[K]) => setDraft((d) => ({ ...d, [key]: value }));
  const kept = (present: boolean | undefined) => (present ? t("cloud.secretSet") : "");

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
    push(t("dest.saved").replace("{name}", () => draft.name), "success");
    pushSaveWarnings(push, t, res.warnings);
    onSaved?.(draft);
  }

  return (
    <div className="my-3 flex flex-col gap-3 rounded-control bg-carbon-surface2 p-3 last:mb-0">
      {onCancel && <p className="text-xs text-carbon-textMuted">{t("storage.creds.newSetHint")}</p>}
      <div className="grid gap-3 sm:grid-cols-2">
        <label className={LABEL}>
          {t("cloud.credSets.name")}
          <input value={draft.name} onChange={(e) => set("name", e.target.value)} className={INPUT} />
        </label>
        {kind === "s3" ? (
          <>
            <label className={LABEL}>
              {t("dest.field.access_key_id")}
              <input value={draft.s3KeyId} onChange={(e) => set("s3KeyId", e.target.value)} spellCheck={false} dir="ltr" className={`${INPUT} text-start`} />
            </label>
            <label className={LABEL}>
              {t("dest.field.secret_access_key")}
              <RevealInput
                {...reveal}
                value={draft.s3Secret}
                onChange={(e) => set("s3Secret", e.target.value)}
                spellCheck={false}
                autoComplete="new-password"
                placeholder={kept(stored?.s3SecretSet)}
                wrapperClassName="w-full"
                className={INPUT}
              />
            </label>
            <label className={LABEL}>
              {t("dest.field.region")}
              <input value={draft.s3Region} onChange={(e) => set("s3Region", e.target.value)} spellCheck={false} dir="ltr" className={`${INPUT} text-start`} />
            </label>
          </>
        ) : (
          <>
            <label className={LABEL}>
              {t("dest.field.user")}
              <input value={draft.restUser} onChange={(e) => set("restUser", e.target.value)} spellCheck={false} dir="ltr" className={`${INPUT} text-start`} />
            </label>
            <label className={LABEL}>
              {t("dest.field.pass")}
              <RevealInput
                {...reveal}
                value={draft.restPassword}
                onChange={(e) => set("restPassword", e.target.value)}
                spellCheck={false}
                autoComplete="new-password"
                placeholder={kept(stored?.restPasswordSet)}
                wrapperClassName="w-full"
                className={INPUT}
              />
            </label>
          </>
        )}
      </div>
      <div className="flex flex-wrap justify-end gap-2">
        {onCancel && <Button label={t("common.cancel")} labelKey="common.cancel" tone="neutral" onClick={onCancel} />}
        <Button
          key={shake}
          label={t("settings.save")}
          labelKey="settings.save"
          tone="neutral"
          onClick={() => void save()}
          disabled={saving}
          busy={saving}
          className={shake ? "glim-shake" : ""}
        />
      </div>
    </div>
  );
}

/**
 * CredentialsCard shows which stored credentials a location signs in with. A
 * location uses the shared ones or a named set, and a set is edited here for
 * every location that uses it.
 */
export function CredentialsCard({ edit, kind, hueIndex }: { edit: LocationEdit; kind: Kind; hueIndex: number }) {
  const { t } = useT();
  const { location, can, save, busy } = edit;
  const sets = useCloudCredSets();
  const [shared, setShared] = useState<{ s3KeyId?: string; restUser?: string }>({});
  const [adding, setAdding] = useState<CloudCredSet | null>(null);
  const ref = location.credsRef ?? "";
  const current = sets.find((s) => s.id === ref);

  useEffect(() => {
    let alive = true;
    getCloud()
      .then((res) => {
        if (alive && res.ok) setShared(res);
      })
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, []);

  const name = current ? credSetLabel(t, current) : t("storage.creds.shared");

  return (
    <Card title={t("storage.credentials")} hint={t("storage.credentialsHint")} hueIndex={hueIndex}>
      <Rows>
        <SettingRow label={t("storage.creds.used")} hint={t("storage.creds.usedHint")} note={identity(kind, current ?? shared)}>
          {can("credsRef") ? (
            <>
              <SelectField
                value={ref}
                onChange={(next) => void save({ credsRef: next })}
                label={t("storage.creds.used")}
                disabled={busy}
                options={[
                  { value: "", label: t("storage.creds.shared") },
                  ...sets.map((s) => ({ value: s.id, label: credSetLabel(t, s) })),
                ]}
                className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus"
              />
              {!adding && (
                <Button
                  label={t("storage.creds.newSet")}
                  labelKey="storage.creds.newSet"
                  glyph={<IconAdd />}
                  tone="neutral"
                  onClick={() => setAdding(emptySet())}
                />
              )}
            </>
          ) : (
            <span className="text-sm text-carbon-textSub wrap-anywhere">{name}</span>
          )}
        </SettingRow>
        {adding ? (
          <CredSetForm
            key={adding.id}
            kind={kind}
            initial={adding}
            sets={sets}
            onCancel={() => setAdding(null)}
            onSaved={(set) => {
              setAdding(null);
              void save({ credsRef: set.id });
            }}
          />
        ) : (
          current && <CredSetForm key={current.id} kind={kind} initial={credSetDraft(current)} stored={current} sets={sets} />
        )}
      </Rows>
    </Card>
  );
}
