import {
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import { computeBubblePosition } from "../lib/bubblePosition";

// ---------------------------------------------------------------------------
// IconTipButton — a plain icon-only <button> whose accessible name AND its
// only hover/focus explanation is a real `.glim-bubble` tooltip, not the OS's
// own native `title=` balloon.
//
// design-language.md, "The tooltip and info bubble": "A plain icon-only
// button's hover tooltip and a control's '(i)' explanatory bubble are the
// same mechanism wearing two different trigger elements, not two separate
// implementations that happen to look similar... A stray native `title=`
// anywhere in the app is auto-upgraded to `data-tip`..." BombVault has no
// single delegated `wireTooltips()` engine the way the GlimStone reference
// does — InfoBubble.tsx (the "(i)" trigger) and Selector.tsx's SelectorTab
// (a Selector segment's own `tip`) are its own two existing, independent
// ports of that engine, each wired to its own specific trigger shape. This
// file is the THIRD: the same open-state/measure-then-clamp-position/
// scroll-and-Escape-close contract as those two, wired to a plain `<button>`
// instead — so a bare icon-only action button (not an "(i)" glyph, not a
// Selector segment) has a real place to render its own tooltip from rather
// than a fourth call site reaching for the native `title=` attribute that
// caused this file to need to exist in the first place.
//
// Root cause (jdp, live-review: "beim Ordnersymbol ist die Hover-Infobubble
// nicht im GlimStone"): FolderBrowser's own "Durchsuchen" icon button carried
// only `title=`/`aria-label=` — the browser's own plain OS balloon, visibly
// disagreeing with PathModeSwitch's Local/Remote icon pair sitting right next
// to it, which already got the real `.glim-bubble` treatment (via Selector's
// own `tip`) in an earlier round. `tip` is REQUIRED here, not optional:
// design-language.md's own tooltip section is explicit that "an icon-only
// button (no visible text at all) needs one unconditionally — there's no
// other way to know what it does."
// ---------------------------------------------------------------------------
export function IconTipButton({
  tip,
  onClick,
  disabled,
  className,
  style,
  children,
  type = "button",
  ariaPressed,
  ariaExpanded,
}: {
  /** Hover/focus-revealed explanation AND the button's accessible name
   *  (`aria-label`) — an icon-only trigger has no other visible text a name
   *  could come from. */
  tip: string;
  onClick?: () => void;
  disabled?: boolean;
  className?: string;
  /** `aria-pressed` for an icon-only trigger that is a TOGGLE rather than a
   *  one-shot action — it stays visibly "on" between clicks, so assistive tech
   *  needs the pressed state as well as the name. Added for Dashboard's
   *  customize pencil, which was one of the icon-only controls still
   *  explaining itself with a native `title=` balloon — exactly the
   *  anti-pattern this file's header says it exists to replace.
   *    (That change's own note called the pencil "the LAST" such control. It
   *  was not: nine more survived it, in dashboardLayout's four card controls
   *  and the Containers/VMs reorder arrows, and they only came to light when
   *  bombvault/icon-badge-needs-tooltip started failing the build on them.
   *  They are converted now, and the rule is what keeps the count at zero.)
   *  Omitted — the default — renders no attribute at all, so every existing
   *  one-shot action button is unchanged. */
  ariaPressed?: boolean;
  /** `aria-expanded` for an icon-only trigger that OPENS something — a
   *  disclosure chevron whose panel appears below it. Same contract as
   *  `ariaPressed` above (omitted renders no attribute), added for
   *  Containers' stack-restore disclosure toggle, which carried
   *  `aria-expanded` on the plain `<button>` it replaced and must not lose
   *  it on the way to a real tooltip. */
  ariaExpanded?: boolean;
  /** Inline style, added for Badge.tsx's own `tip` branch (GlimStone
   *  follow-up round — the off-site tab's four action buttons converting
   *  from text badges to icon-only ones): a hue-enabled Badge needs its
   *  `--item-hue*` custom properties set inline (the same mechanism its own
   *  plain `<button>`/`<a>` branches already use via `style={hueStyle}`),
   *  and this plain-`<button>` wrapper had no way to carry that until now —
   *  every other consumer of this component today has no hue, so this is
   *  additive and optional, not a behaviour change for them. */
  style?: CSSProperties;
  children: ReactNode;
  type?: "button" | "submit";
}) {
  const [open, setOpen] = useState(false);
  const btnRef = useRef<HTMLButtonElement | null>(null);
  const bubbleRef = useRef<HTMLDivElement | null>(null);
  const tooltipId = useId();

  function show() {
    setOpen(true);
  }
  function hide() {
    setOpen(false);
  }

  // Same measure-then-clamp positioning as InfoBubble.tsx/Selector.tsx's
  // SelectorTab — see either file's own header comment for the full
  // root-cause writeup (the "Wiederherstellungskit" viewport-overflow bug)
  // this ports verbatim rather than reinventing.
  useLayoutEffect(() => {
    if (!open) return;
    const trigger = btnRef.current;
    const bubble = bubbleRef.current;
    if (!trigger || !bubble) return;
    const r = trigger.getBoundingClientRect();
    const viewport = {
      width: document.documentElement.clientWidth || window.innerWidth,
      height: document.documentElement.clientHeight || window.innerHeight,
    };
    const { left, top } = computeBubblePosition(
      r,
      { width: bubble.offsetWidth, height: bubble.offsetHeight },
      viewport,
    );
    bubble.style.left = `${left}px`;
    bubble.style.top = `${top}px`;
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const onScroll = () => hide();
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") hide();
    };
    window.addEventListener("scroll", onScroll, true);
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("scroll", onScroll, true);
      window.removeEventListener("keydown", onKey);
    };
  }, [open]);

  // A DISABLED button emits no mouse events and takes no focus, so the hover
  // handlers below never fire and the tip — which for an icon-only control is
  // the only thing naming it — becomes unreachable exactly when the user most
  // wants to know why the control is dead. The wrapper is not disabled, so it
  // still sees the pointer; it is rendered ONLY in that case, so the enabled
  // layout is untouched rather than gaining a box everywhere for a corner case.
  // `inline-flex` matches what these buttons already sit in.
  const withHoverWrapper = (node: ReactNode) =>
    disabled ? (
      <span className="inline-flex" onMouseEnter={show} onMouseLeave={hide}>
        {node}
      </span>
    ) : (
      node
    );

  return (
    <>
      {withHoverWrapper(
        <button
          ref={btnRef}
          type={type}
          onClick={onClick}
          disabled={disabled}
          aria-label={tip}
          aria-pressed={ariaPressed}
          aria-expanded={ariaExpanded}
          aria-describedby={open ? tooltipId : undefined}
          onMouseEnter={show}
          onMouseLeave={hide}
          onFocus={show}
          onBlur={hide}
          className={className}
          style={style}
        >
          {children}
        </button>,
      )}
      {open &&
        createPortal(
          <div
            ref={bubbleRef}
            role="tooltip"
            id={tooltipId}
            className="glim-bubble glim-fade"
          >
            {tip}
          </div>,
          document.body,
        )}
    </>
  );
}
