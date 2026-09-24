import { useState } from "react";
import { replicateOffsite, testOffsite } from "../../../lib/api";
import { useOffsiteTargets, type OffsiteDomain } from "../../../lib/useOffsiteTargets";
import { alsoDirectText } from "../../../lib/directRepo";
import { RcloneCard } from "../RcloneCard";
import { CloudCard } from "../CloudCard";
import { NumberField } from "../../../components/NumberField";
import { OffsiteWizard } from "../../../components/OffsiteWizard";
import { OffsiteLocationInput } from "../../../components/placement/OffsiteLocationInput";
import { InfoBubble } from "../../../components/InfoBubble";
import { OffsiteTargetsSection } from "../../../components/OffsiteTargetsSection";
import { Button } from "../../../components/Button";
import type { Settings } from "../../../lib/api";
import { useT } from "../../../lib/i18n";
import { useToast } from "../../../lib/toast";
import { REPO_LOCAL_HINT_LTR_FRAGMENTS, withLtrFragments } from "../../../lib/ltrFragments";
import { IconCheckCircle, IconSync, IconGear, IconClose } from "../../../components/Sidebar";
import { Card } from "../shared";
import { CloudCredSetsCard } from "../CloudCredSetsCard";
import type { SettingsTabProps } from "./types";

function ReplicateNowButton({
  domain,
  t,
  hueIndex,
}: {
  domain: OffsiteDomain;
  t: ReturnType<typeof useT>["t"];
  /** Offsite-tab card-split follow-up (jdp: "Die Buttons Verbindung testen,
   *  Jetzt replizieren, Einrichten, Ziel hinzufügen in die Farbengine
   *  aufnehmen"): this button's own enclosing per-domain offsite Card's hue
   *  position, the SAME value that Card's own `hueIndex` already got, not a
   *  second independent value, matching every other "thread the enclosing
   *  Card's own hueIndex straight through" call site in this file (e.g.
   *  ContainersSection's CadenceBuilder). `tone="active"` below (not
   *  "neutral", this button's old plain-grey identity) is what makes a
   *  passed hueIndex actually visible, see Badge.tsx's own `hueOn` comment
   *  for why "active" is the one non-heading tone `hueIndex` drives. Still
   *  true after the icon-badge conversion above: `tone` only ever governed
   *  the background wash, never the (now-neutral) glyph ink. */
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
    // The label is the STABLE name; the running state rides in `title` and in
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

// TestConnectionButton probes a domain's off-site repo (reachable / initialised)
// without modifying it, showing the verdict inline, so the user can verify the
// configured location before relying on it.
// GlimStone follow-up round: converted to a square icon-only badge (IconCheckCircle)
// the same way as ReplicateNowButton above, see that function's own comment for
// the full "coloured text -> neutral glyph, wash -> solid fill" writeup;
// the multiTarget-dependent "Test connection"/"Test PRIMARY connection" swap
// survives unchanged, just as `tip` content instead of visible text.
function TestConnectionButton({
  domain,
  t,
  hueIndex,
}: {
  domain: OffsiteDomain;
  t: ReturnType<typeof useT>["t"];
  /** See ReplicateNowButton's own doc above, identical offsite-tab
   *  card-split follow-up, same enclosing Card's hueIndex threaded through,
   *  same tone="active" reasoning. */
  hueIndex?: number;
}) {
  const { push } = useToast();
  const [busy, setBusy] = useState(false);
  // This button probes the PRIMARY target only. Once a domain has more than one
  // off-site copy, say so on the label, an unqualified "Test connection" going
  // green while a second destination was broken is exactly what issue #138
  // reported. Each additional target has its own button in OffsiteTargetsSection.
  const multiTarget = useOffsiteTargets(domain).length > 1;
  async function go() {
    setBusy(true);
    try {
      const r = await testOffsite(domain);
      if (r.ok && r.reachable && r.initialized) {
        push(t("offsite.testOk"), "success");
      } else if (r.ok && r.reachable) {
        push(t("offsite.testUninitialized"), "warn");
      } else {
        push(r.error ?? t("offsite.testFailed"), "fail");
      }
    } catch (e) {
      push(e instanceof Error ? e.message : t("offsite.testFailed"), "fail");
    } finally {
      setBusy(false);
    }
  }
  return (
    <Button
      label={t("offsite.test")}
      labelKey="offsite.test"
      glyph={<IconCheckCircle />}
      tone="accent"
      hueIndex={hueIndex}
      onClick={() => void go()}
      disabled={busy}
      busy={busy}
      // With several destinations this button probes the PRIMARY one, which is
      // worth saying but is not a different button: as a label it would change
      // this control's width the moment a second destination is added.
      title={multiTarget ? t("offsite.testPrimary") : undefined}
    />
  );
}

export function OffsiteTab({
  t,
  advanced,
  allTargets,
  fieldDirects,
  settings,
  setSettings,
  setOffsiteSaveState,
  setOffsiteSaveError,
  offsiteWizard,
  setOffsiteWizard,
  setLimSaveState,
  setLimSaveError,
  save,
  saveOffsiteRetention,
  debouncedSave,
}: SettingsTabProps) {
  let hueSeq = 0;
  const nextHue = () => hueSeq++;

  return (
    <>
      {/* ------------------------------------------------------------------ */}
      {/* OFFSITE: Off-site copy (restic copy replication)                   */}
      {/* Default-mode feature (v4): off-site + ransomware protection is a      */}
      {/* first-class flow, not advanced-only. Deep-linked via /settings#offsite */}
      {/* selects this tab (id kept for back-compat).                          */}
      {/* ------------------------------------------------------------------ */}
      <div id="offsite" className="flex flex-col gap-6">
      {/* Live-review round ("Bei der ersten Card überlappen sich zwei
          Cardtitelbadges. Können wir für Container, VMs, Flash, Ordner
          jeweils eine eigene Card machen?"): this used to be a group-heading
          `<h2>` badge (offsite.sectionTitle) immediately followed by ONE
          shared Card whose body looped over all four domains, the group
          badge's own `-top-[11px]` notch and the Card's own `-top-[11px]`
          notch, only `gap-6` (24px) apart with the group heading rendering
          at ZERO height (its only child is `position: absolute`, so it
          contributes nothing to flow height, see Card's own comment on why
          the badge straddles the card's top edge this way), landed the two
          22px-tall badges overlapping by several px. Splitting into four
          per-domain Cards does NOT fix that geometry on its own, the FIRST
          new Card would sit exactly `gap-6` below the same zero-height group
          heading, reproducing the identical overlap (verified against the
          live math before shipping this, not just assumed). The actual fix
          is the one this file's own Schedules tab already took this same
          round for the identical shape (see that tab's own history: its
          "Backup-Zeitpläne" group heading was removed because "these Cards
          already carry their own clear headings, so the group label was
          redundant"), dropping the group heading entirely, now that each
          of the four Cards below carries an unambiguous
          "OFFSITE-KOPIE <DOMAIN>" title of its own. offsite.sectionTitle
          had no other call site, so it's gone from i18n.ts (en/de) and all
          24 locale files, the same mechanical removal as
          settings.schedulesBackup got.
          `gap-6` on this wrapper: Card's own outer `<div>` no longer needs
          `relative` here (there is no sibling group-heading badge left to
          coexist with), and the four per-domain Cards need the SAME
          vertical rhythm every other multi-Card tab in this file already
          gets from the shared `<div className="flex flex-col gap-6">`
          wrapping the whole tab body two levels up, this nested wrapper
          exists only because `id="offsite"` (the deep-link anchor,
          `/settings#offsite`) needs a real element to attach to, not a
          Fragment. */}
      {/* Self-backup ("config") sits here with the rest since #176 (kramttocs:
          "Self-Backup should probably be more closely related to the other
          Off-site sections"). It was never a lesser domain in the backend, it
          has had configOffsite, its own targets and its own primary-remote row
          all along. It was simply missing from this list, so it alone got a
          bare URL field on its own page instead of a wizard, a connection test
          and per-destination credentials. */}
      {([
        ["containersOffsite", "nav.containers", "containers"],
        ["vmsOffsite", "nav.vms", "vms"],
        ["flashOffsite", "nav.flash", "flash"],
        ["filesOffsite", "nav.files", "files"],
        ["configOffsite", "nav.config", "config"],
      ] as const).map(([repoKey, label, domain]) => {
        const wizardOpen = offsiteWizard === domain;
        // This domain's OWN rainbow position: the SAME value fed to this
        // Card's own heading notch below AND to every clickable control
        // inside it (TestConnectionButton/ReplicateNowButton/the Einrichten
        // toggle/OffsiteTargetsSection's own "Ziel hinzufügen" button), per
        // jdp's explicit ask ("Die Buttons ... in die Farbengine
        // aufnehmen"), not four independent nextHue() calls, which would
        // desync a domain's own action buttons from its own Card's colour.
        const hueIdx = nextHue();
        const fieldTarget = allTargets.find((x) => x.domain === domain && x.sortOrder === 0);
        return (
        <Card key={repoKey} title={t("offsite.copyDomainTitle").replace("{domain}", t(label))} hueIndex={hueIdx}>
          {/* GlimStone follow-up pass: the one genuine toss-up in this pass,
              left as permanent text rather than force a call. It names two
              backend URL prefixes (rest:/s3:), but that's only a partially
              unique reference: the field's own placeholder already shows a
              rest: example, and offsite.repoLocalHint right below each field
              already documents the relative-path option. What it adds beyond
              those is s3: as a valid prefix here specifically, real
              but thinner value than RcloneCard's/CloudCard's own hints above
              (the sole documentation of their syntax anywhere). Whether that
              remainder is enough to justify a permanent paragraph, or should
              fold into the placeholder/caption instead, is a real design call,
              not a mechanical one, flagged rather than decided here.
              CARD-SPLIT FOLLOW-UP: this text applies identically to all four
              domains (it's about repo URL syntax, not domain-specific), so it
              stays a ONE-TIME read rather than repeating verbatim in every
              new Card, shown once, in the first (Containers) Card only. */}
          {domain === "containers" && (
            <p className="text-xs text-carbon-textMuted -mt-1">{t("settings.offsiteHint")}</p>
          )}
          <div className="flex flex-col gap-1">
            <div className="flex items-center justify-between">
              <span className="text-xs text-carbon-textSub">{t(label)}</span>
              <span className="inline-flex items-center gap-2">
                {settings[repoKey] && !wizardOpen && (
                  <>
                    <TestConnectionButton domain={domain} t={t} hueIndex={hueIdx} />
                    <ReplicateNowButton domain={domain} t={t} hueIndex={hueIdx} />
                  </>
                )}
                {/* GlimStone follow-up round (jdp, live review: "Können wir die
                    Buttons in quadratische Badges mit Glyphen umwandeln?"), a
                    square icon-only badge, IconGear when the wizard is closed
                    (offering to open setup) swapping to IconClose when it's
                    open, the exact same open/closed condition that used to
                    swap the button's own visible text between
                    "Einrichten…"/"Schließen". Both strings survive unchanged
                    as the `tip` tooltip's content instead, see
                    ReplicateNowButton's own comment above for the full
                    "coloured text -> neutral glyph, wash -> solid fill"
                    writeup this shares. */}
                {/* The one place a swapping label is right: open and close are
                    two different actions with two different glyphs, not one
                    action reporting its state. Both names are short enough to
                    share a width stage, so the control does not jump. */}
                <Button
                  label={wizardOpen ? t("offsite.wizard.close") : t("offsite.wizard.setup")}
                  labelKey={wizardOpen ? "offsite.wizard.close" : "offsite.wizard.setup"}
                  glyph={wizardOpen ? <IconClose /> : <IconGear />}
                  tone="accent"
                  hueIndex={hueIdx}
                  onClick={() => setOffsiteWizard(wizardOpen ? null : domain)}
                />
              </span>
            </div>
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
                  placeholder="rest:http://host:8000/repo"
                  className="rounded-control bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text font-mono glim-field-focus text-start"
                  onSave={(v) => save({ [repoKey]: v } as Partial<Settings>, setOffsiteSaveState, setOffsiteSaveError)}
                />
                {/* A mounted share is a perfectly valid off-site target, but the
                    placeholder only ever showed a REST URL, so nothing told the
                    operator a bare relative path works here (issue #138). */}
                <span className="text-xs text-carbon-textMuted">
                  {withLtrFragments(t("offsite.repoLocalHint"), REPO_LOCAL_HINT_LTR_FRAGMENTS)}
                </span>
              </>
            )}
            {/* Additional off-site targets (multi-off-site): extra copies of this
                domain beyond the primary editor above, managed via the CRUD API.
                hueIndex threaded through for the same "Ziel hinzufügen" button,
                see that component's own comment. */}
            <OffsiteTargetsSection domain={domain} t={t} hueIndex={hueIdx} />
          </div>
        </Card>
        );
      })}
      </div>

      {/* ------------------------------------------------------------------ */}
      {/* OFFSITE: Retention (off-site repo only; local retention now lives    */}
      {/* in the Storage tab, #51).                                            */}
      {/* ------------------------------------------------------------------ */}
      <Card
        title={t("settings.retentionOffsiteTitle")}
        // Same fix as the local-retention Card above, folding all three
        // sentences (what this Card does, the OR-combination rule, and the
        // immutable-destination override) into the one title-level bubble.
        hint={`${t("settings.retentionOffsiteHint")} ${t("settings.retentionCombineInfo")} ${t("settings.retentionImmutableNotPruned")}`}
        hueIndex={nextHue()}
      >
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          {([
            ["offsiteRetentionKeepLast", "settings.retentionLast", "settings.retentionLastInfo"],
            ["offsiteRetentionKeepDaily", "settings.retentionDaily", "settings.retentionDailyInfo"],
            ["offsiteRetentionKeepWeekly", "settings.retentionWeekly", "settings.retentionWeeklyInfo"],
            ["offsiteRetentionKeepMonthly", "settings.retentionMonthly", "settings.retentionMonthlyInfo"],
          ] as const).map(([key, label, info]) => (
            <label key={key} className="flex flex-col gap-1">
              <span className="flex items-center gap-1 text-xs text-carbon-textSub">
                {t(label)}
                <InfoBubble tip={t(info)} />
              </span>
              <NumberField
                min={0}
                value={settings[key]}
                onChange={(e) => {
                  const n = Math.max(0, parseInt(e.target.value, 10) || 0);
                  setSettings((prev) => (prev ? { ...prev, [key]: n } : prev));
                  debouncedSave(key, () => void saveOffsiteRetention(key, n));
                }}
                className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full glim-field-focus"
              />
            </label>
          ))}
        </div>
        {fieldDirects.map((u) => (
          <p key={u.target.id} className="mt-2 text-xs text-carbon-textMuted">
            {alsoDirectText(t, u)}
          </p>
        ))}
      </Card>

      {/* ------------------------------------------------------------------ */}
      {/* OFFSITE: Off-site bandwidth                                         */}
      {/* ------------------------------------------------------------------ */}
      {/* `advanced &&` inline, not the <Advanced> wrapper, see the Storage
          tab's cacheTitle Card (above) for why: the wrapper's children are
          already built before it decides whether to render them, so a
          hueIndex={nextHue()} inside it fires every render regardless. */}
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

      {/* ------------------------------------------------------------------ */}
      {/* OFFSITE: Off-site backends (rclone + cloud credentials). Same      */}
      {/* "not advanced-only" rule as the off-site repo-path Card above: a   */}
      {/* user can't actually USE an rclone:/s3:/rest: off-site URL without  */}
      {/* these credentials, so hiding them behind Advanced silently broke   */}
      {/* off-site setup for Simple-mode users (they'd only find these two   */}
      {/* cards by way of the Recovery page, which never gated them either). */}
      {/* ------------------------------------------------------------------ */}
      <RcloneCard t={t} hueIndex={nextHue()} />

      <CloudCard t={t} hueIndex={nextHue()} />
      <CloudCredSetsCard t={t} hueIndex={nextHue()} />
    </>
  );
}
