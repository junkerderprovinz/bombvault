// The small pictures of the Pairing tab: one per step card and one per relay
// route. They draw with the theme's own tokens, so they follow dark and light
// and take the hue of the card they sit in through --accent.
import { useId, type ReactNode } from "react";
import type { RelayMode } from "../../../lib/api";

const NODE = "var(--carbon-surface2)";
const RELAY = "var(--carbon-surface3)";
const INK = "var(--carbon-text)";
const LABEL = "var(--carbon-text-sub)";
const LINE = "var(--carbon-text-muted)";
const KEY = "var(--accent)";
const KEY_INK = "var(--accent-contrast)";
const AREA = "color-mix(in srgb, var(--carbon-text) 6%, transparent)";
const OK = "var(--status-ok-solid)";

/** The key: it only ever sits on an instance. */
function Lock({ x, y }: { x: number; y: number }) {
  return (
    <g transform={`translate(${x} ${y})`}>
      <circle r="9" fill={KEY} />
      <rect x="-4" y="-1.2" width="8" height="6.4" rx="1.4" fill={KEY_INK} />
      <path d="M-2.4 -1.2v-1.9a2.4 2.4 0 0 1 4.8 0v1.9" fill="none" stroke={KEY_INK} strokeWidth="1.5" />
    </g>
  );
}

/** A sealed message on its way. */
function Envelope({ x, y }: { x: number; y: number }) {
  return (
    <g transform={`translate(${x} ${y})`}>
      <rect x="-8" y="-6" width="16" height="12" rx="2.2" fill={KEY} />
      <path d="M-6.2 -4 0 .9 6.2 -4" fill="none" stroke={KEY_INK} strokeWidth="1.5" strokeLinejoin="round" strokeLinecap="round" />
    </g>
  );
}

function Instance({ x, y, label, dim = false }: { x: number; y: number; label: string; dim?: boolean }) {
  return (
    <g opacity={dim ? 0.45 : undefined}>
      <rect x={x} y={y} width="56" height="44" rx="10" fill={NODE} />
      <text x={x + 28} y={y + 28} textAnchor="middle" fontSize="17" fontWeight="700" fill={INK}>
        {label}
      </text>
      <Lock x={x + 54} y={y + 2} />
    </g>
  );
}

function Line({ x1, x2, y, dashed = false }: { x1: number; x2: number; y: number; dashed?: boolean }) {
  return (
    <line x1={x1} y1={y} x2={x2} y2={y} stroke={LINE} strokeWidth="2" strokeLinecap="round" strokeDasharray={dashed ? "3 5" : undefined} />
  );
}

function Caption({ x, y, children, anchor = "middle" }: { x: number; y: number; children: ReactNode; anchor?: "middle" | "start" }) {
  return (
    <text x={x} y={y} textAnchor={anchor} fontSize="12" fill={LABEL}>
      {children}
    </text>
  );
}

export interface RouteDiagramLabels {
  relay: string;
  projectHost: string;
  ownAddress: string;
  yourNetwork: string;
  otherNetwork: string;
  yourLan: string;
}

/** RouteDiagram shows how a message travels on one relay route. */
export function RouteDiagram({ mode, labels, alt }: { mode: RelayMode; labels: RouteDiagramLabels; alt: string }) {
  let body: ReactNode;
  if (mode === "off") {
    body = (
      <>
        <rect x="2" y="8" width="170" height="96" rx="14" fill={AREA} />
        <Caption x={14} y={26} anchor="start">
          {labels.yourLan}
        </Caption>
        <Line x1={72} x2={106} y={58} />
        <Envelope x={89} y={58} />
        <Instance x={16} y={40} label="A" />
        <Instance x={106} y={40} label="B" />
        <Line x1={166} x2={196} y={62} dashed />
        <path d="M176 56l8 8M184 56l-8 8" stroke={LINE} strokeWidth="2" strokeLinecap="round" />
        <Instance x={200} y={40} label="C" dim />
        <Caption x={228} y={100}>
          {labels.otherNetwork}
        </Caption>
      </>
    );
  } else {
    const project = mode === "project";
    body = (
      <>
        <Line x1={64} x2={project ? 98 : 104} y={58} />
        <Line x1={project ? 162 : 156} x2={196} y={58} />
        {project ? (
          <path d="M102 78h56a13 13 0 0 0 1-26A20 20 0 0 0 124 41 14 14 0 0 0 103 55 12 12 0 0 0 102 78z" fill={RELAY} />
        ) : (
          <rect x="104" y="37" width="52" height="42" rx="8" fill={RELAY} />
        )}
        <text x={project ? 131 : 130} y={project ? 68 : 62} textAnchor="middle" fontSize="11" fontWeight="600" fill={INK}>
          {labels.relay}
        </text>
        <Envelope x={project ? 81 : 84} y={58} />
        <Envelope x={project ? 179 : 176} y={58} />
        <Instance x={8} y={36} label="A" />
        <Instance x={196} y={36} label="B" />
        <Caption x={130} y={22}>
          {project ? labels.projectHost : labels.ownAddress}
        </Caption>
        <Caption x={36} y={100}>
          {labels.yourNetwork}
        </Caption>
        <Caption x={224} y={100}>
          {labels.otherNetwork}
        </Caption>
      </>
    );
  }
  return (
    <svg viewBox="0 0 272 108" role="img" aria-label={alt} className="block w-full h-auto max-w-[400px] justify-self-center mx-auto">
      <g transform="translate(4 0)">{body}</g>
    </svg>
  );
}

/** Legend entries for the two symbols the diagrams use. */
export function LegendKey() {
  return (
    <svg viewBox="-10 -10 20 20" width="20" height="20" aria-hidden="true">
      <Lock x={0} y={0} />
    </svg>
  );
}

export function LegendMessage() {
  return (
    <svg viewBox="-10 -10 20 20" width="20" height="20" aria-hidden="true">
      <Envelope x={0} y={0} />
    </svg>
  );
}

function Window({ x, y, w, h, label, lit }: { x: number; y: number; w: number; h: number; label: string; lit: boolean }) {
  return (
    <>
      <rect x={x} y={y} width={w} height={h} rx="10" fill={NODE} />
      <circle cx={x + 14} cy={y + 14} r="8" fill={lit ? KEY : RELAY} />
      <text x={x + 14} y={y + 17.5} textAnchor="middle" fontSize="10" fontWeight="700" fill={lit ? KEY_INK : INK}>
        {label}
      </text>
    </>
  );
}

/** Word pills in a grid: the first `filled` are typed, `cursor` is the one
 *  being typed. */
function Words({
  x0,
  y0,
  cols,
  rows,
  w,
  h,
  gx,
  gy,
  filled,
  cursor = -1,
}: {
  x0: number;
  y0: number;
  cols: number;
  rows: number;
  w: number;
  h: number;
  gx: number;
  gy: number;
  filled: number;
  cursor?: number;
}) {
  const out: ReactNode[] = [];
  for (let i = 0; i < cols * rows; i++) {
    const x = x0 + (i % cols) * (w + gx);
    const y = y0 + Math.floor(i / cols) * (h + gy);
    if (i === cursor) {
      out.push(
        <g key={i}>
          <rect x={x + 0.75} y={y + 0.75} width={w - 1.5} height={h - 1.5} rx={h / 2} fill="none" stroke={KEY} strokeWidth="1.5" />
          <path d={`M${x + 6} ${y + 2.5}v${h - 5}`} stroke={INK} strokeWidth="1.5" strokeLinecap="round" />
        </g>,
      );
    } else {
      out.push(<rect key={i} x={x} y={y} width={w} height={h} rx={h / 2} fill={i < filled ? KEY : RELAY} />);
    }
  }
  return <>{out}</>;
}

/** StepPicture is the drawing on step card 1, 2 or 3. */
export function StepPicture({ step }: { step: 1 | 2 | 3 }) {
  let body: ReactNode;
  if (step === 1) {
    body = (
      <>
        <Window x={40} y={6} w={120} h={108} label="A" lit />
        <Words x0={54} y0={34} cols={3} rows={4} w={28} h={10} gx={5} gy={8} filled={12} />
      </>
    );
  } else if (step === 2) {
    body = (
      <>
        <Window x={4} y={32} w={58} h={58} label="A" lit={false} />
        <Words x0={12} y0={56} cols={3} rows={4} w={12} h={4} gx={3} gy={3} filled={12} />
        <path d="M68 61h14" stroke={LINE} strokeWidth="2" strokeLinecap="round" />
        <path d="M80 56l6 5-6 5" fill="none" stroke={LINE} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
        <Window x={92} y={6} w={104} h={108} label="B" lit />
        <Words x0={104} y0={34} cols={3} rows={4} w={24} h={10} gx={5} gy={8} filled={7} cursor={7} />
      </>
    );
  } else {
    body = (
      <>
        <rect x="4" y="6" width="192" height="108" rx="16" fill={AREA} />
        <Window x={16} y={34} w={64} h={62} label="A" lit />
        <Words x0={26} y0={62} cols={3} rows={3} w={12} h={5} gx={3} gy={4} filled={9} />
        <Window x={120} y={34} w={64} h={62} label="B" lit />
        <Words x0={130} y0={62} cols={3} rows={3} w={12} h={5} gx={3} gy={4} filled={9} />
        <path d="M80 65h40" stroke={LINE} strokeWidth="2" />
        <Lock x={100} y={65} />
        <g transform="translate(100 22)">
          <circle r="10" fill={OK} />
          <path d="M-4.4 .2-1.2 3.4 4.6-2.8" fill="none" stroke="var(--carbon-bg)" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" />
        </g>
      </>
    );
  }
  return (
    <svg viewBox="0 0 200 120" aria-hidden="true" className="block w-full h-auto max-w-[230px] mx-auto">
      {body}
    </svg>
  );
}

const CLOUD = "M5.4 16.5h9.2a3.9 3.9 0 0 0 .6-7.8A5.4 5.4 0 0 0 4.9 7.3a4.6 4.6 0 0 0 .5 9.2z";

/** RouteGlyph draws a relay route as a filled glyph, the way the selector's
 *  other glyphs are drawn; the two relay sources keep line glyphs. */
export function RouteGlyph({ kind }: { kind: RelayMode | "relay" | "server" }) {
  // useId has colons, which a url(#...) reference does not take in every browser.
  const cut = `route-cut${useId().replace(/[^\w-]/g, "")}`;
  const box = { viewBox: "0 0 20 20", width: 20, height: 20, "aria-hidden": true as const };
  const line = {
    ...box,
    fill: "none",
    stroke: "currentColor",
    strokeWidth: 1.7,
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
  };
  switch (kind) {
    case "project":
      return (
        <svg {...box} fill="currentColor">
          <path d={CLOUD} />
        </svg>
      );
    case "own":
      return (
        <svg {...box} fill="currentColor">
          <path d="M10 2.4 2.3 9.2c-.5.4-.2 1.2.5 1.2h1.6V17a1 1 0 0 0 1 1h3.1v-4.6h3V18h3.1a1 1 0 0 0 1-1v-6.6h1.6c.7 0 1-.8.5-1.2z" />
        </svg>
      );
    case "off":
      // The slash is cut out of the cloud so it reads on any ground.
      return (
        <svg {...box} fill="currentColor">
          <mask id={cut}>
            <rect width="20" height="20" fill="white" />
            <path d="M3 3l14 14" stroke="black" strokeWidth="3.6" strokeLinecap="round" />
          </mask>
          <path d={CLOUD} mask={`url(#${cut})`} />
          <path d="M3.2 3.2l13.6 13.6" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
        </svg>
      );
    case "relay":
      return (
        <svg {...line}>
          <path d="M3.5 7h12M12.5 4l3 3-3 3M16.5 13h-12M7.5 10l-3 3 3 3" />
        </svg>
      );
    case "server":
      return (
        <svg {...line}>
          <rect x="3.5" y="9" width="13" height="8" rx="1.8" />
          <path d="M6.5 13h.01" />
          <path d="M7 5.8a4.4 4.4 0 0 1 6 0M4.8 3.6a7.6 7.6 0 0 1 10.4 0" />
        </svg>
      );
  }
}

export function FactGlyph({ kind }: { kind: "need" | "sees" | "search" | "shield" | "lock" | "hidden" }) {
  const common = {
    viewBox: "0 0 16 16",
    width: 16,
    height: 16,
    fill: "none",
    stroke: "currentColor",
    strokeWidth: 1.5,
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
    "aria-hidden": true,
    className: "mt-0.5 shrink-0",
  };
  switch (kind) {
    case "need":
      return (
        <svg {...common}>
          <path d="M5 2.5h6M5 2.5a1 1 0 0 0-1 1V14h8V3.5a1 1 0 0 0-1-1" />
          <path d="M6.2 7.3l1.3 1.3 2.4-2.6M6.2 11h3.6" />
        </svg>
      );
    case "sees":
      return (
        <svg {...common}>
          <path d="M1.5 8S4 3.5 8 3.5 14.5 8 14.5 8 12 12.5 8 12.5 1.5 8 1.5 8z" />
          <circle cx="8" cy="8" r="2.1" />
        </svg>
      );
    case "search":
      return (
        <svg {...common}>
          <circle cx="7" cy="7" r="4.4" />
          <path d="M10.3 10.3 14 14" />
        </svg>
      );
    case "shield":
      return (
        <svg {...common}>
          <path d="M8 1.6 13.2 3.7v4c0 3.1-2.2 5.4-5.2 6.7-3-1.3-5.2-3.6-5.2-6.7v-4z" />
          <path d="M5.8 8.1 7.4 9.7 10.4 6.4" />
        </svg>
      );
    case "lock":
      return (
        <svg {...common}>
          <rect x="3" y="7" width="10" height="7" rx="1.6" fill="currentColor" stroke="none" />
          <path d="M5.2 7V5.1a2.8 2.8 0 0 1 5.6 0V7" />
        </svg>
      );
    case "hidden":
      return (
        <svg {...common}>
          <path d="M1.5 8S4 3.5 8 3.5 14.5 8 14.5 8 12 12.5 8 12.5 1.5 8 1.5 8z" />
          <circle cx="8" cy="8" r="2.1" />
          <path d="M2.6 2.6l10.8 10.8" />
        </svg>
      );
  }
}

/** The glyphs of the phrase card: the two answers to its question, the next
 *  step, the path to it, and the warning and done marks of the hints. */
export function PhraseGlyph({ kind, size = 16 }: { kind: "create" | "enter" | "arrow" | "chevron" | "warn" | "check"; size?: number }) {
  const common = {
    viewBox: "0 0 20 20",
    width: size,
    height: size,
    fill: "none",
    stroke: "currentColor",
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
    "aria-hidden": true,
    className: "shrink-0",
  };
  switch (kind) {
    case "create":
      return (
        <svg {...common} strokeWidth={1.8}>
          <rect x="3" y="3" width="14" height="14" rx="3.5" />
          <path d="M10 6.8v6.4M6.8 10h6.4" />
        </svg>
      );
    case "enter":
      return (
        <svg {...common} strokeWidth={1.6}>
          <rect x="2.5" y="5" width="15" height="10" rx="2.2" />
          <path d="M5.5 8.2h.01M8.5 8.2h.01M11.5 8.2h.01M14.5 8.2h.01M6.5 11.8h7" />
        </svg>
      );
    case "arrow":
      return (
        <svg {...common} strokeWidth={2.4} className="shrink-0 rtl:-scale-x-100">
          <path d="M3.8 10h11.9M11.3 5.6 15.7 10l-4.4 4.4" />
        </svg>
      );
    case "chevron":
      return (
        <svg {...common} strokeWidth={2.4} className="shrink-0 rtl:-scale-x-100">
          <path d="M7.5 4.2 13.3 10l-5.8 5.8" />
        </svg>
      );
    case "warn":
      return (
        <svg {...common} strokeWidth={1.7}>
          <path d="M10 3 18 16.5H2z" />
          <path d="M10 8.2v3.6" />
          <circle cx="10" cy="14.1" r=".9" fill="currentColor" stroke="none" />
        </svg>
      );
    case "check":
      return (
        <svg {...common} strokeWidth={2.6}>
          <path d="M4.4 10.5 8.2 14.1 15.6 6.1" />
        </svg>
      );
  }
}
