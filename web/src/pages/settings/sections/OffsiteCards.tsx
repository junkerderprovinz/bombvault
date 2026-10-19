import { useRef, useState } from "react";
import { replicateOffsite, testOffsite } from "../../../lib/api";
import type { OffsiteDomain } from "../../../lib/useOffsiteTargets";
import { NumberField } from "../../../components/NumberField";
import { DomainGlyph, Rows, SettingRow } from "../../../components/storage/rows";
import { OffsiteWizard } from "../../../components/OffsiteWizard";
import { OffsiteLocationInput, type LocationField } from "../../../components/placement/OffsiteLocationInput";
import { CompressionSelector, saveCompression } from "../../../components/CompressionSelector";
import { OffsiteTargetsSection } from "../../../components/OffsiteTargetsSection";
import { TestButton, VerdictLine } from "../../../components/TestButton";
import { useTestVerdict } from "../../../lib/useTestVerdict";
import { offsiteVerdict } from "../../../lib/offsiteVerdict";
import { offQualifier } from "../../../lib/placement";
import { DOMAIN_LABEL } from "../../../lib/storageLocations";
import { Badge } from "../../../components/Badge";
import { Button } from "../../../components/Button";
import type { Settings } from "../../../lib/api";
import { useT } from "../../../lib/i18n";
import { useToast } from "../../../lib/toast";
import { REPO_LOCAL_HINT_LTR_FRAGMENTS, withLtrIsolates } from "../../../lib/ltrFragments";
import { InfoBubble } from "../../../components/InfoBubble";
import { IconCheck, IconSync, IconGear, IconClose } from "../../../components/Sidebar";
import { InstanceWindow } from "../../instances/InstanceWindow";
import { Card, type SaveState } from "../shared";
import { useSettings } from "../settingsStore";

type T = ReturnType<typeof useT>["t"];

// Self-backup ("config") is listed with the other sections: the backend gives
// it its own off-site repository and targets like any other.
const DOMAINS: OffsiteDomain[] = ["containers", "vms", "flash", "files", "zfs", "config"];

function ReplicateNowButton({ domain, t }: { domain: OffsiteDomain; t: T }) {
  const { push } = useToast();
  const [busy, setBusy] = useState(false);
  async function go() {
    setBusy(true);
    try {
      const r = await replicateOffsite(domain);
      if (r.ok) {
        push(t("offsite.replicateStarted"), "success");
      } else {
        push(r.error ?? t("offsite.replicateFailed"), "fail");
      }
    } catch (e) {
      push(e instanceof Error ? e.message : t("offsite.replicateFailed"), "fail");
    } finally {
      setBusy(false);
    }
  }
  return (
    // The running state rides in `title` and the spinner, so the label never
    // changes while the button works.
    <Button
      label={t("offsite.replicateNow")}
      labelKey="offsite.replicateNow"
      glyph={<IconSync />}
      onClick={() => void go()}
      disabled={busy}
      busy={busy}
      title={busy ? t("offsite.replicating") : undefined}
    />
  );
}

/**
 * CopyRow is one section's off-site copy on a line: where it goes, and the
 * test, replicate now and setup it has. A click on the row opens the rest.
 */
function CopyRow({
  domain,
  repo,
  target,
  more,
  setupOpen,
  onOpen,
  onSetup,
}: {
  domain: OffsiteDomain;
  repo: string;
  /** The off-site field's own target, once the server has one for it. */
  target?: { name: string; enabled: boolean; destinationId?: string };
  /** How many more storage locations the section copies to. */
  more: number;
  setupOpen: boolean;
  onOpen: () => void;
  onSetup: () => void;
}) {
  const { t } = useT();
  // Credentials edited in the setup are not part of `repo`, so opening it
  // counts as a change too.
  const test = useTestVerdict([repo, setupOpen], t("offsite.testFailed"));
  const where = target?.destinationId && target.enabled ? target.name : repo;
  return (
    <div className="relative flex flex-col gap-1.5 py-3 first:pt-0 last:pb-0">
      <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <div className="flex min-w-0 flex-1 basis-56 items-center gap-3">
          <DomainGlyph domain={domain} />
          <span className="flex min-w-0 flex-col gap-0.5">
            {/* The button's box is stretched over the row, so a click anywhere
                beside the actions opens the section's copy. */}
            <button
              type="button"
              onClick={onOpen}
              className="text-start text-sm font-semibold text-carbon-text after:absolute after:inset-0 after:rounded-control focus-visible:outline-none focus-visible:after:outline-solid focus-visible:after:outline-2 focus-visible:after:outline-(--focus-ring)"
            >
              {t(DOMAIN_LABEL[domain])}
            </button>
            <span className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-xs text-carbon-textMuted">
              {repo ? (
                <span dir="ltr" className="min-w-0 truncate text-start font-mono">
                  {where}
                </span>
              ) : (
                <span>{t("dashboard.noOffsite")}</span>
              )}
              {target && !target.enabled && (
                <Badge tone="muted" size="small">
                  {offQualifier(t)}
                </Badge>
              )}
              {more > 0 && (
                <span>
                  {t("offsite.targets.title")}: {more}
                </span>
              )}
            </span>
          </span>
        </div>
        <span className="relative z-10 inline-flex flex-wrap items-center gap-2">
          {repo && !setupOpen && (
            <>
              <TestButton
                label={t("offsite.test")}
                labelKey="offsite.test"
                glyph={<IconCheck />}
                test={test}
                onClick={() => void test.run(async () => offsiteVerdict(await testOffsite(domain), t))}
                title={more > 0 ? t("offsite.testPrimary") : undefined}
              />
              <ReplicateNowButton domain={domain} t={t} />
            </>
          )}
          <Button label={t("offsite.wizard.setup")} labelKey="offsite.wizard.setup" glyph={<IconGear />} onClick={onSetup} />
          <Button label={t("offsite.targets.edit")} labelKey="offsite.targets.edit" onClick={onOpen} />
        </span>
      </div>
      <VerdictLine verdict={test.verdict} />
    </div>
  );
}

/** OffsiteCards hold every section's off-site copy and the limits all copies
 *  share. */
export function OffsiteCards({ nextHue }: { nextHue: () => number }) {
  const { t } = useT();
  const { settings, setSettings, savedBaseline, allTargets, save, debouncedSave } = useSettings();

  const [, setOffsiteSaveState] = useState<SaveState>("idle");
  const [, setOffsiteSaveError] = useState<string | null>(null);
  // The section whose window is open, and the one whose setup is.
  const [details, setDetails] = useState<OffsiteDomain | null>(null);
  const [wizard, setWizard] = useState<OffsiteDomain | null>(null);
  const field = useRef<LocationField>(null);

  const [, setLimSaveState] = useState<SaveState>("idle");
  const [, setLimSaveError] = useState<string | null>(null);

  const hue = nextHue();
  const titleOf = (domain: OffsiteDomain) => t("offsite.copyDomainTitle").replace("{domain}", t(DOMAIN_LABEL[domain]));
  const primaryOf = (domain: OffsiteDomain) => allTargets.find((x) => x.domain === domain && x.sortOrder === 0);

  // A location typed but not yet saved is saved, or asked about, before the
  // window goes, as it is when the field loses the focus.
  async function closeDetails() {
    if ((await field.current?.commit()) !== false) setDetails(null);
  }

  function windowFor(domain: OffsiteDomain) {
    const repoKey = `${domain}Offsite` as const;
    const fieldTarget = primaryOf(domain);
    return (
      <InstanceWindow
        title={titleOf(domain)}
        hint={t("settings.offsiteHint")}
        onClose={() => void closeDetails()}
        footer={
          <Button label={t("common.close")} labelKey="common.close" glyph={<IconClose />} tone="accent" onClick={() => void closeDetails()} />
        }
      >
        <div className="flex flex-col gap-4">
          <div className="flex flex-col gap-2">
            <span className="flex items-center gap-1.5 text-sm text-carbon-text">
              {t("dest.location")}
              <InfoBubble tip={withLtrIsolates(t("offsite.repoLocalHint"), REPO_LOCAL_HINT_LTR_FRAGMENTS)} />
            </span>
            <OffsiteLocationInput
              ref={field}
              domain={domain}
              value={settings[repoKey]}
              targetId={fieldTarget?.id}
              targetName={fieldTarget?.name}
              following={fieldTarget?.destinationId && fieldTarget.enabled ? fieldTarget.name : undefined}
              placeholder="rest:http://host:8000/repo"
              className="rounded-control bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text font-mono glim-field-focus text-start"
              onSave={(v) => save({ [repoKey]: v } as Partial<Settings>, setOffsiteSaveState, setOffsiteSaveError)}
              onFromDestination={(location, immutable) => {
                // The server already holds both, so the next save of another
                // card must not send the old values back.
                const patch = { [repoKey]: location, [`${domain}OffsiteImmutable`]: immutable } as Partial<Settings>;
                setSettings((s) => (s ? { ...s, ...patch } : s));
                if (savedBaseline.current) savedBaseline.current = { ...savedBaseline.current, ...patch };
              }}
            />
          </div>
          {settings[repoKey] && (
            <CompressionSelector
              value={settings.compression[`offsite:${domain}`]}
              onChange={(c) => void saveCompression(`offsite:${domain}`, c, settings, setSettings, save)}
            />
          )}
          <OffsiteTargetsSection domain={domain} t={t} hueIndex={hue} />
        </div>
      </InstanceWindow>
    );
  }

  return (
    <>
      {/* The anchor old /settings/offsite links land on. */}
      <div id="offsite" className="flex scroll-mt-6 flex-col gap-6">
        <Card title={t("offsite.copies.title")} hint={t("settings.offsiteHint")} hueIndex={hue} newId="settings-offsite-copies">
          <Rows>
            {DOMAINS.map((domain) => (
              <CopyRow
                key={domain}
                domain={domain}
                repo={settings[`${domain}Offsite`]}
                target={primaryOf(domain)}
                more={allTargets.filter((x) => x.domain === domain && x.sortOrder > 0 && x.enabled).length}
                setupOpen={wizard === domain}
                onOpen={() => setDetails(domain)}
                onSetup={() => setWizard(domain)}
              />
            ))}
          </Rows>
        </Card>
        {details && windowFor(details)}
        {wizard && (
          <InstanceWindow
            title={titleOf(wizard)}
            onClose={() => setWizard(null)}
            footer={
              <Button
                label={t("offsite.wizard.close")}
                labelKey="offsite.wizard.close"
                glyph={<IconClose />}
                tone="accent"
                onClick={() => setWizard(null)}
              />
            }
          >
            <OffsiteWizard domain={wizard} settings={settings} setSettings={setSettings} save={save} t={t} hueIndex={hue} />
          </InstanceWindow>
        )}
      </div>

      <Card title={t("settings.offsiteLimits")} hint={t("settings.limitHint")} hueIndex={nextHue()}>
        <Rows>
          {([
            ["offsiteLimitUpload", "settings.limitUpload"],
            ["offsiteLimitDownload", "settings.limitDownload"],
          ] as const).map(([key, label]) => (
            <SettingRow key={key} label={t(label)} htmlFor={`offsite-${key}`}>
              <NumberField
                id={`offsite-${key}`}
                wrapperClassName="w-28"
                min={0}
                value={settings[key]}
                onChange={(e) => {
                  const n = Math.max(0, parseInt(e.target.value, 10) || 0);
                  setSettings((prev) => (prev ? { ...prev, [key]: n } : prev));
                  debouncedSave(key, () => void save({ [key]: n } as Partial<Settings>, setLimSaveState, setLimSaveError));
                }}
                className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full glim-field-focus"
              />
            </SettingRow>
          ))}
        </Rows>
      </Card>
    </>
  );
}
