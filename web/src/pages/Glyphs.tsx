// ---------------------------------------------------------------------------
// The glyph contact sheet ([330]).
//
// Every icon the app owns, at the size it is actually used, with the one number
// that decides whether a set looks like a set: how much of its box each drawing
// fills.
//
// Why this page exists. Five rounds of live review each began the same way —
// "this symbol looks too big" or "too small" — and every one of them needed
// somebody to open a browser, call getBBox on the real markup and compare.
// That is a slow loop for a question a page can answer at a glance, and the
// answer only ever arrived AFTER a bad import had shipped. Here the outlier is
// the row whose fill percentage does not match its neighbours, visible before
// anything reaches a card.
//
// Deliberately not in the sidebar and not translated. This is a workbench, not
// a feature: reachable at /glyphs by someone who knows it is there, costing the
// interface nothing and the locale catalogue nothing. If it ever becomes a
// feature it needs 42 translations and a nav entry, and that is a different
// decision from "give the person maintaining the icons a mirror".
//
// The fill number is MEASURED here, not read from the generator's declared ink.
// That is the entire point: if the two ever disagree, the declared value is
// wrong and this page is the only thing that would say so.
// ---------------------------------------------------------------------------
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
  // Same name in both sets is a real possibility (Sidebar re-exports), so the
  // list is de-duplicated by name rather than showing a glyph twice and
  // inviting a hunt for the difference.
  const seen = new Set<string>();
  return out
    .filter((r) => (seen.has(r.name) ? false : (seen.add(r.name), true)))
    .sort((a, b) => a.name.localeCompare(b.name));
}

function Cell({ row }: { row: Row }) {
  const box = useRef<HTMLSpanElement>(null);
  const [fill, setFill] = useState<number | null>(null);
  const [ratio, setRatio] = useState<number | null>(null);

  // The number is RASTERISED, not read from geometry ([427]).
  //
  // It used to come from getBBox on the glyph's outermost <g>, which is wrong
  // for exactly one shape and wrong badly: getBBox reports a transformed group's
  // PRE-transform extent, so IconClose — a plus turned 45 degrees — measured as
  // the plus it was before the rotation and scored 101% while painting 58% of
  // its box. The two smallest marks in the set read as the two largest, and the
  // "under 90%" flag below, which exists to catch precisely this, pointed the
  // other way.
  //
  // It stayed hidden because every cheap way of asking is the same way of
  // asking: getBoundingClientRect on the group has the identical blind spot, so
  // a second and a third measurement agreed with the first. jdp saw it by eye
  // instead ("die beiden glyphen ... sind unterschiedlich groß").
  //
  // Drawing it and counting the pixels cannot be fooled by a transform, because
  // it measures the result rather than the recipe. It costs one off-screen
  // 128x128 raster per glyph, on a page that exists to be looked at once.
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
    // A data: URI, not a blob:. The app's own CSP allows img-src 'self' data:
    // and nothing else, so a blob: URL loads nowhere and this silently measures
    // nothing — which is how the first attempt at this failed.
    img.src =
      "data:image/svg+xml;base64," +
      btoa(unescape(encodeURIComponent(new XMLSerializer().serializeToString(clone))));

    return () => {
      cancelled = true;
    };
  }, []);

  // Under 90% is the threshold the sizing rules put the crop there to hold. It
  // colours the number rather than hiding the row: a low fill can be correct
  // for a deliberately airy glyph, and this page reports, it does not judge.
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

      {/* A glyph is `currentColor`, so the ground it sits on is half of whether
          it reads. Both are one click apart rather than one theme switch.

          A Toggle, not the raw checkbox this first had: the house rule is a
          switch everywhere, and a workbench page is not an exemption from the
          design language it exists to serve. */}
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
