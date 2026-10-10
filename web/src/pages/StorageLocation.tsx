// The page of one storage location: what it is, how it is reached, and every
// setting that belongs to the place as a whole. A location is read from one of
// four kinds of object, and a card shows only where its object has something
// to say.
import { Link, useParams } from "react-router-dom";

import { Badge } from "../components/Badge";
import { Button } from "../components/Button";
import { IconBack } from "../components/glyphs";
import { InfoBubble } from "../components/InfoBubble";
import { PageTitle } from "../components/PageTitle";
import { providerName } from "../components/destinations/ProviderPicker";
import { ConnectionCard } from "../components/storage/ConnectionCard";
import { CopyCard } from "../components/storage/CopyCard";
import { CredentialsCard } from "../components/storage/CredentialsCard";
import { KeepCard } from "../components/storage/KeepCard";
import { LocationMark } from "../components/storage/LocationMark";
import { ProtectionCard } from "../components/storage/ProtectionCard";
import { RcloneConfCard } from "../components/storage/RcloneConfCard";
import { useLocationEdit } from "../components/storage/useLocationEdit";
import { TestButton, VerdictLine } from "../components/TestButton";
import { testOffsiteTarget, testPrimaryRemote, type OkEnvelope, type StorageLocation } from "../lib/api";
import { useT } from "../lib/i18n";
import { offsiteVerdict } from "../lib/offsiteVerdict";
import { PAGE_SHELL_RESPONSIVE } from "../lib/pageShell";
import { DOMAIN_LABEL, amountText, credentialKind, rcloneRemote, usedByText } from "../lib/storageLocations";
import { settingsOwned, type LocationRecords } from "../lib/storageWrites";
import { useProvider } from "../lib/useProviders";
import { useStorageLocation } from "../lib/useStorageLocations";
import { useTestVerdict } from "../lib/useTestVerdict";
import { hueCounter } from "./settings/shared";

type Probe = () => Promise<OkEnvelope & { reachable?: boolean; initialized?: boolean }>;

/**
 * connectionProbe is the test that reaches this location, or null where the
 * server has none: a folder on this host needs no test, and a named repository
 * has no route for one. A destination is reached through the repository of one
 * of its sections.
 */
function connectionProbe(location: StorageLocation): Probe | null {
  const copy = location.sections.find((section) => section.use === "copy" && section.targetId);
  if (copy?.targetId) {
    const id = copy.targetId;
    return () => testOffsiteTarget(id);
  }
  if (location.object === "path" && location.backend !== "local" && location.sections.length > 0) {
    const { domain } = location.sections[0];
    return () => testPrimaryRemote(domain);
  }
  return null;
}

function Crumb() {
  const { t } = useT();
  return (
    <Link
      to="/storage"
      className="inline-flex items-center gap-1.5 self-start rounded-control text-sm text-carbon-textSub hover:text-carbon-text"
    >
      <IconBack />
      {t("storage.title")}
    </Link>
  );
}

function Location({ location, records }: { location: StorageLocation; records: LocationRecords }) {
  const { t, lang } = useT();
  const edit = useLocationEdit(location, records);
  const provider = useProvider(location.provider);
  const probe = connectionProbe(location);
  const test = useTestVerdict(location.id, t("offsite.testFailed"));
  const nextHue = hueCounter();

  const local = location.backend === "local";
  const what = provider ? providerName(provider, t) : t(local ? "storage.localFolder" : "source.offsite");
  const kind = location.object === "path" ? null : credentialKind(location.backend);
  const remote = location.backend === "rclone" ? rcloneRemote(location.where) : null;
  const takesCopies = location.object === "destination" || location.object === "target";
  const primary = settingsOwned(location) ? location.sections.find((section) => section.primary) : undefined;

  return (
    <>
      <PageTitle>{location.name}</PageTitle>
      <div className="flex flex-col gap-4">
        <Crumb />
        <div className="flex flex-wrap items-center gap-x-4 gap-y-3">
          <LocationMark location={location} size="head" />
          <div className="flex min-w-0 flex-1 flex-col gap-0.5">
            <span className="truncate text-xl font-semibold text-carbon-text">{location.name}</span>
            <span className="flex flex-wrap items-center gap-x-1.5 text-sm text-carbon-textMuted">
              <span>
                {what} · {usedByText(t, lang, location)}
              </span>
              {primary && (
                <InfoBubble tip={t("storage.primaryNote").replace("{domain}", () => t(DOMAIN_LABEL[primary.domain]))} />
              )}
            </span>
          </div>
          <div className="flex flex-wrap items-center gap-3">
            {location.enabled ? (
              <span className="text-sm tabular-nums text-carbon-textSub">{amountText(t, location.capacity)}</span>
            ) : (
              <Badge tone="neutral">{t("storage.off")}</Badge>
            )}
            {probe && (
              <TestButton
                label={t("offsite.test")}
                labelKey="offsite.test"
                test={test}
                onClick={() => void test.run(async () => offsiteVerdict(await probe(), t))}
              />
            )}
          </div>
        </div>
        <VerdictLine verdict={test.verdict} />
      </div>

      <ConnectionCard edit={edit} hueIndex={nextHue()} />
      {kind && <CredentialsCard edit={edit} kind={kind} hueIndex={nextHue()} />}
      {remote && <RcloneConfCard remote={remote} hueIndex={nextHue()} />}
      {location.object !== "path" && <ProtectionCard edit={edit} provider={provider} hueIndex={nextHue()} />}
      {takesCopies && <CopyCard edit={edit} hueIndex={nextHue()} />}
      {location.retention && <KeepCard edit={edit} retention={location.retention} hueIndex={nextHue()} />}
    </>
  );
}

function LocationView({ id }: { id: string }) {
  const { t } = useT();
  const { location, records, status, retry } = useStorageLocation(id);

  if (location) return <Location location={location} records={records} />;
  return (
    <>
      <PageTitle>{t("storage.title")}</PageTitle>
      <Crumb />
      {status === "missing" ? (
        <p className="text-sm text-carbon-textSub">{t("storage.gone")}</p>
      ) : status === "failed" ? (
        <div className="flex flex-col items-start gap-2">
          <p className="text-sm text-statusWarn">{t("storage.loadFailed")}</p>
          <Button label={t("anomaly.retry")} labelKey="anomaly.retry" onClick={retry} />
        </div>
      ) : (
        <p className="text-sm text-carbon-textSub">{t("dashboard.checking")}</p>
      )}
    </>
  );
}

export function StorageLocationPage() {
  const { id = "" } = useParams();
  return (
    <div className={PAGE_SHELL_RESPONSIVE}>
      {/* Keyed on the id, so one location's drafts and verdicts never show on another. */}
      <LocationView key={id} id={id} />
    </div>
  );
}
