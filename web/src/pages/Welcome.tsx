// The page a new install opens on: what BombVault has found on this server
// and the three steps to the first backup, each card with its part of one
// picture, laid out as the pairing page lays out its steps.
import { useLayoutEffect, useRef, useState, type CSSProperties, type ReactNode } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

import { Button } from "../components/Button";
import { IconForward, IconSearch } from "../components/glyphs";
import { IconRecovery } from "../components/navGlyphs";
import { LateRedirect } from "../components/LateRedirect";
import { PageTitle } from "../components/PageTitle";
import { placeWithin, STEP_GRID, StepCard, useStepGrid } from "../components/recovery/StepCard";
import { AreasPicture, BackupPicture, Parcel, PlacePicture, TILE_AT, VAULT_AT } from "../components/welcomeArt";
import type { BackupItem } from "../lib/api";
import { useAttention } from "../lib/attention";
import { FIRST_STEPS, foundSentence, type FirstStep } from "../lib/firstSteps";
import { seenInUse } from "../lib/freshInstall";
import { useT, type TranslationKey } from "../lib/i18n";
import { PAGE_SHELL } from "../lib/pageShell";
import { tallyEntries } from "../lib/protectionTally";
import { useFirstSteps } from "../lib/useFirstSteps";
import { withinBase } from "../lib/base";

const TITLE: Record<FirstStep, TranslationKey> = {
  sections: "firstSteps.sections",
  location: "storage.add.title",
  backup: "firstSteps.backup",
};

/** useParcelFlights measures, in the pixels of the box around the cards,
 *  where each tile of the first picture and the vault of the second lie, and
 *  hands each parcel its start and its end. Measured again on every resize. */
function useParcelFlights() {
  const box = useRef<HTMLDivElement>(null);
  const [flights, setFlights] = useState<CSSProperties[]>([]);
  useLayoutEffect(() => {
    const root = box.current!;
    const measure = () => {
      const frames = Array.from(root.querySelectorAll<HTMLElement>("[data-step-frame]"));
      if (frames.length < 2) return;
      const at = (frame: HTMLElement, [x, y]: readonly [number, number]) => {
        const [left, top] = placeWithin(frame, root);
        const scale = frame.offsetWidth / 240;
        return [left + x * scale, top + y * scale] as const;
      };
      const [toX, toY] = at(frames[1]!, VAULT_AT);
      // A parcel is as big as the kind on its tile, which is drawn at 1.3 times its 24 units.
      const size = `${((frames[0]!.offsetWidth / 240) * 24 * 1.3).toFixed(1)}px`;
      const next = TILE_AT.map((tile) => {
        const [fromX, fromY] = at(frames[0]!, tile);
        return { "--fly-from-x": `${fromX.toFixed(1)}px`, "--fly-from-y": `${fromY.toFixed(1)}px`, "--fly-to-x": `${toX.toFixed(1)}px`, "--fly-to-y": `${toY.toFixed(1)}px`, "--fly-size": size } as CSSProperties;
      });
      setFlights((prev) => (JSON.stringify(prev) === JSON.stringify(next) ? prev : next));
    };
    measure();
    const watch = new ResizeObserver(measure);
    watch.observe(root);
    return () => watch.disconnect();
  }, []);
  return { box, flights };
}

function FirstStart({ items }: { items: BackupItem[] }) {
  const { t, lang } = useT();
  const navigate = useNavigate();
  const { states, found, platform } = useFirstSteps();

  const { box, flights } = useParcelFlights();
  const picture: Record<FirstStep, ReactNode> = {
    sections: <AreasPicture />,
    location: <PlacePicture />,
    backup: <BackupPicture done={t("firstSteps.backedUp")} />,
  };
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
      <img src={withinBase("/logo.svg")} alt="" draggable={false} className="glim-welcome-logo block w-[120px]" />
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
      {/* The parcels fly above the cards, so they sit beside the list in a
          box of their own. */}
      <div ref={box} className="relative mt-3 w-full">
        <ol ref={steps.grid} style={steps.style} className={`${STEP_GRID} text-start`}>
          {FIRST_STEPS.map((step, i) => (
            <li key={step} data-step={step} data-state={states[step]} aria-current={states[step] === "next" ? "step" : undefined} className="grid">
              <StepCard n={i + 1} title={t(TITLE[step])} state={states[step] === "done" ? "ok" : "idle"} hueIndex={i}>
                {states[step] === "done" && <span className="sr-only">{t("common.done")}</span>}
                {/* On a phone the picture moves beside the words, so three cards
                    do not take three screens. */}
                <div className="grid grid-cols-[132px_minmax(0,1fr)] items-center gap-4 md:flex md:flex-col md:items-stretch md:gap-3">
                  <div data-step-frame className="glim-pic mx-auto w-full max-w-[280px]">
                    {picture[step]}
                  </div>
                  {sub[step] && <p className="text-carbon-textMuted wrap-break-word hyphens-auto">{sub[step]}</p>}
                </div>
              </StepCard>
            </li>
          ))}
        </ol>
        {flights.map((style, i) => (
          <Parcel key={i} turn={i} style={style} />
        ))}
      </div>
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
