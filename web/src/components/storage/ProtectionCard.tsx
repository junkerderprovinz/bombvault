import { tamperTestOffsiteTarget, type Provider } from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { formatList } from "../../lib/placement";
import { relativeTime } from "../../lib/reltime";
import { DOMAIN_LABEL, sectionDomains } from "../../lib/storageLocations";
import { storageLocationsChanged } from "../../lib/useStorageLocations";
import { useTestVerdict, type Verdict } from "../../lib/useTestVerdict";
import { Card } from "../../pages/settings/shared";
import { providerName } from "../destinations/ProviderPicker";
import { TestButton, VerdictLine } from "../TestButton";
import { Toggle } from "../Toggle";
import { Rows, SettingRow } from "./rows";
import { LockScene } from "./scenes";
import type { LocationEdit } from "./useLocationEdit";

const NOTE = "rounded-control px-3 py-2 text-xs leading-relaxed";

const NOTE_TONE = {
  ok: "bg-statusOkBg text-statusOk",
  warn: "bg-statusWarnBg text-statusWarn",
  neutral: "bg-carbon-surface2 text-carbon-textSub",
} as const;

/** Who clears old backups out of a copy, and whether anyone does. */
function pruner(immutable: boolean, keepsAll: boolean): { tone: keyof typeof NOTE_TONE; text: TranslationKey } {
  if (immutable) return { tone: "neutral", text: "storage.lock.pruneFar" };
  if (keepsAll) return { tone: "warn", text: "storage.lock.pruneNone" };
  return { tone: "ok", text: "storage.lock.pruneHere" };
}

/**
 * ProtectionCard holds the delete protection of a location: the switch, a
 * picture of what it does, and for a rest-server the test that proves the far
 * side refuses a delete.
 */
export function ProtectionCard({ edit, provider, hueIndex }: { edit: LocationEdit; provider?: Provider; hueIndex: number }) {
  const { t, lang } = useT();
  const { location, can, save, busy } = edit;
  const { immutable, testable, lastTamper } = location.protection;
  const local = location.backend === "local";
  const targets = location.sections.flatMap((section) => (section.use === "copy" && section.targetId ? [section.targetId] : []));
  const tamper = useTestVerdict(location.id, t("offsite.tamperError"));
  const takesCopies = location.object === "destination" || location.object === "target";
  const keepsAll = !location.retention || Object.values(location.retention).every((n) => n <= 0);
  const prunes = pruner(immutable, keepsAll);
  const name = provider ? providerName(provider, t) : "";

  function toggle(next: boolean) {
    const domains = sectionDomains(location).map((domain) => t(DOMAIN_LABEL[domain]));
    const done =
      domains.length > 0
        ? t(next ? "storage.lock.onFor" : "storage.lock.offFor").replace("{domains}", () => formatList(lang, domains))
        : undefined;
    void save({ immutable: next }, done);
  }

  async function test(): Promise<Verdict> {
    let verdict: Verdict = { ok: true };
    for (const id of targets) {
      const res = await tamperTestOffsiteTarget(id);
      if (!res.ok) verdict = { ok: false, reason: res.error ?? t("offsite.tamperError") };
      else if (!res.testable) verdict = { ok: false, reason: t("offsite.tamperUnverifiable") };
      else if (!res.protected) verdict = { ok: false, reason: res.detail || t("offsite.tamperFail") };
      if (!verdict.ok) break;
    }
    storageLocationsChanged();
    return verdict;
  }

  return (
    <Card title={t("storage.protected")} hueIndex={hueIndex}>
      <LockScene place={location} locked={immutable} />
      <Rows>
        <SettingRow label={t("storage.lock.toggle")} hint={t(local ? "storage.lock.localHint" : "dest.immutableHint")}>
          {can("immutable") ? (
            <Toggle hideLabel label={t("storage.lock.toggle")} checked={immutable} disabled={busy} onChange={toggle} />
          ) : (
            <span className="text-sm text-carbon-textSub">{t(immutable ? "storage.on" : "storage.off")}</span>
          )}
        </SettingRow>

        {immutable && testable && targets.length > 0 && (
          <SettingRow
            label={t("storage.lock.test")}
            hint={t("storage.lock.testHint")}
            note={
              lastTamper &&
              t(lastTamper.protected ? "storage.lock.lastRefused" : "storage.lock.lastOpen").replace(
                "{when}",
                relativeTime(t, lastTamper.at)
              )
            }
          >
            <div className="flex flex-col items-end gap-1.5">
              <VerdictLine verdict={tamper.verdict} />
              <TestButton
                label={t("storage.lock.testNow")}
                labelKey="storage.lock.testNow"
                words="check"
                test={tamper}
                onClick={() => void tamper.run(test)}
              />
            </div>
          </SettingRow>
        )}
      </Rows>

      {provider && !provider.lock && !local && location.backend !== "rest" && (
        <p className={`${NOTE} ${NOTE_TONE.warn}`}>{t("dest.lock.none").replace("{name}", () => name)}</p>
      )}
      {provider && location.backend === "s3" && (
        <p className={`${NOTE} ${NOTE_TONE.neutral}`}>{t("dest.lock.s3").replace("{name}", () => name)}</p>
      )}

      {takesCopies && (
        <div className="flex flex-col gap-2">
          <span className="text-xs font-semibold uppercase tracking-widest text-carbon-textSub">{t("storage.lock.whoPrunes")}</span>
          <p className={`${NOTE} ${NOTE_TONE[prunes.tone]}`}>{t(prunes.text)}</p>
        </div>
      )}
    </Card>
  );
}
