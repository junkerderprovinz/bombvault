import { useState } from "react";
import { replicateOffsite, testOffsite } from "../../../lib/api";
import { useOffsiteTargets, type OffsiteDomain } from "../../../lib/useOffsiteTargets";
import { NumberField } from "../../../components/NumberField";
import { OffsiteWizard } from "../../../components/OffsiteWizard";
import { DestinationsCard } from "../DestinationsCard";
import { OffsiteLocationInput } from "../../../components/placement/OffsiteLocationInput";
import { CompressionSelector, saveCompression } from "../../../components/CompressionSelector";
import { OffsiteTargetsSection } from "../../../components/OffsiteTargetsSection";
import { StreamingCard } from "../StreamingCard";
import { TestButton, VerdictLine } from "../../../components/TestButton";
import { useTestVerdict } from "../../../lib/useTestVerdict";
import { offsiteVerdict } from "../../../lib/offsiteVerdict";
import { Button } from "../../../components/Button";
import type { Settings } from "../../../lib/api";
import { useT } from "../../../lib/i18n";
import { useToast } from "../../../lib/toast";
import { REPO_LOCAL_HINT_LTR_FRAGMENTS, withLtrFragments } from "../../../lib/ltrFragments";
import { useAdvanced } from "../../../lib/advanced";
import { IconCheck, IconSync, IconGear, IconClose } from "../../../components/Sidebar";
import { Card, hueCounter, type SaveState } from "../shared";
import { useSettings } from "../settingsStore";

function ReplicateNowButton({
  domain,
  t,
  hueIndex,
}: {
  domain: OffsiteDomain;
  t: ReturnType<typeof useT>["t"];
  /** Hue position of the enclosing domain card, so the button takes its colour. */
  hueIndex?: number;
}) {
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
    // The label is the stable name; the running state rides in `title` and in
    // the spinner. A label that changed to "Replicating…" mid-action would
    // resize the button while you look at it, which is what the width stages
    // exist to prevent (#178).
    <Button
      label={t("offsite.replicateNow")}
      labelKey="offsite.replicateNow"
      glyph={<IconSync />}
      tone="accent"
      hueIndex={hueIndex}
      onClick={() => void go()}
      disabled={busy}
      busy={busy}
      title={busy ? t("offsite.replicating") : undefined}
    />
  );
}

// OffsiteDomainBar heads a domain's off-site card: the connection test,
// replicate now and the setup switch, with the test's verdict line above them.
function OffsiteDomainBar({
  domain,
  label,
  repo,
  wizardOpen,
  onToggleWizard,
  t,
  hueIndex,
}: {
  domain: OffsiteDomain;
  label: string;
  repo: string;
  wizardOpen: boolean;
  onToggleWizard: () => void;
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
}) {
  // The test probes the primary target only, and each additional target has
  // its own Test in OffsiteTargetsSection (#138). With more than one
  // destination the tooltip says so; as a label it would change the button's
  // width the moment a second destination is added.
  const multiTarget = useOffsiteTargets(domain).length > 1;
  // Opening the wizard counts as a change: credentials edited there are not
  // part of `repo`.
  const test = useTestVerdict([repo, wizardOpen], t("offsite.testFailed"));
  return (
    <>
      <VerdictLine verdict={test.verdict} />
      <div className="flex items-center justify-between">
        <span className="text-xs text-carbon-textSub">{label}</span>
        <span className="inline-flex items-center gap-2">
          {repo && !wizardOpen && (
            <>
              <TestButton
                label={t("offsite.test")}
                labelKey="offsite.test"
                glyph={<IconCheck />}
                tone="accent"
                hueIndex={hueIndex}
                test={test}
                onClick={() => void test.run(async () => offsiteVerdict(await testOffsite(domain), t))}
                title={multiTarget ? t("offsite.testPrimary") : undefined}
              />
              <ReplicateNowButton domain={domain} t={t} hueIndex={hueIndex} />
            </>
          )}
          {/* The one place a swapping label is right: open and close are
              two different actions with two different glyphs, not one
              action reporting its state. Both names are short enough to
              share a width stage, so the control does not jump. */}
          <Button
            label={wizardOpen ? t("offsite.wizard.close") : t("offsite.wizard.setup")}
            labelKey={wizardOpen ? "offsite.wizard.close" : "offsite.wizard.setup"}
            glyph={wizardOpen ? <IconClose /> : <IconGear />}
            tone="accent"
            hueIndex={hueIndex}
            onClick={onToggleWizard}
          />
        </span>
      </div>
    </>
  );
}

export function OffsitePage() {
  const { t } = useT();
  const { advanced } = useAdvanced();
  const { settings, setSettings, savedBaseline, allTargets, save, debouncedSave } = useSettings();

  const [, setOffsiteSaveState] = useState<SaveState>("idle");
  const [, setOffsiteSaveError] = useState<string | null>(null);
  // Which domain's guided off-site setup wizard is expanded (null = none).
  const [offsiteWizard, setOffsiteWizard] = useState<OffsiteDomain | null>(null);

  const [, setLimSaveState] = useState<SaveState>("idle");
  const [, setLimSaveError] = useState<string | null>(null);

  const nextHue = hueCounter();

  return (
    <>
      {/* Off-site copies are part of the default view, since ransomware */}
      {/* protection depends on them. The id is the target of /settings/offsite. */}
      <div id="offsite" className="flex flex-col gap-6">
      <DestinationsCard hueIndex={nextHue()} />
      {/* Self-backup ("config") is listed with the other domains (#176): the
          backend gives it its own off-site repo and targets like any other,
          so it gets the wizard, the connection test and per-destination
          credentials too. */}
      {([
        ["containersOffsite", "nav.containers", "containers"],
        ["vmsOffsite", "nav.vms", "vms"],
        ["flashOffsite", "nav.flash", "flash"],
        ["filesOffsite", "nav.files", "files"],
        ["zfsOffsite", "nav.zfs", "zfs"],
        ["configOffsite", "nav.config", "config"],
      ] as const).map(([repoKey, label, domain]) => {
        const wizardOpen = offsiteWizard === domain;
        // One hue position per domain, shared by the card heading and every
        // control inside it, so a domain's buttons match its card.
        const hueIdx = nextHue();
        const fieldTarget = allTargets.find((x) => x.domain === domain && x.sortOrder === 0);
        return (
        <Card key={repoKey} title={t("offsite.copyDomainTitle").replace("{domain}", t(label))} hueIndex={hueIdx}>
          {/* The repo URL prefixes (rest:, s3:, b2:) are visible reference
              text. They apply to every domain, so they are shown once, in the
              first card. */}
          {domain === "containers" && (
            <p className="text-xs text-carbon-textMuted -mt-1">{t("settings.offsiteHint")}</p>
          )}
          <div className="flex flex-col gap-1">
            <OffsiteDomainBar
              domain={domain}
              label={t(label)}
              repo={settings[repoKey]}
              wizardOpen={wizardOpen}
              onToggleWizard={() => setOffsiteWizard(wizardOpen ? null : domain)}
              t={t}
              hueIndex={hueIdx}
            />
            {wizardOpen ? (
              <OffsiteWizard
                domain={domain}
                settings={settings}
                setSettings={setSettings}
                save={save}
                t={t}
                hueIndex={hueIdx}
              />
            ) : (
              <>
                <OffsiteLocationInput
                  domain={domain}
                  value={settings[repoKey]}
                  targetId={fieldTarget?.id}
                  targetName={fieldTarget?.name}
                  following={fieldTarget?.destinationId && fieldTarget.enabled ? fieldTarget.name : undefined}
                  placeholder="rest:http://host:8000/repo"
                  className="rounded-control bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text font-mono glim-field-focus text-start"
                  onSave={(v) => save({ [repoKey]: v } as Partial<Settings>, setOffsiteSaveState, setOffsiteSaveError)}
                  onFromDestination={(location, immutable) => {
                    // The server already holds both, so the next save of
                    // another card must not send the old values back.
                    const patch = { [repoKey]: location, [`${domain}OffsiteImmutable`]: immutable } as Partial<Settings>;
                    setSettings((s) => (s ? { ...s, ...patch } : s));
                    if (savedBaseline.current) savedBaseline.current = { ...savedBaseline.current, ...patch };
                  }}
                />
                {/* A mounted share is a valid off-site target, but the
                    placeholder shows a REST URL, so this says a bare relative
                    path works too (#138). */}
                <span className="text-xs text-carbon-textMuted">
                  {withLtrFragments(t("offsite.repoLocalHint"), REPO_LOCAL_HINT_LTR_FRAGMENTS)}
                </span>
                {settings[repoKey] && (
                  <CompressionSelector
                    value={settings.compression[`offsite:${domain}`]}
                    onChange={(c) => void saveCompression(`offsite:${domain}`, c, settings, setSettings, save)}
                  />
                )}
              </>
            )}
            {/* Extra copies of this domain beyond the primary above, managed
                through the CRUD API. */}
            <OffsiteTargetsSection domain={domain} t={t} hueIndex={hueIdx} />
          </div>
        </Card>
        );
      })}
      </div>

      {/* `advanced &&` inline, so a hidden card spends no hue slot. */}
      {advanced && (
      <Card title={t("settings.offsiteLimits")} hint={t("settings.limitHint")} hueIndex={nextHue()}>
        <div className="grid grid-cols-2 gap-3">
          {([
            ["offsiteLimitUpload", "settings.limitUpload"],
            ["offsiteLimitDownload", "settings.limitDownload"],
          ] as const).map(([key, label]) => (
            <label key={key} className="flex flex-col gap-1">
              <span className="text-xs text-carbon-textSub">{t(label)}</span>
              <NumberField
                min={0}
                value={settings[key]}
                onChange={(e) => {
                  const n = Math.max(0, parseInt(e.target.value, 10) || 0);
                  setSettings((prev) => (prev ? { ...prev, [key]: n } : prev));
                  debouncedSave(key, () => void save({ [key]: n } as Partial<Settings>, setLimSaveState, setLimSaveError));
                }}
                className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full glim-field-focus"
              />
            </label>
          ))}
        </div>
      </Card>
      )}

      {advanced && <StreamingCard t={t} hueIndex={nextHue()} />}
    </>
  );
}
