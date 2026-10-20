// The pictures on the Verify and Security pages. Each is drawn as its last
// frame and set in motion by the stylesheet, so it stands still wherever
// motion is off.
import type { CSSProperties } from "react";
import { InfoBubble } from "../../components/InfoBubble";
import { IconEye, IconEyeOff, IconKey } from "../../components/glyphs";
import { useT } from "../../lib/i18n";

const PACKS = 100;
const COLUMNS = 20;

// A fixed shuffle of the packs. Raising the sample marks more squares and
// moves none.
const PACK_RANK = (() => {
  const order = Array.from({ length: PACKS }, (_, i) => i);
  const rank: number[] = [];
  let seed = 7;
  for (let i = PACKS - 1; i > 0; i--) {
    seed = (seed * 9301 + 49297) % 233280;
    const j = Math.floor((seed / 233280) * (i + 1));
    [order[i], order[j]] = [order[j], order[i]];
  }
  order.forEach((pack, k) => {
    rank[pack] = k;
  });
  return rank;
})();

function Tick({ x, y, size }: { x: number; y: number; size: number }) {
  const u = size / 8;
  return (
    <path
      className="glim-scene-cut-line"
      d={`M${x + 2 * u} ${y + 4.2 * u}l${1.5 * u} ${1.5 * u} ${2.6 * u}${-3 * u}`}
      strokeLinecap="round"
      strokeLinejoin="round"
    />
  );
}

/** PackGrid draws a backup as a hundred data packs and marks the share a
 *  check reads. */
export function PackGrid({ percent }: { percent: number }) {
  const { t } = useT();
  const sample = Math.max(0, Math.min(PACKS, Math.round(percent)));
  return (
    <figure className="glim-pic glim-scene flex min-w-0 flex-1 basis-64 flex-col gap-2">
      <svg viewBox="-2 -4 204 58" role="img" aria-label={t("tile.verify.packsAlt")}>
        {PACK_RANK.map((rank, i) => {
          const x = (i % COLUMNS) * 10 + 1;
          const y = Math.floor(i / COLUMNS) * 10 + 1;
          return (
            <g key={i}>
              <rect className="glim-pack" x={x} y={y} width="8" height="8" />
              {rank < sample && (
                <g className="glim-pack-hit" style={{ "--pack-at": (x / 200).toFixed(3) } as CSSProperties}>
                  <rect className="glim-pack glim-scene-seal" x={x} y={y} width="8" height="8" />
                  <Tick x={x} y={y} size={8} />
                </g>
              )}
            </g>
          );
        })}
        <rect className="glim-pack-scan" x="200" y="-3" width="3" height="56" rx="1.5" />
      </svg>
      <figcaption className="flex items-center justify-center gap-1.5 text-xs text-carbon-textSub">
        <span className="tabular-nums">{t("tile.verify.packs").replace("{n}", String(sample))}</span>
        <InfoBubble tip={t("tile.verify.packsHint")} />
      </figcaption>
    </figure>
  );
}

const PICKED: [number, number][] = [
  [6, 40],
  [18, 40],
  [6, 52],
  [18, 52],
];

function FileIcon({ x, turn }: { x: number; turn: number }) {
  return (
    <g className="glim-sandbox-file" style={{ "--file-turn": turn } as CSSProperties}>
      <path className="glim-sandbox-sheet" d={`M${x} 44h10l6 6v16h-16Z`} />
      <path className="glim-sandbox-line" d={`M${x + 10} 44v6h6M${x + 3} 56h10M${x + 3} 60h7`} />
    </g>
  );
}

/** SandboxPicture shows where a trial restore lands: in a sealed-off folder,
 *  and with the start test, a container started there once. */
export function SandboxPicture({ startTest }: { startTest: boolean }) {
  const { t } = useT();
  return (
    <figure className="glim-pic glim-scene flex min-w-0 flex-1 basis-64 flex-col gap-2">
      <svg viewBox="0 0 240 100" role="img" aria-label={t("tile.verify.sandboxAlt")}>
        {PICKED.map(([x, y]) => (
          <g key={`${x}:${y}`}>
            <rect className="glim-pack glim-scene-seal" x={x} y={y} width="10" height="10" />
            <Tick x={x} y={y} size={10} />
          </g>
        ))}
        <path className="glim-scene-flow" d="M34 51H70" />
        <path className="glim-scene-arrow" d="M66 46.5l5 4.5-5 4.5" />
        <rect className="glim-scene-box" x="76" y="6" width="160" height="88" />
        <text className="glim-sandbox-word" x="88" y="24">
          {t("tile.verify.sandboxWord")}
        </text>
        <FileIcon x={90} turn={0} />
        <FileIcon x={114} turn={1} />
        <FileIcon x={138} turn={2} />
        <g transform="translate(196 55)">
          {startTest && <circle className="glim-scene-ring glim-sandbox-pulse" r="15" />}
          <circle className={startTest ? "glim-scene-seal" : "glim-sandbox-idle"} r="15" />
          <path className={startTest ? "glim-scene-cut" : "glim-sandbox-idle-mark"} d="M-4.5 -7.5v15l12-7.5Z" />
        </g>
        <text className="glim-sandbox-caption" x="196" y="86" textAnchor="middle">
          {startTest ? t("settings.startTest") : t("tile.verify.startTestOff")}
        </text>
      </svg>
      <figcaption className="flex items-center justify-center gap-1.5 text-xs text-carbon-textSub">
        <span>{t("tile.verify.sandbox")}</span>
        <InfoBubble tip={t("tile.verify.sandboxHint")} />
      </figcaption>
    </figure>
  );
}

const SEALED: [number, number][] = [
  [379, 39],
  [403, 39],
  [379, 63],
  [403, 63],
];
// Where the packs leave from, next to the file they were cut from.
const DESK_X = 56;

/**
 * CryptScene shows what a backup leaves at a storage location: sealed packs
 * when it is encrypted, readable ones when it is not. The key never travels
 * with them and stays in the recovery kit.
 */
export function CryptScene({ sealed }: { sealed: boolean }) {
  const { t } = useT();
  return (
    <div className={`glim-pic glim-scene mx-auto w-full max-w-[680px] ${sealed ? "" : "glim-crypt-open"}`}>
      <svg viewBox="0 0 480 120" role="img" aria-label={t(sealed ? "tile.crypt.sceneAlt" : "tile.crypt.sceneAltOpen")}>
        <rect className="glim-scene-box" x="8" y="8" width="200" height="86" />
        <text className="glim-sandbox-caption" x="108" y="112" textAnchor="middle">
          {t("tile.crypt.atHome")}
        </text>
        <rect className="glim-scene-box" x="272" y="8" width="200" height="86" />
        <text className="glim-sandbox-caption" x="372" y="112" textAnchor="middle">
          {t("tile.crypt.location")}
        </text>
        <g transform="translate(36 51)">
          <path className="glim-sandbox-sheet" d="M-10 -14h12l8 8v20h-20z" />
          <path className="glim-sandbox-line" d="M2 -14v8h8M-6 0h12M-6 4h12M-6 8h7" />
        </g>
        <rect className="glim-crypt-kit" x="66" y="33" width="130" height="36" rx="9" />
        <g className="glim-crypt-key" transform="translate(76 43)">
          <IconKey />
        </g>
        <text className="glim-crypt-kit-word" x="104" y="55">
          {t("recovery.title")}
        </text>
        {/* Both faces of the eye and of each pack are drawn, so a flip of
            the toggle fades one into the other. */}
        <g className="glim-crypt-looker" transform="translate(333 43)">
          <g className="glim-crypt-sealed">
            <IconEyeOff />
          </g>
          <g className="glim-crypt-readable">
            <IconEye />
          </g>
        </g>
        {SEALED.map(([x, y], turn) => (
          <g key={turn} transform={`translate(${x} ${y})`}>
            <g
              style={
                {
                  "--parcel-from": `${DESK_X - x}px`,
                  "--parcel-turn": turn,
                  "--parcel-hop": 1 + turn * 0.35,
                } as CSSProperties
              }
            >
              <g className="glim-parcel-fly">
                <g className="glim-parcel-hop">
                  <g className="glim-crypt-sealed">
                    <rect className="glim-pack glim-scene-seal" x="-7.5" y="-7.5" width="15" height="15" />
                    <rect className="glim-scene-cut" x="-3.6" y="-1" width="7.2" height="5.8" rx="1.2" />
                    <path className="glim-scene-cut-line" d="M-2.1 -1v-1.7a2.1 2.1 0 0 1 4.2 0v1.7" />
                  </g>
                  <g className="glim-crypt-readable">
                    <path className="glim-sandbox-sheet" d="M-6 -7.5h8l4 4v11h-12z" />
                    <path className="glim-sandbox-line" d="M2 -7.5v4h4M-3.5 -0.5h7M-3.5 2.5h7M-3.5 5.5h4" />
                  </g>
                </g>
              </g>
              <circle className="glim-scene-ring glim-parcel-ring" r="8" />
            </g>
          </g>
        ))}
      </svg>
    </div>
  );
}
