// The backups of one entry below each other on a rail, newest on top, so each
// is in sight with its size and the storage locations that hold it. A handle
// rests on the chosen one and can be dragged from backup to backup.
import { useLayoutEffect, useRef, useState, type KeyboardEvent, type PointerEvent } from "react";
import { rowTags } from "../../lib/entry";
import { humanBytes } from "../../lib/forecast";
import { useT } from "../../lib/i18n";
import { formatRecent } from "../../lib/reltime";
import type { Recommendation } from "../../lib/restorePlan";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { usePlaceLabel, type TimelineState } from "../entry/BackupPlaces";
import { IconCheck, IconCloud, IconLocal, IconRestore } from "../navGlyphs";

export function railRowId(index: number): string {
  return `restore-rail-${index}`;
}

export function BackupRail({
  timeline,
  recommendation,
  chosen,
  why,
  onChoose,
  onRestore,
}: {
  timeline: TimelineState;
  recommendation: Recommendation;
  /** The chosen row, counted from the newest. */
  chosen: number;
  /** Why the recommended backup is the recommended one. */
  why: string;
  onChoose: (index: number) => void;
  onRestore: (index: number) => void;
}) {
  const { t, lang } = useT();
  const { rows, places } = timeline;
  const labelOf = usePlaceLabel(places);
  const list = useRef<HTMLDivElement>(null);
  // Where the handle is while it is held, measured from the top of the list.
  const [held, setHeld] = useState<number | null>(null);
  const [rest, setRest] = useState(0);
  const after = recommendation.finding !== undefined;

  function rowElements(): HTMLElement[] {
    return Array.from(list.current?.querySelectorAll<HTMLElement>('[role="option"]') ?? []);
  }

  // A row grows when it is chosen, because only then it says why, and every
  // row moves when the list gets narrower.
  useLayoutEffect(() => {
    const box = list.current;
    if (!box) return;
    const place = () => {
      const row = box.querySelectorAll<HTMLElement>('[role="option"]')[chosen];
      if (row) setRest(row.offsetTop + row.offsetHeight / 2);
    };
    place();
    const observer = new ResizeObserver(place);
    observer.observe(box);
    return () => observer.disconnect();
  }, [chosen, rows]);

  function grab(e: PointerEvent<HTMLSpanElement>) {
    const box = list.current;
    const all = rowElements();
    if (e.button !== 0 || !box || all.length < 2) return;
    e.preventDefault();
    box.focus();
    e.currentTarget.setPointerCapture(e.pointerId);
    setHeld(rest);
  }

  function drag(e: PointerEvent<HTMLSpanElement>) {
    const box = list.current;
    if (held === null || !box) return;
    const all = rowElements();
    const middle = (row: HTMLElement) => row.offsetTop + row.offsetHeight / 2;
    const y = Math.min(middle(all[all.length - 1]), Math.max(middle(all[0]), e.clientY - box.getBoundingClientRect().top));
    const over = all.findIndex((row) => y < row.offsetTop + row.offsetHeight);
    setHeld(y);
    onChoose(over < 0 ? all.length - 1 : over);
  }

  function onKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    if (e.target !== e.currentTarget) return;
    if (e.key === "Enter") {
      onRestore(chosen);
      return;
    }
    const to = { ArrowDown: chosen + 1, ArrowUp: chosen - 1, Home: 0, End: rows.length - 1 }[e.key];
    if (to === undefined) return;
    e.preventDefault();
    const next = Math.min(rows.length - 1, Math.max(0, to));
    onChoose(next);
    // jsdom has no layout to scroll.
    document.getElementById(railRowId(next))?.scrollIntoView?.({ block: "nearest" });
  }

  return (
    <div
      ref={list}
      role="listbox"
      tabIndex={0}
      aria-label={t("restoreWindow.backup")}
      aria-activedescendant={railRowId(chosen)}
      onKeyDown={onKeyDown}
      className={`relative select-none rounded-control outline-none focus-visible:ring-2 focus-visible:ring-(--focus-ring)${
        held === null ? "" : " glim-rail-held"
      }`}
    >
      <div className="flex flex-col">
        {rows.map((row, i) => {
          const on = i === chosen;
          const recommended = i === recommendation.index;
          const late = after && i < recommendation.index;
          const marks = [...row.places].sort(
            (a, b) => places.findIndex((p) => p.place === a.place) - places.findIndex((p) => p.place === b.place)
          );
          const line = late ? "before:bg-statusWarnSolid/45" : "before:bg-carbon-surface3";
          const ring = recommended ? "var(--status-ok-solid)" : late ? "var(--status-warn-solid)" : "var(--carbon-text-muted)";
          return (
            <div
              key={row.key}
              id={railRowId(i)}
              role="option"
              aria-selected={on}
              data-testid="rail-row"
              onClick={() => onChoose(i)}
              className={`group relative flex cursor-pointer flex-wrap items-center gap-x-3.5 gap-y-2 rounded-control py-3 pe-3.5 ps-10 before:absolute before:bottom-0 before:start-[17px] before:top-0 before:w-0.5 first:before:top-1/2 last:before:bottom-1/2 ${line} ${
                on ? "bg-accentSoft" : "hover:bg-carbon-surface2"
              }`}
            >
              <span
                className={`absolute start-3 top-1/2 -mt-1.5 h-3 w-3 rounded-full bg-carbon-surface${
                  recommended ? " glim-rail-pulse" : ""
                }`}
                style={{ boxShadow: `inset 0 0 0 3px ${ring}` }}
              />
              <div className="flex min-w-0 flex-1 basis-56 flex-col gap-1.5">
                <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1.5">
                  <b className="text-title font-semibold tabular-nums text-carbon-text first-letter:uppercase">
                    {formatRecent(Math.floor(Date.parse(row.time) / 1000), lang)}
                  </b>
                  {recommended && (
                    <Badge tone="ok" size="small">
                      <IconCheck />
                      {t("restorePage.recommended")}
                    </Badge>
                  )}
                  {late && (
                    <Badge tone="warn" size="small">
                      {t("restorePage.afterAnomaly")}
                    </Badge>
                  )}
                  {rowTags(row).map((tag) => (
                    <Badge key={tag} tone="neutral" size="small">
                      {tag}
                    </Badge>
                  ))}
                </div>
                <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5 text-subline text-carbon-textSub">
                  {row.size !== undefined && <span className="tabular-nums">{humanBytes(row.size)}</span>}
                  <span className="flex flex-wrap gap-1.5">
                    {marks.map((mark) => (
                      <span
                        key={mark.place}
                        className="inline-flex h-6 items-center gap-1.5 rounded-[calc(var(--radius-control)/1.5)] bg-carbon-surface2 px-2 group-aria-selected:bg-carbon-surface [&_svg]:h-3.5 [&_svg]:w-3.5 [&_svg]:text-accentText"
                      >
                        {places.find((p) => p.place === mark.place)?.remote ? <IconCloud /> : <IconLocal />}
                        {labelOf(mark.place)}
                        {mark.incomplete && ` · ${t("timeline.incomplete")}`}
                      </span>
                    ))}
                  </span>
                </div>
                {on && recommended && <p className="text-xs text-carbon-textMuted">{why}</p>}
              </div>
              <div
                data-new-way={on ? "restore-window" : undefined}
                className={`ms-auto max-md:w-full max-md:[&_button]:w-full max-md:[&_button]:justify-center${
                  on ? "" : " md:opacity-0 md:focus-within:opacity-100 md:group-hover:opacity-100"
                }`}
              >
                <Button
                  label={t("snapshots.restore")}
                  labelKey="snapshots.restore"
                  glyph={<IconRestore />}
                  tone={on ? "accent" : "neutral"}
                  onClick={() => onRestore(i)}
                />
              </div>
            </div>
          );
        })}
      </div>
      <span
        data-testid="rail-handle"
        aria-hidden="true"
        onPointerDown={grab}
        onPointerMove={drag}
        onPointerUp={() => setHeld(null)}
        onPointerCancel={() => setHeld(null)}
        style={{ top: held ?? rest }}
        className={`glim-rail-handle absolute start-[7px] -mt-[11px] h-[22px] w-[22px] rounded-full bg-accent ${
          held === null ? "cursor-grab" : "cursor-grabbing"
        }`}
      />
    </div>
  );
}
