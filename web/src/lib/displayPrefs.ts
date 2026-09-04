// ---------------------------------------------------------------------------
// displayPrefs — the look of the interface lives on the SERVER, the browser
// only caches it (issue #191).
//
// Reported by manilx: he clears Firefox's site data to fix an unrelated problem
// with the VM VNC console, and every time he does, BombVault forgets its theme,
// its view mode, its accent and everything else, because all of it lived in
// localStorage. "I don't see the need for this to be individualized per
// browser. How many are meddling with BV anyway."
//
// localStorage stays, and stays the thing every axis actually reads. That is
// deliberate: those modules read it SYNCHRONOUSLY before first paint, which is
// what stops the app flashing the default theme on every load, and no amount of
// server storage can be synchronous. So the split is:
//
//   localStorage  the cache the page renders from, instantly
//   server        the truth, reconciled a moment later
//
// A browser that has been cleared therefore renders defaults for one moment and
// then snaps to the stored look, instead of losing it.
// ---------------------------------------------------------------------------

/** Every localStorage key that describes how the interface LOOKS.
 *
 *  Deliberately not "every bv-* key": list filters and sort orders
 *  (bv-containers-sort and friends) are about what you were doing on one page,
 *  not how the app looks, and syncing those between machines would move someone
 *  else's filter under your cursor. The password-visibility toggle is likewise
 *  a per-session convenience. */
const KEYS = [
  "bv-theme",
  "bv-accent",
  "bv-accent-presets",
  "bv-rainbow",
  "bv-motion",
  "bv-shape",
  "bv-labels-buttons",
  "bv-labels-sidebar",
  "bv-labels-tabs",
  "bv-lang",
  "bombvault.advanced",
  // Whether a Fleet peer card shows its scorecard. It sits on this side of the
  // line and not with the filters: it was asked for in #179 by the same person,
  // for the same reason ("have to open it every time"), and losing it on a new
  // browser is the same annoyance as losing the theme. It is also the setting
  // he had just changed when the look went missing again in #191.
  "bombvault.fleetDetailsOpen",
] as const;

/** Fired on `window` once this browser has adopted the server's look.
 *
 *  Everything that reads localStorage ONCE has to listen: main.tsx re-applies
 *  the axes that live on the document element, I18nProvider re-reads the
 *  language, AdvancedProvider the advanced view. Between them they cover every
 *  key in KEYS, which is what makes the reload below unnecessary.
 *
 *  This replaces a `location.reload()` and the session-scoped guard around it.
 *  The guard existed so a value the server keeps returning and the browser
 *  keeps rejecting could not reload forever — but it also blocked the ONE
 *  legitimate reload whenever sessionStorage already carried it, which is the
 *  case for a restored tab and for a tab that was open while its site data was
 *  cleared. The page then sat on defaults with the correct values already in
 *  localStorage: white background, English, simple view, exactly as reported
 *  in #191, and only a manual reload fixed it. An event has no such failure
 *  mode: nothing to loop, nothing to suppress. */
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

/** save pushes the current look to the server. Fire-and-forget on purpose: a
 *  theme toggle must not wait on the network or fail visibly if the server is
 *  briefly unreachable, and the browser has already applied it. */
export function save(): void {
  const prefs = collect();
  // A browser with nothing to say says nothing. The server merges what it is
  // given, so an empty object is already harmless there, but this is the half
  // that also covers a page still running in a tab whose storage was cleared
  // underneath it: it would otherwise announce its emptiness on the next
  // change, and it has nothing anyone wants (issue #191).
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

/** sync reconciles this browser with the server, once, at boot.
 *
 *  Three cases, and the third is the one that matters on upgrade:
 *    - the server has a look and it matches → nothing happens
 *    - the server has a look and it differs → adopt it and announce it, so
 *      every axis follows, including the two React holds
 *    - the server has NO look yet → seed it from this browser, so the first
 *      load after upgrading keeps what the user already had instead of
 *      resetting them to factory settings
 */
export async function sync(): Promise<void> {
  let res: Response;
  try {
    res = await fetch("/api/display-prefs");
  } catch {
    return; // Offline: the cache is the look, which is exactly the old behaviour.
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

  // The values are in localStorage now, and every axis reads them from there —
  // but only ever ONCE, at boot, which is why writing them is not enough on a
  // page that has already booted. Saying so out loud is: main.tsx re-applies
  // the document-element axes, and the two providers re-read theirs.
  window.dispatchEvent(new Event(ADOPTED_EVENT));
}
