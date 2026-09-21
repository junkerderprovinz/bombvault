// The look of the interface lives on the server, so clearing a browser does not
// lose it. localStorage stays the cache every axis reads, because those reads
// happen synchronously before first paint and keep the app from flashing the
// default theme. A cleared browser renders defaults for a moment and then picks
// up the stored look.

/** Every localStorage key that describes how the interface looks. List filters
 *  and sort orders (bv-containers-sort and friends) belong to one page, and
 *  syncing them would move someone else's filter under your cursor; the
 *  password-visibility toggle is per session. */
const KEYS = [
  "bv-theme",
  "bv-accent",
  "bv-accent-presets",
  "bv-rainbow",
  "bv-disco",
  "bv-motion",
  "bv-shape",
  "bv-labels-buttons",
  "bv-labels-sidebar",
  "bv-labels-tabs",
  "bv-labels-bottombar",
  "bv-lang",
  "bombvault.advanced",
  // Whether a Fleet peer card shows its scorecard. Unlike a filter, losing it on
  // a new browser is as annoying as losing the theme.
  "bombvault.fleetDetailsOpen",
] as const;

/** Fired on `window` once this browser has adopted the server's look.
 *  main.tsx re-applies the document-element axes, I18nProvider re-reads the
 *  language and AdvancedProvider the advanced view, which covers every key in
 *  KEYS. An event rather than a reload: a reload needs a loop guard, and a
 *  session-scoped guard also blocks the reload a restored tab needs. */
export const ADOPTED_EVENT = "bv-display-prefs-adopted";

export type DisplayPrefs = Record<string, string>;

/** collect reads the current look out of localStorage. Absent keys are omitted
 *  rather than sent as empty, so "this browser has never set a theme" stays
 *  distinguishable from "the theme is the empty string". */
export function collect(): DisplayPrefs {
  const out: DisplayPrefs = {};
  for (const k of KEYS) {
    try {
      const v = localStorage.getItem(k);
      if (v !== null) out[k] = v;
    } catch {
      // Storage disabled (private window, blocked site data). Nothing to
      // collect, and nothing to report: the page still works on defaults.
    }
  }
  return out;
}

/** write puts the server's values into localStorage and reports whether that
 *  actually changed anything. Only known keys are written: the column is
 *  written by this client, but a hand-edited or older value must not be able to
 *  put arbitrary keys into a visitor's browser storage. */
function write(prefs: DisplayPrefs): boolean {
  let changed = false;
  for (const k of KEYS) {
    const v = prefs[k];
    if (typeof v !== "string") continue;
    try {
      if (localStorage.getItem(k) !== v) {
        localStorage.setItem(k, v);
        changed = true;
      }
    } catch {
      // See collect().
    }
  }
  return changed;
}

/** save pushes the current look to the server without waiting: a theme toggle
 *  must not block on the network or fail visibly, and the browser has already
 *  applied it. */
export function save(): void {
  const prefs = collect();
  // A tab whose storage was cleared underneath it has nothing worth sending.
  if (Object.keys(prefs).length === 0) return;
  const body = JSON.stringify(prefs);
  void fetch("/api/display-prefs", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body,
  }).catch(() => {
    // Offline or mid-restart. The next change tries again, and the browser
    // cache means nothing is lost in the meantime.
  });
}

/** sync reconciles this browser with the server once, at boot. A differing look
 *  is adopted and announced with ADOPTED_EVENT. An empty server is seeded from
 *  this browser, so the first load after an upgrade keeps the look the user
 *  already had. */
export async function sync(): Promise<void> {
  let res: Response;
  try {
    res = await fetch("/api/display-prefs");
  } catch {
    return; // Offline: the cache stays the look.
  }
  if (!res.ok) return;
  let body: { ok?: boolean; prefs?: DisplayPrefs; stored?: boolean };
  try {
    body = (await res.json()) as typeof body;
  } catch {
    return;
  }
  if (body.ok === false) return;

  if (!body.stored || !body.prefs || Object.keys(body.prefs).length === 0) {
    save(); // Nothing up there yet: this browser becomes the starting point.
    return;
  }

  if (!write(body.prefs)) return; // Already in agreement.

  // The axes read localStorage only at boot, so a booted page has to be told.
  window.dispatchEvent(new Event(ADOPTED_EVENT));
}
