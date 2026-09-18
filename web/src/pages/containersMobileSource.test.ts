// ---------------------------------------------------------------------------
// Containers page source contract — the single-layout guard suite.
//
// The Containers page's responsive contract is partly DECLARATIVE — one
// layout tree, a page-shell constant, a primitive boundary, a prop wiring —
// and a declaration with no rule is invisible to TypeScript and to every
// assertion that only checks behavior: nothing fails when someone re-splits
// the phone and desktop faces into CSS-hidden twin blocks, hand-rolls a
// second sticky search, or derives the folder tree's interaction mode from
// the width breakpoint. So this suite asserts the SOURCE TEXT itself, the
// same way the mobile shell's source contract (src/app/mobileShellSource
// .test.ts) pins the viewport contract.
//
// The history behind the strictest guard: the page originally carried BOTH
// faces in one file as sibling blocks, the desktop one silenced below the
// breakpoint with display-utility classes. Every "why is this twice?"
// question, every drift between the twins, and every confused bug report
// came from that shape — the page now renders exactly ONE face for the
// viewport, and the banned literals below appear in THIS FILE on purpose,
// as negative-assertion needles (the house pattern from the shell suite);
// within Containers.tsx they must never survive.
//
// Node environment, no DOM: this reads source text, it does not render.
// ---------------------------------------------------------------------------
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

const HERE = dirname(fileURLToPath(import.meta.url));
const containers = readFileSync(join(HERE, "Containers.tsx"), "utf8");

describe("the page renders ONE layout, never CSS-hidden twins", () => {
  it("is reading the real Containers page (self-guard)", () => {
    expect(
      containers,
      "Containers.tsx no longer exports Containers — every assert in this suite " +
        "would be running against a renamed or replaced file."
    ).toContain("export function Containers");
  });

  it("keeps both display-hiding utilities out of the source", () => {
    for (const needle of ["max-md:hidden", "md:hidden"]) {
      expect(
        containers.includes(needle),
        `Containers.tsx contains "${needle}". The page must render ONE layout ` +
          "for the viewport: the phone face and the desktop face are ternary- " +
          "gated in JSX, so exactly one of them exists in the DOM at a time. A " +
          "CSS-hidden twin block reintroduces the double-DOM shape this page " +
          "was rewritten to kill — two copies of every card, two copies of " +
          "every search box, locator ambiguity in tests, and state that can " +
          "silently diverge between the visible and the hidden face. Gate by " +
          "render, never by stylesheet."
      ).toBe(false);
    }
  });
});

describe("the page sits on the responsive page shell", () => {
  it("imports the responsive shell constant from the page-shell module", () => {
    expect(
      containers.includes('import { PAGE_SHELL_RESPONSIVE } from "../lib/pageShell";'),
      "Containers.tsx no longer imports PAGE_SHELL_RESPONSIVE from lib/pageShell. " +
        "The responsive shell is the second rung of the page-shell ladder (same " +
        "1152px cap, a 24px rhythm below 48rem and the settled 40px above it) — " +
        "a page that reaches for raw padding classes instead drifts out of the " +
        "rhythm every other page shares. Its contract lives in lib/pageShell.ts."
    ).toBe(true);
  });

  it("applies the responsive shell as the page root's className", () => {
    expect(
      containers.includes("<div className={PAGE_SHELL_RESPONSIVE}>"),
      "Containers.tsx's page root no longer uses PAGE_SHELL_RESPONSIVE as its " +
        "className. The import is only half the contract: the constant must be " +
        "the root's class for the rhythm switch to wrap the whole page. If the " +
        "root moved into a shared component, update this guard deliberately — " +
        "do not delete it."
    ).toBe(true);
  });
});

describe("the mobile toolbar is the shared ListToolbar", () => {
  it("is reading the real Containers page (self-guard)", () => {
    expect(
      containers,
      "Containers.tsx no longer exports Containers — the toolbar asserts below " +
        "are running against a renamed or replaced file."
    ).toContain("export function Containers");
  });

  it("binds the toolbar through the shared ListToolbar component and the shared search key", () => {
    expect(
      /import \{ ListToolbar \} from "\.\.\/components\/mobile\/ListToolbar";/.test(containers),
      "Containers.tsx no longer imports ListToolbar. The mobile toolbar is the " +
        "ONE shared sticky-toolbar primitive (sticky-in-flow chrome, 44px chip " +
        "bleed, the keyboard discipline) — hand-rolling a second toolbar " +
        "reintroduces exactly the divergence the primitive exists to prevent."
    ).toBe(true);
    expect(
      containers.includes('placeholder="containers.searchPlaceholder"'),
      "Containers.tsx's ListToolbar no longer binds the shared search key as " +
        "its placeholder. The placeholder key is what ties the mobile input and " +
        "the desktop search to the SAME filter state — a page-local string " +
        "breaks that tie invisibly."
    ).toBe(true);
  });

  it("keeps every hand-rolled toolbar signature out of the page", () => {
    expect(
      containers.includes("sticky top-0"),
      "Containers.tsx contains a hand-rolled sticky chrome signature (the " +
        "`sticky top-0` classes). Sticky toolbar chrome belongs to ListToolbar " +
        "alone; a page-level copy would double the sticky bars on the phone " +
        "and drift out of the primitive's keyboard/scroll discipline. Compose " +
        "the shared toolbar instead."
    ).toBe(false);
    expect(
      containers.includes("Search containers"),
      "Containers.tsx hard-codes the search placeholder string instead of the " +
        "placeholder KEY. User-visible text goes through i18n (lint-enforced) " +
        "and the literal copy would bypass the shared-key tie to the desktop " +
        "search. Pass the key, as ListToolbar's placeholder prop expects."
    ).toBe(false);
  });
});

describe("the card list paginates through the shared load-more primitive", () => {
  it("windows the card list with useLoadMore", () => {
    expect(
      /const \{ visible: visibleCards, showMore, hasMore \} = useLoadMore\(mobileCards\);/.test(containers),
      "Containers.tsx no longer windows its mobile card list through useLoadMore. " +
        "The primitive owns the constant window, the Load-more affordance, the " +
        "honest hasMore and the reset-on-identity contract — a page that " +
        "re-derives them re-derives the bugs the primitive fixed."
    ).toBe(true);
  });

  it("keeps every auto-load or hand-windowed mechanism out of the page", () => {
    expect(
      containers.includes("IntersectionObserver"),
      "Containers.tsx references IntersectionObserver. The card list NEVER " +
        "auto-loads: scrolling to the bottom must never append rows — the " +
        "Load-more button is the only way the window grows (proven from the " +
        "outside by the ergonomics e2e). An observer is the exact mechanism " +
        "that contract bans."
    ).toBe(false);
    expect(
      containers.includes('addEventListener("scroll"'),
      "Containers.tsx attaches a scroll listener. Same ban as the observer " +
        "above: the list never auto-loads on scroll, and a page-level scroll " +
        "listener would also fight the shell's own scroll restoration (the " +
        "saved list scroll position that Back restores)."
    ).toBe(false);
    expect(
      containers.includes("slice(0,"),
      "Containers.tsx hand-windows the list with slice(0, n). Windowing is " +
        "useLoadMore's job — its showMore/hasMore pair is what keeps the Load " +
        "more button honest. A parallel slice drifts from the button's state " +
        "the first time someone changes one and not the other."
    ).toBe(false);
  });
});

describe("the folder tree's interaction mode follows the pointer axis", () => {
  it("derives the tree's interaction mode from the coarse-pointer query", () => {
    expect(
      containers.includes("const coarsePointer = useIsCoarsePointer();"),
      "Containers.tsx no longer reads the pointer axis through " +
        "useIsCoarsePointer. The folder tree's touch mode must follow the " +
        "POINTER (a coarse pointer gets the 44px touch tree, a fine pointer " +
        "the desktop tree), never the width — a folded-window desktop user " +
        "has a mouse, a large-tablet user has a finger."
    ).toBe(true);
    expect(
      containers.includes('interactionMode={coarsePointer ? "touch" : "pointer"}'),
      "Containers.tsx no longer wires the selection tree's interactionMode " +
        "from the coarse-pointer query. This wiring is the touch-mode " +
        "contract in one line; the geometry e2e (touch-tree.spec.ts) only " +
        "passes because the mobile projects' hasTouch makes this query true."
    ).toBe(true);
  });

  it("never derives the interaction mode from the width breakpoint", () => {
    expect(
      /interactionMode=\{[^}]*isDesktop/.test(containers),
      "Containers.tsx derives the folder tree's interactionMode from " +
        "useIsDesktop. The mode must follow the pointer axis, never the " +
        "width: at a width on the wrong side of the breakpoint the two " +
        "disagree (narrow desktop window, wide touch tablet), and the width " +
        "answer is the wrong one in both directions."
    ).toBe(false);
  });
});
