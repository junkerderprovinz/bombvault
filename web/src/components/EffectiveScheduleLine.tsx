import { useT } from "../lib/i18n";
import { formatCadence } from "./CadenceBuilder";
import type { EffectiveSchedule } from "../lib/api";

// ---------------------------------------------------------------------------
// EffectiveScheduleLine (#199)
//
// One sentence per folder set: what actually happens to it.
//
// The Folders card carries three controls that all read like scheduling, and
// manilx put them together in the only way that looked sensible and got the
// opposite of what he wanted. "Include in schedule" reads as "part of the
// Folders schedule" but gates every automatic backup, so switching it off makes
// a set invisible to the domain job, to its own per-item entry AND to Backup
// Everything. Meanwhile the Folders schedule and Backup Everything can both
// cover the same set, which quietly doubles it.
//
// A hint text can explain those rules. It cannot say what THIS row does right
// now, and that is the only thing the reader wants to know. So the sentence is
// computed, on the server, by schedule.EffectiveFileSetSchedule, from the same
// four settings the scheduler reads. The interface only formats it: there is no
// second copy of the rule here to drift from the first.
//
// Two of the five outcomes are warnings, and they are the reason this exists:
// "not backed up automatically" is a set the user believes is protected, and
// "runs twice" is a set paying for two backups a day it never asked for.
// ---------------------------------------------------------------------------

export function EffectiveScheduleLine({ effective }: { effective?: EffectiveSchedule }) {
  const { t, lang } = useT();
  if (!effective) return null;

  const when = effective.spec ? formatCadence(effective.spec, t, lang) : "";
  const alsoWhen = effective.alsoSpec ? formatCadence(effective.alsoSpec, t, lang) : "";

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
    // The two names come from the keys the cards themselves already carry, not
    // from new copies: whatever the Folders card and the Backup Everything card
    // are called in this language, that is what the sentence says.
    case "everything":
      text = t("files.effectiveEverything").replace("{when}", when).replace("{domain}", t("settings.everythingTitle"));
      tone = "text-carbon-textSub";
      break;
    case "domain":
      text = t("files.effectiveDomain").replace("{when}", when).replace("{domain}", t("jobs.filesSection"));
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
