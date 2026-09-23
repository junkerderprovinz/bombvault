// Carries a rainbow position across a portal boundary.
//
// The rainbow works by inheritance: `[data-rainbow] .glim-hue` in index.css
// redefines `--color-accent` and friends for its whole subtree, so anything
// painted with `var(--accent)` inside a card takes that card's hue without a
// prop. createPortal moves a panel to <body>, out of that subtree, and every
// hued colour in it silently falls back to the global accent. DropdownListbox
// and FolderBrowser both portal their panels and share this hook.
//
// The property list is read off hueVars() rather than typed out, so it cannot
// drift from what that function writes.
import { useLayoutEffect, useState, type RefObject } from "react";
import { hueVars } from "./appearance";

/** The exact set `hueVars()` writes. Every position gives the same key set,
 *  so the sample is arbitrary and never rendered. */
const HUE_VARS = Object.keys(hueVars(0));

/** What a portalled panel needs to stand in its trigger's palette position. */
export interface PortalHue {
  /** Inline custom properties to spread onto the portalled element. */
  style: Record<string, string> | undefined;
  /** `"glim-hue"` when there is a hue to apply, otherwise `""`. */
  className: string;
}

/**
 * Copies the trigger's rainbow position onto a portalled panel.
 *
 * It reads the hue off the trigger or the nearest ancestor that sets it
 * rather than taking a prop, so call sites pass nothing and the panel matches
 * whatever hue its trigger stands in. The inline values are copied, not the
 * computed ones: they are `var(--rb-N)` references, so the open panel keeps
 * following a palette change or the disco glide on the root.
 * `.glim-hue` travels with the properties because index.css's
 * `[data-rainbow] .glim-hue` rules derive `--accent` from them, which keeps
 * every rainbow mode working without this hook knowing about modes.
 *
 * With rainbow off there are no `--item-hue*` values to copy: `style` stays
 * undefined, `className` empty, and the panel keeps the global accent. The
 * class is never applied without the properties, since `.glim-hue` without
 * `--item-hue` resolves the accent to nothing.
 */
export function usePortalHue(
  open: boolean,
  triggerRef: RefObject<HTMLElement | null>
): PortalHue {
  const [hue, setHue] = useState<Record<string, string> | null>(null);
  useLayoutEffect(() => {
    if (!open) {
      setHue(null);
      return;
    }
    const trigger = triggerRef.current;
    if (!trigger) return;
    let owner: HTMLElement | null = trigger;
    while (owner && !owner.style.getPropertyValue("--item-hue")) owner = owner.parentElement;
    const vars: Record<string, string> = {};
    for (const name of HUE_VARS) {
      const value = owner?.style.getPropertyValue(name);
      if (value) vars[name] = value;
    }
    setHue(Object.keys(vars).length > 0 ? vars : null);
  }, [open, triggerRef]);

  return { style: hue ?? undefined, className: hue ? "glim-hue" : "" };
}
