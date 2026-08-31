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

  useEffect(() => {
    const svg = box.current?.querySelector("svg");
    if (!svg) return;
    // getBBox throws in a detached or display:none subtree; a glyph that cannot
    // be measured shows no number rather than a wrong one.
    try {
      // The drawn extent, in the glyph's OWN units, against its viewBox — the
      // same comparison gen_glyphs.py's crop is built on.
      const target = (svg.querySelector("g") ?? svg) as SVGGraphicsElement;
      const b = target.getBBox();
      const vb = (svg.getAttribute("viewBox") ?? "0 0 1 1").trim().split(/\s+/).map(Number);
      if (!vb[2] || !vb[3] || !b.width || !b.height) return;
      setFill(Math.max(b.width / vb[2], b.height / vb[3]));
      setRatio(b.width / b.height);
    } catch {
      /* unmeasurable; leave the number blank */
    }
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
