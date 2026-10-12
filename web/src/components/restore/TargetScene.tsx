// What a restore does to the original, drawn per target: a copy lands next
// to it, it is replaced, files drop into a folder, or a ZIP leaves for the
// browser. The markup is the last frame and the stylesheet sets it in motion.
import type { CSSProperties, ReactNode } from "react";
import { useT } from "../../lib/i18n";
import type { RestoreWhere } from "../../lib/restorePlan";
import { LogoImage } from "../../pages/instances/InstanceCard";

/** What the server puts behind the name of a copy next to the original. */
export const COPY_SUFFIX = "-restored";

const VAULT = { x: 19, y: 30 };

function Box({ word, fresh = false }: { word: string; fresh?: boolean }) {
  return (
    <>
      <rect className={`glim-target-box${fresh ? " glim-target-box-fresh" : ""}`} x="-41" y="-11" width="82" height="22" rx="5" />
      <text className="glim-target-word" y="3.4">
        {word}
      </text>
    </>
  );
}

/** A part that leaves the vault at `at` and reaches its place at `land`. */
function fromVault(x: number, y: number, at: number, land: number, turn = 0): CSSProperties {
  return {
    "--from-x": `${VAULT.x - x}px`,
    "--from-y": `${VAULT.y - y}px`,
    "--at": at,
    "--land": land,
    "--turn": `${turn}deg`,
  } as CSSProperties;
}

function span(at: number, land: number): CSSProperties {
  return { "--at": at, "--land": land } as CSSProperties;
}

const FILE_TURNS = [-16, -5, 5, 16];

export function TargetScene({
  where,
  playing,
  kept = false,
}: {
  where: RestoreWhere;
  /** Only the chosen target plays. The others hold their last frame. */
  playing: boolean;
  /** Replacing: the state of today is backed up before it goes. */
  kept?: boolean;
}) {
  const { t } = useT();
  const original = t("restoreWindow.original");
  let body: ReactNode;

  if (where === "beside" || where === "newDataset") {
    body = (
      <>
        <g transform="translate(97 15)">
          <Box word={original} />
        </g>
        <g transform="translate(97 45)">
          <g className="glim-clock-show" style={{ "--at": 0.12 } as CSSProperties}>
            <g className="glim-clock-fly glim-clock-grow" style={fromVault(97, 45, 0.12, 0.5)}>
              <Box word={COPY_SUFFIX} fresh />
            </g>
          </g>
        </g>
      </>
    );
  } else if (where === "replace") {
    body = (
      <>
        <g transform="translate(97 30)">
          <g className="glim-clock-fly glim-clock-fold" style={span(0.46, 0.56)}>
            <Box word={original} />
          </g>
          <g className="glim-clock-fly glim-clock-unfold" style={span(0.56, 0.66)}>
            <Box word={original} fresh />
          </g>
        </g>
        {kept && (
          <>
            <g transform="translate(19 26)">
              <g className="glim-clock-show" style={{ "--at": 0.06, "--until": 0.36 } as CSSProperties}>
                <circle
                  className="glim-target-dot glim-clock-fly"
                  r="4"
                  style={{ "--from-x": "37px", "--from-y": "4px", "--at": 0.06, "--land": 0.34 } as CSSProperties}
                />
              </g>
            </g>
            <g transform="translate(19 30)">
              <circle className="glim-scene-ring glim-clock-ring" r="17" style={span(0.34, 0.48)} />
            </g>
            <g transform="translate(34 15)" className="glim-clock-show" style={{ "--at": 0.35 } as CSSProperties}>
              <circle className="glim-target-kept" r="6" />
              <path className="glim-target-tick" d="M-2.8 .2l1.9 1.9 3.6-3.8" />
            </g>
          </>
        )}
      </>
    );
  } else if (where === "folder") {
    body = (
      <>
        <path
          className="glim-target-folder glim-target-folder-back"
          d="M95 21.5a2.5 2.5 0 0 1 2.5-2.5h8l3.5 3.5h21.5a2.5 2.5 0 0 1 2.5 2.5V53H95z"
        />
        {FILE_TURNS.map((turn, k) => {
          const x = 102 + k * 8;
          const y = k % 3 ? 24 : 26;
          const at = 0.1 + k * 0.09;
          return (
            <g key={turn} transform={`translate(${x} ${y})`}>
              <g className="glim-clock-show" style={{ "--at": at } as CSSProperties}>
                <g className="glim-clock-fly" style={fromVault(x, y, at, at + 0.28, turn)}>
                  <rect className="glim-target-file" x="-6" y="-8" width="12" height="15" rx="1.5" />
                  <path className="glim-target-file-line" d="M-3 -4h6M-3 -1h6M-3 2h4" />
                </g>
              </g>
            </g>
          );
        })}
        <path className="glim-target-folder" d="M95 33.5a2 2 0 0 1 2-2h34a2 2 0 0 1 2 2V51a2 2 0 0 1-2 2H97a2 2 0 0 1-2-2z" />
      </>
    );
  } else {
    body = (
      <>
        <g transform="translate(115 34)">
          <g className="glim-clock-show" style={{ "--at": 0.1 } as CSSProperties}>
            <g className="glim-clock-fly" style={fromVault(115, 34, 0.1, 0.55)}>
              <rect className="glim-target-file" x="-9" y="-12" width="18" height="22" rx="2" />
              <path className="glim-target-zip" d="M0 -12v2.5h2v2.5h-2v2.5h2v2.5h-2" />
              <rect className="glim-target-dot" x="-2" y="1" width="4" height="4" rx="1" />
            </g>
          </g>
        </g>
        <path className="glim-target-tray" d="M96 40v9h38v-9" />
      </>
    );
  }

  return (
    <svg
      viewBox="0 0 140 60"
      aria-hidden="true"
      data-testid={`target-scene-${where}`}
      className={`glim-pic h-[60px] w-[140px] flex-none overflow-visible max-md:h-[52px] max-md:w-[120px]${
        playing ? " glim-clock glim-clock-quick" : " opacity-45"
      }`}
    >
      <LogoImage x={2} y={13} size={34} />
      {body}
    </svg>
  );
}
