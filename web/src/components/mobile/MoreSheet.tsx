// MoreSheet; the mobile "More" sheet.
//
// The desktop rail's overflow surface, re-expressed for the thumb: the
// destinations that do not own a bottom-bar slot; Settings among them; in
// desktop Sidebar order. The row list is derived from the one nav registry
// (lib/navModel.ts, via moreDestinations); never a second hand-written
// ordering, which is the drift the registry exists to kill. The sheet is a
// consumer of the BottomSheet primitive and fills only its body: title row,
// close button, Escape/scrim dismissal, focus trap, scroll containment and
// safe area are the primitive's contract, already proven there.
//
// Rows sit on the colour engine like every rail row: each carries
// `glim-hue` + `hueVars(...)`, continuing the caller's rotation
// from hueOffset (the bar passes its slot count plus the trigger's own
// position, so the sheet never replays the bar's colours), and the row
// whose route is current is filled; the whole row takes the accent edge
// to edge and its marks take the paired ink via currentColor; so a user who
// landed on /vms or /flash is never lost: no bar slot is active there, this
// row is. Active detection rides NavLink's isActive, the same
// className-by-isActive shape the rail's NavItem and the bar's slots use;
// no parallel active-state bookkeeping.
//
// Settings is always among the rows, so the sheet has content on every
// instance and the bar's More trigger can render unconditionally.
//
// Tap-on-active parity with the bar: tapping the already-current row scrolls
// the scroller back to the top; the mechanism is Layout's
// <main id="bv-main"> scroll, passed down through BottomNav, so this file
// never queries the DOM for the scroller itself.
//
// The Anomalies row carries the rail's count of open critical and warning
// findings, in the same badge, since this sheet is where a phone keeps that
// row.
//
// Sign-out: the sidebar footer's row re-expressed at the bottom of the
// sheet, visually muted (muted text token, power glyph) and with no
// confirmation; the exact Sidebar signOut mechanism copied verbatim
// (best-effort logout, then a location reload, which is what puts the login
// screen back). Gated by the same authEnabled flag as the desktop footer,
// so an instance without a password offers the row on neither surface.
import { NavLink, useLocation } from "react-router-dom";
import type { CSSProperties } from "react";
import type { Settings } from "../../lib/api";
import { logout } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { hueVars } from "../../lib/appearance";
import { moreDestinations } from "../../lib/navModel";
import { useLoudAnomalies } from "../../lib/useAnomalies";
import { Badge } from "../Badge";
import { IconSignOut } from "../glyphs";
import { BottomSheet } from "./BottomSheet";

export interface MoreSheetProps {
  /** Whether the sheet is open (the trigger lives in BottomNav). */
  open: boolean;
  /** Every close path funnels here; including a row navigation, which
   *  closes the sheet so the destination is visible behind it. */
  onClose: () => void;
  /** Already-loaded settings, owned by Layout; the chrome performs no
   *  fetches of its own. */
  settings: Settings | null;
  /** Whether a login password exists. Gates the sign-out row exactly like
   *  the desktop Sidebar footer (one flag, both surfaces). */
  authEnabled: boolean;
  /** Layout's tap-on-active scroll (the scroller is Layout's
   *  <main id="bv-main">); fired when an already-current row is tapped. */
  scrollMainToTop: () => void;
  /** Where this sheet's hue rotation starts. The bar passes its slot count
   *  plus the More trigger's own position, so the sheet's rows continue the
   *  bar's rotation instead of replaying it; standalone mounts (the
   *  default 0) start at the palette's first colour. */
  hueOffset?: number;
}

// One row (destination or sign-out alike): 52px minimum height
// (min-h-[3.25rem], the iOS list cell measure); comfortably over the 44px
// touch floor; the padding never carries the floor, the min-height does.
const rowBase = "flex min-h-[3.25rem] items-center gap-3 rounded-control px-3 text-body hover:bg-carbon-hover";

export function MoreSheet({ open, onClose, settings, authEnabled, scrollMainToTop, hueOffset = 0 }: MoreSheetProps) {
  const { t } = useT();
  const location = useLocation();
  const rows = moreDestinations(settings);
  const loudAnomalies = useLoudAnomalies();

  // The Sidebar footer's signOut, copied verbatim (Sidebar.tsx): best-effort
  // logout; a failed call must never trap the user in a signed-out UI;
  // then the reload that re-runs the auth gate and puts the login screen
  // back. No confirmation dialog: a password is the confirmation.
  const signOut = async () => {
    await logout().catch(() => undefined);
    const g = globalThis as unknown as { location: { reload(): void } };
    g.location.reload();
  };

  // Hue positions: one sequence through the sheet starting at hueOffset, in
  // render order. The offset is what keeps the sheet inside the bar's
  // rotation: the bar's slots and the More trigger already consumed the
  // palette's first positions, and replaying them here would paint the same
  // colour twice on one screen.
  const signOutHue = hueOffset + rows.length;

  return (
    <BottomSheet open={open} onClose={onClose} title={t("nav.more")}>
      <div data-testid="more-sheet" className="flex flex-col gap-1 py-2">
        {rows.map((d, i) => {
          const Icon = d.icon;
          return (
            <NavLink
              key={d.to}
              to={d.to}
              onClick={(e) => {
                // NavLink's onClick fires before react-router's own Link
                // handler, and Link checks event.defaultPrevented before
                // navigating; preventDefault() on a tap-on-active suppresses
                // the re-navigation (the bar slot's exact contract; an
                // unsuppressed tap would stack a duplicate history entry).
                // The close below fires in both cases: the navigate case
                // closes the sheet so the destination is visible behind it,
                // and the tap-on-active case reveals the scroller it just
                // sent back to the top.
                if (location.pathname === d.to) {
                  e.preventDefault();
                  scrollMainToTop();
                }
                onClose();
              }}
              className={({ isActive }) =>
                `${rowBase} glim-hue glim-hue-icon ${isActive ? "bg-accent text-accentContrast glim-active" : "text-carbon-text"}`
              }
              style={hueVars(hueOffset + i) as CSSProperties}
            >
              {/* 20px glyph; the rail's glim-nav-row sizing (the generated
                  glyphs come out at 16px and are scaled up here, exactly as
                  index.css scales them for the Sidebar). */}
              <span className="flex h-5 w-5 items-center justify-center [&_svg]:h-5 [&_svg]:w-5">
                <Icon />
              </span>
              <span className="min-w-0 truncate">{t(d.labelKey)}</span>
              {d.to === "/anomalies" && loudAnomalies.count > 0 && (
                <Badge
                  tone={loudAnomalies.tone}
                  size="small"
                  shape="pill"
                  ariaLabel={t("anomaly.navCountAria").replace("{n}", loudAnomalies.count.toLocaleString())}
                  className="ms-auto shrink-0"
                >
                  {loudAnomalies.count}
                </Badge>
              )}
            </NavLink>
          );
        })}
        {/* Sign-out is the last row of the sheet, set off by spacing alone
            (the sheets carry no lines). Muted text token, visually quiet next
            to the rows above it, exactly as the desktop footer's row reads.
            The door glyph is the rail's; the power symbol means shutting a VM
            down. */}
        {authEnabled && (
          <div className="mt-2 pt-2">
            <button
              type="button"
              onClick={() => void signOut()}
              className={`${rowBase} w-full glim-hue glim-hue-icon text-carbon-textMuted`}
              style={hueVars(signOutHue) as CSSProperties}
            >
              <span className="flex h-5 w-5 items-center justify-center [&_svg]:h-5 [&_svg]:w-5">
                <IconSignOut />
              </span>
              <span className="min-w-0 truncate">{t("auth.logout")}</span>
            </button>
          </div>
        )}
      </div>
    </BottomSheet>
  );
}
