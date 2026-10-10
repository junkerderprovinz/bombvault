// Every watched item as a ring of ten parts, one for each backup detection
// has learned from. A ring still filling takes the accent and a full one
// turns green, so the items that are done stand out.

import { useState, type ComponentType, type CSSProperties } from "react";

import { IconConfig, IconContainers, IconFiles, IconFlash, IconVM, IconZFS } from "../navGlyphs";
import { anomalyDomainsLabel, type TranslateAnomaly } from "../../lib/anomalies";
import type { AnomalyItem } from "../../lib/api";
import { useTipBubble } from "../../lib/useTipBubble";
import { learningProgressText } from "./ItemMonitoring";

const DOMAIN_GLYPH: Record<string, ComponentType> = {
  container: IconContainers,
  vm: IconVM,
  files: IconFiles,
  zfs: IconZFS,
  flash: IconFlash,
  config: IconConfig,
};

const PARTS = 10;
const RADIUS = 23;
const ROUND = 2 * Math.PI * RADIUS;
const PART = ROUND / PARTS;
const GAP = 3;

function Ring({ item, t, onOpen }: { item: AnomalyItem; t: TranslateAnomaly; onOpen: () => void }) {
  const { samples, needed } = item.learning;
  const lit = needed > 0 ? Math.min(PARTS, Math.floor((samples * PARTS) / needed)) : 0;
  // Only the parts a new backup added while the page was open grow in.
  const [grown, setGrown] = useState({ from: lit, to: lit });
  if (grown.to !== lit) setGrown({ from: grown.to, to: lit });

  const done = samples >= needed;
  const name = item.name || anomalyDomainsLabel(item.domain, t);
  const say = [name, learningProgressText(t, item.learning), !item.scheduled && t("anomaly.items.notScheduled")]
    .filter(Boolean)
    .join(" · ");
  const tooltip = useTipBubble(say);
  const Glyph = DOMAIN_GLYPH[item.domain];

  return (
    <li className="flex min-w-0">
      <button
        ref={tooltip.ref}
        type="button"
        aria-label={say}
        aria-describedby={tooltip.describedBy}
        {...tooltip.handlers}
        onClick={onOpen}
        className={`flex min-h-11 min-w-0 flex-1 flex-col items-center gap-1.5 rounded-control px-1 pb-2 pt-2.5 hover:bg-carbon-hover ${
          item.scheduled ? "text-carbon-text" : "text-carbon-textMuted"
        }`}
      >
        <span
          className={`relative flex h-[54px] w-[54px] items-center justify-center [&>svg:last-child]:h-5 [&>svg:last-child]:w-5 ${
            item.scheduled ? "text-carbon-textSub" : "text-carbon-textMuted"
          }`}
        >
          {/* A circle's stroke starts at three o'clock; the turn puts the first
              part at the top. */}
          <svg
            viewBox="0 0 54 54"
            aria-hidden="true"
            className="absolute inset-0 h-full w-full -rotate-90 fill-none stroke-[3.5]"
          >
            {Array.from({ length: PARTS }, (_, i) => {
              const grows = i >= grown.from && i < grown.to;
              return (
                <circle
                  key={i}
                  data-lit={i < lit}
                  cx="27"
                  cy="27"
                  r={RADIUS}
                  strokeDasharray={`${(PART - GAP).toFixed(2)} ${ROUND.toFixed(2)}`}
                  strokeDashoffset={(-(i * PART + GAP / 2)).toFixed(2)}
                  style={grows ? ({ "--ring-i": i - grown.from } as CSSProperties) : undefined}
                  className={`${i < lit ? (done ? "stroke-statusOkSolid" : "stroke-accent") : "stroke-carbon-surface3"}${
                    grows ? " glim-ring-grow" : ""
                  }`}
                />
              );
            })}
          </svg>
          {Glyph && <Glyph />}
        </span>
        <span className="max-w-full truncate text-xs font-medium">{name}</span>
      </button>
      {tooltip.bubble}
    </li>
  );
}

export function LearningRings({
  items,
  t,
  onOpen,
}: {
  items: AnomalyItem[];
  t: TranslateAnomaly;
  onOpen: (item: AnomalyItem) => void;
}) {
  return (
    <>
      <ul className="grid grid-cols-[repeat(auto-fill,minmax(76px,1fr))] gap-1 sm:grid-cols-[repeat(auto-fill,minmax(92px,1fr))]">
        {items.map((item) => (
          <Ring key={item.targetId} item={item} t={t} onOpen={() => onOpen(item)} />
        ))}
      </ul>
      <p className="flex flex-wrap justify-end gap-x-4 gap-y-1 text-xs text-carbon-textMuted">
        <span className="inline-flex items-center gap-1.5">
          <i aria-hidden="true" className="h-2.5 w-2.5 rounded-full shadow-[inset_0_0_0_2.5px_var(--accent)]" />
          {t("anomaly.learn.learning")}
        </span>
        <span className="inline-flex items-center gap-1.5">
          <i aria-hidden="true" className="h-2.5 w-2.5 rounded-full shadow-[inset_0_0_0_2.5px_var(--status-ok-solid)]" />
          {t("anomaly.learningDone")}
        </span>
      </p>
    </>
  );
}
