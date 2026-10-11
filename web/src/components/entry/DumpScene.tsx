import type { ReactNode } from "react";
import { useT } from "../../lib/i18n";
import { IconPause } from "../glyphs";
import { InfoBubble } from "../InfoBubble";
import { IconContainers, IconDatabase, IconFiles } from "../navGlyphs";

const SHEET = "M-10 -13h13l7 7v19h-20z";
const SHEET_FOLD = "M3 -13v7h7";

/** A 16px glyph set into the picture with its top left corner at x, y. */
function Glyph({ x, y, size, className, children }: { x: number; y: number; size: number; className: string; children: ReactNode }) {
  return (
    <g transform={`translate(${x} ${y}) scale(${size / 16})`} className={className}>
      {children}
    </g>
  );
}

function Sheet() {
  return (
    <>
      <path d={SHEET} className="fill-(--pic)" />
      <path d={SHEET_FOLD} className="fill-none stroke-carbon-surface2/50" />
    </>
  );
}

function Chip({ className = "", children }: { className?: string; children: ReactNode }) {
  return (
    <foreignObject x="0" y="14" width="100" height="24" className={className}>
      <div className="flex h-full items-center justify-center">
        <span className="inline-flex items-center gap-1.5 rounded-pill bg-carbon-surface3 px-2 py-0.5 text-[10px] leading-tight text-carbon-text">
          {children}
        </span>
      </div>
    </foreignObject>
  );
}

function Caption({ className = "", children }: { className?: string; children: ReactNode }) {
  return (
    <foreignObject x="0" y="148" width="304" height="24" className={className}>
      <p className="flex h-full items-center justify-center text-center text-[11px] leading-tight text-carbon-textSub">
        {children}
      </p>
    </foreignObject>
  );
}

/** A place in the repository: an outline while it is empty, a filled field
 *  once the backup has put its restore point there. */
function Slot({ y, lit, className = "", children }: { y: number; lit: boolean; className?: string; children?: ReactNode }) {
  return (
    <>
      <rect
        x="192"
        y={y - 14}
        width="100"
        height="28"
        rx="8"
        strokeDasharray="3 4"
        className="fill-none stroke-carbon-textMuted/50"
      />
      {lit && (
        <g className={className}>
          <rect x="192" y={y - 14} width="100" height="28" rx="8" className="fill-carbon-surface3" />
          {children}
        </g>
      )}
    </>
  );
}

function SlotLabel({ y, mono = false, children }: { y: number; mono?: boolean; children: ReactNode }) {
  return (
    <foreignObject x="216" y={y - 9} width="74" height="18">
      <div className={`flex h-full items-center truncate text-[10px] leading-none text-carbon-text ${mono ? "font-mono" : ""}`}>
        {children}
      </div>
    </foreignObject>
  );
}

/**
 * DumpScene shows one backup of a container that is a database: the dump
 * leaves the running database as a file, the container's files follow, and
 * both lie in the repository as two restore points. Its markup is the last
 * frame, and the stylesheet plays the way there as a loop. Without the dump
 * the picture stands still and shows the one restore point that is left.
 */
export function DumpScene({ on, stops }: { on: boolean; /** The container is stopped while its files are read. */ stops: boolean }) {
  const { t } = useT();
  const play = (name: string) => (on ? name : "");

  return (
    <figure className="glim-pic glim-dump m-0 flex min-w-0 flex-col gap-1 rounded-card bg-carbon-surface2 px-3.5 pb-3 pt-2.5">
      <figcaption className="flex items-center gap-1.5 text-sm font-semibold text-carbon-text">
        {t("entry.dump.title")}
        <InfoBubble tip={t("entry.dump.hint")} />
      </figcaption>
      <svg
        viewBox="0 0 304 172"
        role="img"
        aria-label={t(on ? "entry.dump.label" : "entry.dump.labelWithout")}
        className="mx-auto block w-full max-w-[380px] overflow-visible"
      >
        <rect
          x="184"
          y="32"
          width="116"
          height="106"
          rx="12"
          strokeDasharray="3 5"
          className="fill-carbon-surface stroke-carbon-textMuted/40"
        />
        <foreignObject x="184" y="14" width="116" height="16">
          <div className="flex h-full items-center justify-end text-[9px] leading-none text-carbon-textSub">
            {t("entry.settings.repository")}
          </div>
        </foreignObject>
        <path
          d="M100 58H182"
          strokeWidth="2"
          strokeLinecap="round"
          strokeDasharray="2 7"
          className={`fill-none ${on ? "stroke-carbon-textMuted/60" : "stroke-carbon-textMuted/25"}`}
        />
        <path
          d="M88 110H182"
          strokeWidth="2"
          strokeLinecap="round"
          strokeDasharray="2 7"
          className="fill-none stroke-carbon-textMuted/60"
        />
        <rect x="16" y="52" width="68" height="68" rx="14" className="fill-carbon-surface3" />
        <Glyph x={31} y={69} size={38} className="text-carbon-textSub">
          <IconContainers />
        </Glyph>

        <Chip className={stops ? play("glim-dump-running") : ""}>
          <i aria-hidden="true" className="h-[7px] w-[7px] flex-none rounded-full bg-statusOkSolid" />
          {t("entry.dump.running")}
        </Chip>
        {on && stops && (
          <Chip className="glim-dump-paused opacity-0">
            <span aria-hidden="true" className="flex text-carbon-textSub [&_svg]:h-2 [&_svg]:w-2">
              <IconPause />
            </span>
            {t("entry.dump.paused")}
          </Chip>
        )}

        <Slot y={58} lit={on} className="glim-dump-settle">
          <g transform="translate(206 58) scale(.62)">
            <Sheet />
          </g>
          <SlotLabel y={58} mono>
            database.sql
          </SlotLabel>
        </Slot>
        {!on && (
          <foreignObject x="192" y="49" width="100" height="18">
            <div className="flex h-full items-center justify-center text-[10px] leading-none text-carbon-textMuted">
              {t("entry.dump.none")}
            </div>
          </foreignObject>
        )}
        <Slot y={110} lit className={play("glim-dump-settle")}>
          <Glyph x={200} y={104} size={12} className="text-carbon-textSub">
            <IconFiles />
          </Glyph>
          <SlotLabel y={110}>{t("entry.dump.backup")}</SlotLabel>
        </Slot>

        <g transform="translate(84 58)">
          {on && <circle r="13" strokeWidth="2" className="glim-dump-ring fill-none stroke-(--pic) opacity-0" />}
          <g className={play("glim-dump-badge")}>
            <circle r="13" strokeWidth="3" className={`stroke-carbon-surface2 ${on ? "fill-(--pic)" : "fill-carbon-surface3"}`} />
            <Glyph x={-7} y={-7} size={14} className={on ? "text-carbon-surface2" : "text-carbon-textMuted"}>
              <IconDatabase />
            </Glyph>
          </g>
        </g>

        {on && (
          <>
            <g transform="translate(202 110)">
              <g className="glim-dump-pack opacity-0">
                <circle r="11" className="fill-carbon-surface3" />
                <Glyph x={-6} y={-6} size={12} className="text-carbon-textSub">
                  <IconFiles />
                </Glyph>
              </g>
            </g>
            <g transform="translate(206 58)">
              <g className="glim-dump-sheet opacity-0">
                <g className="glim-dump-sheet-grow">
                  <g transform="scale(.62)">
                    <Sheet />
                    <text y="7" textAnchor="middle" fontSize="6.5" fontWeight="700" className="fill-carbon-surface2">
                      SQL
                    </text>
                  </g>
                </g>
              </g>
            </g>
            <text
              x="142"
              y="88"
              textAnchor="middle"
              fontSize="9.5"
              className="glim-dump-name fill-carbon-text font-mono opacity-0"
            >
              database.sql
            </text>
            <Caption className="glim-dump-say-dump opacity-0">{t("entry.dump.fromRunning")}</Caption>
            <Caption className="glim-dump-say-files opacity-0">{t("entry.dump.thenFiles")}</Caption>
          </>
        )}
        <Caption className={play("glim-dump-settle")}>{t(on ? "entry.dump.twoPoints" : "entry.dump.without")}</Caption>
      </svg>
    </figure>
  );
}
