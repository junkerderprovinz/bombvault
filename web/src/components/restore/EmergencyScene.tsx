// The emergency as one picture: the kit is read, the key opens, the storage
// locations are found and the entries come back. The markup is the last
// frame, with every station done, and the stylesheet plays it.
import type { CSSProperties, ReactNode } from "react";
import { useT, type TranslationKey } from "../../lib/i18n";
import { IconSearch } from "../glyphs";
import { IconCloud, IconContainers, IconFolder, IconLocal, IconRestore, IconTabStorage, IconVM } from "../navGlyphs";

// When each station is done, as a share of the pass.
const DONE = [0.22, 0.44, 0.66, 0.9];
const WORDS: TranslationKey[] = ["emergency.scene.kit", "emergency.scene.key", "emergency.scene.find", "emergency.scene.back"];
const SPOTS: [number, number][] = [
  [-11, -8],
  [11, -8],
  [0, 11],
];
const Y = 46;

function centre(k: number): number {
  return 50 + k * 100;
}

function starts(k: number): number {
  return k === 0 ? 0.04 : DONE[k - 1] + 0.04;
}

function at(share: number, more: Record<string, number | string> = {}): CSSProperties {
  return { "--at": share, ...more } as CSSProperties;
}

function Glyph({ size, children }: { size: number; children: ReactNode }) {
  const scale = size / 16;
  return <g transform={`translate(${-size / 2} ${-size / 2}) scale(${scale})`}>{children}</g>;
}

/** Three small things that turn up one after the other in a station. */
function Found({ from, step, glyphs }: { from: number; step: number; glyphs: (() => ReactNode)[] }) {
  return (
    <>
      {glyphs.map((Shape, i) => (
        <g key={i} transform={`translate(${SPOTS[i][0]} ${SPOTS[i][1]})`}>
          <g className="glim-rescue-glyph glim-clock-show glim-clock-pop" style={at(from + i * step)}>
            <Glyph size={14}>
              <Shape />
            </Glyph>
          </g>
        </g>
      ))}
    </>
  );
}

function Kit() {
  return (
    <>
      <path className="glim-rescue-fill" d="M-7 -16H3L11 -8V12a4 4 0 0 1-4 4H-7a4 4 0 0 1-4-4V-12a4 4 0 0 1 4-4Z" />
      <path className="glim-rescue-cut" d="M3 -16v5a3 3 0 0 0 3 3h5M-6 -2h12M-6 3h12M-6 8h7" />
    </>
  );
}

/** The lock turns its key and then swings its shackle open. */
function Lock() {
  return (
    <g className="glim-clock-dim" style={at(starts(1))}>
      <path
        className="glim-rescue-shackle glim-clock-fly"
        d="M-6 -3v-5a6 6 0 0 1 12 0v5"
        style={{ ...at(0.36, { "--land": 0.41, "--turn": "32deg" }), transformOrigin: "6px -3px" }}
      />
      <rect className="glim-rescue-fill" x="-11" y="-3" width="22" height="17" rx="4" />
      <g className="glim-clock-fly" style={{ ...at(0.29, { "--land": 0.35, "--turn": "90deg" }), transformOrigin: "0 4px" }}>
        <path className="glim-rescue-cut" d="M0 4v5.5" />
        <circle className="glim-rescue-cut-dot" cy="4" r="2.4" />
      </g>
    </g>
  );
}

function Search({ from, step, waits, glyphs }: { from: number; step: number; waits: () => ReactNode; glyphs: (() => ReactNode)[] }) {
  const Waits = waits;
  return (
    <>
      <g className="glim-rescue-glyph-wait glim-clock-wait" style={at(from)}>
        <Glyph size={18}>
          <Waits />
        </Glyph>
      </g>
      <Found from={from} step={step} glyphs={glyphs} />
    </>
  );
}

const STATIONS: (() => ReactNode)[] = [
  Kit,
  Lock,
  () => <Search from={0.52} step={0.045} waits={IconSearch} glyphs={[IconLocal, IconCloud, IconTabStorage]} />,
  () => <Search from={0.74} step={0.05} waits={IconRestore} glyphs={[IconContainers, IconVM, IconFolder]} />,
];

export function EmergencyScene({ words = false }: { words?: boolean }) {
  const { t } = useT();

  return (
    <div className="glim-pic glim-clock glim-clock-long mx-auto flex w-full max-w-[520px] flex-col gap-1.5">
      <svg viewBox="0 0 400 92" role="img" aria-label={t("emergency.scene.alt")} className="block h-auto w-full overflow-visible">
        {[0, 1, 2].map((k) => {
          const d = `M${centre(k) + 35} ${Y}H${centre(k + 1) - 35}`;
          return (
            <g key={k}>
              <path className="glim-rescue-track" d={d} />
              <path className="glim-rescue-track glim-rescue-track-lit glim-clock-show" d={d} style={at(DONE[k] + 0.03)} />
              <g transform={`translate(${centre(k + 1) - 35} ${Y})`}>
                <g className="glim-clock-show" style={at(DONE[k], { "--until": DONE[k] + 0.04 })}>
                  <circle
                    className="glim-rescue-fill glim-clock-fly"
                    r="4"
                    style={at(DONE[k], { "--land": DONE[k] + 0.04, "--from-x": "-30px" })}
                  />
                </g>
              </g>
            </g>
          );
        })}
        {STATIONS.map((Inside, k) => (
          <g key={k} transform={`translate(${centre(k)} ${Y})`}>
            <circle className="glim-rescue-station" r="30" />
            <circle className="glim-rescue-ring glim-clock-show" r="30" style={at(starts(k), { "--until": DONE[k] + 0.02 })} />
            <Inside />
            <g transform="translate(22 -22)">
              <g className="glim-clock-show glim-clock-pop" style={at(DONE[k])}>
                <circle className="glim-rescue-ok" r="8" />
                <path className="glim-rescue-tick" d="M-3.4 .2l2.4 2.4 4.4-4.8" />
              </g>
            </g>
          </g>
        ))}
      </svg>
      {words && (
        <div className="grid grid-cols-4" aria-hidden="true">
          {WORDS.map((key, k) => (
            <span
              key={key}
              className="glim-rescue-word flex flex-col items-center gap-1 px-1 text-center text-xs leading-tight"
              style={at(starts(k), { "--until": DONE[k] })}
            >
              <b className="inline-flex h-[18px] w-[18px] items-center justify-center rounded-full text-meta tabular-nums">{k + 1}</b>
              <span>{t(key)}</span>
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
