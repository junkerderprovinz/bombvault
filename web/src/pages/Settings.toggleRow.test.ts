// ---------------------------------------------------------------------------
// ToggleRow — shakeNonce (#142, Domains card auto-save error feedback).
//
// ToggleRow (like Toggle itself — see Toggle.test.ts's own header comment)
// is a pure, hookless function component: props in, a plain React element
// tree out. Same reasoning applies here — call it directly as a plain
// function and inspect the returned element tree, no jsdom/testing-library
// needed, keeping this on the "node environment, no DOM" footing
// vitest.config.ts documents as the default for this repo.
//
// This covers the WIRING that makes the `.glim-shake` error-feedback
// animation (index.css) replay reliably even for a repeated identical
// failure of the SAME toggle (e.g. the Domains card's VMs row failing twice
// in a row because SSH still isn't configured): shakeNonce is forwarded as
// the underlying Toggle's React `key`, so a NEW nonce value always produces a
// NEW element identity — the same "fresh identity per occurrence" mechanism
// lib/toast.tsx's push() already uses (a fresh toast `id` per push, so an
// identical repeated message still gets a fresh DOM node and its entrance
// animation always plays). Whether the CSS keyframe itself actually animates
// in a real browser is verified live with Playwright (not something this
// node-environment tree-inspection test can see) — see the live-verification
// notes for that proof.
// ---------------------------------------------------------------------------
import { describe, expect, it } from "vitest";
import { ToggleRow } from "./Settings";
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

// Finds the (still-unexpanded) <Toggle .../> element ToggleRow renders —
// its `type` is the imported Toggle function reference itself, and its `key`
// lives OUTSIDE `props` (React stores an element's key separately), which is
// exactly the field this suite cares about.
function findToggleElement(tree: unknown): ElementNode {
  const found = findAll(tree, (n) => n.type === Toggle);
  expect(found.length).toBe(1);
  return found[0];
}

describe("ToggleRow — shakeNonce", () => {
  it("passes no key and no .glim-shake class when shakeNonce is never provided (normal render, never shaken)", () => {
    const el = findToggleElement(
      ToggleRow({ label: "VMs", checked: true, onChange: () => {} })
    );
    expect(el.key == null).toBe(true);
    expect(String(el.props?.className)).not.toContain("glim-shake");
  });

  it("still renders no .glim-shake on a fresh page load even though a domain toggle map may hand back 0/undefined", () => {
    // Mirrors how Settings.tsx reads an unset entry out of its
    // domainToggleShake map (`domainToggleShake.vmsEnabled`), which is
    // `undefined` until that row has actually failed once.
    const el = findToggleElement(
      ToggleRow({ label: "VMs", checked: true, onChange: () => {}, shakeNonce: undefined })
    );
    expect(String(el.props?.className)).not.toContain("glim-shake");
  });

  it("a truthy shakeNonce both keys the Toggle AND applies .glim-shake — the first recorded failure", () => {
    const el = findToggleElement(
      ToggleRow({ label: "VMs", checked: false, onChange: () => {}, shakeNonce: 1 })
    );
    expect(el.key).toBe("1"); // React stringifies numeric keys
    expect(String(el.props?.className)).toContain("glim-shake");
  });

  it("a SECOND consecutive failure of the SAME row gets a genuinely NEW key, not a reused one", () => {
    // This is the actual replay mechanism: ToggleRow doesn't toggle a class
    // on a persistent node (which would need an animationend listener or a
    // forced-reflow class-remove-then-readd trick to replay on an identical
    // trigger) — it changes `key`, so React unmounts and remounts the real
    // <button>, giving the CSS animation a fresh timeline every time,
    // mirroring lib/toast.tsx's fresh-id-per-push precedent.
    const first = findToggleElement(
      ToggleRow({ label: "VMs", checked: false, onChange: () => {}, shakeNonce: 1 })
    );
    const second = findToggleElement(
      ToggleRow({ label: "VMs", checked: false, onChange: () => {}, shakeNonce: 2 })
    );
    expect(first.key).not.toBe(second.key);
    // Both renders still carry the class — only the identity needs to change
    // for the animation to replay, not the class list.
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
