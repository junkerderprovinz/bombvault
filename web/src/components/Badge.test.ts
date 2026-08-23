// ---------------------------------------------------------------------------
// Badge — the one shared status chip/pill (GlimStone form-engine Task 5).
//
// Same rationale as Toggle.test.ts: Badge is a pure, hookless function
// component, so it's invoked directly as a plain function and its returned
// element tree walked as plain objects — no jsdom/testing-library needed, on
// the same environment: "node" footing as the rest of this repo's tests.
// ---------------------------------------------------------------------------
import { describe, expect, it } from "vitest";
import { Badge } from "./Badge";
import type { BadgeShape, BadgeSize, BadgeTone } from "./Badge";

interface ElementNode {
  type?: unknown;
  props?: { children?: unknown; [key: string]: unknown };
}

function isElementNode(node: unknown): node is ElementNode {
  return typeof node === "object" && node !== null;
}

function root(node: unknown): ElementNode {
  if (!isElementNode(node)) throw new Error("not an element");
  return node;
}

function visibleText(node: unknown): string {
  if (node == null || typeof node === "boolean") return "";
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(visibleText).join("");
  if (isElementNode(node) && node.props?.children !== undefined) return visibleText(node.props.children);
  return "";
}

describe("Badge — size stages", () => {
  it("small stage is an 18px-tall, 11px-text, tight-padding chip", () => {
    const el = root(Badge({ children: "x", size: "small" }));
    expect(el.type).toBe("span");
    const cls = el.props!.className as string;
    expect(cls).toContain("h-[18px]");
    expect(cls).toContain("text-caption");
    expect(cls).toContain("px-1.5");
  });

  it("medium stage is a 20px-tall, 12px-text chip (the dominant predecessor weight)", () => {
    const el = root(Badge({ children: "x", size: "medium" }));
    const cls = el.props!.className as string;
    expect(cls).toContain("h-5");
    expect(cls).toContain("text-dense");
    expect(cls).toContain("px-2 ");
  });

  it("large stage is a 24px-tall, 12px-text chip with roomier padding", () => {
    const el = root(Badge({ children: "x", size: "large" }));
    const cls = el.props!.className as string;
    expect(cls).toContain("h-6");
    expect(cls).toContain("text-dense");
    expect(cls).toContain("px-2.5");
  });

  it("defaults to the medium stage when size is omitted", () => {
    const el = root(Badge({ children: "x" }));
    const cls = el.props!.className as string;
    expect(cls).toContain("h-5");
  });

  it("heading stage is a 22px-tall, 12px-text, uppercase+tracked chip (rule 11)", () => {
    const el = root(Badge({ children: "x", size: "heading" }));
    const cls = el.props!.className as string;
    expect(cls).toContain("h-[22px]");
    expect(cls).toContain("text-dense");
    expect(cls).toContain("uppercase");
    expect(cls).toContain("tracking-widest");
    expect(cls).toContain("px-3");
  });

  it("icon stage is a 28px-tall chip, matching Settings.tsx's PaletteSwatch (h-7 w-7) for a same-row icon-only badge", () => {
    const el = root(Badge({ children: "x", size: "icon" }));
    const cls = el.props!.className as string;
    expect(cls).toContain("h-7");
  });

  it("heading stage is a distinct height from every status-chip stage, so a heading never has the exact footprint of a real status/activity chip", () => {
    const heading = root(Badge({ children: "x", size: "heading" })).props!.className as string;
    for (const size of ["small", "medium", "large"] as BadgeSize[]) {
      const statusCls = root(Badge({ children: "x", size })).props!.className as string;
      const statusHeight = statusCls.split(/\s+/).find((c) => /^h-/.test(c));
      expect(statusHeight).toBeTruthy();
      expect(heading.split(/\s+/)).not.toContain(statusHeight);
    }
  });
});

describe("Badge — pixel-identical height between <span>, <button> and <a> at the same stage", () => {
  const sizes: BadgeSize[] = ["small", "medium", "large", "heading"];

  it.each(sizes)("the %s stage's height/padding/font classes match across span/button/a", (size) => {
    const span = root(Badge({ children: "x", size, as: "span" }));
    const button = root(Badge({ children: "x", size, as: "button" }));
    const anchor = root(Badge({ children: "x", size, as: "a", href: "https://example.test" }));
    expect(span.type).toBe("span");
    expect(button.type).toBe("button");
    expect(anchor.type).toBe("a");

    const spanCls = span.props!.className as string;
    const buttonCls = button.props!.className as string;
    const anchorCls = anchor.props!.className as string;
    const { height, text, padding } = {
      small: { height: "h-[18px]", text: "text-caption", padding: "px-1.5" },
      medium: { height: "h-5", text: "text-dense", padding: "px-2" },
      large: { height: "h-6", text: "text-dense", padding: "px-2.5" },
      heading: { height: "h-[22px]", text: "text-dense", padding: "px-3" },
    }[size];

    for (const token of [height, text, padding]) {
      expect(spanCls).toContain(token);
      expect(buttonCls).toContain(token);
      expect(anchorCls).toContain(token);
    }
  });

  it("the button variant additionally claims box-sizing, min-height:0 and appearance:none", () => {
    const button = root(Badge({ children: "x", as: "button" }));
    const cls = button.props!.className as string;
    expect(cls).toContain("box-border");
    expect(cls).toContain("min-h-0");
    expect(cls).toContain("appearance-none");
  });

  it("the span variant also claims box-sizing and min-height:0 (same box model contract)", () => {
    const span = root(Badge({ children: "x", as: "span" }));
    const cls = span.props!.className as string;
    expect(cls).toContain("box-border");
    expect(cls).toContain("min-h-0");
  });

  it("the a variant also claims box-sizing and min-height:0 (same box model contract), with no appearance-none (anchors have no native chrome to strip)", () => {
    const anchor = root(Badge({ children: "x", as: "a", href: "https://example.test" }));
    const cls = anchor.props!.className as string;
    expect(cls).toContain("box-border");
    expect(cls).toContain("min-h-0");
    expect(cls).not.toContain("appearance-none");
  });

  it("sets type=button and propagates onClick/disabled only on the button variant", () => {
    let clicked = false;
    const button = root(
      Badge({ children: "x", as: "button", onClick: () => (clicked = true), disabled: true })
    );
    expect(button.props!.type).toBe("button");
    expect(button.props!.disabled).toBe(true);
    (button.props!.onClick as () => void)();
    expect(clicked).toBe(true);
  });

  it("the a variant passes href/target/rel straight through, for real anchor semantics (rule 13)", () => {
    const anchor = root(
      Badge({ children: "x", as: "a", href: "https://example.test", target: "_blank", rel: "noopener noreferrer" })
    );
    expect(anchor.props!.href).toBe("https://example.test");
    expect(anchor.props!.target).toBe("_blank");
    expect(anchor.props!.rel).toBe("noopener noreferrer");
  });
});

describe("Badge — shape", () => {
  it("pill shape uses the plain rounded-pill utility, never a percentage-capped radius", () => {
    const el = root(Badge({ children: "3", shape: "pill" }));
    const cls = el.props!.className as string;
    expect(cls).toContain("rounded-pill");
    expect(cls).not.toContain("50%");
    expect(cls).not.toContain("min(");
  });

  it("rounded (default) shape uses rounded-control", () => {
    const el = root(Badge({ children: "x" }));
    const cls = el.props!.className as string;
    expect(cls).toContain("rounded-control");
  });

  // FIXED (GlimStone follow-up round, jdp's live review of the off-site
  // tab's four square icon badges: "nicht in der Formengine... die sind
  // falsch eingefaerbt"): `square` used to hard-code `rounded-none` (0px,
  // in EVERY shape-engine mode) instead of reading the shape engine's own
  // --radius-control token like every other control does — the same mistake
  // pattern as a Selector or field ignoring the shape engine. Now shares the
  // exact same `rounded-control` class `rounded` uses below.
  it("square shape reads the shape-engine's own rounded-control token, not a hard-coded 0", () => {
    const el = root(Badge({ children: "x", shape: "square" }));
    const cls = el.props!.className as string;
    expect(cls).toContain("rounded-control");
    expect(cls).not.toContain("rounded-none");
  });

  it("circle shape uses rounded-pill, locks aspect-square, and zeroes horizontal padding", () => {
    const el = root(Badge({ children: "!", shape: "circle", size: "medium" }));
    const cls = el.props!.className as string;
    expect(cls).toContain("rounded-pill");
    expect(cls).toContain("aspect-square");
    expect(cls).toContain("px-0");
    expect(cls).not.toContain("px-2 ");
  });

  it.each<BadgeShape>(["pill", "rounded", "square", "circle"])(
    "shape=%s still claims the size stage's own height",
    (shape) => {
      const el = root(Badge({ children: "x", shape, size: "large" }));
      expect(el.props!.className as string).toContain("h-6");
    }
  );

  it("circle + icon stage locks the same 28px footprint as PaletteSwatch, for an icon-only badge sitting beside it", () => {
    const el = root(Badge({ children: "!", shape: "circle", size: "icon" }));
    const cls = el.props!.className as string;
    expect(cls).toContain("h-7");
    expect(cls).toContain("aspect-square");
    expect(cls).toContain("px-0");
  });
});

describe("Badge — tone/status-color mapping", () => {
  const cases: Array<[BadgeTone, string, string]> = [
    ["ok", "bg-statusOkBg", "text-statusOk"],
    ["fail", "bg-statusFailBg", "text-statusFail"],
    // warn uses the STRONG token (-bg-strong), not the plain one: index.css
    // labels -strong "emphasised warn chip" — matching Receiver/Fleet's old
    // local Badge and Files.tsx/Containers.tsx's still-inline warn chips,
    // not the softer tone full-width warning panels use.
    ["warn", "bg-statusWarnBgStrong", "text-statusWarn"],
    // active replaces the old "info" tone (Task 7: resolve the fifth hue) —
    // a soft accent wash, not one of the four real state hues. text-accentText,
    // not the flat text-accent: a spec-compliance review measured the flat
    // accent gold at only 1.50:1 on this exact accent-soft-tinted background
    // in light theme (badly fails the 4.5:1 text minimum, though dark theme
    // was fine) — see index.css's --accent-text comment for the fix and the
    // measured numbers.
    ["active", "bg-accentSoft", "text-accentText"],
    ["neutral", "bg-carbon-surface2", "text-carbon-textSub"],
    // heading (rule 11, REVISED — live-review round: "the notch reads as
    // darkened/dimmed, not the real accent colour"): the full, solid
    // bg-accent fill + text-accentContrast ink, the same pairing this app's
    // other solid CTAs (navActive, "Speichern") already use — not a
    // translucent or opaque-composited wash of it. See the file header's
    // tone="heading" section for the two earlier (now-superseded) rounds.
    ["heading", "bg-accent", "text-accentContrast"],
  ];

  it.each(cases)("tone=%s renders %s / %s", (tone, bg, text) => {
    const el = root(Badge({ children: "x", tone }));
    const cls = el.props!.className as string;
    expect(cls).toContain(bg);
    expect(cls).toContain(text);
  });

  it("heading tone renders the full solid bg-accent fill (REVISED, live-review round: a pale accent-soft wash always reads as darkened, whatever its alpha)", () => {
    const el = root(Badge({ children: "x", tone: "heading" }));
    const cls = el.props!.className as string;
    const tokens = cls.split(/\s+/);
    expect(tokens).toContain("bg-accent");
    expect(tokens).not.toContain("bg-accentSoft");
  });

  it("defaults to the neutral tone when omitted", () => {
    const el = root(Badge({ children: "x" }));
    const cls = el.props!.className as string;
    expect(cls).toContain("bg-carbon-surface2");
    expect(cls).toContain("text-carbon-textSub");
  });
});

describe("Badge — heading notch (tone=heading + size=heading straddles the card's top edge)", () => {
  it("positions absolutely, straddling the edge via top-0 + a self-relative -50% translate (not a fixed pixel offset — see Badge.tsx's own REGRESSION comment: a fixed px value is only ever right for ONE assumed height, silently wrong the moment a wrapped badge renders taller)", () => {
    const el = root(Badge({ children: "x", tone: "heading", size: "heading" }));
    const cls = el.props!.className as string;
    expect(cls).toContain("absolute");
    expect(cls).toContain("top-0");
    expect(cls).toContain("-translate-y-1/2");
    expect(cls).not.toContain("-top-[11px]");
    expect(cls).toContain("z-10");
  });

  it("forces a pill radius, overriding the default shape-engine-driven rounded-control", () => {
    const el = root(Badge({ children: "x", tone: "heading", size: "heading" }));
    const cls = el.props!.className as string;
    expect(cls).toContain("rounded-pill");
    expect(cls).not.toContain("rounded-control");
  });

  it("forces rounded-pill even if a shape prop is explicitly passed — the notch is fixed chrome, not shape-engine-governed", () => {
    const el = root(Badge({ children: "x", tone: "heading", size: "heading", shape: "square" }));
    const cls = el.props!.className as string;
    expect(cls).toContain("rounded-pill");
    expect(cls).not.toContain("rounded-none");
  });

  it("carries the elevation shadow token, and only elevation (no --hairline)", () => {
    const el = root(Badge({ children: "x", tone: "heading", size: "heading" }));
    const cls = el.props!.className as string;
    expect(cls).toContain("shadow-[var(--elevation)]");
    expect(cls).not.toContain("hairline");
  });

  it("sets no explicit left/right/start/end offset — relies on the CSS static-position fallback so the notch inherits each call site's own padding and flips correctly under RTL for free", () => {
    const el = root(Badge({ children: "x", tone: "heading", size: "heading" }));
    const cls = el.props!.className as string;
    const tokens = cls.split(/\s+/);
    expect(tokens.some((c) => /^-?(left|right|start|end|inset)-/.test(c))).toBe(false);
  });

  it("a plain heading-SIZED badge without tone=heading does NOT get the notch treatment (both props must match)", () => {
    const el = root(Badge({ children: "x", tone: "neutral", size: "heading" }));
    const cls = el.props!.className as string;
    expect(cls).not.toContain("absolute");
    expect(cls).not.toContain("-translate-y-1/2");
  });

  it("a heading-TONED badge at a non-heading size does NOT get the notch treatment (both props must match)", () => {
    const el = root(Badge({ children: "x", tone: "heading", size: "medium" }));
    const cls = el.props!.className as string;
    expect(cls).not.toContain("absolute");
    expect(cls).not.toContain("-translate-y-1/2");
  });

  it("every other tone/size combination stays static-positioned (no regression to non-heading badges)", () => {
    const cases: Array<[BadgeTone, BadgeSize]> = [
      ["ok", "small"],
      ["fail", "medium"],
      ["warn", "large"],
      ["active", "medium"],
      ["neutral", "icon"],
    ];
    for (const [tone, size] of cases) {
      const cls = root(Badge({ children: "x", tone, size })).props!.className as string;
      expect(cls).not.toContain("absolute");
    }
  });

  // Live-review round history: an earlier fix here swapped the notch's fill
  // to an OPAQUE color-mix() of --accent-soft (to fix a semi-transparent
  // two-tone seam at the card edge), but jdp reviewed that live and said it
  // still read as "abgedunkelt" (darkened/dimmed) — a 14%-accent-into-
  // surface wash is inherently pale no matter its alpha. Fix: drop the wash
  // entirely and use the same full, solid bg-accent/text-accentContrast
  // pairing this app's other solid CTAs already use — see Badge.tsx's file
  // header for the full history and index.css's (now-removed)
  // --accent-soft-solid comment for the superseded intermediate fix.
  it("uses the full solid bg-accent fill, not any accent-soft wash (opaque or translucent)", () => {
    const el = root(Badge({ children: "x", tone: "heading", size: "heading" }));
    const cls = el.props!.className as string;
    const tokens = cls.split(/\s+/);
    expect(tokens).toContain("bg-accent");
    expect(tokens).toContain("text-accentContrast");
    expect(tokens).not.toContain("bg-accentSoft");
    expect(tokens).not.toContain("bg-accentSoftSolid");
  });

  it("a heading-toned badge WITHOUT the notch (non-heading size) renders the identical solid bg-accent fill — no separate 'notch-only' colour anymore", () => {
    const el = root(Badge({ children: "x", tone: "heading", size: "medium" }));
    const cls = el.props!.className as string;
    const tokens = cls.split(/\s+/);
    expect(tokens).toContain("bg-accent");
    expect(tokens).not.toContain("bg-accentSoft");
    expect(tokens).not.toContain("bg-accentSoftSolid");
  });
});

describe("Badge — hueIndex (rainbow position) and the card-wide reactive-hover marker", () => {
  // jdp, live-review: a reactive-mode section-title badge only lit up on
  // hover of the ~22px badge glyph itself — an impractically small target.
  // The fix lives mostly in index.css (a `.glim-notch-card:hover
  // .glim-notch-hue` rule scoped to each notch's own enclosing card), but
  // that rule needs a marker class narrower than the general `.glim-hue`
  // every rainbow-hued element carries (Selector segments, ContainerRow/
  // VMRow/FileSetRow) — otherwise hovering a card would also light up some
  // unrelated hued control sitting in the same card body. `.glim-notch-hue`
  // is that marker, applied only when hueIndex is actually driving a real
  // heading NOTCH (tone AND size both "heading" — badgeClassName's own
  // isHeadingNotch gates the identical pair for the positioning treatment).
  it("a hueIndex'd heading notch carries both glim-hue and the notch-specific glim-notch-hue marker", () => {
    const el = root(Badge({ children: "x", tone: "heading", size: "heading", hueIndex: 2 }));
    const cls = el.props!.className as string;
    expect(cls).toContain("glim-hue");
    expect(cls).toContain("glim-notch-hue");
  });

  it("omitting hueIndex on a heading notch renders neither hue class (the pre-existing flat-accent singleton case)", () => {
    const el = root(Badge({ children: "x", tone: "heading", size: "heading" }));
    const cls = el.props!.className as string;
    expect(cls).not.toContain("glim-hue");
    expect(cls).not.toContain("glim-notch-hue");
  });

  it("hueIndex is silently ignored for any real state tone (ok/fail/warn/neutral) — no glim-hue, no glim-notch-hue", () => {
    for (const tone of ["ok", "fail", "warn", "neutral"] as BadgeTone[]) {
      const cls = root(Badge({ children: "x", tone, size: "medium", hueIndex: 2 })).props!.className as string;
      expect(cls).not.toContain("glim-hue");
      expect(cls).not.toContain("glim-notch-hue");
    }
  });

  // offsite-tab card-split follow-up (jdp: "Die Buttons Verbindung testen,
  // Jetzt replizieren, Einrichten, Ziel hinzufügen in die Farbengine
  // aufnehmen"): tone="active" is accent-derived (bg-accentSoft/
  // text-accentText), never one of rule 4's four state hues, so it is the
  // one other tone `hueIndex` is allowed to drive — see Badge()'s own
  // `hueOn` comment for the full reasoning.
  it("hueIndex DOES drive tone=\"active\" — the one other accent-derived (non-state) tone", () => {
    const el = root(Badge({ children: "x", tone: "active", size: "medium", hueIndex: 3 }));
    const cls = el.props!.className as string;
    expect(cls).toContain("glim-hue");
  });

  it("a hueIndex'd tone=\"active\" badge never picks up the card-notch-only glim-notch-hue marker (only a real heading NOTCH gets that)", () => {
    const el = root(Badge({ children: "x", tone: "active", size: "medium", hueIndex: 3 }));
    const cls = el.props!.className as string;
    expect(cls).not.toContain("glim-notch-hue");
  });

  it("omitting hueIndex on tone=\"active\" renders the flat, un-rainbowed accent-soft wash (no glim-hue)", () => {
    const el = root(Badge({ children: "x", tone: "active", size: "medium" }));
    const cls = el.props!.className as string;
    expect(cls).not.toContain("glim-hue");
  });

  it("a hueIndex'd heading badge at a non-heading size gets the general glim-hue but NOT the card-wide glim-notch-hue marker (it isn't the notch treatment, so it must not opt into the card-wide reveal)", () => {
    const el = root(Badge({ children: "x", tone: "heading", size: "medium", hueIndex: 2 }));
    const cls = el.props!.className as string;
    expect(cls).toContain("glim-hue");
    expect(cls).not.toContain("glim-notch-hue");
  });
});

// GlimStone follow-up round (jdp's live review of the off-site tab's four
// icon-only "active" badges, on top of the earlier "farbige Schrift" fix
// that made this branch neutral-ink-on-a-wash in the first place): "die sind
// falsch eingefaerbt, so halb abgedunkelt" — bg-accentSoft IS a 14%-alpha
// wash (the exact same "half-darkened" failure mode tone="heading" above
// already fixed once), so an icon-only tone="active" badge gets the
// identical treatment: full solid bg-accent, computed-contrast ink.
describe("Badge — icon-only tone=\"active\" (tip set) uses a full solid fill, not the accent-soft wash", () => {
  it("renders bg-accent + text-accentContrast, never the accent-soft wash or the old flat neutral ink", () => {
    const el = root(Badge({ children: "!", as: "button", tone: "active", shape: "square", tip: "Test" }));
    const cls = el.props!.className as string;
    const tokens = cls.split(/\s+/);
    expect(tokens).toContain("bg-accent");
    expect(tokens).toContain("text-accentContrast");
    expect(tokens).not.toContain("bg-accentSoft");
    expect(tokens).not.toContain("text-carbon-textSub");
  });

  it("still holds with a hueIndex — a rainbow-positioned icon badge is a solid hue fill, not a hued wash", () => {
    const el = root(
      Badge({ children: "!", as: "button", tone: "active", shape: "square", tip: "Test", hueIndex: 1 })
    );
    const cls = el.props!.className as string;
    const tokens = cls.split(/\s+/);
    expect(tokens).toContain("bg-accent");
    expect(tokens).toContain("text-accentContrast");
    expect(tokens).toContain("glim-hue");
    expect(tokens).not.toContain("bg-accentSoft");
  });

  it("a non-icon-only tone=\"active\" text badge is unaffected — still the soft wash + text-accentText pairing", () => {
    const el = root(Badge({ children: "running", tone: "active" }));
    const cls = el.props!.className as string;
    const tokens = cls.split(/\s+/);
    expect(tokens).toContain("bg-accentSoft");
    expect(tokens).toContain("text-accentText");
  });
});

describe("Badge — content and extension", () => {
  it("renders children as visible text", () => {
    const el = Badge({ children: "3 failed" });
    expect(visibleText(el)).toBe("3 failed");
  });

  it("appends an extra className without dropping the stage's own classes", () => {
    const el = root(Badge({ children: "3", className: "tabular-nums" }));
    const cls = el.props!.className as string;
    expect(cls).toContain("tabular-nums");
    expect(cls).toContain("h-5");
  });

  it("passes title through on both span and button variants", () => {
    const span = root(Badge({ children: "x", title: "hint" }));
    const button = root(Badge({ children: "x", as: "button", title: "hint" }));
    expect(span.props!.title).toBe("hint");
    expect(button.props!.title).toBe("hint");
  });

  it("passes ariaLabel through as aria-label on span, button and a — the accessible name for an icon-only badge whose content is a decorative glyph", () => {
    const span = root(Badge({ children: "!", ariaLabel: "Reset" }));
    const button = root(Badge({ children: "!", as: "button", ariaLabel: "Reset" }));
    const anchor = root(Badge({ children: "!", as: "a", href: "https://example.test", ariaLabel: "Reset" }));
    expect(span.props!["aria-label"]).toBe("Reset");
    expect(button.props!["aria-label"]).toBe("Reset");
    expect(anchor.props!["aria-label"]).toBe("Reset");
  });
});

describe("Badge — wrap (grow-to-fit instead of clipping)", () => {
  it("without wrap, a stage renders a fixed h-* height with leading-none and no min-h floor", () => {
    const el = root(Badge({ children: "x", size: "medium" }));
    const cls = el.props!.className as string;
    expect(cls).toContain("h-5");
    expect(cls).toContain("leading-none");
    expect(cls).not.toContain("min-h-5");
  });

  it.each<[BadgeSize, string]>([
    ["small", "min-h-[18px]"],
    ["medium", "min-h-5"],
    ["large", "min-h-6"],
  ])("wrap swaps the %s stage's fixed height for a %s floor", (size, minHeight) => {
    const el = root(Badge({ children: "x", size, wrap: true }));
    const cls = el.props!.className as string;
    expect(cls).toContain(minHeight);
  });

  it("wrap drops leading-none and gains readable multi-line spacing + word wrapping", () => {
    const el = root(Badge({ children: "x", wrap: true }));
    const cls = el.props!.className as string;
    expect(cls).not.toContain("leading-none");
    expect(cls).toContain("leading-tight");
    expect(cls).toContain("wrap-break-word");
  });

  it("wrap never emits the stage's fixed h-* class alongside its min-h-* floor", () => {
    const el = root(Badge({ children: "x", size: "large", wrap: true }));
    const cls = el.props!.className as string;
    expect(cls).toContain("min-h-6");
    // "h-6" would also match as a substring of "min-h-6", so check for the
    // exact standalone class token instead of a naive .toContain("h-6").
    const tokens = cls.split(/\s+/);
    expect(tokens).not.toContain("h-6");
  });

  it("wrap works on the button variant too, at the same stage's min-h-* floor", () => {
    const button = root(Badge({ children: "x", as: "button", size: "large", wrap: true }));
    const cls = button.props!.className as string;
    expect(cls).toContain("min-h-6");
  });

  it("wrap still keeps the stage's own horizontal padding and text size", () => {
    const el = root(Badge({ children: "x", size: "small", wrap: true }));
    const cls = el.props!.className as string;
    expect(cls).toContain("px-1.5");
    expect(cls).toContain("text-caption");
  });
});
