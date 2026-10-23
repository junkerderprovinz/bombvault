import { tamperTestOffsiteTarget, type Provider, type RetentionKeep } from "../../lib/api";
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
import { ownFolderSections } from "./OwnFoldersCard";
import { Rows, SettingRow } from "./rows";
import { LockScene } from "./scenes";
import type { LocationEdit } from "./useLocationEdit";

const NOTE = "rounded-control px-3 py-2 text-xs leading-relaxed";

const NOTE_TONE = {
  ok: "bg-statusOkBg text-statusOk",
  warn: "bg-statusWarnBg text-statusWarn",
  neutral: "bg-carbon-surface2 text-carbon-textSub",
} as const;

type Pruner = "far" | "none" | "here";

const PRUNER_TONE: Record<Pruner, keyof typeof NOTE_TONE> = { far: "neutral", none: "warn", here: "ok" };

const COPY_PRUNER: Record<Pruner, TranslationKey> = {
  far: "storage.lock.pruneFar",
  none: "storage.lock.pruneNone",
  here: "storage.lock.pruneHere",
};

const OWN_PRUNER: Record<Pruner, TranslationKey> = {
  far: "storage.lock.ownPruneFar",
  none: "storage.lock.ownPruneNone",
  here: "storage.lock.ownPruneHere",
};

/** Who clears old backups out of a repository, and whether anyone does. */
function pruner(immutable: boolean, keep: RetentionKeep | undefined): Pruner {
  if (immutable) return "far";
  if (!keep || Object.values(keep).every((n) => n <= 0)) return "none";
  return "here";
}

/**
 * ProtectionCard holds the delete protection of a location: the switch, a
 * picture of what it does, and for a rest-server the test that proves the far
 * side refuses a delete. A folder on this host cannot refuse anything to
 * whoever holds BombVault, so it gets the card only to turn an earlier
 * protection off.
 */
export function ProtectionCard({ edit, provider, hueIndex }: { edit: LocationEdit; provider?: Provider; hueIndex: number }) {
  const { t, lang } = useT();
  const { location, can, save, busy } = edit;
  const { immutable, testable, lastTamper } = location.protection;
  const local = location.backend === "local";
  const copies = location.sections.filter((section) => section.use === "copy" && section.targetId);
  // The server refuses to test a switched-off copy: its verdict would stand
  // for backups that no longer go there.
  const targets = copies.flatMap((section) => (section.enabled && section.targetId ? [section.targetId] : []));
  const tamper = useTestVerdict(location.id, t("offsite.tamperError"));
  const takesCopies = location.object === "destination" || location.object === "target";
  const prunes = pruner(immutable, location.retention);
  // An own folder ages by its section's keep rule after each backup, not by
  // this location's rule for copies.
  const own = new Map<Pruner, string[]>();
  for (const section of ownFolderSections(location)) {
    const by = pruner(section.immutable, section.retention);
    own.set(by, [...(own.get(by) ?? []), t(DOMAIN_LABEL[section.domain])]);
  }
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

        {immutable && testable && copies.length > 0 && (
          <SettingRow
            label={t("storage.lock.test")}
            hint={t("storage.lock.testHint")}
            note={
              targets.length === 0
                ? t("storage.lock.testOff")
                : lastTamper &&
                  t(lastTamper.protected ? "storage.lock.lastRefused" : "storage.lock.lastOpen").replace(
                    "{when}",
                    relativeTime(t, lastTamper.at)
                  )
            }
          >
            {targets.length > 0 && (
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
            )}
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
          {(copies.length > 0 || own.size === 0) && <p className={`${NOTE} ${NOTE_TONE[PRUNER_TONE[prunes]]}`}>{t(COPY_PRUNER[prunes])}</p>}
          {[...own].map(([by, domains]) => (
            <p key={by} className={`${NOTE} ${NOTE_TONE[PRUNER_TONE[by]]}`}>
              {t(OWN_PRUNER[by], domains.length).replace("{domains}", () => formatList(lang, domains))}
            </p>
          ))}
        </div>
      )}
    </Card>
  );
}
