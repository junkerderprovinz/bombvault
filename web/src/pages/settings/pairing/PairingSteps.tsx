// PairingSteps opens the page with three numbered cards before any button
// does anything: generate a phrase on the first instance, enter it on every
// other, done. Their pictures play one scene, and one pointer walks through
// all three.
import { Fragment, useLayoutEffect, useRef, useState, type CSSProperties, type ReactNode } from "react";
import { StepCard } from "../../../components/recovery/StepCard";
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

/** The spots the pointer visits, as the card and a place in its picture's
 *  own 240 by 130 units. */
const STOPS: Record<string, [card: number, x: number, y: number]> = {
  gen: [0, 120, 26],
  copy: [0, 192, 114],
  enter: [1, 120, 26],
  paste: [1, 192, 114],
  hold: [2, 120, 80],
  done: [2, 168, 30],
};

/** Where an element lies inside an ancestor, by layout alone. A card that is
 *  still sliding in would shift what its bounding box reports. */
function placeWithin(el: HTMLElement, root: HTMLElement): [number, number] {
  let x = 0;
  let y = 0;
  for (let at: HTMLElement | null = el; at && at !== root; at = at.offsetParent as HTMLElement | null) {
    x += at.offsetLeft;
    y += at.offsetTop;
  }
  return [x, y];
}

/** useSceneStops measures the pointer's stops in the grid's own pixels and
 *  hands them to the stylesheet. */
function useSceneStops() {
  const grid = useRef<HTMLDivElement>(null);
  const [stops, setStops] = useState<CSSProperties>({});
  useLayoutEffect(() => {
    const root = grid.current!;
    const measure = () => {
      const frames = Array.from(root.querySelectorAll<HTMLElement>("[data-step-frame]"));
      const next: Record<string, string> = {};
      for (const [name, [card, x, y]] of Object.entries(STOPS)) {
        const frame = frames[card];
        const [left, top] = placeWithin(frame, root);
        const scale = frame.offsetWidth / 240;
        next[`--at-${name}-x`] = `${(left + x * scale).toFixed(1)}px`;
        next[`--at-${name}-y`] = `${(top + y * scale).toFixed(1)}px`;
      }
      setStops((prev) => (JSON.stringify(prev) === JSON.stringify(next) ? prev : (next as CSSProperties)));
    };
    measure();
    const watch = new ResizeObserver(measure);
    watch.observe(root);
    return () => watch.disconnect();
  }, []);
  return { grid, stops };
}

export function PairingSteps({ t, hues }: { t: T; hues: [number, number, number] }) {
  const { grid, stops } = useSceneStops();
  const steps = [
    {
      title: t("pairing.step1Title"),
      body: t("pairing.step1Body"),
    },
    {
      title: t("pairing.stepOtherTitle"),
      body: emphasize(t("pairing.step2Body"), "button", t("pairing.enter")),
    },
    {
      title: t("pairing.step3Title"),
      body: t("pairing.step3Body"),
    },
  ];
  return (
    <section aria-label={t("pairing.howTitle")} data-new="pairing-steps">
      {/* The step cards' number badges straddle their top edge, so stacked
          cards keep more room between them than a card gap. */}
      <div ref={grid} className="relative grid grid-cols-1 gap-10 md:grid-cols-3 md:gap-6">
        {steps.map((s, i) => (
          <StepCard key={i} n={i + 1} title={s.title} hueIndex={hues[i]}>
            {/* On a phone the picture moves beside the words, so three cards
                do not take three screens. */}
            <div className="grid grid-cols-[132px_minmax(0,1fr)] items-center gap-4 md:flex md:flex-col md:items-stretch md:gap-3">
              <div data-step-frame className="glim-pic mx-auto w-full max-w-[280px]">
                <StepPicture step={(i + 1) as 1 | 2 | 3} t={t} />
              </div>
              <p className="text-carbon-textMuted wrap-break-word hyphens-auto">{s.body}</p>
            </div>
          </StepCard>
        ))}
        <div className="glim-step-pointer" style={stops} aria-hidden="true">
          <svg viewBox="0 0 14 19" width="14" height="19">
            <path className="glim-scene-pointer" d="M0 0v16l4.2-4 3 7 2.8-1.2-3-6.8H13z" />
          </svg>
        </div>
      </div>
    </section>
  );
}
