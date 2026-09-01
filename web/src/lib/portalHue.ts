// Carrying a rainbow position across a portal boundary ([543]).
// ---------------------------------------------------------------------------
// The rainbow engine works by inheritance: `[data-rainbow] .glim-hue` (index.css)
// redefines `--color-accent` and friends for its whole SUBTREE, so anything
// painted with `var(--accent)` inside a card takes THAT card's palette position
// with no per-call-site wiring. That is the mechanism half this codebase relies
// on, and several comments cite it as the reason they need no `hueIndex` prop.
//
// A portal breaks it, silently. `createPortal` moves the DOM node to <body>,
// where there is no `.glim-hue` ancestor left, so every hued colour inside it
// falls back to the single global accent. Nothing errors, nothing looks broken
// in isolation, and the panel simply wears the wrong colour — which is only
// visible next to the control that opened it.
//
// This happened twice. DropdownListbox.tsx solved it for its own panel and
// wrote down exactly why ("portalling without this would have silently dropped
// them back to the single global accent, which is exactly the regression that
// rule exists to prevent"). Then [478] portalled FolderBrowser's dialog and did
// not apply it, so a folder dialog opened from a card tinted #FF8389 painted
// its one accent control #BE95FF, the global default. Measured live, not
// guessed. That is the `guard-applied-once-is-a-guard-missing-twice` shape, and
// the fix for the SECOND occurrence is to stop having two copies rather than to
// make a third.
//
// The property list is DERIVED from `hueVars()` rather than typed out. The
// original list carried the note "kept in step with it by hand, the same way
// index.css's rules that consume these names are" — an honest description of a
// list that goes stale the first time a token is added, and one token was in
// fact removed from hueVars() since. Reading the keys off the function that
// writes them cannot drift.
import { useLayoutEffect, useState, type RefObject } from "react";
import { hueVars } from "./appearance";

/** The exact set `hueVars()` writes. Any hex parses to the same key set, so
 *  the sample colour is arbitrary and never rendered. */
const HUE_VARS = Object.keys(hueVars("#000000"));

/** What a portalled panel needs to stand in its trigger's palette position. */
export interface PortalHue {
  /** Inline custom properties to spread onto the portalled element. */
  style: Record<string, string> | undefined;
  /** `"glim-hue"` when there is a hue to apply, otherwise `""`. */
  className: string;
}

/**
 * Copy the trigger's rainbow position onto a portalled panel.
 *
 * Read off the trigger's *resolved* computed style rather than taken as a prop,
 * so it needs nothing from the call sites and keeps working for any future one:
 * whatever hue the trigger stands in, the panel it opens stands in too.
 *
 * `.glim-hue` travels with the properties because index.css's own
 * `[data-rainbow] .glim-hue` and `[data-rainbow="reactive"] .glim-hue:hover`
 * rules do the actual `--accent` derivation. Applying the class here rather
 * than deriving colours means every rainbow MODE keeps working, instead of this
 * hook hard-coding one mode's answer.
 *
 * With rainbow off there are no `--item-hue*` values to copy, `style` stays
 * undefined and `className` empty, and the panel keeps the global accent
 * exactly as before. The class is never applied without the properties: as
 * appearance.ts puts it, "`.glim-hue` with no `--item-hue` under it would
 * resolve the accent to nothing".
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
    const cs = getComputedStyle(trigger);
    const vars: Record<string, string> = {};
    for (const name of HUE_VARS) {
      const value = cs.getPropertyValue(name).trim();
      if (value) vars[name] = value;
    }
    setHue(Object.keys(vars).length > 0 ? vars : null);
  }, [open, triggerRef]);

  return { style: hue ?? undefined, className: hue ? "glim-hue" : "" };
}
