import { useT, type TranslationKey } from "../lib/i18n";
import { formatCadence } from "./CadenceBuilder";
import type { EffectiveSchedule } from "../lib/api";
import { useForeignScheduleZone, zoneLabel } from "../lib/scheduleZone";

/**
 * EffectiveScheduleLine says in one sentence what happens to one item, since
 * "Include in schedule", the domain schedule and Backup Everything interact.
 * The server computes the outcome (schedule.EffectiveFileSetSchedule) and this
 * only formats it. `domainLabelKey` is the schedule card's own title key, so
 * the sentence names the card the reader would go to.
 */
export function EffectiveScheduleLine({
  effective,
  domainLabelKey = "jobs.filesSection",
}: {
  effective?: EffectiveSchedule;
  domainLabelKey?: TranslationKey;
}) {
  const { t, lang } = useT();
  const zone = useForeignScheduleZone();
  if (!effective) return null;

  const cadence = (spec: string) => {
    const text = spec ? formatCadence(spec, t, lang) : "";
    if (!text || !zone) return text;
    return t("cadence.serverClock").replace("{when}", text).replace("{zone}", zoneLabel(zone));
  };
  const when = cadence(effective.spec);
  const alsoWhen = cadence(effective.alsoSpec);

  let text: string;
  let tone: string;
  switch (effective.kind) {
    case "none":
      text = t("files.effectiveNone");
      tone = "text-statusFail";
      break;
    case "both":
      text = t("files.effectiveBoth").replace("{when}", when).replace("{when2}", alsoWhen);
      tone = "text-statusWarn";
      break;
    case "own":
      text = t("files.effectiveOwn").replace("{when}", when);
      tone = "text-carbon-textSub";
      break;
    // The card names come from the cards' own title keys, so the sentence
    // matches them in every language.
    case "everything":
      text = t("files.effectiveEverything").replace("{when}", when).replace("{domain}", t("settings.everythingTitle"));
      tone = "text-carbon-textSub";
      break;
    case "domain":
      text = t("files.effectiveDomain").replace("{when}", when).replace("{domain}", t(domainLabelKey));
      tone = "text-carbon-textSub";
      break;
    default:
      // An unknown kind from a newer backend: say nothing rather than guess.
      return null;
  }

  return (
    <p className={`text-xs ${tone}`}>
      <span className="text-carbon-textMuted">{t("files.effectiveLabel")}: </span>
      {text}
    </p>
  );
}
