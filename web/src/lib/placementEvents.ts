const PLACEMENT_CHANGED = "bv:placement-changed";

/** placementChanged announces a written default, rule or pause, so every card
 *  and the defaults card read theirs again. After a successful write only. */
export function placementChanged(): void {
  window.dispatchEvent(new Event(PLACEMENT_CHANGED));
}

export function subscribePlacement(onChange: () => void): () => void {
  window.addEventListener(PLACEMENT_CHANGED, onChange);
  return () => window.removeEventListener(PLACEMENT_CHANGED, onChange);
}
