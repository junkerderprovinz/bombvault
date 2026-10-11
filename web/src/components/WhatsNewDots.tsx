// The green dots that mark what is new after an update: one on every changed
// element in view, and one on each link, row and tab that leads to a change
// elsewhere. They lie in a layer over the page and are placed from where the
// marked elements stand, so a page only names its elements and draws nothing.

import { useContext, useEffect, useMemo, useState } from "react";
import { createPortal } from "react-dom";
import { matchPath, useLocation } from "react-router-dom";

import { AttentionContext } from "../lib/attention";
import { useT } from "../lib/i18n";
import { useIsDesktop } from "../lib/useMediaQuery";
import { useTipBubble } from "../lib/useTipBubble";
import type { Change, WayStep } from "../lib/whatsNew";
import { markSeen, useWhatsNew } from "../lib/whatsNewStore";

interface Spot {
  key: string;
  x: number;
  y: number;
  /** The changes behind it: the one it marks, or those it leads to. */
  changes: Change[];
  /** The link, row or tab a dot on the way stands on. */
  way?: HTMLElement;
}

function shown(el: Element): el is HTMLElement {
  return el instanceof HTMLElement && el.getClientRects().length > 0;
}

function first(selector: string): HTMLElement | undefined {
  return Array.from(document.querySelectorAll(selector)).find(shown);
}

/** The element of a step on the way, where the page shows one. A link in the
 *  navigation goes before one in the page. */
function stepElement(step: WayStep): HTMLElement | undefined {
  if (typeof step !== "string") {
    return first(`[data-new-way="${step.within}"] [data-sel-id="${step.tab}"]`);
  }
  if (!step.startsWith("/")) return first(`[data-new-way="${step}"]`);
  return first(`nav a[href="${step}"]`) ?? first(`a[href="${step}"]`);
}

function find(unseen: Change[], pathname: string): Spot[] {
  // A window over the page hides the marks of everything behind it.
  const windows = document.querySelectorAll('[aria-modal="true"]');
  const front = windows[windows.length - 1];
  const scroller = document.getElementById("bv-main");
  const view = scroller?.getBoundingClientRect();
  const rtl = document.documentElement.dir === "rtl";

  const place = (el: HTMLElement): { x: number; y: number } | null => {
    if (front && !front.contains(el)) return null;
    const box = el.getBoundingClientRect();
    const [x, y] = [rtl ? box.left + 4 : box.right - 4, box.top + 4];
    if (view && scroller?.contains(el) && (y < view.top || y > view.bottom)) return null;
    return { x: Math.round(x), y: Math.round(y) };
  };

  const spots: Spot[] = [];
  const ways = new Map<HTMLElement, Change[]>();
  for (const change of unseen) {
    const el = first(`[data-new="${change.id}"]`);
    if (el) {
      const at = place(el);
      if (at) spots.push({ key: change.id, ...at, changes: [change] });
      continue;
    }
    const onPage = matchPath(change.page, pathname) !== null;
    let led = onPage;
    for (const step of change.way) {
      const link = typeof step === "string" && step.startsWith("/");
      if (link && (onPage || step === pathname)) continue;
      const on = stepElement(step);
      if (!on) continue;
      led = true;
      ways.set(on, [...(ways.get(on) ?? []), change]);
    }
    // A phone keeps the entries its bar has no room for behind More.
    const more = led ? undefined : first('[data-new-way="more"]');
    if (more) ways.set(more, [...(ways.get(more) ?? []), change]);
  }
  for (const [way, changes] of ways) {
    const at = place(way);
    if (at) spots.push({ key: `way:${changes.map((c) => c.id).join(",")}:${at.x}:${at.y}`, ...at, changes, way });
  }
  return spots;
}

function same(a: Spot[], b: Spot[]): boolean {
  return a.length === b.length && a.every((spot, i) => spot.key === b[i].key && spot.x === b[i].x && spot.y === b[i].y && spot.way === b[i].way);
}

function Dot({ spot }: { spot: Spot }) {
  const { t } = useT();
  const sentences = spot.changes.map((change) => t(change.text));
  const tip = spot.way ? `${t("whatsNew.way", sentences.length)} ${sentences.join(" ")}` : sentences[0];
  const bubble = useTipBubble(tip);
  return (
    <>
      <button
        ref={bubble.ref}
        type="button"
        data-new-dot={spot.way ? "way" : spot.changes[0].id}
        aria-label={tip}
        aria-describedby={bubble.describedBy}
        {...bubble.handlers}
        onClick={() => {
          bubble.hide();
          if (spot.way) spot.way.click();
          else markSeen([spot.changes[0].id]);
        }}
        className="glim-new-dot"
        style={{ left: spot.x, top: spot.y }}
      />
      {bubble.bubble}
    </>
  );
}

/**
 * WhatsNewDots draws the dots of the changes nobody has clicked away. It
 * draws none during the first start, where nothing can be new, and none while
 * they are switched off.
 */
export function WhatsNewDots() {
  const { pathname } = useLocation();
  // Whether this is a first start is known once the entries are read.
  const { fresh, items } = useContext(AttentionContext);
  const isDesktop = useIsDesktop();
  const { on, unseen } = useWhatsNew();
  const marked = useMemo(() => (isDesktop ? unseen : unseen.filter((change) => !change.wide)), [isDesktop, unseen]);
  const [spots, setSpots] = useState<Spot[]>([]);
  const active = on && items.loaded && !fresh && marked.length > 0 && pathname !== "/welcome";

  useEffect(() => {
    if (!active) {
      setSpots([]);
      return;
    }
    let frame = 0;
    const look = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => {
        const next = find(marked, pathname);
        setSpots((prev) => (same(prev, next) ? prev : next));
      });
    };
    look();
    // The marked elements come, go and move with the page, which says so in
    // no single place: it changes its tree, scrolls, resizes or ends a move.
    const tree = new MutationObserver((changes) => {
      if (changes.some((change) => !(change.target instanceof Element && change.target.closest("[data-new-layer]")))) look();
    });
    tree.observe(document.body, { childList: true, subtree: true, attributes: true, attributeFilter: ["class", "hidden", "aria-modal"] });
    const moved = ["scroll", "transitionend", "animationend"] as const;
    for (const event of moved) document.addEventListener(event, look, true);
    window.addEventListener("resize", look);
    return () => {
      cancelAnimationFrame(frame);
      tree.disconnect();
      for (const event of moved) document.removeEventListener(event, look, true);
      window.removeEventListener("resize", look);
    };
  }, [active, marked, pathname]);

  if (!active || spots.length === 0) return null;
  return createPortal(
    <div data-new-layer className="glim-new-layer">
      {spots.map((spot) => (
        <Dot key={spot.key} spot={spot} />
      ))}
    </div>,
    document.body
  );
}
