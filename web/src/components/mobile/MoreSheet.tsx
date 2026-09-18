// ---------------------------------------------------------------------------
// MoreSheet — the mobile "More" sheet.
//
// The desktop rail's overflow surface, re-expressed for the thumb: the
// destinations that do NOT own a bottom-bar slot, in desktop Sidebar order.
// The row list is derived from the ONE nav registry (lib/navModel.ts, via
// moreDestinations) — never a second hand-written ordering, which is the
// drift the registry exists to kill. The sheet is a consumer of the
// BottomSheet primitive and fills ONLY its body: title row, close
// button, Escape/scrim dismissal, focus trap, scroll containment and safe
// area are the primitive's contract, already proven there.
//
// Rows use the same accent-tint language as the bar's active slot: the row
// whose route is current reads --accentText on an --accentSoft backdrop, so a
// user who landed on /vms or /flash is never lost — no bar slot is active
// there, the sheet's row is. Active detection rides NavLink's isActive, the
// same className-by-isActive shape the rail's NavItem and the bar's slots
// use — no parallel active-state bookkeeping.
//
// Tap-on-active parity with the bar: tapping the ALREADY-current row scrolls
// the scroller back to the top — the mechanism is Layout's
// <main id="bv-main"> scroll, passed down through BottomNav, so this file
// never queries the DOM for the scroller itself.
//
// Sign-out: the sidebar footer's row re-expressed at the BOTTOM of the
// sheet, visually muted (muted text token, hairline separation, power glyph)
// and with NO confirmation — the exact Sidebar signOut mechanism copied
// verbatim (best-effort logout, then a location reload, which is what puts
// the login screen back). Gated by the same authEnabled flag as the desktop
// footer, so an instance without a password offers the row on NEITHER
// surface.
// ---------------------------------------------------------------------------
import { NavLink, useLocation } from "react-router-dom";
import type { Settings } from "../../lib/api";
import { logout } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { moreDestinations } from "../../lib/navModel";
import { IconPower } from "../navGlyphs";
import { BottomSheet } from "./BottomSheet";

export interface MoreSheetProps {
  /** Whether the sheet is open (the trigger lives in BottomNav). */
  open: boolean;
  /** Every close path funnels here — including a row navigation, which
   *  closes the sheet so the destination is visible behind it. */
  onClose: () => void;
  /** Already-loaded settings, owned by Layout — the chrome performs no
   *  fetches of its own. */
  settings: Settings | null;
  /** Whether a login password exists. Gates the sign-out row exactly like
   *  the desktop Sidebar footer (one flag, both surfaces). */
  authEnabled: boolean;
  /** Layout's tap-on-active scroll (the scroller is Layout's
   *  <main id="bv-main">) — fired when an already-current row is tapped. */
  scrollMainToTop: () => void;
}

// One destination row: 52px minimum height (min-h-[3.25rem], the iOS list
// cell measure) — comfortably over the 44px touch floor; the padding never
// carries the floor, the min-height does.
const rowBase = "flex min-h-[3.25rem] items-center gap-3 rounded-control px-3 text-body hover:bg-carbon-hover";

export function MoreSheet({ open, onClose, settings, authEnabled, scrollMainToTop }: MoreSheetProps) {
  const { t } = useT();
  const location = useLocation();

  // The Sidebar footer's signOut, copied verbatim (Sidebar.tsx): best-effort
  // logout — a failed call must never trap the user in a signed-out UI —
  // then the reload that re-runs the auth gate and puts the login screen
  // back. NO confirmation dialog: a password IS the confirmation.
  const signOut = async () => {
    await logout().catch(() => undefined);
    const g = globalThis as unknown as { location: { reload(): void } };
    g.location.reload();
  };

  return (
    <BottomSheet open={open} onClose={onClose} title={t("nav.more")}>
      <div data-testid="more-sheet" className="flex flex-col gap-1 py-2">
        {moreDestinations(settings).map((d) => {
          const Icon = d.icon;
          return (
            <NavLink
              key={d.to}
              to={d.to}
              onClick={(e) => {
                // NavLink's onClick fires BEFORE react-router's own Link
                // handler, and Link checks event.defaultPrevented before
                // navigating — preventDefault() on a tap-on-active suppresses
                // the re-navigation (the bar slot's exact contract; an
                // unsuppressed tap would stack a duplicate history entry).
                // The close below fires in BOTH cases: the navigate case
                // closes the sheet so the destination is visible behind it,
                // and the tap-on-active case reveals the scroller it just
                // sent back to the top.
                if (location.pathname === d.to) {
                  e.preventDefault();
                  scrollMainToTop();
                }
                onClose();
              }}
              className={({ isActive }) => `${rowBase} ${isActive ? "bg-accentSoft text-accentText" : "text-carbon-text"}`}
            >
              {/* 20px glyph — the rail's glim-nav-row sizing (the generated
                  glyphs come out at 16px and are scaled up here, exactly as
                  index.css scales them for the Sidebar). */}
              <span className="flex h-5 w-5 items-center justify-center [&_svg]:h-5 [&_svg]:w-5">
                <Icon />
              </span>
              <span className="min-w-0 truncate">{t(d.labelKey)}</span>
            </NavLink>
          );
        })}
        {authEnabled && (
          // The sign-out group: LAST row group of the sheet, behind a
          // hairline (border token), the row itself in the muted text token
          // with the power glyph — visually quiet next to the destination
          // rows above it, exactly as the desktop footer's row reads.
          <div className="mt-1 border-t border-carbon-border pt-2">
            <button
              type="button"
              onClick={() => void signOut()}
              className={`${rowBase} w-full text-carbon-textMuted motion-safe:active:scale-[.97]`}
            >
              <span className="flex h-5 w-5 items-center justify-center [&_svg]:h-5 [&_svg]:w-5">
                <IconPower />
              </span>
              <span className="min-w-0 truncate">{t("auth.logout")}</span>
            </button>
          </div>
        )}
      </div>
    </BottomSheet>
  );
}
