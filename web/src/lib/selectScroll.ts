// A picker answers the mouse wheel, whether or not the platform draws it.
//
// Taken from GlimStone's reference/selectScroll.ts, plus enableSelectScrollHere
// at the bottom for this app. Everything above that talks only to the element
// it is given.

/** Attaches the behaviour to one <select>. Idempotent, safe to call twice. */
export function enableSelectScroll(select: HTMLSelectElement): void {
  if (select.dataset["glimScroll"] === "1") return;
  select.dataset["glimScroll"] = "1";

  select.addEventListener(
    "wheel",
    (event) => {
      if (select.disabled || select.options.length < 2) return;
      // Keeps the page from scrolling while the pointer is over the control.
      event.preventDefault();

      const delta = event.deltaY > 0 ? 1 : -1;
      const next = stepIndex(select.options.length, select.selectedIndex, delta);
      if (next === select.selectedIndex) return;

      select.selectedIndex = next;
      // A real "change" event rather than a state write, so every onChange
      // handler picks it up the same way as a click on an <option>.
      select.dispatchEvent(new Event("change", { bubbles: true }));
    },
    { passive: false },
  );
}

/**
 * The same behaviour for a custom picker that replaces a native <select>.
 * Attach it to the trigger, the button that opens the list; it returns the
 * detach. `step` receives 1 for a wheel roll downwards and -1 for one upwards.
 *
 * This is a native listener rather than an onWheel prop because React
 * registers onWheel as a passive listener on its root, where preventDefault
 * does nothing and the page would scroll while the value changes.
 */
export function enableWheelStep(el: HTMLElement, step: (delta: 1 | -1) => void): () => void {
  function onWheel(event: WheelEvent) {
    if (event.deltaY === 0) return;
    event.preventDefault();
    step(event.deltaY > 0 ? 1 : -1);
  }
  el.addEventListener("wheel", onWheel, { passive: false });
  return () => el.removeEventListener("wheel", onWheel);
}

/** Where a wheel notch lands in a list of options: the next index, clamped.
 *  Clamped rather than wrapping, because one notch too many should not land a
 *  value from the other end of the list. */
export function stepIndex(length: number, at: number, delta: 1 | -1): number {
  if (length < 2) return at;
  return Math.min(length - 1, Math.max(0, at + delta));
}

/**
 * One delegated listener on the document for every <select>, instead of one
 * per element. Selects here come and go with routes and disclosures, so a
 * boot-time sweep would miss most of them, and a MutationObserver is a lot of
 * machinery for a wheel. The listener does nothing unless the pointer is over a
 * <select>, so the rest of the page keeps its own scrolling.
 *
 * Returns the detach, so an effect can hand it straight back.
 */
export function enableSelectScrollHere(root: Document = document): () => void {
  function onWheel(event: WheelEvent) {
    const target = event.target as Element | null;
    const select = target?.closest?.("select") as HTMLSelectElement | null;
    if (!select || select.disabled || select.options.length < 2) return;
    event.preventDefault();

    const delta = event.deltaY > 0 ? 1 : -1;
    const next = stepIndex(select.options.length, select.selectedIndex, delta);
    if (next === select.selectedIndex) return;

    select.selectedIndex = next;
    select.dispatchEvent(new Event("change", { bubbles: true }));
  }
  root.addEventListener("wheel", onWheel, { passive: false });
  return () => root.removeEventListener("wheel", onWheel);
}
