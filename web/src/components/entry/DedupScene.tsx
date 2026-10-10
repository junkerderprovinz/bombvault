import { useEffect, useRef, useState, type CSSProperties } from "react";
import { humanBytes } from "../../lib/forecast";
import { useT } from "../../lib/i18n";

const SIZE = 24;
const SLOTS = 12;
// Three of the twelve blocks are new to the repository, the rest meet a twin.
const NEW = new Set([2, 7, 10]);
// The order the blocks fall in, by slot.
const ORDER = [8, 6, 10, 11, 7, 9, 1, 4, 0, 2, 5, 3];
// A linked block is drawn in one of three strengths, so the row reads as
// different content and not as one bar.
const LINKED = ["opacity-50", "opacity-70", "opacity-90"];

function slotAt(k: number): [number, number] {
  return [58 + (k % 6) * 32, 72 + Math.floor(k / 6) * 32];
}

// The pictures that have played while the app is open. A picture seen once
// shows its last frame when its card comes back.
const played = new Set<string>();

/**
 * DedupScene shows what deduplication does to one backup: twelve blocks fall
 * into the repository, nine meet a block that is there already and are only
 * linked, three are stored. Its markup is the last frame, and the fall plays
 * once when the picture is in view.
 */
export function DedupScene({ id, read, added }: { id: string; read: number; added: number }) {
  const { t } = useT();
  const picture = useRef<SVGSVGElement>(null);
  const [playing, setPlaying] = useState(false);

  useEffect(() => {
    const el = picture.current;
    if (!el || played.has(id) || typeof IntersectionObserver === "undefined") return;
    const watch = new IntersectionObserver(
      (seen) => {
        if (!seen[0].isIntersecting) return;
        watch.disconnect();
        played.add(id);
        setPlaying(true);
      },
      { threshold: 0.6 }
    );
    watch.observe(el);
    return () => watch.disconnect();
  }, [id]);

  return (
    <div className="flex flex-col items-center gap-2">
      <svg
        ref={picture}
        viewBox="0 0 300 146"
        role="img"
        aria-label={t("entry.dedup.label")}
        className={`w-full max-w-[19rem] ${playing ? "glim-dedup-play" : ""}`}
      >
        <rect
          x="40"
          y="62"
          width="220"
          height="76"
          rx="12"
          strokeDasharray="3 5"
          className="fill-carbon-surface2 stroke-carbon-textMuted/40"
        />
        <text x="256" y="56" textAnchor="end" fontSize="9" className="fill-carbon-textSub">
          {t("entry.settings.repository")}
        </text>
        {Array.from({ length: SLOTS }, (_, k) => {
          const [x, y] = slotAt(k);
          const turn = { "--block-i": ORDER.indexOf(k) } as CSSProperties;
          return (
            <g key={k}>
              {NEW.has(k) ? (
                <>
                  <rect
                    x={x + 1}
                    y={y + 1}
                    width={SIZE - 2}
                    height={SIZE - 2}
                    rx="4"
                    strokeDasharray="3 3"
                    className="fill-none stroke-carbon-textMuted/60"
                  />
                  <rect x={x} y={y} width={SIZE} height={SIZE} rx="5" style={turn} className="glim-block-land fill-accent" />
                </>
              ) : (
                <rect
                  x={x}
                  y={y}
                  width={SIZE}
                  height={SIZE}
                  rx="5"
                  style={turn}
                  className={`glim-block-swell fill-carbon-textMuted ${LINKED[k % LINKED.length]}`}
                />
              )}
              <rect
                x={x}
                y={y}
                width={SIZE}
                height={SIZE}
                rx="5"
                opacity="0"
                style={turn}
                className={`glim-block-fall ${NEW.has(k) ? "fill-accent" : "fill-carbon-textMuted"}`}
              />
            </g>
          );
        })}
      </svg>
      <div className="flex flex-col items-center gap-1 text-center">
        <b className="text-sm font-semibold tabular-nums text-carbon-text">
          {t("entry.dedup.figures").replace("{read}", humanBytes(read)).replace("{added}", humanBytes(added))}
        </b>
        <span className="flex flex-wrap justify-center gap-x-4 gap-y-1 text-xs text-carbon-textMuted">
          <span className="inline-flex items-center gap-1.5">
            <i aria-hidden="true" className="inline-block h-2 w-2 rounded-xs bg-carbon-textMuted opacity-70" />
            {t("entry.dedup.linked")}
          </span>
          <span className="inline-flex items-center gap-1.5">
            <i aria-hidden="true" className="inline-block h-2 w-2 rounded-xs bg-accent" />
            {t("entry.dedup.stored")}
          </span>
        </span>
      </div>
    </div>
  );
}
