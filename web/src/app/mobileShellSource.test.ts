// ---------------------------------------------------------------------------
// Mobile shell source contract; the viewport correctness guard suite.
//
// The viewport contract is mostly declarative; custom properties, a meta
// tag, a class name; and a declaration with no rule is invisible to
// TypeScript and to every assertion that only checks behavior: nothing fails
// when `viewport-fit=cover` is dropped from the meta, when a safe-area
// property loses its env() pairing, or when someone "simplifies" the FOUC
// script. So this suite asserts the source text itself, the same way
// routedPages.test.ts pins the page-shell rule's scope.
//
// Three of these guards are load-bearing beyond style:
//   - The FOUC byte-identity guard: the inline head script is first-paint
//     theme logic whose bytes are security-adjacent. The script's full text
//     is copied below as EXPECTED_FOUC_SCRIPT and compared verbatim; any
//     edit to those bytes fails this suite, so a change there has to be a
//     act that updates this constant.
//   - The banned-literal negatives: 100vh and min-h-screen (and h-screen)
//     are the viewport-height trap (they keep the pre-keyboard height under
//     the iOS keyboard and strand content off-screen). The literals appear
//     in this file on purpose, as negative-assertion needles; that is their
//     only sanctioned appearance; in index.css and Layout.tsx they must
//     never survive.
//   - The login-before-shell source order: the login page's chrome-free
//     guarantee is structural; Layout's blocked branch returns LoginPage
//     before the shell root div is ever reached; so the guard asserts
//     source order, not rendered DOM.
//
// Node environment, no DOM: this reads source text, it does not render.
// ---------------------------------------------------------------------------
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

const HERE = dirname(fileURLToPath(import.meta.url));
const SRC = join(HERE, "..");
const WEB = join(SRC, "..");

const indexHtml = readFileSync(join(WEB, "index.html"), "utf8");
const indexCss = readFileSync(join(SRC, "index.css"), "utf8");
const layout = readFileSync(join(HERE, "Layout.tsx"), "utf8");
const sidebar = readFileSync(join(SRC, "components", "Sidebar.tsx"), "utf8");
const bottomNav = readFileSync(join(SRC, "components", "mobile", "BottomNav.tsx"), "utf8");
const moreSheet = readFileSync(join(SRC, "components", "mobile", "MoreSheet.tsx"), "utf8");

// The shell root div, anchored on the ref plus the template-literal className
// rather than any includes() for a unit name: Layout's comments discuss
// `h-dvh` (the viewport-discipline guard below), and an includes() positive
// would pass forever on that dead text even after the root itself changed.
// The root also carries the shell ref (the keyboard mechanism's focus
// listeners attach to it), which is what makes the ref anchor stable. The
// `\s+` joints tolerate the root's attributes wrapping across lines; the
// className stays a template literal (the desktop/mobile ternary).
const SHELL_ROOT = /<div\s+ref=\{shellRef\}\s+className=\{`flex h-dvh/;

// The banned viewport-height literals. h-screen joins 100vh and min-h-screen:
// it is the same static-largest-viewport trap in Tailwind's vocabulary, and a
// ban list that omits the common spelling is a ban with a door in it.
const BANNED_VIEWPORT_HEIGHTS = ["100vh", "min-h-screen", "h-screen"];

// The FOUC script copied verbatim from index.html; indentation included.
// This is a byte constant: it must not be reformatted, requoted, or "tidied",
// because its entire job is to differ from index.html the moment index.html's
// copy changes. (It necessarily contains no backticks and no ${ sequences,
// which is what makes a plain template literal safe here.)
const EXPECTED_FOUC_SCRIPT = `    <script>
      (function () {
        var THEME_COLOR = { dark: "#161616", light: "#f4f4f4" };
        try {
          var stored = localStorage.getItem("bv-theme");
          var resolved =
            stored === "dark" || stored === "light"
              ? stored
              : window.matchMedia("(prefers-color-scheme: dark)").matches
                ? "dark"
                : "light";
          document.documentElement.setAttribute("data-theme", resolved);
          document
            .querySelector('meta[name="theme-color"]')
            ?.setAttribute("content", THEME_COLOR[resolved]);
        } catch (e) {
          document.documentElement.setAttribute("data-theme", "dark");
        }
      })();
    </script>`;

// The first bare <script> in the head; the literal `<script>` (no
// attributes) matches only the FOUC script; the module script at the end of
// <body> carries type/src, so it can never shadow this match.
const FOUC_REGION = /<script>\n([\s\S]*?)<\/script>/;

describe("safe-area custom properties exist and are env-paired", () => {
  // Three sides, not four: --safe-area-top was removed
  // because nothing ever consumed it: the phone scroller
  // is a flat p-4 and the fullHeight sheet's header starts at y=0. A dead
  // token reads as a live contract; a future top consumer reintroduces the
  // declaration beside its own padding, and this list, in the same change.
  const DEFINITIONS = [
    ...indexCss.matchAll(/--safe-area-[a-z]+:\s*max\(env\(safe-area-inset-/g),
  ];

  it("finds the safe-area block at all (guards against this suite silently matching nothing)", () => {
    expect(
      DEFINITIONS.length,
      "no `--safe-area-*: max(env(...))` definitions were found in index.css. " +
        "The block was removed, renamed, or rewritten in a form this guard no longer " +
        "recognizes; safe areas are half of the viewport contract, so restore it or " +
        "update this guard with it."
    ).toBeGreaterThanOrEqual(3);
  });

  it.each(["right", "bottom", "left"])(
    "defines --safe-area-%s as max(env(safe-area-inset-%s), 0px)",
    (side) => {
      // The 0px clamp is not decoration: env() can behave as invalid/unset in
      // a var() chain on browsers that parse the syntax but report nothing,
      // and every consumer's padding/calc must stay a valid length there.
      expect(
        new RegExp(
          `--safe-area-${side}:\\s*max\\(env\\(safe-area-inset-${side}\\),\\s*0px\\)`
        ).test(indexCss),
        `--safe-area-${side} is missing or no longer clamped as ` +
          `max(env(safe-area-inset-${side}), 0px). The clamp keeps every consumer's ` +
          `padding a valid length where env() resolves to nothing, and the env() ` +
          `pairing is what makes the inset real on notched devices.`
      ).toBe(true);
    }
  );
});

// Side insets reach every surface that touches the screen's left/right edge.
// The bottom bar carried them from the start, but the phone main and the
// desktop frame (the chrome a landscape phone renders once the width
// breakpoint matches) did not: with viewport-fit=cover the layout viewport
// extends under the side notch, so page content and the rail slide beneath
// it. Each consumer floors the inset at the surface's own gutter (1rem, the
// page gutter), which keeps the rendering byte-identical wherever env()
// reads 0px; max() with a 0px floor is the token's own clamp, the pairing
// the guards above pin. Declarative classes carry the contract, so it is
// pinned at source level like every guard in this file.
describe("side safe-area insets reach the phone main and the desktop frame", () => {
  it("is reading the real Layout component (self-guard)", () => {
    expect(
      layout,
      "Layout.tsx no longer exports Layout; the side-inset asserts below are " +
        "running against a file that no longer contains the component, so " +
        "they would pass against dead text."
    ).toContain("export function Layout");
  });

  it.each(["left", "right"])(
    "pads both edge-touching surfaces by max(1rem, var(--safe-area-%s))",
    (side) => {
      const needle = `[max(1rem,var(--safe-area-${side}))]`;
      const uses = layout.split(needle).length - 1;
      expect(
        uses,
        `Layout.tsx applies --safe-area-${side} ${uses} time(s). Two surfaces ` +
          "touch the screen's side edges: the phone main and the desktop " +
          "frame a landscape phone renders; both need the inset or " +
          "viewport-fit=cover puts their content under the side notch. The " +
          "1rem floor is the page gutter, so devices without an inset " +
          "render exactly as before. Restore the missing consumer."
      ).toBeGreaterThanOrEqual(2);
    }
  );
});

describe("the viewport meta carries the mobile directives", () => {
  const viewport = /<meta\s+name="viewport"\s+content="([^"]*)"\s*\/>/.exec(indexHtml);

  it("finds the viewport meta at all (guards against this suite silently matching nothing)", () => {
    expect(
      viewport,
      'no <meta name="viewport" content="..."> found in index.html. The meta was ' +
        "removed or reformatted beyond this guard's shape; restore it or update the " +
        "regex with it."
    ).not.toBeNull();
  });

  it("keeps the original width/device-width + initial-scale directives", () => {
    expect(
      viewport?.[1],
      "the viewport meta lost `width=device-width, initial-scale=1.0`; every mobile " +
        "surface depends on it; the mobile extension is additive, never a replacement."
    ).toContain("width=device-width, initial-scale=1.0");
  });

  it("declares viewport-fit=cover; the other half of the safe-area contract", () => {
    expect(
      viewport?.[1],
      'viewport-fit=cover is missing from the viewport meta. Without it the layout ' +
        "viewport is inset by the system chrome and every env(safe-area-inset-*); " +
        "and therefore every --safe-area-* custom property in index.css; silently " +
        "reads 0px. The meta line and the CSS block are one contract."
    ).toContain("viewport-fit=cover");
  });

  it("declares interactive-widget=resizes-content for the Android keyboard", () => {
    expect(
      viewport?.[1],
      "interactive-widget=resizes-content is missing from the viewport meta. The " +
        "Android keyboard then overlays instead of resizing the layout viewport, and " +
        "the Layout-level keyboard mechanism loses the resize signal it builds on."
    ).toContain("interactive-widget=resizes-content");
  });
});

describe("the FOUC script's bytes are guarded", () => {
  const region = FOUC_REGION.exec(indexHtml);

  it("finds a bare <script> region that is the FOUC script (self-guard)", () => {
    expect(
      region?.[1],
      'no bare `<script>` block found in index.html, or it is not the FOUC script. ' +
        "The FOUC killer is the first bare script in <head>; if it moved or grew " +
        "attributes, update FOUC_REGION with them."
    ).toContain("bv-theme");
  });

  it("matches index.html byte for byte; any edit to those bytes fails here", () => {
    expect(
      indexHtml,
      "index.html's FOUC script no longer matches the guarded byte constant. Those " +
        "bytes are first-paint theme logic: they stamp data-theme synchronously " +
        "before CSS paints, and a mutation is tampering with a byte-critical region: " +
        "duplicates of theme.ts's resolution logic that must be kept in sync by " +
        "hand. If the change is wanted, update EXPECTED_FOUC_SCRIPT in this test " +
        "in the same commit and say why."
    ).toContain(EXPECTED_FOUC_SCRIPT);
  });
});

describe("the theme-color meta sits above the FOUC script", () => {
  // The meta comes before the script, because the script writes it. A tag
  // below the script is not there yet when the synchronous theme-color write
  // runs, so every first paint keeps the static dark value even in light
  // mode. Above the script the tag exists when the script runs, so the
  // resolved value lands before first
  // paint; paint() in lib/theme.ts takes over on every theme application
  // afterwards.
  const foucOpen = indexHtml.indexOf("<script>");
  const themeColor = indexHtml.indexOf('<meta name="theme-color"');

  it("finds both markers at all (guards against this suite silently matching nothing)", () => {
    expect(foucOpen, "no bare <script> open tag found; see the byte-identity guard above").toBeGreaterThan(-1);
    expect(
      themeColor,
      'no <meta name="theme-color"> found in index.html. The static tag is the ' +
        "no-JS browser-chrome color; the FOUC script mirrors the resolved theme " +
        "into it before first paint, and paint() in lib/theme.ts mirrors the " +
        "live theme into it afterwards."
    ).toBeGreaterThan(-1);
  });

  it("orders the tag before the script, so the script's write can land", () => {
    expect(
      themeColor,
      "the theme-color meta sits below the FOUC script's open tag. It must stay " +
        "above: the script resolves the theme and mirrors it into this tag " +
        "synchronously before first paint, and a tag below the script does not " +
        "exist yet when the script runs (bug B8: light-mode boots painted the " +
        "static dark value in the browser chrome). The script region itself is " +
        "byte-guarded; this guard pins only the order."
    ).toBeLessThan(foucOpen);
  });

  it("is addressed by the FOUC script's theme-color mirror", () => {
    expect(
      indexHtml,
      "the FOUC script no longer writes the theme-color meta. The static value " +
        "would then be the only value a no-JS-less browser ever sees: dark chrome " +
        "on a light boot. Keep the script's mirror (and its THEME_COLOR map in " +
        "sync with theme.ts's)."
    ).toContain('meta[name="theme-color"]');
  });
});

describe("login renders before any shell, structurally", () => {
  // Scope note: this asserts order only. The guarantee: on a password-
  // protected instance the login screen can never render with the app chrome
  // around it, because the blocked branch returns before any shell renders.
  const blocked = /if \(authGate === "blocked"\) \{\s*\n\s*return <LoginPage/.exec(layout);
  // The shell root's height is h-dvh (the dynamic-viewport discipline below)
  // and its className is a template literal, because the one chrome switch
  // appends `flex-col` on the mobile branch; so the root div no longer
  // matches a plain `className="flex` string.
  const shell = SHELL_ROOT.exec(layout);

  it("finds both the blocked branch and the shell root at all (self-guard)", () => {
    expect(
      blocked,
      'Layout.tsx no longer has a `return <LoginPage` in the authGate === "blocked" ' +
        "branch; the login screen's chrome-free guarantee is this early return, so " +
        "its removal is a regression unless the shell was restructured."
    ).not.toBeNull();
    expect(
      shell,
      "Layout.tsx no longer renders a `<div ref={shellRef} className={`flex h-dvh ...`}>` " +
        "shell root; the shell root moved or was renamed; update this guard with it."
    ).not.toBeNull();
  });

  it("returns LoginPage before the shell root appears in source order", () => {
    expect(
      blocked!.index,
      "Layout.tsx renders the shell before (or instead of) the LoginPage blocked " +
        "return. The no-chrome-on-login guarantee is structural: the bottom bar can " +
        "never appear on login because the blocked branch returns before any shell " +
        "renders. Preserve that order."
    ).toBeLessThan(shell!.index);
  });
});

describe("the shell root's viewport discipline", () => {
  // Vacuity guard: every assert below reads dead text if the component moved.
  it("is reading the real Layout component (self-guard)", () => {
    expect(
      layout,
      "Layout.tsx no longer exports Layout; the shell asserts below are running " +
        "against a file that no longer contains the component."
    ).toContain("export function Layout");
  });

  it("sizes the shell root with the dynamic viewport unit", () => {
    // Anchored on the SHELL_ROOT element pattern, not a bare includes():
    // comments in Layout.tsx legitimately discuss `h-dvh`, and a substring
    // scan would keep passing on that prose after the root itself dropped
    // the unit.
    expect(
      SHELL_ROOT.test(layout),
      "Layout.tsx's shell root no longer uses h-dvh. The dynamic unit is what keeps " +
        "the shell filling the visual viewport as mobile browser chrome collapses " +
        "and expands; a static unit keeps the pre-chrome height and strands " +
        "bottom-docked content under expanded chrome."
    ).toBe(true);
  });

  it("contains no banned viewport-height literal", () => {
    const found = BANNED_VIEWPORT_HEIGHTS.filter((needle) => layout.includes(needle));
    expect(
      found,
      "Layout.tsx contains a banned viewport-height literal. The mobile shell uses " +
        "dvh/svh exclusively: the static forms (100vh, min-h-screen, h-screen) keep " +
        "the largest viewport height, which is exactly the trap that strands content " +
        "under the iOS keyboard or expanded browser chrome. Use the dynamic unit."
    ).toEqual([]);
  });

  it("keeps the stylesheet free of the banned viewport-height literals too", () => {
    const found = BANNED_VIEWPORT_HEIGHTS.filter((needle) => indexCss.includes(needle));
    expect(
      found,
      "index.css contains a banned viewport-height literal. Same trap as the Layout " +
        "ban: static viewport heights (100vh, min-h-screen, h-screen) strand " +
        "bottom-docked content under the iOS keyboard or expanded browser chrome; " +
        "use the dvh/svh forms."
    ).toEqual([]);
  });

  it("keeps the bv-main id on the scroller, the chrome's stable scroll target", () => {
    // Anchored on the id attribute inside its real <main> tag context (id
    // followed by the className attribute, `\s+` spanning the wrapping), not
    // a bare includes(): Layout's own comments discuss `<main id="bv-main">`
    // in prose (the tap-on-active block), and the bare scan kept passing on
    // that dead text even after the tags themselves were renamed (maintainer
    // Proven by breaking both tags: the
    // assert went red, and restoring them. Both chrome branches carry the
    // id on their own scroller; the count of two is the contract.
    const tagged = layout.match(/<main\s+id="bv-main"\s+className=/g)?.length ?? 0;
    expect(
      tagged,
      `Layout.tsx carries id="bv-main" on ${tagged} real <main> tag(s); both ` +
        "chrome surfaces address the scroller through that id (tap-on-active, " +
        "the keyboard mechanism) and never query for it any other way, so " +
        "both branch scrollers must carry it. Without it the tap-on-active " +
        "scroll silently no-ops. Restore the attribute on the <main> tag(s)."
    ).toBe(2);
  });
});

describe("the one keyboard mechanism lives at Layout level", () => {
  it("guards the mechanism on visualViewport presence, so jsdom and old browsers no-op", () => {
    expect(
      /typeof window\.visualViewport\s*===\s*"undefined"/.test(layout),
      "Layout.tsx's keyboard mechanism lost the visualViewport presence guard. The " +
        "guard is the contract (the jsdom discipline, lib/testSetup/matchMedia.ts): " +
        "environments without the API must no-op the whole mechanism, not throw; " +
        "without it every Layout-rendering dom test crashes and old browsers break."
    ).toBe(true);
  });

  it("wires the focusin listener exactly once; one mechanism, not one per component", () => {
    const count = layout.match(/addEventListener\("focusin"/g)?.length ?? 0;
    expect(
      count,
      "Layout.tsx wires addEventListener(\"focusin\") more than once (or not at " +
        "all). Exactly one listener set at Layout level is the rule: a " +
        "per-component listener would multiply with every new input-bearing page " +
        "and drift exactly the way duplicated logic does."
    ).toBe(1);
  });

  it("tears down every listener it adds (cleanup in the effect's return)", () => {
    for (const event of ["focusin", "focusout", "resize"]) {
      expect(
        layout.includes(`removeEventListener("${event}"`),
        `Layout.tsx's keyboard mechanism no longer removes its "${event}" listener ` +
          "on cleanup. The mechanism attaches only while the mobile branch renders; " +
          "without the teardown a branch switch to desktop (or an unmount) leaks " +
          "listeners that keep scrolling a shell that no longer exists."
      ).toBe(true);
    }
  });

  it("attaches the mechanism only while the mobile branch renders", () => {
    expect(
      /if \(isDesktop \|\| authGate !== "pass"\) return;/.test(layout),
      "Layout.tsx's keyboard mechanism lost its isDesktop/authGate bail-out. The " +
        "mechanism must attach only on the mobile branch of a rendered shell: " +
        "desktop never pays the cost, and the blocked/loading states render no " +
        "shell root to attach to."
    ).toBe(true);
  });

  it("is the only visualViewport consumer in the chrome (zero listeners outside Layout)", () => {
    for (const [name, source] of [
      ["Sidebar.tsx", sidebar],
      ["BottomNav.tsx", bottomNav],
      ["MoreSheet.tsx", moreSheet],
    ] as const) {
      expect(
        source.includes("visualViewport"),
        `${name} touches visualViewport. The keyboard mechanism is one Layout-level ` +
          "listener set; a second viewport-resize consumer in a chrome component " +
          "is exactly the drift the single-listener rule forbids."
      ).toBe(false);
    }
  });
});

describe("the chrome testids live in the mobile component sources", () => {
  // Vacuity guards: the testid asserts below pass vacuously against renamed
  // or replaced components.
  it("is reading the real mobile chrome components (self-guards)", () => {
    expect(
      bottomNav,
      "BottomNav.tsx no longer exports BottomNav; the testid asserts below are " +
        "running against a file that no longer contains the component."
    ).toContain("export function BottomNav");
    expect(
      moreSheet,
      "MoreSheet.tsx no longer exports MoreSheet; the testid asserts below are " +
        "running against a file that no longer contains the component."
    ).toContain("export function MoreSheet");
  });

  it("carries data-testid=bottom-nav on the bar and data-testid=more-sheet in the sheet", () => {
    expect(
      bottomNav.includes('data-testid="bottom-nav"'),
      "BottomNav.tsx lost data-testid=\"bottom-nav\". The e2e suite locates the bar " +
        "through that testid on both mobile projects; without it the chrome-switch " +
        "smoke and every sheet assertion fail."
    ).toBe(true);
    expect(
      moreSheet.includes('data-testid="more-sheet"'),
      "MoreSheet.tsx lost data-testid=\"more-sheet\". The e2e suite locates the " +
        "sheet content through that testid; without it the sheet assertions " +
        "cannot run."
    ).toBe(true);
  });

  it("names the nav landmark for assistive technology", () => {
    expect(
      bottomNav.includes('aria-label={t("nav.mobileNavigation")}'),
      "BottomNav.tsx's <nav> lost its accessible name. Exactly one of the two nav " +
        "landmarks mounts at a time, but a landmark still needs a name for screen " +
        "readers; one attribute carries the name; keep it."
    ).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// Filled-tab contract; the bottom bar's active tab is
// filled, not accent-coloured text.
//
// The reason is the design language's own selection statement: the active
// tab in the bottom bar is filled, not coloured text; coloured text is how
// a link looks, filled is how this language says "this one is selected". The language's own canonical
// statement of the filled state already exists in index.css:
// .glim-coin-tile.glim-active ("the tile is filled with the accent, so every
// mark drops its brand colour and takes the ink that fill was paired with")
// and its .glim-hue-icon twin (the contrast ink "comes for free from
// text-accentContrast ... via ordinary currentColor inheritance"). The bar's
// active treatment is declarative; a class string inside a ternary; so no
// behavioral assert can catch a slide back into the link idiom: the contract
// is pinned at source level, like every guard in this file.
//
// The two banned literals below appear in this file on purpose, as
// negative-assertion needles; their only sanctioned appearance, the same
// house pattern as the 100vh needles above. The negatives scan the raw
// BottomNav text, comments included rather than stripped,
// which is exactly why the component's own comments paraphrase the retired
// treatments instead of citing the literals.
// ---------------------------------------------------------------------------
describe("Filled tab; the active bottom-bar slot is a filled accent surface, not coloured text", () => {
  it("is reading the real BottomNav source (self-guard)", () => {
    expect(
      bottomNav,
      "BottomNav.tsx no longer exports BottomNav; the filled-tab asserts below " +
        "are running against a file that no longer contains the component, so " +
        "the negatives would pass against dead text."
    ).toContain("export function BottomNav");
  });

  it("fills the whole active slot with the accent and pairs every mark on it with the contrast ink", () => {
    expect(
      bottomNav.includes('isActive ? "bg-accent text-accentContrast glim-active" : "text-carbon-textMuted"'),
      "BottomNav.tsx's active branch no longer fills the slot. The active tab " +
        "must read as filled: bg-accent over the entire NavLink; glyph and " +
        "caption sit on the fill together; with the ink paired to that fill " +
        "(text-accentContrast; glyphs inherit it because every navGlyph draws " +
        'fill="currentColor"). Accent-coloured text on the bar ground is the ' +
        "link idiom, not the selected idiom."
    ).toBe(true);
    // The More trigger fills by the same idiom while a More-side route is
    // current; its own active branch must not drift back to coloured text.
    expect(
      bottomNav.includes('moreActive ? "bg-accent text-accentContrast glim-active" : "text-carbon-textMuted"'),
      "BottomNav.tsx's More trigger no longer fills while its side of the " +
        "registry is current. The same filled contract applies: a filled " +
        "trigger is how this language says \"you are on one of the More " +
        "sheet's routes\", coloured text would read as a link."
    ).toBe(true);
  });

  it.each([
    "text-accentText",
    "bg-accentSoft",
  ] as const)("keeps the retired link-idiom literal out of the bar: %s", (needle) => {
    expect(
      bottomNav.includes(needle),
      `${needle} reappeared in BottomNav.tsx. The active tab is a filled accent ` +
        "slot: accent-coloured text on the bar " +
        "ground reads as a link, and the soft glyph-box backdrop wash is gone " +
        "with it; both belong to the retired treatment this guard bans. The " +
        "scan runs on raw text (comments included), so paraphrase in prose; " +
        "never cite the literal."
    ).toBe(false);
  });
});

// reactive-at-rest; reactive labels reveal on coarse pointers without a
// hover. A tap fires before any reveal state can exist, so under
// `(pointer: coarse)` reactive mode degenerates into permanent icon-only
// unless the reveal happens at rest; the recovery wizard's step-1 CTA was
// an unlabeled check glyph on a touch screen otherwise.
// index.css reveals at rest on coarse pointers, and because every reveal
// state must also reset the lively motion levels' resting
// `translateX`/`scaleX` (a revealed label left displaced and squashed
// without it), it carries a companion coarse reset after the reduced-motion
// gate. These asserts read the source so the reset cannot be removed
// independently.
describe("Reactive at rest; coarse pointers reveal reactive labels at rest, with the motion companion", () => {
  it("is reading the real reactive-label source (self-guard)", () => {
    // Anchored on the declaration INSIDE the .glim-label-reactive rule (same
    // `[^}]*` rule-scope discipline as the coarse-pointer guard below), not a
    // bare includes("max-width: 0"): a prose comment elsewhere in index.css
    // discusses `max-width: 0` in backticks, and the bare scan kept passing
    // on that dead text after the real declaration changed. Proven by
    // breaking the declaration: the
    // assert went red, and restoring it.
    expect(
      /\.glim-label-reactive\s*\{[^}]*max-width:\s*0\s*;/.test(indexCss),
      "index.css no longer contains the resting collapse (`max-width: 0;` in " +
        "the .glim-label-reactive rule). The positives below prove nothing " +
        "without it: they assert the coarse reveal exists, which is only " +
        "meaningful against the collapse it must override. The resting rule " +
        "was renamed or removed; restore it or rewrite this guard's needle " +
        "to the new resting declaration."
    ).toBe(true);
  });

  it("reveals the reactive label at rest under (pointer: coarse), at the chars-sized ceiling", () => {
    // Anchored on the @media rule, not on the first prose mention of the
    // query: comments in index.css discuss `(pointer: coarse)` several
    // blocks above the real at-rest rule, and a scan that starts at any
    // `(pointer: coarse)` occurrence skids across the reveal/hover block to
    // a matching declaration; passing even with the coarse block deleted.
    // `[^}]*` holds the scan inside the rule's own declaration list.
    expect(
      /@media \(pointer: coarse\)\s*\{\s*\.glim-label-reactive\s*\{[^}]*max-width: calc\(var\(--reactive-chars/
        .test(indexCss),
      "index.css no longer reveals .glim-label-reactive at rest on coarse " +
        "pointers. Without the (pointer: coarse) at-rest block, touch users " +
        "in reactive mode see only the glyph of every isolated primary button: " +
        "a tap never produces the hover the reveal needs, so the at-rest block " +
        "is what restores the words. The block must size its ceiling from " +
        "`--reactive-chars` (the reveal state's own contract), not a fixed " +
        "width."
    ).toBe(true);
  });

  it("carries the companion transform reset for the coarse at-rest reveal in the lively motion levels", () => {
    expect(
      /\(prefers-reduced-motion: no-preference\) and \(pointer: coarse\)[\s\S]*?translateX\(0\) scaleX\(1\)/
        .test(indexCss),
      "index.css no longer resets the resting transform for the coarse " +
        "at-rest reveal. In the lively motion levels the revealed label would " +
        "sit displaced and squashed, the revealed-but-still-displaced shape " +
        "the `.glim-active` repair already fixed on the hover path. A reveal " +
        "state added to the (pointer: coarse) block must have its reset in a block combining " +
        "prefers-reduced-motion: no-preference with (pointer: coarse); the " +
        "two lists move together or not at all."
    ).toBe(true);
  });
});
