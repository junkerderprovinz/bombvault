// GlyphSheet shows every icon the app owns at the size it is used, with how
// much of its box each drawing fills. A glyph whose fill differs from its
// neighbours will look the wrong size beside them. The fill is measured here
// rather than taken from the generator's declared ink, so a wrong declaration
// shows up. It is a workbench at /glyphs: not in the navigation, not translated.
import { useEffect, useRef, useState } from "react";
import { PAGE_SHELL } from "../lib/pageShell";
import { Toggle } from "../components/Toggle";
import * as nav from "../components/navGlyphs";
import * as action from "../components/glyphs";

type Row = { name: string; node: React.ReactNode; set: string };

/** Every exported Icon* from both generated modules, in one list. */
function collect(): Row[] {
  const out: Row[] = [];
  for (const [set, mod] of [
    ["action", action as Record<string, unknown>],
    ["nav", nav as Record<string, unknown>],
  ] as const) {
    for (const [name, value] of Object.entries(mod)) {
      if (!name.startsWith("Icon") || typeof value !== "function") continue;
      const Comp = value as () => React.ReactNode;
      out.push({ name, node: <Comp />, set });
    }
  }
  // A name can appear in both sets through Sidebar's re-exports.
  const seen = new Set<string>();
  return out
    .filter((r) => (seen.has(r.name) ? false : (seen.add(r.name), true)))
    .sort((a, b) => a.name.localeCompare(b.name));
}

function Cell({ row }: { row: Row }) {
  const box = useRef<HTMLSpanElement>(null);
  const [fill, setFill] = useState<number | null>(null);
  const [ratio, setRatio] = useState<number | null>(null);

  // The glyph is rasterised and its pixels counted. getBBox and
  // getBoundingClientRect report a transformed group's extent before the
  // transform, so IconClose, a plus turned 45 degrees, would measure as the
  // unrotated plus.
  useEffect(() => {
    const svg = box.current?.querySelector("svg");
    if (!svg) return;
    let cancelled = false;

    const clone = svg.cloneNode(true) as SVGSVGElement;
    clone.setAttribute("width", "128");
    clone.setAttribute("height", "128");
    // currentColor has no meaning inside an <img>; pin it so the ink is opaque.
    clone.querySelectorAll("*").forEach((e) => {
      if (e.getAttribute("fill") === "currentColor") e.setAttribute("fill", "#000");
      if (e.getAttribute("stroke") === "currentColor") e.setAttribute("stroke", "#000");
    });
    clone.setAttribute("fill", "#000");

    const img = new Image();
    img.onload = () => {
      if (cancelled) return;
      const c = document.createElement("canvas");
      c.width = c.height = 128;
      const ctx = c.getContext("2d");
      if (!ctx) return;
      ctx.drawImage(img, 0, 0, 128, 128);
      let minX = 128, minY = 128, maxX = -1, maxY = -1;
      const d = ctx.getImageData(0, 0, 128, 128).data;
      for (let y = 0; y < 128; y++) {
        for (let x = 0; x < 128; x++) {
          if (d[(y * 128 + x) * 4 + 3] > 24) {
            if (x < minX) minX = x;
            if (x > maxX) maxX = x;
            if (y < minY) minY = y;
            if (y > maxY) maxY = y;
          }
        }
      }
      if (maxX < 0) return; // nothing drawn
      const w = maxX - minX + 1;
      const h = maxY - minY + 1;
      setFill(Math.max(w, h) / 128);
      setRatio(w / h);
    };
    // The CSP allows img-src 'self' data: only, so a blob: URL would not load.
    img.src =
      "data:image/svg+xml;base64," +
      btoa(unescape(encodeURIComponent(new XMLSerializer().serializeToString(clone))));

    return () => {
      cancelled = true;
    };
  }, []);

  // The sizing rules crop glyphs to fill at least 90%. A lower fill is only
  // coloured, since it can be right for an airy glyph.
  const low = fill !== null && fill < 0.9;

  return (
    <div className="glim-card flex flex-col items-center gap-2 p-3">
      <span ref={box} className="flex h-10 w-10 items-center justify-center [&>svg]:h-5 [&>svg]:w-5">
        {row.node}
      </span>
      <span className="text-xs text-carbon-text break-all text-center">{row.name}</span>
      <span className={`text-xs tabular-nums ${low ? "text-statusWarn" : "text-carbon-textMuted"}`}>
        {fill === null ? "—" : `${Math.round(fill * 100)}%`}
        {ratio !== null && <span className="text-carbon-textMuted"> · {ratio.toFixed(2)}</span>}
      </span>
    </div>
  );
}

export function GlyphSheet() {
  const rows = collect();
  const [dark, setDark] = useState(true);

  return (
    <div className={PAGE_SHELL}>
      <div>
        <h1 className="text-lg font-semibold text-carbon-text">Glyph sheet</h1>
        <p className="mt-1 max-w-3xl text-sm text-carbon-textMuted">
          {rows.length} glyphs at their real 20px size. The percentage is how much of its own
          viewBox each drawing fills, measured here rather than taken from the generator. An
          outlier is a glyph that will look the wrong size beside its neighbours, and the
          second number is its aspect ratio. Not translated and not in the navigation: this
          is a workbench, not a feature.
        </p>
      </div>

      {/* A glyph is `currentColor`, so the ground it sits on is half of
          whether it reads. */}
      <Toggle checked={dark} onChange={setDark} label="Dark ground" />

      <div
        className={`grid gap-3 rounded-card p-3 ${
          dark ? "bg-carbon-background text-carbon-text" : "bg-white text-black"
        }`}
        style={{ gridTemplateColumns: "repeat(auto-fill, minmax(7rem, 1fr))" }}
      >
        {rows.map((r) => (
          <Cell key={r.name} row={r} />
        ))}
      </div>
    </div>
  );
}

export default GlyphSheet;
