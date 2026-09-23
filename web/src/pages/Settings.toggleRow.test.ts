// ToggleRow is a pure, hookless component, so these tests call it as a
// function and inspect the element tree. shakeNonce and pulseNonce together
// form the inner Toggle's key: every new failure or success remounts the
// button and replays its .glim-shake or .glim-pulse animation, even when the
// same outcome repeats.
import { describe, expect, it } from "vitest";
import { ToggleRow } from "./settings/shared";
import { Toggle } from "../components/Toggle";

interface ElementNode {
  type?: unknown;
  key?: unknown;
  props?: { children?: unknown; [key: string]: unknown };
}

function isElementNode(node: unknown): node is ElementNode {
  return typeof node === "object" && node !== null;
}

function findAll(node: unknown, pred: (n: ElementNode) => boolean, out: ElementNode[] = []): ElementNode[] {
  if (!isElementNode(node)) return out;
  if (Array.isArray(node)) {
    for (const c of node) findAll(c, pred, out);
    return out;
  }
  if (pred(node)) out.push(node);
  if (node.props?.children !== undefined) findAll(node.props.children, pred, out);
  return out;
}

// The unexpanded <Toggle> element has the Toggle function as its type, and
// React keeps its key outside props.
function findToggleElement(tree: unknown): ElementNode {
  const found = findAll(tree, (n) => n.type === Toggle);
  expect(found.length).toBe(1);
  return found[0];
}

describe("ToggleRow shakeNonce", () => {
  it("passes no key and no .glim-shake class when shakeNonce is never provided (normal render, never shaken)", () => {
    const el = findToggleElement(
      ToggleRow({ label: "VMs", checked: true, onChange: () => {} })
    );
    expect(el.key == null).toBe(true);
    expect(String(el.props?.className)).not.toContain("glim-shake");
  });

  it("still renders no .glim-shake on a fresh page load even though a domain toggle map may hand back 0/undefined", () => {
    // Settings.tsx reads domainToggleShake.vmsEnabled, which is undefined
    // until that row has failed once.
    const el = findToggleElement(
      ToggleRow({ label: "VMs", checked: true, onChange: () => {}, shakeNonce: undefined })
    );
    expect(String(el.props?.className)).not.toContain("glim-shake");
  });

  it("a truthy shakeNonce keys the Toggle and applies .glim-shake", () => {
    const el = findToggleElement(
      ToggleRow({ label: "VMs", checked: false, onChange: () => {}, shakeNonce: 1 })
    );
    expect(el.key).toBe("1:0"); // shake:pulse
    expect(String(el.props?.className)).toContain("glim-shake");
  });

  it("a second consecutive failure of the same row gets a new key", () => {
    // A new key remounts the <button>, which gives the animation a fresh
    // timeline without an animationend listener or a forced reflow.
    const first = findToggleElement(
      ToggleRow({ label: "VMs", checked: false, onChange: () => {}, shakeNonce: 1 })
    );
    const second = findToggleElement(
      ToggleRow({ label: "VMs", checked: false, onChange: () => {}, shakeNonce: 2 })
    );
    expect(first.key).not.toBe(second.key);
    // Only the identity changes; both renders carry the class.
    expect(String(first.props?.className)).toContain("glim-shake");
    expect(String(second.props?.className)).toContain("glim-shake");
  });

  it("still forwards checked/onChange/disabled/label to the underlying Toggle unchanged", () => {
    let seen: boolean | undefined;
    const el = findToggleElement(
      ToggleRow({
        label: "VMs backup",
        checked: true,
        onChange: (v) => {
          seen = v;
        },
        disabled: true,
        shakeNonce: 3,
      })
    );
    expect(el.props?.checked).toBe(true);
    expect(el.props?.disabled).toBe(true);
    expect(el.props?.label).toBe("VMs backup");
    (el.props?.onChange as (v: boolean) => void)(false);
    expect(seen).toBe(false);
  });
});

describe("ToggleRow pulseNonce", () => {
  it("a truthy pulseNonce keys the Toggle and applies .glim-pulse", () => {
    const el = findToggleElement(
      ToggleRow({ label: "VMs", checked: true, onChange: () => {}, pulseNonce: 1 })
    );
    expect(el.key).toBe("0:1");
    expect(String(el.props?.className)).toContain("glim-pulse");
    expect(String(el.props?.className)).not.toContain("glim-shake");
  });

  it("a second consecutive success gets a new key", () => {
    const first = findToggleElement(
      ToggleRow({ label: "VMs", checked: true, onChange: () => {}, pulseNonce: 1 })
    );
    const second = findToggleElement(
      ToggleRow({ label: "VMs", checked: true, onChange: () => {}, pulseNonce: 2 })
    );
    expect(first.key).not.toBe(second.key);
    expect(String(first.props?.className)).toContain("glim-pulse");
    expect(String(second.props?.className)).toContain("glim-pulse");
  });

  it("prefers .glim-shake over .glim-pulse when both are truthy", () => {
    // A save either fails or succeeds, but the precedence should not be left
    // to chance.
    const el = findToggleElement(
      ToggleRow({ label: "VMs", checked: true, onChange: () => {}, shakeNonce: 1, pulseNonce: 1 })
    );
    expect(String(el.props?.className)).toContain("glim-shake");
    expect(String(el.props?.className)).not.toContain("glim-pulse");
  });

  it("a failure after a prior success still gets a fresh key", () => {
    // Saved once, then failed once: a key from either counter alone would
    // collide here.
    const afterSuccess = findToggleElement(
      ToggleRow({ label: "VMs", checked: true, onChange: () => {}, pulseNonce: 1 })
    );
    const afterFailure = findToggleElement(
      ToggleRow({ label: "VMs", checked: true, onChange: () => {}, shakeNonce: 1, pulseNonce: 1 })
    );
    expect(afterSuccess.key).not.toBe(afterFailure.key);
  });

  it("still renders no .glim-pulse on a fresh page load even when the map hands back 0/undefined", () => {
    const el = findToggleElement(
      ToggleRow({ label: "VMs", checked: true, onChange: () => {}, pulseNonce: undefined })
    );
    expect(el.key == null).toBe(true);
    expect(String(el.props?.className)).not.toContain("glim-pulse");
  });
});
