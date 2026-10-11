import { useState, type CSSProperties } from "react";
import { Link } from "react-router-dom";
import type { OffsiteDomain, StorageLocation } from "../../lib/api";
import { hueVars } from "../../lib/appearance";
import { setFlowView, useFlowView, type FlowView } from "../../lib/flowView";
import { useT, type TranslationKey } from "../../lib/i18n";
import type { FlowCounts, FlowLine, FlowRow } from "../../lib/storageFlow";
import { DOMAIN_LABEL, locationRoute } from "../../lib/storageLocations";
import { Card } from "../../pages/settings/shared";
import { Selector } from "../Selector";
import { FlowPicture } from "./FlowPicture";
import { LocationMark } from "./LocationMark";
import { PausedDefault } from "./PausedDefault";
import { DomainGlyph } from "./rows";
import { useStorageFlow, type PlacementDefaults } from "./useStorageFlow";

const COUNTS: [keyof FlowCounts, TranslationKey][] = [
  ["copies", "storage.flow.copies"],
  ["media", "storage.flow.media"],
  ["offsite", "storage.flow.offsite"],
];

/** A section's 3-2-1 count: copies, media and how many of them are off the
 *  premises. */
function Counts({ counts }: { counts: FlowCounts }) {
  const { t } = useT();
  return (
    <span className="inline-flex flex-wrap items-baseline gap-x-2">
      {COUNTS.map(([field, key], i) => (
        <span key={field} className="inline-flex items-baseline gap-x-2">
          {i > 0 && <span className="text-carbon-textMuted">·</span>}
          <span>
            <b className="font-semibold tabular-nums text-carbon-text">{counts[field]}</b> {t(key, counts[field])}
          </span>
        </span>
      ))}
    </span>
  );
}

function OwnRules({ row }: { row: FlowRow }) {
  const { t } = useT();
  return (
    <>
      {row.ownRules.map((location) => (
        <Link
          key={location.id}
          to={locationRoute(location.id)}
          className="underline decoration-carbon-border underline-offset-[3px] hover:text-accentText"
        >
          {t("storage.flow.ownRules").replace("{name}", () => location.name)}
        </Link>
      ))}
    </>
  );
}

/** The same lines as plain switches: a row of buttons per section. */
function FlowSwitches({ rows, busy, onToggle }: { rows: FlowRow[]; busy: boolean; onToggle: (line: FlowLine) => void }) {
  const { t } = useT();
  return (
    <div className="flex flex-col divide-y divide-carbon-border/60">
      {rows.map((row, i) => {
        const label = t(DOMAIN_LABEL[row.domain]);
        return (
          <div key={row.domain} className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 py-3 first:pt-0 last:pb-0">
            <div className="flex min-w-0 flex-col gap-0.5">
              <span className="glim-hue flex items-center gap-2 text-sm text-carbon-text" style={hueVars(i) as CSSProperties}>
                <DomainGlyph domain={row.domain} />
                {label}
              </span>
              <span className="flex flex-wrap items-baseline gap-x-2 text-xs text-carbon-textMuted">
                <Counts counts={row.counts} />
                <OwnRules row={row} />
              </span>
              {row.unreadable && <span className="text-xs text-statusWarn">{t("placement.unreadable")}</span>}
            </div>
            {row.lines.length > 0 && (
              <div className="min-w-0 max-w-full max-md:w-full">
                <Selector
                  items={row.lines.map((line) => ({
                    id: line.key,
                    label: line.location.enabled ? line.location.name : t("placement.off").replace("{name}", () => line.location.name),
                    icon: <LocationMark location={line.location} size="chip" />,
                    disabled: !!line.locked,
                    title: line.locked,
                  }))}
                  label={label}
                  select="many"
                  inline
                  pairsOnPhone
                  hueOffset={3 * i}
                  active={new Set(row.lines.filter((line) => line.on).map((line) => line.key))}
                  disabled={busy}
                  onChange={(key) => {
                    const line = row.lines.find((x) => x.key === key);
                    if (line) onToggle(line);
                  }}
                />
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}

function score(counts: FlowCounts): number {
  return counts.offsite * 100 + counts.media * 10 + counts.copies;
}

/**
 * FlowCard shows where each section is backed up and switches it: as a
 * picture whose lines are the switches, or as rows of buttons. The choice
 * between the two is remembered.
 */
export function FlowCard({
  locations,
  defaults,
  hueIndex,
}: {
  locations: StorageLocation[];
  defaults: PlacementDefaults;
  hueIndex: number;
}) {
  const { t } = useT();
  const view = useFlowView();
  const flow = useStorageFlow(locations, defaults);
  const [pointed, setPointed] = useState<OffsiteDomain | null>(null);
  const { rows } = flow;

  const best = rows.reduce<FlowRow | undefined>((top, row) => (top && score(top.counts) >= score(row.counts) ? top : row), undefined);
  const counted = rows.find((row) => row.domain === pointed) ?? best;
  const withRules = rows.filter((row) => row.ownRules.length > 0);
  const unreadable = rows.filter((row) => row.unreadable);
  const graph = view === "graph";

  return (
    <Card title={t("storage.flow.title")} hint={t("storage.flow.hint")} hueIndex={hueIndex} newId="storage-flow">
      {flow.dialogs}
      <div className={`flex justify-end ${graph ? "md:absolute md:end-5 md:top-5 md:z-1" : ""}`}>
        <Selector
          items={[
            { id: "graph", label: t("storage.flow.graph") },
            { id: "switches", label: t("storage.flow.switches") },
          ]}
          label={t("storage.flow.view")}
          select="one"
          activation="manual"
          inline
          hue={false}
          active={view}
          onChange={(id) => setFlowView(id as FlowView)}
        />
      </div>
      {!flow.ready ? (
        <p className="text-sm text-carbon-textSub">{t("dashboard.checking")}</p>
      ) : (
        <div key={flow.shake} className={`flex flex-col gap-3${flow.shake ? " glim-shake" : ""}`}>
          {graph ? (
            <>
              <FlowPicture rows={rows} locations={locations} busy={flow.busy} onToggle={flow.toggle} onPoint={setPointed} />
              {counted && (
                <p className="flex flex-wrap items-baseline justify-center gap-x-2 gap-y-1 text-sm text-carbon-textSub" aria-live="polite">
                  <span className="font-semibold text-carbon-text max-sm:basis-full max-sm:text-center">{t(DOMAIN_LABEL[counted.domain])}</span>
                  <Counts counts={counted.counts} />
                </p>
              )}
              {withRules.length > 0 && (
                <p className="flex flex-wrap justify-center gap-x-2 gap-y-1 text-xs text-carbon-textMuted">
                  {withRules.map((row) => (
                    <span key={row.domain} className="inline-flex flex-wrap gap-x-1.5">
                      {t(DOMAIN_LABEL[row.domain])}:
                      <OwnRules row={row} />
                    </span>
                  ))}
                </p>
              )}
              {unreadable.map((row) => (
                <p key={row.domain} className="text-center text-xs text-statusWarn">
                  {t(DOMAIN_LABEL[row.domain])}: {t("placement.unreadable")}
                </p>
              ))}
            </>
          ) : (
            <FlowSwitches rows={rows} busy={flow.busy} onToggle={flow.toggle} />
          )}
          {flow.paused.map((domain) => (
            <PausedDefault key={domain} domain={domain} />
          ))}
        </div>
      )}
    </Card>
  );
}
