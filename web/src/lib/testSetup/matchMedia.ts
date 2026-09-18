// ---------------------------------------------------------------------------
// VITEST SETUP — guarded desktop-default window.matchMedia stub.
//
// jsdom does not implement window.matchMedia, and lib/theme.ts's
// getResolvedTheme()/onSystemThemeChange() — plus lib/useMediaQuery.ts during
// the Layout render — call it the moment the component they serve mounts.
// This module is wired via test.setupFiles in vitest.config.ts and landed in
// the SAME change as useMediaQuery (the coupling is deliberate): the first
// suite that renders the real Layout in jsdom (app/Layout.displayPrefs
// .dom.test.tsx) would otherwise throw "window.matchMedia is not a function"
// the moment the chrome switch hooks in.
//
// GUARD: setupFiles execute for node-env suites too, where window itself is
// undefined (see ConfirmDialog.tsx's header for the repo's own statement of
// that fact) — and per-file stubs that need to CONTROL what the OS "prefers"
// (e.g. Settings.themeCard.dom.test.tsx's stubMatchMedia) must keep winning.
// So the stub installs only when window exists AND matchMedia is not already
// a function; in every other case it is a no-op.
//
// DESKTOP DEFAULT: min-width queries answer matches=true, everything else
// (e.g. prefers-color-scheme) false — the same desktop-default answer
// useIsDesktop's windowless snapshot gives, so the existing jsdom suites keep
// asserting the desktop layout they were written against.
// ---------------------------------------------------------------------------
type ChangeListener = (event: MediaQueryListEvent) => void;

function stubbedMatchMedia(query: string): MediaQueryList {
  const listeners = new Set<ChangeListener>();
  const add = (cb: ChangeListener) => {
    listeners.add(cb);
  };
  const remove = (cb: ChangeListener) => {
    listeners.delete(cb);
  };
  const mql = {
    media: query,
    matches: /\bmin-width\s*:/.test(query),
    onchange: null as MediaQueryList["onchange"],
    addEventListener: (_type: "change", cb: ChangeListener) => {
      add(cb);
    },
    removeEventListener: (_type: "change", cb: ChangeListener) => {
      remove(cb);
    },
    // Safari < 14 fallback — routed through the SAME listener set so both
    // APIs stay consistent, mirroring the shape theme.ts's
    // onSystemThemeChange uses.
    addListener: (cb: ChangeListener) => {
      add(cb);
    },
    removeListener: (cb: ChangeListener) => {
      remove(cb);
    },
    dispatchEvent: () => false,
  };
  return mql as unknown as MediaQueryList;
}

if (typeof window !== "undefined" && typeof window.matchMedia !== "function") {
  window.matchMedia = stubbedMatchMedia;
}
