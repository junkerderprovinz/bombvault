// The settings search bar appears above the first card only when somebody
// scrolls up for it, and changing settings page puts it away again.
//
// It listens for wheel as well as scroll because at the top of the page the
// browser fires no scroll event; scroll still catches a scrollbar drag, a
// swipe and Page Up, which fire no wheel event.
import { useEffect, useLayoutEffect, useRef, useState } from "react";

/** How much movement counts as a direction rather than as settling noise. */
const DELTA = 6;

/** A page change scrolls the page itself (to the top, an anchor or a search
 *  hit); scroll events this soon after one are not the reader's. */
const QUIET_MS = 400;

/**
 * @param resetKey  changes with the settings page; the bar goes away.
 * @param pinned    true while the bar is in use, so typing in it cannot make
 *                  it vanish under the hand using it.
 */
export function useRevealOnScrollUp(resetKey: string, pinned: boolean) {
  const [revealed, setRevealed] = useState(false);
  const barRef = useRef<HTMLDivElement>(null);
  const lastTop = useRef(0);
  const quietUntil = useRef(0);
  /** Set while a reveal still has to be paid for by the scroll compensation. */
  const owed = useRef(false);

  useEffect(() => {
    setRevealed(false);
    quietUntil.current = performance.now() + QUIET_MS;
  }, [resetKey]);

  useEffect(() => {
    if (pinned) return;
    const main = document.getElementById("bv-main");
    if (!main) return;
    lastTop.current = main.scrollTop;

    const up = () =>
      setRevealed((was) => {
        // Only a reveal from nothing moves content, so only that one is owed.
        if (!was) owed.current = true;
        return true;
      });
    const onWheel = (e: WheelEvent) => {
      if (e.deltaY < -DELTA) up();
      else if (e.deltaY > DELTA) setRevealed(false);
    };
    const onScroll = () => {
      const y = main.scrollTop;
      const dy = y - lastTop.current;
      lastTop.current = y;
      if (performance.now() < quietUntil.current || Math.abs(dy) < DELTA) return;
      if (dy < 0) up();
      else setRevealed(false);
    };

    main.addEventListener("wheel", onWheel, { passive: true });
    main.addEventListener("scroll", onScroll, { passive: true });
    return () => {
      main.removeEventListener("wheel", onWheel);
      main.removeEventListener("scroll", onScroll);
    };
  }, [pinned, resetKey]);

  // The bar enters the flow above the cards and would push them down
  // mid-gesture, so its height is added to scrollTop before the browser
  // paints. Nothing is owed at the top of the page.
  useLayoutEffect(() => {
    if (!revealed || !owed.current) return;
    owed.current = false;
    const main = document.getElementById("bv-main");
    const h = barRef.current?.offsetHeight ?? 0;
    if (main && h > 0 && main.scrollTop > 0) {
      main.scrollTop += h;
      lastTop.current = main.scrollTop;
    }
  }, [revealed]);

  return { revealed, barRef };
}
