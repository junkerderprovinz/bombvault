// Badge has no hooks, so these tests call it as a plain function and inspect
// the element tree it returns, without a DOM.
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

describe("Badge size stages", () => {
  it("small stage is an 18px-tall, 11px-text, tight-padding chip", () => {
    const el = root(Badge({ children: "x", size: "small" }));
    expect(el.type).toBe("span");
    const cls = el.props!.className as string;
    expect(cls).toContain("h-[18px]");
    expect(cls).toContain("text-caption");
    expect(cls).toContain("px-1.5");
  });

  it("medium stage is a 20px-tall, 12px-text chip", () => {
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

  it("heading stage is a 22px-tall, 12px-text, uppercase and tracked chip", () => {
    const el = root(Badge({ children: "x", size: "heading" }));
    const cls = el.props!.className as string;
    expect(cls).toContain("h-[22px]");
    expect(cls).toContain("text-dense");
    expect(cls).toContain("uppercase");
    expect(cls).toContain("tracking-widest");
    expect(cls).toContain("px-3");
  });

  it("icon stage is 32px", () => {
    const el = root(Badge({ children: "x", size: "icon" }));
    const cls = el.props!.className as string;
    expect(cls).toContain("h-8");
  });

  // Icon badges sit side by side in the same card, so they share one size.
  // A new BadgeSize has to be added to this list as well.
  it("has exactly one size stage for square icon badges", () => {
    const allSizes: BadgeSize[] = ["small", "medium", "large", "heading", "icon"];
    const heights = allSizes.map((size) => {
      const cls = root(Badge({ children: "x", size })).props!.className as string;
      return [size, cls.split(/\s+/).find((c) => /^h-/.test(c))] as const;
    });
    expect(heights.filter(([, h]) => h === "h-8").map(([s]) => s)).toEqual(["icon"]);
  });

  it.each<BadgeShape>(["pill", "rounded", "square", "circle"])(
    "an icon-only badge at shape=%s is 32px square",
    (shape) => {
      const cls = root(Badge({ children: "x", shape, size: "icon", tip: "t" })).props!
        .className as string;
      expect(cls).toContain("h-8");
      expect(cls).toContain("aspect-square");
      expect(cls).toContain("px-0");
    }
  );

  it.each<BadgeTone>(["ok", "fail", "warn", "active", "neutral"])(
    "an icon-only badge at tone=%s is 32px square",
    (tone) => {
      const cls = root(Badge({ children: "x", shape: "square", size: "icon", tone, tip: "t" }))
        .props!.className as string;
      expect(cls).toContain("h-8");
      expect(cls).toContain("aspect-square");
    }
  );

  it("heading stage differs in height from every status-chip stage", () => {
    const heading = root(Badge({ children: "x", size: "heading" })).props!.className as string;
    for (const size of ["small", "medium", "large"] as BadgeSize[]) {
      const statusCls = root(Badge({ children: "x", size })).props!.className as string;
      const statusHeight = statusCls.split(/\s+/).find((c) => /^h-/.test(c));
      expect(statusHeight).toBeTruthy();
      expect(heading.split(/\s+/)).not.toContain(statusHeight);
    }
  });
});

describe("Badge span, button and a variants", () => {
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

  it("the span variant also claims box-sizing and min-height:0", () => {
    const span = root(Badge({ children: "x", as: "span" }));
    const cls = span.props!.className as string;
    expect(cls).toContain("box-border");
    expect(cls).toContain("min-h-0");
  });

  it("the a variant also claims box-sizing and min-height:0, without appearance-none", () => {
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

  it("the a variant passes href, target and rel through", () => {
    const anchor = root(
      Badge({ children: "x", as: "a", href: "https://example.test", target: "_blank", rel: "noopener noreferrer" })
    );
    expect(anchor.props!.href).toBe("https://example.test");
    expect(anchor.props!.target).toBe("_blank");
    expect(anchor.props!.rel).toBe("noopener noreferrer");
  });
});

describe("Badge shape", () => {
  it("pill shape uses the plain rounded-pill utility, not a percentage-capped radius", () => {
    const el = root(Badge({ children: "3", shape: "pill" }));
    const cls = el.props!.className as string;
    expect(cls).toContain("rounded-pill");
    expect(cls).not.toContain("50%");
    expect(cls).not.toContain("min(");
  });

  it("rounded (default) shape uses rounded-pill, so round draws a true pill", () => {
    const el = root(Badge({ children: "x" }));
    const cls = el.props!.className as string;
    expect(cls).toContain("rounded-pill");
    expect(cls).not.toContain("rounded-control");
  });

  it("square shape follows the shape engine's rounded-pill radius, not a hard-coded 0", () => {
    const el = root(Badge({ children: "x", shape: "square" }));
    const cls = el.props!.className as string;
    expect(cls).toContain("rounded-pill");
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

  it("circle at the icon stage is a 32px square footprint with no padding", () => {
    const el = root(Badge({ children: "!", shape: "circle", size: "icon" }));
    const cls = el.props!.className as string;
    expect(cls).toContain("h-8");
    expect(cls).toContain("aspect-square");
    expect(cls).toContain("px-0");
  });
});

describe("Badge tones", () => {
  const cases: Array<[BadgeTone, string, string]> = [
    ["ok", "bg-statusOkBg", "text-statusOk"],
    ["fail", "bg-statusFailBg", "text-statusFail"],
    // The strong warn background is for chips; the plain one is for
    // full-width warning panels.
    ["warn", "bg-statusWarnBgStrong", "text-statusWarn"],
    // The flat accent is too faint on accentSoft in the light theme, so the
    // text uses accentText.
    ["active", "bg-accentSoft", "text-accentText"],
    ["neutral", "bg-carbon-surface2", "text-carbon-textSub"],
    ["heading", "bg-accent", "text-accentContrast"],
  ];

  it.each(cases)("tone=%s renders %s / %s", (tone, bg, text) => {
    const el = root(Badge({ children: "x", tone }));
    const cls = el.props!.className as string;
    expect(cls).toContain(bg);
    expect(cls).toContain(text);
  });

  it("heading tone renders a solid bg-accent fill, not the accent-soft wash", () => {
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

  // Not in the table above: muted has no background class at all, and an
  // empty bg string would make toContain("") pass trivially.
  it("muted renders plain caption text with no background utility at all", () => {
    const el = root(Badge({ children: "x", tone: "muted" }));
    const cls = (el.props!.className as string).split(/\s+/);
    expect(cls).toContain("text-carbon-textMuted");
    expect(cls.some((c) => c.startsWith("bg-"))).toBe(false);
  });
});

describe("Badge heading notch on the card's top edge", () => {
  it("straddles the edge with top-0 and a -50% translate, which stays centred when a wrapped badge grows", () => {
    const el = root(Badge({ children: "x", tone: "heading", size: "heading" }));
    const cls = el.props!.className as string;
    expect(cls).toContain("absolute");
    expect(cls).toContain("top-0");
    expect(cls).toContain("-translate-y-1/2");
    expect(cls).not.toContain("-top-[11px]");
    expect(cls).toContain("z-10");
  });

  it("forces a pill radius instead of rounded-control", () => {
    const el = root(Badge({ children: "x", tone: "heading", size: "heading" }));
    const cls = el.props!.className as string;
    expect(cls).toContain("rounded-pill");
    expect(cls).not.toContain("rounded-control");
  });

  it("forces rounded-pill even when a shape is passed", () => {
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

  it("without insetStart, sets no horizontal offset, so the static position follows the card's padding and RTL", () => {
    const el = root(Badge({ children: "x", tone: "heading", size: "heading" }));
    const cls = el.props!.className as string;
    const tokens = cls.split(/\s+/);
    expect(tokens.some((c) => /^-?(left|right|start|end|inset)-/.test(c))).toBe(false);
  });

  it("a heading-sized badge with another tone is not a notch", () => {
    const el = root(Badge({ children: "x", tone: "neutral", size: "heading" }));
    const cls = el.props!.className as string;
    expect(cls).not.toContain("absolute");
    expect(cls).not.toContain("-translate-y-1/2");
  });

  it("a heading-toned badge at another size is not a notch", () => {
    const el = root(Badge({ children: "x", tone: "heading", size: "medium" }));
    const cls = el.props!.className as string;
    expect(cls).not.toContain("absolute");
    expect(cls).not.toContain("-translate-y-1/2");
  });

  it("other tone and size combinations stay statically positioned", () => {
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

  // An accent-soft wash reads as dimmed at any opacity, so the notch uses the
  // solid fill of the app's other calls to action.
  it("uses a solid bg-accent fill, not an opaque or translucent accent-soft wash", () => {
    const el = root(Badge({ children: "x", tone: "heading", size: "heading" }));
    const cls = el.props!.className as string;
    const tokens = cls.split(/\s+/);
    expect(tokens).toContain("bg-accent");
    expect(tokens).toContain("text-accentContrast");
    expect(tokens).not.toContain("bg-accentSoft");
    expect(tokens).not.toContain("bg-accentSoftSolid");
  });

  it("a heading-toned badge without the notch has the same solid bg-accent fill", () => {
    const el = root(Badge({ children: "x", tone: "heading", size: "medium" }));
    const cls = el.props!.className as string;
    const tokens = cls.split(/\s+/);
    expect(tokens).toContain("bg-accent");
    expect(tokens).not.toContain("bg-accentSoft");
    expect(tokens).not.toContain("bg-accentSoftSolid");
  });
});

// insetStart places the notch on cards split into an outer and an inner box,
// where the static position would miss the content padding.
describe("Badge insetStart", () => {
  it.each<[4 | 5 | 6, string]>([
    [4, "start-4"],
    [5, "start-5"],
    [6, "start-6"],
  ])("insetStart=%s renders the matching %s class on the notch", (insetStart, expected) => {
    const el = root(Badge({ children: "x", tone: "heading", size: "heading", insetStart }));
    const cls = (el.props!.className as string).split(/\s+/);
    expect(cls).toContain(expected);
  });

  it("insetStart is ignored on a badge that is not a notch", () => {
    const el = root(Badge({ children: "x", tone: "heading", size: "medium", insetStart: 5 }));
    const cls = (el.props!.className as string).split(/\s+/);
    expect(cls.some((c) => /^start-[456]$/.test(c))).toBe(false);
  });

  it("insetStart only sets the horizontal position and keeps the other notch classes", () => {
    const el = root(Badge({ children: "x", tone: "heading", size: "heading", insetStart: 5 }));
    const cls = (el.props!.className as string).split(/\s+/);
    expect(cls).toContain("absolute");
    expect(cls).toContain("top-0");
    expect(cls).toContain("-translate-y-1/2");
    expect(cls).toContain("z-10");
    expect(cls).toContain("start-5");
  });
});

// In reactive mode, index.css lights a notch while its whole card is hovered
// (.glim-notch-card:hover .glim-notch-hue). Other hued controls in the card
// carry glim-hue too, so the notch needs a marker of its own.
describe("Badge hueIndex and the notch hover marker", () => {
  it("a heading notch with hueIndex carries glim-hue and glim-notch-hue", () => {
    const el = root(Badge({ children: "x", tone: "heading", size: "heading", hueIndex: 2 }));
    const cls = el.props!.className as string;
    expect(cls).toContain("glim-hue");
    expect(cls).toContain("glim-notch-hue");
  });

  it("a heading notch without hueIndex carries neither hue class", () => {
    const el = root(Badge({ children: "x", tone: "heading", size: "heading" }));
    const cls = el.props!.className as string;
    expect(cls).not.toContain("glim-hue");
    expect(cls).not.toContain("glim-notch-hue");
  });

  it("hueIndex is ignored on the ok, fail, warn and neutral tones", () => {
    for (const tone of ["ok", "fail", "warn", "neutral"] as BadgeTone[]) {
      const cls = root(Badge({ children: "x", tone, size: "medium", hueIndex: 2 })).props!.className as string;
      expect(cls).not.toContain("glim-hue");
      expect(cls).not.toContain("glim-notch-hue");
    }
  });

  it("hueIndex also applies to tone=\"active\", which is accent-derived rather than a state colour", () => {
    const el = root(Badge({ children: "x", tone: "active", size: "medium", hueIndex: 3 }));
    const cls = el.props!.className as string;
    expect(cls).toContain("glim-hue");
  });

  it("a tone=\"active\" badge with hueIndex does not get glim-notch-hue", () => {
    const el = root(Badge({ children: "x", tone: "active", size: "medium", hueIndex: 3 }));
    const cls = el.props!.className as string;
    expect(cls).not.toContain("glim-notch-hue");
  });

  it("a tone=\"active\" badge without hueIndex has no glim-hue", () => {
    const el = root(Badge({ children: "x", tone: "active", size: "medium" }));
    const cls = el.props!.className as string;
    expect(cls).not.toContain("glim-hue");
  });

  it("a heading-toned badge at another size gets glim-hue but not glim-notch-hue", () => {
    const el = root(Badge({ children: "x", tone: "heading", size: "medium", hueIndex: 2 }));
    const cls = el.props!.className as string;
    expect(cls).toContain("glim-hue");
    expect(cls).not.toContain("glim-notch-hue");
  });
});

// An icon on the accent-soft wash reads as dimmed, so icon-only active badges
// get the solid fill the heading notch uses.
describe("Badge icon-only tone=\"active\" (tip set)", () => {
  it("renders bg-accent and text-accentContrast, not the accent-soft wash or neutral ink", () => {
    const el = root(Badge({ children: "!", as: "button", tone: "active", shape: "square", tip: "Test" }));
    const cls = el.props!.className as string;
    const tokens = cls.split(/\s+/);
    expect(tokens).toContain("bg-accent");
    expect(tokens).toContain("text-accentContrast");
    expect(tokens).not.toContain("bg-accentSoft");
    expect(tokens).not.toContain("text-carbon-textSub");
  });

  it("stays a solid fill with hueIndex", () => {
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

  it("a text badge with tone=\"active\" keeps the soft wash and text-accentText", () => {
    const el = root(Badge({ children: "running", tone: "active" }));
    const cls = el.props!.className as string;
    const tokens = cls.split(/\s+/);
    expect(tokens).toContain("bg-accentSoft");
    expect(tokens).toContain("text-accentText");
  });
});

describe("Badge content and pass-through props", () => {
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

  it("passes ariaLabel through as aria-label on span, button and a", () => {
    const span = root(Badge({ children: "!", ariaLabel: "Reset" }));
    const button = root(Badge({ children: "!", as: "button", ariaLabel: "Reset" }));
    const anchor = root(Badge({ children: "!", as: "a", href: "https://example.test", ariaLabel: "Reset" }));
    expect(span.props!["aria-label"]).toBe("Reset");
    expect(button.props!["aria-label"]).toBe("Reset");
    expect(anchor.props!["aria-label"]).toBe("Reset");
  });
});

describe("Badge wrap", () => {
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

  it("wrap swaps leading-none for multi-line spacing and word wrapping", () => {
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
    // "h-6" is a substring of "min-h-6", so compare whole tokens.
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

// inFlow keeps the notch's look but drops its placement, so a caller can place
// several heading badges as one group (StepCard's number and name). If the
// placement came back, the pair would stack on the same spot.
describe("Badge inFlow", () => {
  const notch = { children: "Attach", tone: "heading", size: "heading" } as const;

  it("drops every placement class so the caller's own wrapper positions it", () => {
    const cls = root(Badge({ ...notch, inFlow: true })).props!.className as string;
    const tokens = cls.split(/\s+/);
    expect(tokens).not.toContain("absolute");
    expect(tokens).not.toContain("top-0");
    expect(tokens).not.toContain("-translate-y-1/2");
    expect(tokens).not.toContain("z-10");
  });

  it("keeps the notch's solid fill, pill radius and elevation", () => {
    const cls = root(Badge({ ...notch, inFlow: true })).props!.className as string;
    expect(cls).toContain("bg-accent");
    expect(cls).toContain("text-accentContrast");
    expect(cls).toContain("rounded-pill");
    expect(cls).toContain("shadow-[var(--elevation)]");
  });

  it("keeps the colour engine wired, including the card-wide reactive hover hook", () => {
    const cls = root(Badge({ ...notch, inFlow: true, hueIndex: 2 })).props!.className as string;
    expect(cls).toContain("glim-hue");
    expect(cls).toContain("glim-notch-hue");
  });

  it("ignores insetStart, since the caller sets the position", () => {
    const cls = root(Badge({ ...notch, inFlow: true, insetStart: 5 })).props!.className as string;
    expect(cls.split(/\s+/)).not.toContain("start-5");
  });

  it("leaves a self-positioning notch untouched when the flag is absent", () => {
    const cls = root(Badge({ ...notch, insetStart: 5 })).props!.className as string;
    expect(cls).toContain("absolute");
    expect(cls).toContain("top-0");
    expect(cls).toContain("-translate-y-1/2");
    expect(cls).toContain("z-10");
    expect(cls).toContain("start-5");
    expect(cls).toContain("shadow-[var(--elevation)]");
  });

  it("still wraps and still keeps the stage's own padding, like any heading badge", () => {
    const cls = root(Badge({ ...notch, inFlow: true, wrap: true })).props!.className as string;
    expect(cls).toContain("min-h-[22px]");
    expect(cls).toContain("py-0.5");
    expect(cls).toContain("px-3");
    expect(cls).toContain("items-center");
    expect(cls).toContain("gap-1");
  });

  it("has no effect on a badge that is not a notch", () => {
    for (const tone of ["ok", "fail", "warn", "active", "neutral", "muted"] as BadgeTone[]) {
      const withFlag = root(Badge({ children: "x", tone, inFlow: true })).props!.className as string;
      const without = root(Badge({ children: "x", tone })).props!.className as string;
      expect(withFlag).toBe(without);
    }
    // Nor on a heading-toned badge at another size.
    const small = root(Badge({ children: "x", tone: "heading", size: "small", inFlow: true })).props!.className as string;
    expect(small).toBe(root(Badge({ children: "x", tone: "heading", size: "small" })).props!.className as string);
  });

  it("carries through the button and anchor variants too", () => {
    for (const as of ["button", "a"] as const) {
      const el = root(Badge({ ...notch, inFlow: true, as }));
      expect(visibleText(el)).toBe("Attach");
      expect(el.props!.className as string).not.toContain("absolute");
    }
  });
});
