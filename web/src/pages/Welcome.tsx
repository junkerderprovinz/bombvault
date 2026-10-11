// The page a new install opens on: what BombVault has found on this server
// and the three steps to the first backup.
import type { CSSProperties } from "react";
import { Navigate, useNavigate } from "react-router-dom";

import { Button } from "../components/Button";
import { IconForward, IconSearch } from "../components/glyphs";
import { IconRecovery } from "../components/navGlyphs";
import { PageTitle } from "../components/PageTitle";
import { StepMark } from "../components/StepMark";
import type { BackupItem } from "../lib/api";
import { hueVars } from "../lib/appearance";
import { useAttention } from "../lib/attention";
import { FIRST_STEPS, foundSentence, type FirstStep } from "../lib/firstSteps";
import { useT, type TranslationKey } from "../lib/i18n";
import { PAGE_SHELL } from "../lib/pageShell";
import { tallyEntries } from "../lib/protectionTally";
import { useFirstSteps } from "../lib/useFirstSteps";

const TITLE: Record<FirstStep, TranslationKey> = {
  sections: "firstSteps.sections",
  location: "storage.add.title",
  backup: "firstSteps.backup",
};

function FirstStart({ items }: { items: BackupItem[] }) {
  const { t, lang } = useT();
  const navigate = useNavigate();
  const { states, found, platform } = useFirstSteps();

  const sentence = foundSentence(found, t, lang);
  // Only a schedule keeps the promise that the backups go on by themselves.
  const scheduled = tallyEntries(items).pending > 0;
  const sub: Record<FirstStep, string | false> = {
    sections: t("firstSteps.sectionsSub"),
    location: t("firstSteps.locationSub"),
    backup: scheduled && t("firstSteps.backupSub"),
  };

  return (
    <div className="mx-auto mt-[6vh] flex w-full max-w-[47.5rem] flex-col items-center gap-[22px] pb-6 text-center">
      <img src="/logo.svg" alt="" draggable={false} className="block w-[120px] dark:hidden" />
      <img src="/logo-light.svg" alt="" draggable={false} className="hidden w-[120px] dark:block" />
      {/* The page title says the same to a screen reader. */}
      <p aria-hidden="true" className="text-status font-semibold leading-tight text-carbon-text">
        {t("welcome.title")}
      </p>
      <p className="text-base text-carbon-textSub">{t(platform === "unraid" ? "welcome.leadUnraid" : "welcome.lead")}</p>
      {sentence && (
        <p data-testid="welcome-found" className="flex items-center gap-2 text-sm text-carbon-textSub">
          <span className="flex flex-none text-accentText [&_svg]:h-4 [&_svg]:w-4">
            <IconSearch />
          </span>
          <span>{sentence}</span>
        </p>
      )}
      <ol className="grid w-full gap-4 text-start md:grid-cols-3">
        {FIRST_STEPS.map((step, i) => (
          <li
            key={step}
            data-step={step}
            data-state={states[step]}
            aria-current={states[step] === "next" ? "step" : undefined}
            style={hueVars(i) as CSSProperties}
            className="glim-hue flex gap-3 rounded-card bg-carbon-surface p-[18px]"
          >
            <StepMark n={i + 1} state={states[step]} />
            <div className="flex min-w-0 flex-col gap-0.5">
              <span className="text-sm font-semibold text-carbon-text">
                {t(TITLE[step])}
                {states[step] === "done" && <span className="sr-only">{` (${t("common.done")})`}</span>}
              </span>
              {sub[step] && <span className="text-subline text-carbon-textMuted">{sub[step]}</span>}
            </div>
          </li>
        ))}
      </ol>
      <div className="flex flex-wrap justify-center gap-3">
        <Button
          label={t("welcome.restore")}
          labelKey="welcome.restore"
          glyph={<IconRecovery />}
          className="glim-btn-key"
          onClick={() => navigate("/restore")}
        />
        <Button
          label={t("welcome.start")}
          labelKey="welcome.start"
          glyph={<IconForward />}
          tone="accent"
          className="glim-btn-key"
          onClick={() => navigate("/dashboard")}
        />
      </div>
      <p className="text-base text-carbon-textSub">{t("welcome.restoreHint")}</p>
    </div>
  );
}

export function Welcome() {
  const { t } = useT();
  const { items, fresh } = useAttention();

  // An install that has a backup starts on the overview, and so does one
  // whose entries cannot be read: the overview says what is wrong.
  if (!fresh) return items.loaded || items.failed ? <Navigate to="/dashboard" replace /> : null;

  return (
    <div className={PAGE_SHELL}>
      <PageTitle>{t("welcome.title")}</PageTitle>
      <FirstStart items={items.items} />
    </div>
  );
}
