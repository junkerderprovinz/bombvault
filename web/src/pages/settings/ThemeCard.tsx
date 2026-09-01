// ThemeCard, lifted out of Settings.tsx ([337]).
//
// A MOVE, not a rewrite: the component is byte-identical to what stood
// in Settings.tsx, and it was already module-level and prop-driven, so
// nothing crosses a new seam. See that file's own note for why the cut
// stops here rather than continuing into SettingsPage itself.
import type { ResolvedTheme } from "../../lib/theme";
import { Card } from "../settings/shared";
import { Selector } from "../../components/Selector";
import { getResolvedTheme, getTheme, onSystemThemeChange, setTheme } from "../../lib/theme";
import { useEffect, useState } from "react";
import { useT } from "../../lib/i18n";

// ---------------------------------------------------------------------------
// Theme Card (GlimStone follow-up pass, live-review round — jdp asked for the
// dark/light toggle to move out of Sidebar.tsx's footer into its own Card in
// Settings' General tab, the exact same "move, don't duplicate" request
// already applied to the language picker above — see LanguageCard's own
// header comment for that precedent, and Sidebar.tsx's SidebarControls for
// where this toggle used to render). Reuses lib/theme.ts's own
// getResolvedTheme()/getTheme()/setTheme()/onSystemThemeChange() — the exact
// same state machine the sidebar button used, only the surrounding chrome
// changed.
//
// A horizontal Selector, not the old single toggle button (jdp, live-review:
// "das design dunkel/hell bitte ein horizontaler selektor machen") — this IS
// "one of a small, mutually exclusive set of options," design-language.md's
// own "The one horizontal selector" case, the exact shape the Shape Card's
// own round/soft/square picker right below it already uses. Matched to that
// picker's OWN exact established treatment (jdp approved that look live for
// the same kind of control): `size="lg"` (the page's own full Settings-
// decision register, not a tight toolbar chip) and `variant="well"
// equalWidth` (TrickWork's shared padded groove, flush crossfade-only
// segments, at the big pinned scale — see Selector.tsx's file header items 5
// and 6). No `hue={false}` — see that Card's own
// comment for why an opt-out here would be exactly the self-authored
// aesthetic exception jdp has ruled out; this Selector uses the plain `true`
// default like every other hue-enabled one in the app, so its active segment
// reads its own rainbow position in Rainbow Mode instead of one flat colour
// (positions 0/1 — RAINBOW[0]/[1] — light/dark in that order below).
//
// Two segments only (light/dark), not three (adding "system"): lib/theme.ts's
// own toggleTheme() header already flags "no UI path back to system" as a
// KNOWN GAP, explicitly deferred as its own separate piece of UI design work,
// not a mechanical follow-on — jdp's own ask here named "dunkel/hell"
// specifically, not a third state, so this stays scoped to what was asked
// rather than silently expanding it. onChange sets the theme DIRECTLY to the
// clicked segment's id (never toggleTheme()'s old flip-the-current-value
// logic, which only made sense for a single two-state button) — clicking the
// segment that's already active is a harmless no-op, same as clicking the
// already-active Shape segment.
export function ThemeCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  // getResolvedTheme(), not the raw stored preference: the default is
  // "system" (GlimStone form-engine #1), and this picker only ever shows/
  // sets an explicit dark or light state, so it must reflect what's actually
  // painted rather than the unresolved "system" value.
  const [theme, setThemeState] = useState(getResolvedTheme);

  function selectTheme(next: ResolvedTheme) {
    setTheme(next);
    setThemeState(next);
  }

  // Keep the displayed selection live-accurate on "system": lib/theme.ts's own
  // matchMedia listener repaints data-theme (and every colour token)
  // immediately on an OS-level flip, but this component's `theme` state was
  // only ever set at mount and on an explicit pick — without this, the
  // Card's active segment goes stale the moment the OS changes out from
  // under a "system" user, even though the rest of the UI has already
  // repainted correctly.
  useEffect(() => {
    return onSystemThemeChange(() => {
      if (getTheme() === "system") setThemeState(getResolvedTheme());
    });
  }, []);

  // Filled + coloured glyphs (live-review point 4: "the sun/moon icon is a
  // thin outline, jdp wants it filled and preferably coloured"). Fixed hex
  // fills rather than `currentColor`/an accent token on purpose — a sun and
  // a moon each already carry their OWN conventional colour (warm gold, cool
  // indigo) independent of whatever accent hue or rainbow position the rest
  // of the app is using; tying either to --accent would make the sun render
  // blue if the user's accent happened to be blue, which reads wrong
  // regardless of theme. Both values are solid, mode-independent literals —
  // no [data-theme] variance needed, they read fine on a "well" segment's
  // idle/hover/active fills in both palettes.
  // FILLED rays (design-language.md "Icon glyphs", rule 220 — "a structural
  // detail that has to stay a thin line... renders as a thin filled shape,
  // never stroke"): the disc itself was already filled (see the fixed-hex
  // colour note above, unrelated to this fix); only the 8 sunburst rays were
  // still stroked lines. Each ray is now a thin filled rounded rect — the
  // four cardinal ones axis-aligned, the four diagonal ones a `<rect>`
  // rotated to its own 45°, same start/end points as the old stroke
  // segments — same `#FACC15` fixed fill as the disc (still deliberately
  // not `currentColor`/the accent, per this Card's own header comment).
  // `currentColor`, not a fixed yellow and indigo (jdp: "Die glyphen von
  // hell/dunkel modus sollen schwarz/weiß sein wie alle anderen"). These two
  // were the only glyphs in the app painting their own colour, which also put
  // them outside the colour engine: on a selected segment every other glyph
  // flips to the contrast ink and these two stayed yellow and blue on the fill.
  const sunIcon = (
    <svg width="20" height="20" viewBox="0 0 20 20" fill="currentColor" className="shrink-0" aria-hidden="true">
      <circle cx="10" cy="10" r="4.25" />
      <rect x="9.15" y="2" width="1.7" height="2" rx="0.85" />
      <rect x="9.15" y="16" width="1.7" height="2" rx="0.85" />
      <rect x="2" y="9.15" width="2" height="1.7" rx="0.85" />
      <rect x="16" y="9.15" width="2" height="1.7" rx="0.85" />
      <rect x="4.435" y="4.785" width="2.4" height="1.7" rx="0.85" transform="rotate(45 5.635 5.635)" />
      <rect x="13.165" y="13.515" width="2.4" height="1.7" rx="0.85" transform="rotate(45 14.365 14.365)" />
      <rect x="4.435" y="13.515" width="2.4" height="1.7" rx="0.85" transform="rotate(-45 5.635 14.365)" />
      <rect x="13.165" y="4.785" width="2.4" height="1.7" rx="0.85" transform="rotate(-45 14.365 5.635)" />
    </svg>
  );
  const moonIcon = (
    <svg width="20" height="20" viewBox="0 0 20 20" fill="none" className="shrink-0" aria-hidden="true">
      {/* Solid fill, no stroke — the crescent path already closes with "z",
          so a plain fill renders the correct silhouette. */}
      <path
        d="M17.5 12.5A7.5 7.5 0 017.5 2.5a7.5 7.5 0 100 15 7.5 7.5 0 0010-5z"
        fill="currentColor"
      />
    </svg>
  );

  return (
    <Card title={t("settings.theme")} hueIndex={hueIndex}>
      {/* NO `inline-flex self-start max-w-full` wrapper any more (jdp,
          live-review: "Die horizontalen Selektoren sollen nicht auf die ganze
          Card-Breite gestreckt werden, sondern eine standardisierte Breite
          bekommen"). That ask is still honoured — this Card's own root is
          `flex flex-col` (Card's own comment above), whose default
          `align-items: stretch` would otherwise blockify this row to the
          Card's full content width — but round 8 moved the mechanism INTO
          the variant: `variant="well"` now carries `w-fit max-w-full` itself,
          and `width: fit-content` is not `auto`, so a stretch alignment no
          longer applies to it. Three call sites hand-rolling the identical
          wrapper div was the same "one control, two mechanisms" drift that
          round's whole change is about; see Selector.tsx's file header item
          6. (The Settings tab strip further down this file KEEPS its own
          `tabStripEl` wrapper — it is `variant="chip"`, not a grooved well,
          so nothing in the variant hugs for it.) */}
      <Selector
        items={[
          { id: "light", label: t("theme.light"), icon: sunIcon },
          { id: "dark", label: t("theme.dark"), icon: moonIcon },
        ]}
        label={t("settings.theme")}
        select="one"
        active={theme}
        onChange={(id) => selectTheme(id as ResolvedTheme)}
        size="lg"
        variant="well"
        // The BIG scale (Selector.tsx file header item 6): every segment
        // pinned to the widest one's own measured width, floored at
        // MIN_PINNED_WIDTH, at the fixed --badge-md height. Round 8 made
        // this an explicit opt-in — "well" alone is now the small,
        // content-hugging scale the CadenceBuilder mode pickers use.
        equalWidth
      />
    </Card>
  );
}
