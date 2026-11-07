// The browser as a source for the device's colour: the CSS system colour
// AccentColor, read off a probe element. Inside the Android app the phone
// hands the colour over instead (appDevice.ts).
//
// The limits are all the browser's. Firefox and Safari resolve AccentColor to
// the system's colour. Chromium resolves it to a fixed colour of its own,
// whatever the system uses, so Chrome, Edge, Opera and every other Chromium
// browser count as no source. Where the system has no accent of its own, as on
// Linux without a theme that names one or on iOS, the browser answers with its
// default, and Firefox with resistFingerprinting answers with a fixed colour
// too, which nothing on the page can tell. No event fires when the system's
// colour changes, so it is read again when the window comes back to the front,
// which is when somebody returns from the system's settings.

/** Whether AccentColor carries the system's colour in this browser. */
export function browserAccentWorks(): boolean {
  if (typeof CSS === "undefined" || !CSS.supports("color", "AccentColor")) return false;
  // No word boundary, so a headless Chrome's `HeadlessChrome/` counts as well.
  return !/Chrom(e|ium)\//.test(navigator.userAgent);
}

/** The system's colour as the browser reports it, or undefined where it is no source. */
export function browserAccent(): string | undefined {
  if (!browserAccentWorks()) return undefined;
  const probe = document.createElement("span");
  probe.style.color = "AccentColor";
  probe.style.display = "none";
  document.documentElement.append(probe);
  const colour = getComputedStyle(probe).color;
  probe.remove();
  return cssColourToHex(colour);
}

/** A computed `rgb()` or `rgba()` colour as `#RRGGBB`, or undefined for anything else. */
export function cssColourToHex(colour: string): string | undefined {
  const m = colour.match(/^rgba?\(\s*(\d+)[\s,]+(\d+)[\s,]+(\d+)/);
  if (!m) return undefined;
  return "#" + m.slice(1, 4).map((v) => Number(v).toString(16).padStart(2, "0")).join("").toUpperCase();
}

/**
 * Calls `onChange` with the browser's colour now and whenever it has changed
 * on the window's return. Returns the function that stops watching.
 */
export function watchBrowserAccent(onChange: (hex: string | undefined) => void): () => void {
  let last = browserAccent();
  onChange(last);
  const check = () => {
    if (document.visibilityState === "hidden") return;
    const next = browserAccent();
    if (next === last) return;
    last = next;
    onChange(next);
  };
  window.addEventListener("focus", check);
  document.addEventListener("visibilitychange", check);
  return () => {
    window.removeEventListener("focus", check);
    document.removeEventListener("visibilitychange", check);
  };
}
