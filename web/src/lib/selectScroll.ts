// Rule 14's mouse-wheel addendum: a picker answers the wheel, whether or not
// the platform is the one drawing it.
//
// A copy of GlimStone's reference/selectScroll.ts, plus the one adaptation this
// app needs (see enableSelectScrollHere at the bottom). Framework-free above
// that line: it talks only to the element it is given.

/** Attaches the behaviour to one <select>. Idempotent, safe to call twice. */
export function enableSelectScroll(select: HTMLSelectElement): void {
  if (select.dataset["glimScroll"] === "1") return;
  select.dataset["glimScroll"] = "1";

  select.addEventListener(
    "wheel",
    (event) => {
      if (select.disabled || select.options.length < 2) return;
      // Prevents the page itself from scrolling while the pointer sits over the
      // control: this handler is the scroll, not a bystander to it.
      event.preventDefault();

      const delta = event.deltaY > 0 ? 1 : -1;
      const next = stepIndex(select.options.length, select.selectedIndex, delta);
      if (next === select.selectedIndex) return;

      select.selectedIndex = next;
      // A real "change" event, not a manual state write: every existing
      // onChange call site picks this up for free, the same way a click on an
      // <option> already would.
      select.dispatchEvent(new Event("change", { bubbles: true }));
    },
    { passive: false },
  );
}

/**
 * The same promise, for a picker that is NOT a native <select>.
 *
 * Rule 18 says a native control gets replaced rather than persuaded, and an app
 * that follows it ends up with no <select> left for the function above to
 * reach. The behaviour must not be lost on the way ("Dropdownlisten soll man
 * überall auch per scrollen umschalten können"), so the wheel belongs to the
 * PICKER rather than to the element the platform happens to draw.
 *
 * Attach it to the trigger, the button that opens the list, and it returns the
 * detach. `step` receives 1 for a wheel roll downwards and -1 for one upwards.
 *
 * WHY THIS IS A LISTENER AND NOT AN onWheel PROP, which is the part worth
 * knowing before somebody simplifies it away: React registers `onWheel` as a
 * PASSIVE listener on its root, so `preventDefault` inside such a handler does
 * nothing but log a warning. The page would scroll while the value changed,
 * which is the one behaviour this exists to avoid.
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
 * This app's adaptation: ONE delegated listener instead of one per element.
 *
 * The reference attaches to each <select> it is handed, which suits a page that
 * renders its controls once. Here the selects come and go with every route and
 * every disclosure, so a boot-time sweep would cover the ones that happened to
 * exist at boot and silently miss the rest, and a MutationObserver to keep up
 * with React is a lot of machinery for a wheel.
 *
 * A single non-passive listener on the document does the same job for every
 * select, now and later. It stays inert unless the pointer is actually over a
 * <select>, so nothing else on the page loses its own scrolling.
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
