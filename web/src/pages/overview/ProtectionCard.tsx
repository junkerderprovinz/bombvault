import type { ReactNode } from "react";

import { KIND_LABEL } from "../../components/backups/kinds";
import { formatCadence } from "../../components/CadenceBuilder";
import { IconForward, IconSearch } from "../../components/glyphs";
import { InfoBubble } from "../../components/InfoBubble";
import { IconCloud, IconLocal, IconRestore } from "../../components/navGlyphs";
import type { useT } from "../../lib/i18n";
import { formatRecent, formatTs } from "../../lib/reltime";
import { useTipBubble } from "../../lib/useTipBubble";
import { FactGlyph } from "../settings/pairing/pairingArt";
import { Card } from "./Card";
import { CHECK_KEYS, CHECK_LABEL, type Check, type CheckKey, type CheckTone, type Section } from "./protectionChecks";

type T = ReturnType<typeof useT>["t"];

const CHECK_GLYPH: Record<CheckKey, () => ReactNode> = {
  local: IconLocal,
  offsite: IconCloud,
  lock: () => <FactGlyph kind="lock" />,
  drill: IconRestore,
};

export const TONE_DOT: Record<CheckTone, string> = {
  ok: "bg-statusOkSolid",
  warn: "bg-statusWarnSolid",
  fail: "bg-statusFailSolid",
  run: "bg-accent",
  // A check that does not apply is a ring, so it does not read as passed.
  neutral: "shadow-[inset_0_0_0_2px_var(--carbon-surface3)]",
};

// One row and the head above it share these columns: the section's dot, its
// name, a column per check and the mark that the row opens.
const COLUMNS =
  "grid grid-cols-[9px_minmax(0,1fr)_repeat(4,28px)_14px] items-center gap-x-2.5 @[37.5rem]:grid-cols-[9px_minmax(0,1fr)_repeat(4,76px)_14px]";

function CheckDot({ check, t }: { check: Check; t: T }) {
  const say = t(check.say);
  const tip = useTipBubble(say);
  return (
    <span className="relative z-10 flex justify-center">
      <span
        ref={tip.ref}
        tabIndex={0}
        role="img"
        aria-label={say}
        aria-describedby={tip.describedBy}
        data-check={check.key}
        data-tone={check.tone}
        {...tip.handlers}
        className="flex h-7 w-7 items-center justify-center rounded-full focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-(--focus-ring)"
      >
        <i className={`block h-2.5 w-2.5 rounded-full ${TONE_DOT[check.tone]}`} />
      </span>
      {tip.bubble}
    </span>
  );
}

function SectionRow({ section, t, lang, onOpen }: { section: Section; t: T; lang: string; onOpen: () => void }) {
  const { status, tally, checks } = section;
  const gaps = checks
    .filter((c) => c.tone !== "ok")
    .map((c) => `${t(c.say)}.`)
    .join(" ");
  const schedule = status.coveredBy
    ? t("dashboard.rpoViaEverything").replace("{cadence}", formatCadence(status.coveredBy, t, lang))
    : formatCadence(status.schedule, t, lang) || t(status.entrySchedules ? "dashboard.rpoEntries" : "dashboard.rpoOff");
  return (
    <li
      data-testid="protection-row"
      data-domain={status.domain}
      className={`relative rounded-control border-t border-carbon-border px-1.5 py-3 first:border-t-0 hover:bg-carbon-hover ${COLUMNS}`}
    >
      <span aria-hidden="true" data-tone={section.tone} className={`h-2 w-2 rounded-full ${TONE_DOT[section.tone]}`} />
      <span className="flex min-w-0 items-center gap-1 text-sm font-semibold text-carbon-text">
        {/* The button's box is stretched over the row, so the whole row opens
            the section while the dots keep their own bubbles above it. */}
        <button
          type="button"
          onClick={onOpen}
          className="min-w-0 truncate text-start after:absolute after:inset-0 after:rounded-control focus-visible:outline-none focus-visible:after:outline-solid focus-visible:after:outline-2 focus-visible:after:outline-(--focus-ring)"
        >
          {t(KIND_LABEL[section.kind])}
        </button>
        {gaps && (
          <span className="relative z-10 flex">
            <InfoBubble tip={gaps} />
          </span>
        )}
      </span>
      {checks.map((check) => (
        <CheckDot key={check.key} check={check} t={t} />
      ))}
      <span className="inline-flex text-carbon-textMuted">
        <IconForward />
      </span>
      <span className="col-[2/-1] mt-0.5 flex flex-wrap gap-x-1.5 text-xs text-carbon-textMuted [&>*+*]:before:me-1.5 [&>*+*]:before:content-['·']">
        <span>{schedule}</span>
        {status.lastSuccess > 0 && (
          <span title={formatTs(status.lastSuccess)}>
            {t("overview.head.last").replace("{when}", formatRecent(status.lastSuccess, lang))}
          </span>
        )}
        {tally.total > 0 && (
          <span>{t("coverage.ratio").replace("{protected}", String(tally.ok)).replace("{total}", String(tally.total))}</span>
        )}
      </span>
    </li>
  );
}

/**
 * ProtectionCard is a row per switched-on section and a column per check. A
 * row opens the section's window with everything behind its four dots.
 */
export function ProtectionCard({
  t,
  lang,
  sections,
  loading,
  failed,
  hueIndex,
  found,
  onOpen,
}: {
  t: T;
  lang: string;
  sections: Section[];
  /** What the server reports that could be backed up, as a sentence. */
  found?: string | null;
  loading: boolean;
  /** The sections never arrived. */
  failed: boolean;
  hueIndex?: number;
  onOpen: (section: Section) => void;
}) {
  return (
    <Card title={t("dashboard.protectionTitle")} hint={t("overview.protection.hint")} hueIndex={hueIndex}>
      {loading ? (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      ) : failed ? (
        <p className="text-sm text-statusWarn">{t("dashboard.statusLoadFailed")}</p>
      ) : sections.length === 0 ? (
        <p className="text-sm text-carbon-textMuted">{t("overview.protection.none")}</p>
      ) : (
        <div className="@container glim-content-fade">
          <div aria-hidden="true" className={`px-1.5 pb-2 text-meta font-medium text-carbon-textMuted ${COLUMNS}`}>
            <span />
            <span />
            {CHECK_KEYS.map((key) => {
              const Glyph = CHECK_GLYPH[key];
              return (
                <span key={key} className="flex min-w-0 flex-col items-center gap-1 text-center [&_svg]:h-3.5 [&_svg]:w-3.5">
                  <Glyph />
                  <span className="hidden max-w-full leading-tight wrap-anywhere @[37.5rem]:block">{t(CHECK_LABEL[key])}</span>
                </span>
              );
            })}
            <span />
          </div>
          <ul className="flex flex-col">
            {sections.map((section) => (
              <SectionRow key={section.kind} section={section} t={t} lang={lang} onOpen={() => onOpen(section)} />
            ))}
          </ul>
          {found && (
            <p data-testid="protection-found" className="flex items-center gap-2 px-1.5 pt-4 text-xs text-carbon-textMuted">
              <span className="flex flex-none [&_svg]:h-3.5 [&_svg]:w-3.5">
                <IconSearch />
              </span>
              <span className="min-w-0 flex-1">{found}</span>
              <InfoBubble tip={t("overview.protection.foundTip")} />
            </p>
          )}
        </div>
      )}
    </Card>
  );
}
