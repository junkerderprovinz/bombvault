// ---------------------------------------------------------------------------
// Action glyphs (#178, [202]) — the symbols buttons wear.
//
// Sidebar.tsx already owns the NAVIGATION and domain glyphs (containers, VMs,
// files, trash, pencil, gear, sync, …) and those are reused verbatim rather
// than redrawn here. This file adds the verbs the app has buttons for and had
// no symbol for: save, cancel, refresh, upload, search, unlock, and so on.
//
// Same recipe as Sidebar's own: a 16x16 viewBox, `fill="currentColor"` so the
// glyph inherits whatever ink the button paints (which is what keeps them
// correct in every theme and in rainbow mode), `shrink-0` so a flex row cannot
// squash them, and `aria-hidden` because the button's label is the accessible
// name — a glyph that announced itself would double every control's name.
//
// Drawn from rectangles, circles and simple paths on the same 16-unit grid, so
// they sit at the same optical weight as the existing set instead of looking
// like a second icon family bolted on.
// ---------------------------------------------------------------------------

const SVG = "shrink-0";

function G({ children }: { children: React.ReactNode }) {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" className={SVG} aria-hidden="true">
      {children}
    </svg>
  );
}

/** Save: a floppy, the same shape the app's own logo quotes. */
export function IconSave() {
  return (
    <G>
      <path d="M2.6 2.2h8.1l2.7 2.7v8.9c0 .3-.3.6-.6.6H2.6a.6.6 0 0 1-.6-.6V2.8c0-.33.27-.6.6-.6Zm2 .9v3.3h5.2V3.1H4.6Zm-.5 6.1v4.3h7.8V9.2H4.1Z" />
    </G>
  );
}

/** Cancel / dismiss: a circle with a slash, distinct from IconClose's plain X
 *  so "cancel this action" does not look like "close this panel". */
export function IconCancel() {
  return (
    <G>
      <path d="M8 1.6a6.4 6.4 0 1 0 0 12.8A6.4 6.4 0 0 0 8 1.6Zm0 1.8c1 0 2 .32 2.8.9l-6.3 6.3A4.6 4.6 0 0 1 8 3.4Zm0 9.2c-1 0-2-.32-2.8-.9l6.3-6.3A4.6 4.6 0 0 1 8 12.6Z" />
    </G>
  );
}

/** Refresh / reload: an open circular arrow. */
export function IconRefresh() {
  return (
    <G>
      <path d="M8 2.6a5.4 5.4 0 0 1 4.75 2.85l1.5-.75A7.1 7.1 0 0 0 8 .9a7.1 7.1 0 0 0-7.1 7.1h1.8A5.3 5.3 0 0 1 8 2.6Z" />
      <path d="M8 13.4a5.4 5.4 0 0 1-4.75-2.85l-1.5.75A7.1 7.1 0 0 0 8 15.1a7.1 7.1 0 0 0 7.1-7.1h-1.8A5.3 5.3 0 0 1 8 13.4Z" />
      <path d="M12.2 3.1h2.6v2.6l-2.6-2.6ZM3.8 12.9H1.2v-2.6l2.6 2.6Z" />
    </G>
  );
}

/** Upload / send: an arrow leaving a tray, the mirror of IconDownload. */
export function IconUpload() {
  return (
    <G>
      <rect x="7" y="5" width="2" height="7" rx="0.6" />
      <path d="M8 1.9 4.6 5.6h6.8L8 1.9Z" />
      <path d="M2.6 12.4h10.8v1.8H2.6z" />
    </G>
  );
}

/** Search / discover: a magnifier. */
export function IconSearch() {
  return (
    <G>
      <path d="M7 1.8a5.2 5.2 0 1 0 3.1 9.37l3 3a.9.9 0 0 0 1.28-1.27l-3-3A5.2 5.2 0 0 0 7 1.8Zm0 1.8a3.4 3.4 0 1 1 0 6.8 3.4 3.4 0 0 1 0-6.8Z" />
    </G>
  );
}

/** Unlock: an open padlock, for clearing a stale lock. */
export function IconUnlock() {
  return (
    <G>
      <path d="M8 1.4a3.5 3.5 0 0 0-3.5 3.5v1.3h1.8V4.9a1.7 1.7 0 1 1 3.4 0v1.3h1.8V4.9A3.5 3.5 0 0 0 8 1.4Z" />
      <rect x="2.8" y="6.9" width="8" height="7.1" rx="1" />
    </G>
  );
}

/** Broom / prune: sweeping space back. */
export function IconPrune() {
  return (
    <G>
      <rect x="7.1" y="1.6" width="1.8" height="6.4" rx="0.6" transform="rotate(35 8 4.8)" />
      <path d="M4.2 8.6h7.6l1.5 5.4a.6.6 0 0 1-.58.76H3.28a.6.6 0 0 1-.58-.76L4.2 8.6Z" />
    </G>
  );
}

/** Play / run now: a triangle. */
export function IconPlay() {
  return (
    <G>
      <path d="M4.4 2.6 13 8l-8.6 5.4V2.6Z" />
    </G>
  );
}

/** Stop: a square. */
export function IconStop() {
  return (
    <G>
      <rect x="3.4" y="3.4" width="9.2" height="9.2" rx="1" />
    </G>
  );
}

/** Back / previous: a left chevron. */
export function IconBack() {
  return (
    <G>
      <path d="M10.4 2.3 4.7 8l5.7 5.7 1.3-1.3L7.3 8l4.4-4.4-1.3-1.3Z" />
    </G>
  );
}

/** Forward / next / continue: a right chevron. */
export function IconForward() {
  return (
    <G>
      <path d="M5.6 2.3 11.3 8l-5.7 5.7-1.3-1.3L8.7 8 4.3 3.6l1.3-1.3Z" />
    </G>
  );
}

/** Select all / apply to every row: a checklist. */
export function IconSelectAll() {
  return (
    <G>
      <rect x="1.6" y="2.4" width="4" height="4" rx="0.8" />
      <rect x="1.6" y="9.2" width="4" height="4" rx="0.8" />
      <rect x="7.2" y="3.5" width="7.2" height="1.8" rx="0.6" />
      <rect x="7.2" y="10.3" width="7.2" height="1.8" rx="0.6" />
    </G>
  );
}

/** Clear selection: a checklist with a slash. */
export function IconClearSelection() {
  return (
    <G>
      <rect x="1.6" y="2.4" width="4" height="4" rx="0.8" />
      <rect x="7.2" y="3.5" width="7.2" height="1.8" rx="0.6" />
      <path d="M2.1 13.6 13.4 2.3l1.1 1.1L3.2 14.7l-1.1-1.1Z" />
    </G>
  );
}

/** Key / credentials. */
export function IconKey() {
  return (
    <G>
      <path d="M10.2 1.8a4.2 4.2 0 0 0-4 5.5L1.7 11.8v2.4h2.4v-1.6h1.6v-1.6h1.6l1.1-1.1a4.2 4.2 0 1 0 1.8-8.1Zm1.1 3.6a1.1 1.1 0 1 1 0-2.2 1.1 1.1 0 0 1 0 2.2Z" />
    </G>
  );
}

/** Link / connect. */
export function IconLink() {
  return (
    <G>
      <path d="M6.4 9.6a2.6 2.6 0 0 1 0-3.7l2.1-2.1a2.6 2.6 0 0 1 3.7 3.7l-1 1-1.3-1.3 1-1a.8.8 0 0 0-1.1-1.1l-2.1 2.1a.8.8 0 0 0 0 1.1L6.4 9.6Z" />
      <path d="M9.6 6.4a2.6 2.6 0 0 1 0 3.7l-2.1 2.1a2.6 2.6 0 0 1-3.7-3.7l1-1 1.3 1.3-1 1a.8.8 0 0 0 1.1 1.1l2.1-2.1a.8.8 0 0 0 0-1.1l1.3-1.3Z" />
    </G>
  );
}

/** Eye / show, reveal, preview. */
export function IconEye() {
  return (
    <G>
      <path d="M8 3.2C4.4 3.2 1.6 8 1.6 8s2.8 4.8 6.4 4.8S14.4 8 14.4 8 11.6 3.2 8 3.2Zm0 7.8A3 3 0 1 1 8 5a3 3 0 0 1 0 6Z" />
      <circle cx="8" cy="8" r="1.4" />
    </G>
  );
}

/** Info / details. */
export function IconInfo() {
  return (
    <G>
      <path d="M8 1.5a6.5 6.5 0 1 0 0 13 6.5 6.5 0 0 0 0-13Zm0 2.6a1.1 1.1 0 1 1 0 2.2 1.1 1.1 0 0 1 0-2.2Zm1.2 8.1H6.8v-1.1h.7V8.4h-.7V7.3h2.4v3.8h.7v1.1Z" />
    </G>
  );
}
