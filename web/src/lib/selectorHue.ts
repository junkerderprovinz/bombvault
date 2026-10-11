// Under rainbow every selector that sets a value wears a colour of its own on
// its chosen segment. The selectors of a page count through the palette in the
// order they stand, so four selectors under each other show four colours.
//
// A selector cannot know its place from its props: it renders inside cards,
// rows and windows that know nothing of each other. So each one hands its track
// to this module, which sorts the tracks of one page by where they stand in the
// document and sets each its palette position. The count is the same on every
// render of the same page, and a window counts its own selectors from the
// start, so opening one moves no colour on the page behind it.

import { hueVars } from "./appearance";

const tracks = new Set<HTMLElement>();
let queued = false;

/** The page or the window a selector counts in. */
function scopeOf(track: HTMLElement): Element {
  return track.closest('[role="dialog"], [role="alertdialog"]') ?? track.closest("#bv-main") ?? document.body;
}

function recount(): void {
  queued = false;
  const scopes = new Map<Element, HTMLElement[]>();
  for (const track of tracks) {
    if (!track.isConnected) continue;
    const scope = scopeOf(track);
    scopes.set(scope, [...(scopes.get(scope) ?? []), track]);
  }
  for (const list of scopes.values()) {
    list.sort((a, b) => (a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING ? -1 : 1));
    list.forEach((track, i) => {
      for (const [name, value] of Object.entries(hueVars(i))) track.style.setProperty(name, value);
      track.dataset.hue = String(i);
    });
  }
}

function queue(): void {
  if (queued) return;
  queued = true;
  // Before the next paint, after every selector of this render has joined.
  queueMicrotask(recount);
}

/**
 * countSelector puts a selector's track into its page's count and returns the
 * function that takes it out again, which recounts the rest.
 */
export function countSelector(track: HTMLElement): () => void {
  tracks.add(track);
  queue();
  return () => {
    tracks.delete(track);
    queue();
  };
}
