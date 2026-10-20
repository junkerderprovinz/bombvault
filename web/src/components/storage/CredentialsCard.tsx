import { useState } from "react";
import { useT } from "../../lib/i18n";
import { formatList } from "../../lib/placement";
import { defaultSignIn, signInOption, signInsFor, useCloudCredSets } from "../../lib/useCloudCredSets";
import { Card } from "../../pages/settings/shared";
import { Button } from "../Button";
import { SelectField } from "../SelectField";
import { Rows, SettingRow } from "./rows";
import { SignInWindow } from "./SignInWindow";
import type { LocationEdit } from "./useLocationEdit";

/**
 * CredentialsCard shows which saved sign-in a location signs in with. One
 * that names none uses the default. Edit opens the sign-in for every location
 * that uses it, in the window the Sign-ins card opens too.
 */
export function CredentialsCard({ edit, kind, hueIndex }: { edit: LocationEdit; kind: "s3" | "rest"; hueIndex: number }) {
  const { t, lang } = useT();
  const { location, can, save, busy } = edit;
  const sets = useCloudCredSets();
  const [editing, setEditing] = useState(false);
  const ref = location.credsRef ?? "";
  const current = sets.find((s) => s.id === ref) ?? (ref === "" ? defaultSignIn(sets) : undefined);
  const offered = signInsFor(sets, kind, current?.id);
  const others = (current?.usedBy ?? []).filter((user) => user.id !== location.id);
  const identity = (kind === "s3" ? current?.s3KeyId : current?.restUser) || undefined;

  return (
    <Card
      title={t("storage.credentials")}
      hint={t("storage.credentialsHint")}
      hueIndex={hueIndex}
      newId="location-sign-in"
      foot={current && <Button label={t("offsite.targets.edit")} labelKey="offsite.targets.edit" tone="neutral" onClick={() => setEditing(true)} />}
    >
      <Rows>
        <SettingRow label={t("storage.creds.used")} hint={t("storage.creds.usedHint")} note={identity}>
          {can("credsRef") && offered.length > 0 ? (
            <SelectField
              value={current?.id ?? ""}
              onChange={(next) => next !== current?.id && void save({ credsRef: next })}
              label={t("storage.creds.used")}
              disabled={busy}
              options={offered.map((s) => ({ value: s.id, label: signInOption(t, s) }))}
              className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus"
            />
          ) : (
            <span className="text-sm text-carbon-textSub wrap-anywhere">{current ? signInOption(t, current) : t("signIns.none")}</span>
          )}
        </SettingRow>
        {others.length > 0 && (
          <SettingRow label={t("storage.creds.alsoUsedBy")}>
            <span className="text-sm text-carbon-textSub wrap-anywhere">{formatList(lang, others.map((user) => user.name))}</span>
          </SettingRow>
        )}
      </Rows>
      {editing && current && <SignInWindow set={current} sets={sets} onClose={() => setEditing(false)} />}
    </Card>
  );
}
