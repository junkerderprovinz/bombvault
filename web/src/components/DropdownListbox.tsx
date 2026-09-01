import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type KeyboardEvent as ReactKeyboardEvent,
  type ReactNode,
  type RefObject,
} from "react";
import { createPortal } from "react-dom";
import { computeBubblePosition } from "../lib/bubblePosition";
import { usePortalHue } from "../lib/portalHue";

// ---------------------------------------------------------------------------
// DropdownListbox — the shared `role="listbox"` panel every button-opens-a-
// list picker in this app drops open, PORTALLED to document.body and
// positioned off its trigger's own live rect.
//
// Bug this exists to fix (jdp, live review, Containers tab): "Wenn ich die
// Dropdownliste für 'Andere Container stoppen' öffne, wird sie abgeschnitten,
// weil sie nur bis zum unteren Ende der Card eingeblendet wird. Sie soll über
// die Card hinausgehen und voll angezeigt werden."
//   Root cause was NOT z-index. Both listboxes in this app (Containers.tsx's
// StopContainersEditor multi-select and Settings.tsx's LanguageCard) rendered
// their panel as an ordinary `absolute start-0 top-full … z-50` child of the
// trigger's own `relative` wrapper. ContainerRow's card — the wrapper that
// ends up being an ANCESTOR of the Containers one — is `relative
// overflow-hidden` (it needs that: ProgressBar.tsx renders as `absolute
// bottom-0 start-0 end-0 h-1` with square ends and is documented to require a
// `relative overflow-hidden` card so the bar gets clipped to the card's own
// rounded bottom corners; the `.glim-tint` wash and the row's rounded corners
// ride on the same clip). `overflow: hidden` clips absolutely-positioned
// DESCENDANTS whatever their z-index — z-50 could never have escaped it, so
// the panel physically stopped at the card's bottom edge.
//   The fix is the one this codebase already uses for every other floating
// surface: render through a portal so no ancestor's overflow can reach it
// (InfoBubble.tsx, IconTipButton.tsx, ColorPickerPopover.tsx, TimePicker.tsx),
// and position it with lib/bubblePosition.ts's shared computeBubblePosition —
// the same helper TimePicker/InfoBubble/IconTipButton call, so viewport
// clamping and the flip-above-the-trigger fallback come for free instead of
// being re-derived here.
//
// Structural reference is TimePicker.tsx, the closest existing sibling: same
// portal + measure-then-position useLayoutEffect, same dismissal set (outside
// mousedown, Escape, scroll, resize), and critically the same
// scroll-from-INSIDE-the-panel guard — this panel is itself scrollable
// (`max-h-60 overflow-y-auto`), so a naive capture-phase `window` scroll
// listener would close the list the moment a user scrolled it.
//
// The panel is sized to its TRIGGER's measured width, and centred on the
// trigger via `translateX(-50%)`, because computeBubblePosition's contract is
// "here is where the CENTRE goes" (see .glim-time-popover's own CSS comment
// for why that pairing has to be honoured exactly). Equal widths make centred
// and start-aligned identical, so this keeps the pre-portal look — while also
// making RTL correct for free (no `start-0`/`end-0` branch at all) and
// keeping a `w-64 max-w-full` trigger that got squeezed by a narrow card from
// popping open a visibly wider list than the button that opened it.
// ---------------------------------------------------------------------------

/** Gap between trigger and panel AND the panel's minimum distance from the
 *  viewport edge, handed to computeBubblePosition as its `margin`. 4px, not
 *  the helper's own 8px default, so the gap stays exactly the `mt-1` the two
 *  pre-portal listboxes rendered with. */
const DROPDOWN_GAP = 4;

export interface DropdownListboxProps {
  /** Whether the panel is mounted. Owned by the caller — the trigger button
   *  lives at the call site (each picker's trigger is bespoke: a flag+label
   *  row here, a title+chevron row there), only the PANEL is shared. */
  open: boolean;
  /** Called when the panel dismisses itself (outside click, Escape, scroll,
   *  resize). Picking an option does NOT go through this — a single-select
   *  list closes itself at the call site, a multi-select one stays open. */
  onClose: () => void;
  /** The trigger's own wrapper/button, used both to anchor the panel and to
   *  exempt the trigger from the outside-click dismissal (otherwise clicking
   *  an open trigger would close-then-immediately-reopen). */
  triggerRef: RefObject<HTMLElement | null>;
  /** Accessible name for the listbox, normally the same label the trigger
   *  carries. */
  label: string;
  /** `aria-multiselectable` — set for a checkbox multi-select list. */
  multiselectable?: boolean;
  /** The `role="option"` buttons. */
  children: ReactNode;
}

export function DropdownListbox({
  open,
  onClose,
  triggerRef,
  label,
  multiselectable,
  children,
}: DropdownListboxProps) {
  const panelRef = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState<{ left: number; top: number; width: number } | null>(null);
  // One focus move per opening, and only once the panel sits at its real
  // coordinates — see the focus effect below for why both halves matter.
  const focusedOnOpen = useRef(false);
  // Tracks whether focus currently sits inside the panel — see the focus-return
  // effect below for why this cannot be read at teardown instead.
  const focusInsideRef = useRef(false);
  useEffect(() => {
    if (!open) focusedOnOpen.current = false;
  }, [open]);
  // `onClose` is read through a ref inside the listener effect so that a call
  // site passing an inline arrow function (both of them do) doesn't tear down
  // and re-attach every listener on each of its own re-renders — a re-render
  // that happens on literally every option toggle in the multi-select case.
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;

  // Reset the measured position on every close, so the next open re-measures
  // from scratch (the trigger may have moved, the list may have changed
  // length) instead of flashing at the previous open's coordinates.
  useLayoutEffect(() => {
    if (!open) setPos(null);
  }, [open]);

  // Rainbow-engine inheritance across the portal (this repo's standing "every
  // new element goes into the colour engine" rule; TimePicker.tsx hit and
  // documented the identical trap first). A portalled panel is a child of
  // document.body, NOT of the trigger, so the per-position `--item-hue*`
  // custom properties an ancestor sets inline — ContainerRow's own
  // `hueVars(rainbowAt(index))` on the row card — stop reaching it entirely.
  // Before the portal, this listbox's option checkboxes (`accentColor:
  // var(--accent)`) painted in THEIR ROW'S rainbow position; portalling
  // without this would have silently dropped them back to the single global
  // accent, which is exactly the regression that rule exists to prevent.
  //   Copied off the trigger's own *resolved* computed style rather than
  // taken as a prop, so it needs nothing from the call sites and keeps
  // working for any future one: whatever hue the trigger stands in, the panel
  // it opens stands in too. `.glim-hue` is added alongside so index.css's own
  // `[data-rainbow] .glim-hue` / `[data-rainbow="reactive"] .glim-hue:hover`
  // rules do the actual --accent derivation, in every rainbow mode, instead
  // of this component hard-coding one mode's answer. With rainbow off there
  // are no --item-hue* values to copy, `hue` stays empty, and the panel keeps
  // the global accent exactly as before.
  //   The implementation moved to lib/portalHue.ts in [543], unchanged in
  // behaviour: FolderBrowser's dialog needed the identical thing and had gone
  // without it since [478] portalled it, so the second occurrence became a
  // shared hook rather than a second copy. The hand-kept property list went
  // with it — it is derived from `hueVars()` now, which is the function that
  // writes the properties in the first place.
  const hue = usePortalHue(open, triggerRef);

  // Measure, then place — in a useLayoutEffect so the corrected position
  // lands before the browser's next paint rather than as a visible jump from
  // the off-screen parking spot the JSX renders at first.
  useLayoutEffect(() => {
    if (!open) return;
    const trigger = triggerRef.current;
    const panel = panelRef.current;
    if (!trigger || !panel) return;
    const rect = trigger.getBoundingClientRect();
    // Written straight to the DOM node BEFORE the height is read, not via
    // state: the panel's height depends on its own width (option labels
    // wrap/truncate against it), so measuring height at the panel's natural
    // width and only then narrowing it to the trigger's width would hand
    // computeBubblePosition a height that no longer describes the box being
    // positioned. Same value goes into `pos` below, so React's own next
    // render writes back an identical inline width — this is a measurement
    // pre-pass, not a source of truth React doesn't know about.
    panel.style.width = `${rect.width}px`;
    const viewport = {
      width: document.documentElement.clientWidth || window.innerWidth,
      height: document.documentElement.clientHeight || window.innerHeight,
    };
    const { left, top } = computeBubblePosition(
      rect,
      { width: rect.width, height: panel.offsetHeight },
      viewport,
      DROPDOWN_GAP
    );
    // Bail out when nothing moved. `children` is in this effect's dep list on
    // purpose — a list that changes length while open (the multi-select drops
    // a stale row the moment it's unticked) has to re-measure or it keeps the
    // old height's placement — but children's identity is fresh on every
    // single render, so an unconditional setPos with a fresh object literal
    // would re-render, re-run this effect, and spin forever. Returning the
    // PREVIOUS object when the numbers are unchanged makes React bail out of
    // the re-render instead, so the loop settles after one pass.
    setPos((prev) =>
      prev && prev.left === left && prev.top === top && prev.width === rect.width
        ? prev
        : { left, top, width: rect.width }
    );
  }, [open, triggerRef, children]);

  // Dismissal: outside pointerdown, Escape, scroll, resize — TimePicker.tsx's
  // and ColorPickerPopover.tsx's documented set, for the same reason (a fixed
  // panel de-anchored from its trigger reads as broken either way).
  //
  // mousedown, NOT click, is what closes on an outside press — and the panel
  // itself is exempted by `panelRef.contains(target)`. That exemption is
  // load-bearing now that the panel is portalled: the call sites' old
  // handlers tested "is the target inside the trigger's wrapper", which a
  // portalled option button no longer is, so pressing an option would have
  // unmounted the list on mousedown and the ensuing click would never have
  // reached the option at all — the list would have looked like it closed
  // without selecting anything.
  //
  // The scroll listener is capture-phase on `window` so it also sees scrolls
  // of intermediate scroll containers (this app's `main` is the real page
  // scroller, not the document). Scrolls whose target is inside the panel are
  // ignored: this panel is `overflow-y-auto` with a 15rem cap, so scrolling a
  // long list — 42 languages, or a long container list — is an INTERNAL
  // scroll that never de-anchors it from its trigger. Without this guard the
  // list would close the instant the user tried to scroll it. (TimePicker.tsx
  // carries the same guard for its own two scrollable columns.)
  useEffect(() => {
    if (!open) return;
    function onPointerDown(e: MouseEvent) {
      const target = e.target as Node;
      if (panelRef.current?.contains(target)) return;
      if (triggerRef.current?.contains(target)) return;
      onCloseRef.current();
    }
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") onCloseRef.current();
    }
    function onScroll(e: Event) {
      const target = e.target;
      if (target instanceof Node && panelRef.current?.contains(target)) return;
      onCloseRef.current();
    }
    function onResize() {
      onCloseRef.current();
    }
    document.addEventListener("mousedown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    window.addEventListener("scroll", onScroll, true);
    window.addEventListener("resize", onResize);
    return () => {
      document.removeEventListener("mousedown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
      window.removeEventListener("scroll", onScroll, true);
      window.removeEventListener("resize", onResize);
    };
  }, [open, triggerRef]);

  // The options are the caller's children, so they are found in the DOM rather
  // than held as refs (TimePicker owns its own options and can keep refs; this
  // panel cannot). `[role="option"]` is the contract every call site already
  // renders — it is what makes this a listbox at all.
  function options(): HTMLElement[] {
    const panel = panelRef.current;
    if (!panel) return [];
    return Array.from(panel.querySelectorAll<HTMLElement>('[role="option"]'));
  }

  // Move focus INTO the panel when it opens, at the selected option, so a
  // keyboard user reaches the list at all. Without this, focus stayed on the
  // trigger: the next Tab went to the following control on the page rather than
  // into the 42 languages, and because the capture-phase scroll listener above
  // closes the panel, tabbing away often shut it before it could be reached.
  //
  // Gated on `pos`, not just `open`, and this is load-bearing — TimePicker
  // documents the same trap for the same reason: focusing an option while the
  // panel is still parked at left/top -9999px makes the browser auto-scroll to
  // bring it into view, which trips this component's own scroll-closes-the-panel
  // listener a few milliseconds later and closes the panel it just opened.
  useLayoutEffect(() => {
    if (!open || !pos || focusedOnOpen.current) return;
    const opts = options();
    if (opts.length === 0) return;
    const active = opts.find((o) => o.getAttribute("aria-selected") === "true") ?? opts[0];
    focusedOnOpen.current = true;
    active.scrollIntoView?.({ block: "nearest" });
    active.focus();
  }, [open, pos]);

  // Give focus back to the trigger when the panel closes, but ONLY if focus is
  // still inside the panel: a user who closed it by clicking elsewhere has
  // already chosen where focus should go, and yanking it back would fight them.
  // Whether focus is inside the panel is TRACKED as it moves, not asked for at
  // teardown. By the time any cleanup runs, React has removed the portal's nodes
  // and the browser has already moved focus to <body>, so a
  // panel.contains(document.activeElement) test there can never be true — it
  // reads as "focus was outside" for every close, including the one case that
  // should return focus. Measured: both a passive and a layout cleanup see
  // <body>.
  //
  // The two events are enough because they bubble: onFocus fires for any option
  // gaining focus, and onBlur only counts as leaving when focus is going
  // somewhere OUTSIDE the panel (relatedTarget), so moving between options with
  // the arrow keys does not clear it.
  useLayoutEffect(() => {
    const trigger = triggerRef.current;
    return () => {
      if (focusInsideRef.current) trigger?.focus();
      focusInsideRef.current = false;
    };
  }, [open, triggerRef]);

  // Arrow/Home/End move between options; the panel handles it rather than each
  // call site, so every listbox in the app behaves the same. Enter/Space are NOT
  // handled here: the options are real buttons and already activate themselves.
  function onPanelKeyDown(e: ReactKeyboardEvent<HTMLDivElement>) {
    if (e.key !== "ArrowDown" && e.key !== "ArrowUp" && e.key !== "Home" && e.key !== "End") return;
    const opts = options();
    if (opts.length === 0) return;
    e.preventDefault();
    const at = opts.indexOf(document.activeElement as HTMLElement);
    let next: number;
    if (e.key === "Home") next = 0;
    else if (e.key === "End") next = opts.length - 1;
    else {
      const dir = e.key === "ArrowDown" ? 1 : -1;
      // Wrapping, like TimePicker's own columns.
      next = at < 0 ? (dir === 1 ? 0 : opts.length - 1) : (at + dir + opts.length) % opts.length;
    }
    opts[next].scrollIntoView?.({ block: "nearest" });
    opts[next].focus();
  }

  if (!open) return null;

  return createPortal(
    <div
      ref={panelRef}
      role="listbox"
      onKeyDown={onPanelKeyDown}
      onFocus={() => {
        focusInsideRef.current = true;
      }}
      onBlur={(e) => {
        if (!e.currentTarget.contains(e.relatedTarget as Node | null)) focusInsideRef.current = false;
      }}
      aria-multiselectable={multiselectable ? "true" : undefined}
      aria-label={label}
      className={`fixed z-50 max-h-60 overflow-y-auto rounded-card bg-carbon-surface shadow-xl glim-fade${
        hue.className ? ` ${hue.className}` : ""
      }`}
      style={{
        // Parked off-screen until the layout effect above has measured: the
        // panel has to be in the DOM (and at its real width) to have a height
        // worth clamping against, and TimePicker.tsx documents what happens
        // when something focuses/scrolls an element still sitting at -9999px.
        left: pos?.left ?? -9999,
        top: pos?.top ?? -9999,
        width: pos?.width,
        transform: "translateX(-50%)",
        scrollbarColor: "var(--carbon-border) transparent",
        // Spread LAST so the rainbow position the trigger stands in reaches
        // the options across the portal boundary (see the effect that reads
        // these). Spreading `null` would be a type error, `{}` is a no-op.
        ...(hue.style ?? {}),
      } as CSSProperties}
    >
      {children}
    </div>,
    document.body
  );
}
