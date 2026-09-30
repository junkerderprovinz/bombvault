// PairingSteps opens the tab with what pairing is for in one sentence, then
// three numbered cards before any button does anything: generate a phrase on
// the first instance, enter it on every other, done.
import { Fragment, type ReactNode } from "react";
import { StepCard } from "../../../components/recovery/StepCard";
import { InfoBubble } from "../../../components/InfoBubble";
import type { useT } from "../../../lib/i18n";
import { StepPicture } from "./pairingArt";

type T = ReturnType<typeof useT>["t"];

/** emphasize replaces {token} in text with a bold copy of what. */
export function emphasize(text: string, token: string, what: string): ReactNode {
  const parts = text.split(`{${token}}`);
  return parts.map((part, i) => (
    <Fragment key={i}>
      {part}
      {i < parts.length - 1 && <strong className="font-medium text-carbon-text">{what}</strong>}
    </Fragment>
  ));
}

export function PairingSteps({ t, hues }: { t: T; hues: [number, number, number] }) {
  const steps = [
    {
      title: t("pairing.step1Title"),
      body: t("pairing.step1Body"),
    },
    {
      title: t("pairing.step2Title"),
      body: emphasize(t("pairing.step2Body"), "button", t("pairing.enter")),
    },
    {
      title: t("pairing.step3Title"),
      body: t("pairing.step3Body"),
    },
  ];
  return (
    <section aria-label={t("pairing.howTitle")} className="flex flex-col gap-8 md:gap-10">
      {/* The step cards' number badges straddle their top edge and would
          crowd the sentence at the usual gap. */}
      <p className="text-base font-medium text-carbon-text md:text-lg">
        {t("pairing.lead")} <InfoBubble tip={t("pairing.keyNote")} />
      </p>
      {/* On a phone the picture moves beside the words, so three cards do
          not take three screens. */}
      <div className="grid grid-cols-1 gap-10 md:grid-cols-3 md:gap-6">
        {steps.map((s, i) => (
          <StepCard key={i} n={i + 1} title={s.title} hueIndex={hues[i]}>
            <div className="grid grid-cols-[112px_minmax(0,1fr)] items-center gap-4 md:flex md:flex-col md:items-stretch md:gap-3">
              <StepPicture step={(i + 1) as 1 | 2 | 3} />
              <p className="text-carbon-textSub">{s.body}</p>
            </div>
          </StepCard>
        ))}
      </div>
    </section>
  );
}
