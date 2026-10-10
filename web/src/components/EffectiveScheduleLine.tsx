import { useT, type TranslationKey } from "../lib/i18n";
import { formatCadence } from "./CadenceBuilder";
import type { EffectiveSchedule } from "../lib/api";
import { useForeignScheduleZone, zoneLabel } from "../lib/scheduleZone";

// The sentence for each reason an item is switched off on purpose. Any other
// "none" is a schedule the scheduler cannot read, and stays a failure.
const CHOSEN_OFF: Partial<Record<NonNullable<EffectiveSchedule["reason"]>, TranslationKey>> = {
  "domain-off": "files.effectiveNoneDomainOff",
  excluded: "files.effectiveNoneExcluded",
  "override-off": "files.effectiveNoneOverrideOff",
  "schedule-off": "files.effectiveNoneScheduleOff",
};

export interface ScheduleSentence {
  text: string;
  tone: string;
}

/**
 * useScheduleSentence words what happens to one item, since "Include in
 * schedule", the domain schedule and Backup Everything interact. The server
 * computes the outcome (schedule.EffectiveFileSetSchedule) and this only
 * formats it. `domainLabelKey` is the schedule card's own title key, so the
 * sentence names the card the reader would go to.
 */
export function useScheduleSentence(): (
  effective: EffectiveSchedule,
  domainLabelKey: TranslationKey
) => ScheduleSentence | null {
  const { t, lang } = useT();
  const zone = useForeignScheduleZone();

  const cadence = (spec: string) => {
    const text = spec ? formatCadence(spec, t, lang) : "";
    if (!text || !zone) return text;
    return t("cadence.serverClock").replace("{when}", text).replace("{zone}", zoneLabel(zone));
  };

  return (effective, domainLabelKey) => {
    const when = cadence(effective.spec);
    switch (effective.kind) {
      case "none": {
        const chosen = effective.reason ? CHOSEN_OFF[effective.reason] : undefined;
        return {
          text: chosen
            ? t(chosen).replace("{domain}", t(domainLabelKey)).replace("{everything}", t("settings.everythingTitle"))
            : t("files.effectiveNone"),
          tone: chosen ? "text-carbon-textMuted" : "text-statusFail",
        };
      }
      case "both":
        return {
          text: t("files.effectiveBoth").replace("{when}", when).replace("{when2}", cadence(effective.alsoSpec)),
          tone: "text-statusWarn",
        };
      case "own":
        return { text: t("files.effectiveOwn").replace("{when}", when), tone: "text-carbon-textSub" };
      // The card names come from the cards' own title keys, so the sentence
      // matches them in every language.
      case "everything":
        return {
          text: t("files.effectiveEverything").replace("{when}", when).replace("{domain}", t("settings.everythingTitle")),
          tone: "text-carbon-textSub",
        };
      case "domain":
        return {
          text: t("files.effectiveDomain").replace("{when}", when).replace("{domain}", t(domainLabelKey)),
          tone: "text-carbon-textSub",
        };
      default:
        // An unknown kind from a newer backend: say nothing rather than guess.
        return null;
    }
  };
}

/** EffectiveScheduleLine shows that sentence as a line of its own. */
export function EffectiveScheduleLine({
  effective,
  domainLabelKey = "jobs.filesSection",
}: {
  effective?: EffectiveSchedule;
  domainLabelKey?: TranslationKey;
}) {
  const { t } = useT();
  const sentence = useScheduleSentence();
  const said = effective ? sentence(effective, domainLabelKey) : null;
  if (!said) return null;

  return (
    <p className={`text-xs ${said.tone}`}>
      <span className="text-carbon-textMuted">{t("files.effectiveLabel")}: </span>
      {said.text}
    </p>
  );
}
