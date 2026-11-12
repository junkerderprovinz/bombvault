// The page a new install opens on: what BombVault has found on this server
// and the three steps to the first backup, each card with its part of one
// picture, laid out as the pairing page lays out its steps.
import type { ReactNode } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

import { Button } from "../components/Button";
import { IconForward, IconSearch } from "../components/glyphs";
import { IconRecovery } from "../components/navGlyphs";
import { LateRedirect } from "../components/LateRedirect";
import { PageTitle } from "../components/PageTitle";
import { STEP_GRID, StepCard, useStepGrid } from "../components/recovery/StepCard";
import { AreasPicture, BackupPicture, PlacePicture } from "../components/welcomeArt";
import type { BackupItem } from "../lib/api";
import { useAttention } from "../lib/attention";
import { FIRST_STEPS, foundSentence, type FirstStep } from "../lib/firstSteps";
import { seenInUse } from "../lib/freshInstall";
import { useT, type TranslationKey } from "../lib/i18n";
import { PAGE_SHELL } from "../lib/pageShell";
import { tallyEntries } from "../lib/protectionTally";
import { useFirstSteps } from "../lib/useFirstSteps";

const TITLE: Record<FirstStep, TranslationKey> = {
  sections: "firstSteps.sections",
  location: "storage.add.title",
  backup: "firstSteps.backup",
};

const PICTURE: Record<FirstStep, ReactNode> = {
  sections: <AreasPicture />,
  location: <PlacePicture />,
  backup: <BackupPicture />,
};

function FirstStart({ items }: { items: BackupItem[] }) {
  const { t, lang } = useT();
  const navigate = useNavigate();
  const { states, found, platform } = useFirstSteps();

  const steps = useStepGrid<HTMLOListElement>(FIRST_STEPS.map((step) => t(TITLE[step])).join("\n"));
  const sentence = foundSentence(found, t, lang);
  // Only a schedule keeps the promise that the backups go on by themselves.
  const scheduled = tallyEntries(items).pending > 0;
  const sub: Record<FirstStep, string | false> = {
    sections: t("firstSteps.sectionsSub"),
    location: t("firstSteps.locationSub"),
    backup: scheduled && t("firstSteps.backupSub"),
  };

  return (
    <div className="mx-auto mt-[6vh] flex w-full max-w-[64rem] flex-col items-center gap-[22px] pb-6 text-center">
      <img src="/logo.svg" alt="" draggable={false} className="glim-welcome-logo block w-[120px]" />
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
      <ol ref={steps.grid} style={steps.style} className={`${STEP_GRID} mt-3 w-full text-start`}>
        {FIRST_STEPS.map((step, i) => (
          <li key={step} data-step={step} data-state={states[step]} aria-current={states[step] === "next" ? "step" : undefined}>
            <StepCard n={i + 1} title={t(TITLE[step])} state={states[step] === "done" ? "ok" : "idle"} hueIndex={i}>
              {states[step] === "done" && <span className="sr-only">{t("common.done")}</span>}
              {/* On a phone the picture moves beside the words, so three cards
                  do not take three screens. */}
              <div className="grid grid-cols-[132px_minmax(0,1fr)] items-center gap-4 md:flex md:flex-col md:items-stretch md:gap-3">
                <div data-step-frame className="glim-pic mx-auto w-full max-w-[280px]">
                  {PICTURE[step]}
                </div>
                {sub[step] && <p className="text-carbon-textMuted wrap-break-word hyphens-auto">{sub[step]}</p>}
              </div>
            </StepCard>
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
  // Info opens the page on purpose, for an install that is long past it.
  const again = useSearchParams()[0].has("again");

  // An install that has a backup starts on the overview, and so does one
  // whose entries cannot be read: the overview says what is wrong. One this
  // browser saw in use goes there at once, so the overview's own reads do not
  // wait for the entries, which are slow to read on a host with many
  // containers.
  if (!fresh && !again) return items.loaded || items.failed || seenInUse() ? <LateRedirect to="/dashboard" /> : null;

  return (
    <div className={PAGE_SHELL}>
      <PageTitle>{t("welcome.title")}</PageTitle>
      <FirstStart items={items.items} />
    </div>
  );
}
