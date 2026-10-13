import { useEffect, useLayoutEffect, useRef, useState, type CSSProperties, type KeyboardEvent } from "react";
import type { OffsiteDomain, StorageLocation } from "../../lib/api";
import { hueVars } from "../../lib/appearance";
import { farPath, flowLayout, flowMeasures, nearPath, stubPath, type FlowPath } from "../../lib/flowGeometry";
import { useT } from "../../lib/i18n";
import type { FlowLine, FlowRow } from "../../lib/storageFlow";
import { DOMAIN_LABEL } from "../../lib/storageLocations";
import { BusyRing } from "../BusyRing";
import { LocationMark } from "./LocationMark";
import { DomainGlyph } from "./rows";

function hueStyle(index: number, extra: Record<string, string | number> = {}): CSSProperties {
  return { ...hueVars(index), ...extra } as CSSProperties;
}

function sectionAt(target: EventTarget): OffsiteDomain | null {
  return ((target as Element).closest("[data-row]")?.getAttribute("data-row") as OffsiteDomain | undefined) ?? null;
}

function onSwitchKey(e: KeyboardEvent<SVGGElement>, toggle: () => void) {
  if (e.key !== "Enter" && e.key !== " ") return;
  e.preventDefault();
  toggle();
}

interface Drawn {
  line: FlowLine;
  row: number;
  path: FlowPath;
  near: boolean;
  /** Where the line ends: the port on a band or on a box. */
  end: [number, number];
  /** Whether the parcel sets out from a band the section is stored on. */
  relay: boolean;
}

/**
 * FlowPicture draws every section's backups as lines: from the section to the
 * local band it is stored on, and from there to each off-site box. Every line
 * is a switch, and a dashed one is off.
 */
export function FlowPicture({
  rows,
  locations,
  busy,
  pending,
  onToggle,
  onPoint,
}: {
  rows: FlowRow[];
  locations: StorageLocation[];
  busy: boolean;
  /** The key of the line whose switch the server is still working on. */
  pending: string | null;
  onToggle: (line: FlowLine) => void;
  /** The section under the pointer or the focus, null when it left. */
  onPoint: (domain: OffsiteDomain | null) => void;
}) {
  const { t } = useT();
  const wrap = useRef<HTMLDivElement>(null);
  const [room, setRoom] = useState(860);
  const refocus = useRef<string | null>(null);

  useLayoutEffect(() => {
    const el = wrap.current;
    if (!el) return;
    const measure = () => {
      if (el.clientWidth > 0) setRoom(el.clientWidth);
    };
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    measure();
    return () => observer.disconnect();
  }, []);

  // A line that changes sides is drawn in another layer, which would drop the
  // focus a keyboard user switched it with.
  useEffect(() => {
    const key = refocus.current;
    if (!key || busy) return;
    refocus.current = null;
    const lines = wrap.current?.querySelectorAll<SVGGElement>("[data-line]") ?? [];
    [...lines].find((el) => el.dataset.line === key)?.focus();
  }, [rows, busy]);

  const bands = locations.filter((location) => location.kind === "local");
  const nodes = locations.filter((location) => location.kind !== "local");
  const layout = flowLayout(flowMeasures(room), rows.length, bands.length, nodes.length);
  const m = layout.measures;

  const drawn: Drawn[] = [];
  const origins = rows.map((row) => {
    const stored = row.lines.filter((line) => line.on && bands.includes(line.location));
    return stored.length > 0 ? bands.indexOf(stored[stored.length - 1].location) : null;
  });
  rows.forEach((row, i) => {
    for (const line of row.lines) {
      const band = bands.indexOf(line.location);
      if (band >= 0) {
        drawn.push({ line, row: i, path: nearPath(layout, i, band), near: true, end: [layout.bandCentre(band), layout.rowY(i)], relay: false });
        continue;
      }
      const node = nodes.indexOf(line.location);
      drawn.push({
        line,
        row: i,
        path: farPath(layout, i, node, origins[i]),
        near: false,
        end: layout.port(node, i),
        relay: origins[i] !== null,
      });
    }
  });
  // Local sits on top: an off-site line without a local copy runs right under
  // it, and the port has to stay Local's switch.
  const layer = (d: Drawn) => (d.near ? 2 : d.line.on ? 1 : 0);
  const ordered = [...drawn].sort((a, b) => layer(a) - layer(b));

  function toggle(line: FlowLine) {
    if (busy) return;
    refocus.current = line.key;
    onToggle(line);
  }

  const name = (location: StorageLocation) =>
    location.enabled ? location.name : t("placement.off").replace("{name}", () => location.name);

  return (
    <div
      ref={wrap}
      className="glim-flow"
      onPointerOver={(e) => onPoint(sectionAt(e.target))}
      onPointerLeave={() => onPoint(null)}
      onFocus={(e) => onPoint(sectionAt(e.target))}
      onBlur={() => onPoint(null)}
    >
      <svg
        viewBox={`0 0 ${m.width} ${layout.height}`}
        role="group"
        aria-label={t("storage.flow.alt")}
        aria-busy={busy}
        className={`[direction:ltr] ${m.narrow ? "glim-flow-narrow" : ""}`}
      >
        {bands.map((location, k) => (
          <g key={location.id} className={location.enabled ? undefined : "glim-flow-off"}>
            <rect
              className="glim-flow-band"
              x={layout.bandLeft(k)}
              y={layout.rowsTop + 2}
              width={m.bandWidth}
              height={layout.rowsHeight - 4}
              rx="14"
            />
            <foreignObject
              x={layout.bandCentre(k) - layout.headWidth / 2}
              y={layout.rowsTop - m.top}
              width={layout.headWidth}
              height={m.top - 6}
            >
              <div className="glim-flow-head">
                <LocationMark location={location} size="scene" bare />
                {/* Several bands in the small picture leave room for the mark alone. */}
                <span className={m.narrow && bands.length > 1 ? "sr-only" : undefined}>{name(location)}</span>
              </div>
            </foreignObject>
          </g>
        ))}
        {nodes.map((location, j) => (
          <g key={location.id} className={location.enabled ? undefined : "glim-flow-off"}>
            <rect className="glim-flow-node" x={m.nodeX} y={layout.nodeTop(j)} width={m.nodeWidth} height={layout.nodeHeight} rx="12" />
            <foreignObject x={m.nodeX + 8} y={layout.nodeTop(j)} width={m.nodeWidth - 12} height={layout.nodeHeight}>
              <div className="glim-flow-name">
                <LocationMark location={location} size="scene" bare />
                <span>{name(location)}</span>
              </div>
            </foreignObject>
          </g>
        ))}

        {ordered.map(({ line, row, path, near, end }) => {
          const label = t("storage.flow.line")
            .replace("{domain}", () => t(DOMAIN_LABEL[line.domain]))
            .replace("{name}", () => line.location.name);
          return (
            <g
              key={line.key}
              data-line={line.key}
              data-row={line.domain}
              aria-label={label}
              className={`glim-flow-line glim-pic glim-hue${line.on ? " glim-flow-on" : ""}${near ? " glim-flow-near" : ""}`}
              style={hueStyle(row)}
              role="switch"
              aria-checked={line.on}
              aria-busy={line.key === pending || undefined}
              tabIndex={0}
              onClick={() => toggle(line)}
              onKeyDown={(e) => onSwitchKey(e, () => toggle(line))}
            >
              <path className="glim-flow-hit" d={path.d} />
              <path className="glim-flow-stroke" d={path.d} />
              {near && (
                <>
                  <circle className="glim-flow-ring" cx={end[0]} cy={end[1]} r="11" />
                  <circle className="glim-flow-port" cx={end[0]} cy={end[1]} r="6" />
                  <circle className="glim-flow-reach" cx={end[0]} cy={end[1]} r={Math.min(16, m.bandWidth / 2)} />
                </>
              )}
              {!near && line.on && <circle className="glim-flow-dock" cx={end[0]} cy={end[1]} r="3.5" />}
              {line.key === pending && (
                <foreignObject x={end[0] - 12} y={end[1] - 12} width="24" height="24" className="glim-flow-busy">
                  <BusyRing />
                </foreignObject>
              )}
            </g>
          );
        })}

        {rows.map((row, i) => {
          const y = layout.rowY(i);
          const far = row.lines.filter((line) => line.on && nodes.includes(line.location));
          const stub = far.length === 0 ? stubPath(layout, i, origins[i]) : null;
          return (
            <g key={row.domain}>
              {stub && (
                <>
                  <g className="glim-flow-stub">
                    <path d={stub.d} />
                    <path d={stub.cross} />
                  </g>
                  {/* A row with nowhere to go says so under the picture, where the
                      words have room at every width. */}
                  {!row.nowhere && (
                    <foreignObject x={stub.labelX} y={y - 17} width={m.stubLabel} height="34">
                      <div className="glim-flow-stub-label">
                        <span>{t("dashboard.noOffsite")}</span>
                      </div>
                    </foreignObject>
                  )}
                </>
              )}
              <foreignObject x="0" y={y - 17} width={m.label - 8} height="34">
                <div className="glim-flow-section glim-hue" data-row={row.domain} style={hueStyle(i)}>
                  <DomainGlyph domain={row.domain} />
                  <span>{t(DOMAIN_LABEL[row.domain])}</span>
                </div>
              </foreignObject>
            </g>
          );
        })}

        <g pointerEvents="none" aria-hidden="true">
          {drawn
            .filter(({ line }) => line.on && line.location.enabled)
            .map(({ line, row, path, near, end, relay }) => (
              <g key={line.key} className="glim-pic glim-hue" style={hueStyle(row, { "--flow-turn": row })}>
                <circle
                  r="4.5"
                  className={`glim-flow-parcel ${near ? "glim-flow-parcel-near" : relay ? "glim-flow-parcel-far" : "glim-flow-parcel-through"}`}
                  style={{ offsetPath: `path("${path.d}")` }}
                />
                <circle
                  r="5"
                  cx={end[0]}
                  cy={end[1]}
                  className={`glim-flow-pulse ${near ? "glim-flow-pulse-near" : "glim-flow-pulse-far"}`}
                />
              </g>
            ))}
        </g>
      </svg>
    </div>
  );
}
