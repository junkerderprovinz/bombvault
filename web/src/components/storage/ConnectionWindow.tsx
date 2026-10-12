import { useEffect, useMemo, useState } from "react";
import { getCloud, getRcloneSections, type RcloneSection } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { placementErrorText } from "../../lib/placementCodes";
import { connectionPatch, storedConnection, type StoredSignIn } from "../../lib/storageConnection";
import { rcloneRemote } from "../../lib/storageLocations";
import { useCloudCredSets } from "../../lib/useCloudCredSets";
import type { ProviderForm } from "../../lib/useProviders";
import { useTestVerdict } from "../../lib/useTestVerdict";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { Button } from "../Button";
import { RemoteFolderBrowser } from "../destinations/RemoteFolderBrowser";
import { DraftField, masked } from "../destinations/steps";
import { formFields } from "../destinations/useDestinationDraft";
import { IconClose } from "../navGlyphs";
import { TestButton, VerdictLine } from "../TestButton";
import { Toggle } from "../Toggle";
import type { LocationEdit } from "./useLocationEdit";

/**
 * ConnectionWindow changes how a destination is reached: the fields its
 * provider's form has and the bucket or folder. The server tries the changed
 * connection before it saves, so the window stays open until that passed.
 */
export function ConnectionWindow({ edit, form, onClose }: { edit: LocationEdit; form: ProviderForm; onClose: () => void }) {
  const { t, lang } = useT();
  const { location } = edit;
  const { provider, backend } = form;
  const sets = useCloudCredSets();
  const [shared, setShared] = useState<StoredSignIn>({});
  const [section, setSection] = useState<RcloneSection>();
  const [typed, setTyped] = useState<Record<string, string>>({});
  const [dir, setDir] = useState<string | null>(null);
  const [showMore, setShowMore] = useState(false);
  const test = useTestVerdict([typed, dir], t("offsite.testFailed"));
  const remote = rcloneRemote(location.where);

  useEffect(() => {
    let alive = true;
    if (provider.route === "rclone") {
      void getRcloneSections()
        .then((res) => {
          if (alive && res.ok) setSection(res.sections?.find((s) => s.name === remote));
        })
        .catch(() => undefined);
    } else if (!location.credsRef) {
      void getCloud()
        .then((res) => {
          if (alive && res.ok) setShared(res);
        })
        .catch(() => undefined);
    }
    return () => {
      alive = false;
    };
  }, [provider.route, remote, location.credsRef]);

  const fields = useMemo(() => formFields(provider, backend), [provider, backend]);
  const signIn = sets.find((s) => s.id === location.credsRef) ?? shared;
  const stored = useMemo(() => storedConnection(provider, location.where, signIn, section), [provider, location.where, signIn, section]);
  const set = (name: string) => name in stored.values || stored.hidden.includes(name);
  const more = fields.more.filter((o) => showMore || set(o.name) || o.name in typed);
  const shown = [...fields.main, ...more];
  const browsable = provider.route === "s3" || provider.route === "rclone";
  const folder = dir ?? stored.dir;
  const change = connectionPatch(shown, masked, stored, typed, folder);
  const changed = change.settings !== undefined || change.dir !== undefined;

  async function save() {
    const verdict = await test.run(async () => {
      const res = await edit.saveConnection(change);
      return res.ok ? { ok: true } : { ok: false, reason: placementErrorText(t, lang, res, "offsite.testFailed") };
    });
    if (verdict.ok) onClose();
  }

  return (
    <InstanceWindow
      title={t("storage.conn.title").replace("{name}", () => location.name)}
      facts={
        <code dir="ltr" className="break-all text-start font-mono text-xs">
          {location.where}
        </code>
      }
      onClose={onClose}
      footer={
        <>
          <Button label={t("common.cancel")} labelKey="common.cancel" glyph={<IconClose />} tone="neutral" onClick={onClose} />
          <TestButton
            label={t("storage.conn.save")}
            labelKey="storage.conn.save"
            tone="accent"
            test={test}
            disabled={!changed}
            onClick={() => void save()}
          />
        </>
      }
    >
      <div className="grid gap-3 md:grid-cols-2">
        {shown.map((option) => (
          <DraftField
            key={option.name}
            option={option}
            kept={stored.hidden.includes(option.name) ? t("cloud.secretSet") : undefined}
            draft={{
              picked: provider,
              settings: { ...stored.values, ...typed },
              setSetting: (name, value) => setTyped((seen) => ({ ...seen, [name]: value })),
            }}
          />
        ))}
        {browsable && (
          <div className="md:col-span-2">
            <RemoteFolderBrowser
              label={t("storage.folder")}
              hint={t("storage.add.folderTip")}
              value={folder}
              onChange={setDir}
              provider={provider.id}
              settings={{ ...stored.values, ...typed }}
              needsSecret={stored.hidden.some((name) => !typed[name])}
            />
          </div>
        )}
      </div>
      {fields.more.length > 0 && <Toggle label={t("dest.more")} checked={showMore} onChange={setShowMore} />}
      <VerdictLine verdict={test.verdict} />
    </InstanceWindow>
  );
}
