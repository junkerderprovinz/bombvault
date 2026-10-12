// The green dots that mark what is new after an update: one on every changed
// element in view, and one on the link, row or tab that leads furthest towards
// a change elsewhere. They lie in a layer over the page and are placed from
// where the marked elements stand, so a page only names its elements and draws
// nothing.

import { useContext, useEffect, useMemo, useState } from "react";
import { createPortal } from "react-dom";
import { matchPath, useLocation } from "react-router-dom";

import { AttentionContext } from "../lib/attention";
import { useT } from "../lib/i18n";
import { useIsDesktop } from "../lib/useMediaQuery";
import { useTipBubble } from "../lib/useTipBubble";
import type { Change, WayStep } from "../lib/whatsNew";
import { markAbsent, markSeen, useWhatsNew } from "../lib/whatsNewStore";

/** How long a page may take to draw a change or the way to it before the
 *  change counts as absent. A page that draws it later brings it back. */
const SETTLED_MS = 2000;

interface Spot {
  key: string;
  x: number;
  y: number;
  /** The changes behind it: the one it marks, or those it leads to. */
  changes: Change[];
  /** The link, row or tab a dot on the way stands on. */
  way?: HTMLElement;
}

interface Found {
  spots: Spot[];
  /** Of the changes whose page is open, those it draws or leads to. */
  met: string[];
  /** Of the changes whose page is open, those it has nothing for. */
  lacking: string[];
}

function shown(el: Element): el is HTMLElement {
  return el instanceof HTMLElement && el.getClientRects().length > 0;
}

function first(selector: string): HTMLElement | undefined {
  return Array.from(document.querySelectorAll(selector)).find(shown);
}

/** The element of a step on the way, where the page shows one. A link in the
 *  navigation goes before one in the page, and the tab that is open leads
 *  nowhere. */
function stepElement(step: WayStep): HTMLElement | undefined {
  if (typeof step !== "string") {
    return first(`[data-new-way="${step.within}"] [data-sel-id="${step.tab}"]:not([aria-selected="true"])`);
  }
  if (!step.startsWith("/")) return first(`[data-new-way="${step}"]`);
  return first(`nav a[href="${step}"]`) ?? first(`a[href="${step}"]`);
}

/** Whether a box that scrolls or cuts its content keeps the point out of view. */
function cutOff(el: HTMLElement, x: number, y: number): boolean {
  const cuts = (overflow: string) => /auto|scroll|hidden|clip/.test(overflow);
  for (let box = el.parentElement; box; box = box.parentElement) {
    const { overflowX, overflowY } = getComputedStyle(box);
    if (!cuts(overflowX) && !cuts(overflowY)) continue;
    const edge = box.getBoundingClientRect();
    if (cuts(overflowY) && (y < edge.top || y > edge.bottom)) return true;
    if (cuts(overflowX) && (x < edge.left || x > edge.right)) return true;
  }
  return false;
}

/** What a click on a way's dot presses: the way itself, or the link or the
 *  button that a row or a wrapper around it holds. */
function pressed(way: HTMLElement): HTMLElement {
  if (way.matches("a, button")) return way;
  return way.querySelector<HTMLElement>("a[href]") ?? way.querySelector<HTMLElement>("button") ?? way;
}

function find(unseen: Change[], absent: ReadonlySet<string>, pathname: string): Found {
  // A window over the page hides the marks of everything behind it.
  const windows = document.querySelectorAll('[aria-modal="true"]');
  const front = windows[windows.length - 1];
  const rtl = document.documentElement.dir === "rtl";

  const place = (el: HTMLElement): { x: number; y: number } | null => {
    if (front && !front.contains(el)) return null;
    const box = el.getBoundingClientRect();
    const [x, y] = [Math.round(rtl ? box.left + 4 : box.right - 4), Math.round(box.top + 4)];
    return cutOff(el, x, y) ? null : { x, y };
  };

  const found: Found = { spots: [], met: [], lacking: [] };
  const ways = new Map<HTMLElement, Change[]>();
  for (const change of unseen) {
    const onPage = matchPath(change.page, pathname) !== null;
    const el = first(`[data-new="${change.id}"]`);
    if (el) {
      const at = place(el);
      if (at) found.spots.push({ key: change.id, ...at, changes: [change] });
      if (onPage) found.met.push(change.id);
      continue;
    }
    const away = absent.has(change.id);
    if (away && !onPage) continue;
    const steps = change.way
      .filter((step) => typeof step !== "string" || !step.startsWith("/") || (!onPage && step !== pathname))
      .map(stepElement)
      .filter((on) => on !== undefined);
    if (onPage) (steps.length > 0 ? found.met : found.lacking).push(change.id);
    if (away) continue;
    // The step furthest along that is in view carries the dot, and a phone
    // keeps the entries its bar has no room for behind More.
    const way = steps.reverse().find(place) ?? (steps.length === 0 && !onPage ? first('[data-new-way="more"]') : undefined);
    if (way) ways.set(way, [...(ways.get(way) ?? []), change]);
  }
  for (const [way, changes] of ways) {
    const at = place(way);
    if (at) found.spots.push({ key: `way:${changes.map((c) => c.id).join(",")}:${at.x}:${at.y}`, ...at, changes, way });
  }
  return found;
}

function same(a: Spot[], b: Spot[]): boolean {
  return a.length === b.length && a.every((spot, i) => spot.key === b[i].key && spot.x === b[i].x && spot.y === b[i].y && spot.way === b[i].way);
}

function Dot({ spot }: { spot: Spot }) {
  const { t } = useT();
  const sentences = spot.changes.map((change) => t(change.text));
  // A way lists its changes one to a line, so they can be told apart.
  const tip = spot.way ? [t("whatsNew.way", sentences.length), ...sentences.map((sentence) => `• ${sentence}`)].join("\n") : sentences[0];
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
          if (spot.way) pressed(spot.way).click();
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
 * draws none during the first start, where nothing can be new.
 */
export function WhatsNewDots() {
  const { pathname } = useLocation();
  // Whether this is a first start is known once the entries are read.
  const { fresh, items } = useContext(AttentionContext);
  const isDesktop = useIsDesktop();
  const { unseen, absent } = useWhatsNew();
  const marked = useMemo(() => (isDesktop ? unseen : unseen.filter((change) => !change.wide)), [isDesktop, unseen]);
  const [spots, setSpots] = useState<Spot[]>([]);
  const active = items.loaded && !fresh && marked.length > 0 && pathname !== "/welcome";

  useEffect(() => {
    if (!active) {
      setSpots([]);
      return;
    }
    let frame = 0;
    let settle = 0;
    // Since when the open page has had nothing for a change.
    const lackingSince = new Map<string, number>();
    const look = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => {
        const next = find(marked, absent, pathname);
        setSpots((prev) => (same(prev, next.spots) ? prev : next.spots));

        const back = next.met.filter((id) => absent.has(id));
        if (back.length > 0) markAbsent(back, false);

        const now = Date.now();
        for (const id of lackingSince.keys()) if (!next.lacking.includes(id)) lackingSince.delete(id);
        const waiting = next.lacking.filter((id) => !absent.has(id));
        const gone: string[] = [];
        for (const id of waiting) {
          const since = lackingSince.get(id);
          if (since === undefined) lackingSince.set(id, now);
          else if (now - since >= SETTLED_MS) gone.push(id);
        }
        if (gone.length > 0) markAbsent(gone);
        clearTimeout(settle);
        if (waiting.length > gone.length) settle = window.setTimeout(look, SETTLED_MS);
      });
    };
    look();
    // The marked elements come, go and move with the page, which says so in
    // no single place: it changes its tree, scrolls, resizes or ends a move.
    const tree = new MutationObserver((changes) => {
      if (changes.some((change) => !(change.target instanceof Element && change.target.closest("[data-new-layer]")))) look();
    });
    tree.observe(document.body, {
      childList: true,
      subtree: true,
      attributes: true,
      attributeFilter: ["class", "hidden", "aria-modal", "aria-selected"],
    });
    const moved = ["scroll", "transitionend", "animationend"] as const;
    for (const event of moved) document.addEventListener(event, look, true);
    window.addEventListener("resize", look);
    return () => {
      cancelAnimationFrame(frame);
      clearTimeout(settle);
      tree.disconnect();
      for (const event of moved) document.removeEventListener(event, look, true);
      window.removeEventListener("resize", look);
    };
  }, [active, marked, absent, pathname]);

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
