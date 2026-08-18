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
});

describe("Badge — pixel-identical height between <span> and <button> at the same stage", () => {
  const sizes: BadgeSize[] = ["small", "medium", "large"];

  it.each(sizes)("the %s stage's height/padding/font classes match for as=span and as=button", (size) => {
    const span = root(Badge({ children: "x", size, as: "span" }));
    const button = root(Badge({ children: "x", size, as: "button" }));
    expect(span.type).toBe("span");
    expect(button.type).toBe("button");

    const spanCls = span.props!.className as string;
    const buttonCls = button.props!.className as string;
    const { height, text, padding } = {
      small: { height: "h-[18px]", text: "text-caption", padding: "px-1.5" },
      medium: { height: "h-5", text: "text-dense", padding: "px-2" },
      large: { height: "h-6", text: "text-dense", padding: "px-2.5" },
    }[size];

    for (const token of [height, text, padding]) {
      expect(spanCls).toContain(token);
      expect(buttonCls).toContain(token);
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

  it("square shape uses rounded-none", () => {
    const el = root(Badge({ children: "x", shape: "square" }));
    const cls = el.props!.className as string;
    expect(cls).toContain("rounded-none");
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
    ["info", "bg-statusInfoBg", "text-statusInfo"],
    ["neutral", "bg-carbon-surface2", "text-carbon-textSub"],
  ];

  it.each(cases)("tone=%s renders %s / %s", (tone, bg, text) => {
    const el = root(Badge({ children: "x", tone }));
    const cls = el.props!.className as string;
    expect(cls).toContain(bg);
    expect(cls).toContain(text);
  });

  it("defaults to the neutral tone when omitted", () => {
    const el = root(Badge({ children: "x" }));
    const cls = el.props!.className as string;
    expect(cls).toContain("bg-carbon-surface2");
    expect(cls).toContain("text-carbon-textSub");
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
