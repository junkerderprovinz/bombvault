import { Outlet, useLocation } from "react-router-dom";
import { Sidebar } from "../components/Sidebar";
import { BottomNav } from "../components/mobile/BottomNav";
import { useEffect, useRef, useState, useCallback } from "react";
import { getSettings, getAuth, getHealth, type Settings } from "../lib/api";
import { useIsDesktop } from "../lib/useMediaQuery";
import { LoginPage } from "../pages/Login";
import { WhatsNewDialog } from "../components/WhatsNewDialog";
import { sync as syncDisplayPrefs } from "../lib/displayPrefs";

// The last BombVault version this browser has seen. The "What's new" dialog
// opens once when the running version differs.
const LAST_SEEN_VERSION_KEY = "bombvault.lastSeenVersion";

// releaseTag reduces a build version such as "v5.0.0+main.fcc0544" to the
// release tag "v5.0.0". The release notes are published under that tag, and
// comparing tags keeps a :latest rebuild with a new short SHA from reopening
// the dialog. It returns null for "dev", "0.0.0" and anything without an x.y.z
// core.
function releaseTag(version: string): string | null {
  const m = version.match(/\d+\.\d+\.\d+/);
  if (!m || m[0] === "0.0.0") return null;
  return `v${m[0]}`;
}

// "blocked" means a password is set and this browser is not signed in.
type AuthGateState = "loading" | "pass" | "blocked";

export function Layout() {
  const [settings, setSettings] = useState<Settings | null>(null);
  const [authGate, setAuthGate] = useState<AuthGateState>("loading");
  // Whether a password is set at all, which decides if the sidebar offers a
  // sign-out row. authGate only says whether this browser may enter.
  const [authEnabled, setAuthEnabled] = useState(false);
  const [whatsNewVersion, setWhatsNewVersion] = useState<string | null>(null);
  const location = useLocation();
  // the one chrome switch: at/above Tailwind's md breakpoint the
  // desktop shell renders exactly as it always has; below it the mobile shell
  // renders in its place. The breakpoint literal lives only in
  // lib/useMediaQuery.ts, and this hook call is its only consumer; a second
  // js breakpoint anywhere else would let the two chrome systems disagree for
  // the 1px window where their answers differ.
  const isDesktop = useIsDesktop();

  // Tap-on-active: tapping the already-active destination scrolls
  // the main scroller back to the top instead of navigating. The scroller is
  // this component's <main id="bv-main"> below, so the mechanism lives here
  // and the chrome surfaces receive it as a prop; they never query the DOM
  // for the scroller themselves. `?.` guards the guarded: the element exists
  // in every state this can be called from, but a callback that throws because
  // of a render-order surprise is worse than a no-op.
  const scrollMainToTop = useCallback(() => {
    document.getElementById("bv-main")?.scrollTo({ top: 0 });
  }, []);

  // Ref on the shell root: the keyboard mechanism's focus listeners attach
  // here (not document-wide), so they exist only while a shell is rendered.
  const shellRef = useRef<HTMLDivElement>(null);

  // The keyboard mechanism; one listener set at Layout level:
  // a per-component listener would multiply with every new input-bearing page
  // and drift exactly the way duplicated logic does.
  //
  // Why it exists at all: index.html's `interactive-widget=resizes-content`
  // viewport meta makes the layout viewport shrink when the Android keyboard
  // opens, and the h-dvh root tracks that shrinkage; but a focused field can
  // still end up above the shrunken scroller's visible area if it sat below
  // the fold before the keyboard opened. This mechanism guarantees the field
  // the user is typing into stays visible: when the visual viewport resizes
  // while a text field inside the shell holds focus, that field is scrolled
  // back into view within the bv-main scroller (block "nearest"; never a
  // jump, only the minimal scroll that reveals it), deferred to the next
  // frame so the resize has settled and the browser's own scroll anchoring
  // has had its pass first.
  //
  // The mechanism only ever scrolls (no layout mutation, no focus stealing;
  // the failure class it could add is an availability one, and scrolling is
  // the gentlest possible response). The visualViewport presence guard is the
  // jsdom discipline (lib/testSetup/matchMedia.ts): environments without the
  // API; jsdom, old browsers; no-op the whole mechanism.
  //
  // Attachment discipline: active only on the mobile branch (desktop never
  // pays the cost, and a desktop resize is a window resize the browser
  // already handles), and only while the shell actually renders; hence the
  // authGate dependency, because blocked/loading render no shell root at all.
  // Every listener added here is removed in the cleanup, so a branch switch
  // or unmount tears the whole set down (asserted by mobileShellSource.test).
  useEffect(() => {
    if (isDesktop || authGate !== "pass") return;
    const root = shellRef.current;
    if (!root) return;
    // The field currently holding focus, or null. focusout carries
    // relatedTarget (where focus is going), so a hop between two fields
    // never blips the tracking through null.
    let focusedField: HTMLElement | null = null;
    const isTextField = (el: EventTarget | null): el is HTMLElement => {
      if (!(el instanceof HTMLElement)) return false;
      if (el.isContentEditable) return true;
      if (el.tagName === "TEXTAREA") return true;
      if (el.tagName === "INPUT") {
        // Buttons and pickers open no keyboard; tracking them would scroll
        // on pure UI clicks.
        const type = (el as HTMLInputElement).type;
        return type !== "checkbox" && type !== "radio" && type !== "button" && type !== "submit" && type !== "file";
      }
      return false;
    };
    const onFocusIn = (e: FocusEvent) => {
      if (isTextField(e.target)) focusedField = e.target;
    };
    const onFocusOut = (e: FocusEvent) => {
      if (isTextField(e.relatedTarget)) return;
      focusedField = null;
    };
    // The guard is the contract: without visualViewport there is no resize
    // signal to listen to, and synthesizing one (polling innerHeight) is the
    // kind of cleverness that fires spuriously on desktop-browser chrome.
    if (typeof window === "undefined" || typeof window.visualViewport === "undefined") return;
    const viewport = window.visualViewport;
    // TS's lib types keep `| null` after the typeof guard, and some engines
    // report the property as null rather than absent; the bail-out covers
    // both shapes of "no visual viewport here".
    if (!viewport) return;
    const onViewportResize = () => {
      if (!focusedField) return;
      // Next frame: the resize event lands before the layout viewport has
      // settled; scrolling in the same tick measures a stale box.
      requestAnimationFrame(() => {
        focusedField?.scrollIntoView({ block: "nearest" });
      });
    };
    root.addEventListener("focusin", onFocusIn);
    root.addEventListener("focusout", onFocusOut);
    viewport.addEventListener("resize", onViewportResize);
    return () => {
      root.removeEventListener("focusin", onFocusIn);
      root.removeEventListener("focusout", onFocusOut);
      viewport.removeEventListener("resize", onViewportResize);
    };
  }, [isDesktop, authGate]);

  // Runs on mount and again after a successful login.
  const checkAuth = useCallback(() => {
    getAuth()
      .then((res) => {
        setAuthEnabled(res.enabled);
        if (res.enabled && !res.authed) {
          setAuthGate("blocked");
        } else {
          setAuthGate("pass");
        }
      })
      .catch(() => {
        // A failed probe lets the app through instead of stranding it on the
        // login screen; the server still enforces auth on every call.
        setAuthGate("pass");
      });
  }, []);

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  // Settings decide which domain tabs the sidebar shows.
  const loadSettings = useCallback(() => {
    getSettings()
      .then((res) => {
        if (res.ok) setSettings(res.settings);
      })
      .catch(() => {
        // Without settings the sidebar only leaves out the VMs and Flash tabs.
      });
  }, []);

  useEffect(() => {
    if (authGate !== "pass") return;
    loadSettings();
  }, [authGate, loadSettings]);

  // Reconcile the look with the server whenever the gate opens. On a
  // password-protected instance the boot-time sync in main.tsx runs before the
  // login and gets a 401, and signing in does not reload the page, so without
  // this the browser would never receive the stored settings. Doing it on every
  // "pass" costs one idempotent GET and keeps this effect from having to know
  // why the gate opened.
  useEffect(() => {
    if (authGate !== "pass") return;
    void syncDisplayPrefs();
  }, [authGate]);

  // Refresh when settings change elsewhere, such as enabling a domain on the
  // Settings page, so the new tab appears without a reload.
  useEffect(() => {
    const onChange = () => loadSettings();
    window.addEventListener("bv:settings-changed", onChange);
    return () => window.removeEventListener("bv:settings-changed", onChange);
  }, [loadSettings]);

  // Show "What's new" when the running release differs from the last one this
  // browser saw. A browser's first visit only records the version. The stored
  // value is updated as soon as the dialog is shown, so it opens once per
  // release.
  useEffect(() => {
    if (authGate !== "pass") return;
    let active = true;
    getHealth()
      .then((h) => {
        if (!active) return;
        const tag = h.version ? releaseTag(h.version) : null;
        if (!tag) return;
        let last: string | null;
        try {
          last = localStorage.getItem(LAST_SEEN_VERSION_KEY);
        } catch {
          /* no localStorage, so no dialog */
          return;
        }
        if (last === null) {
          try {
            localStorage.setItem(LAST_SEEN_VERSION_KEY, tag);
          } catch {
            /* ignore */
          }
          return;
        }
        if (last !== tag) {
          try {
            localStorage.setItem(LAST_SEEN_VERSION_KEY, tag);
          } catch {
            /* ignore */
          }
          setWhatsNewVersion(tag);
        }
      })
      .catch(() => {
        /* version is best-effort; no dialog on a failed health probe */
      });
    return () => {
      active = false;
    };
  }, [authGate]);

  // Render nothing until the auth probe answers, so app content never flashes.
  if (authGate === "loading") {
    return null;
  }

  // Auth is on and not authenticated; show the login screen. This branch
  // returns before the shell root below, which is why login never renders any
  // chrome (no Sidebar, no bottom bar): the guarantee is structural, not CSS.
  if (authGate === "blocked") {
    return <LoginPage onLogin={checkAuth} />;
  }

  // The scroller; per-page content (the Outlet subtree) must never know which
  // chrome is mounted around it, so each branch keeps its own `main` contract
  // and both carry the `bv-main` id, the stable scroll target the bottom bar
  // addresses (tap-on-active). A page cannot tell, and a resize across the
  // breakpoint re-attaches to the same id either way.
  const scroller = isDesktop ? (
    <main id="bv-main" className="flex-1 flex flex-col overflow-y-auto min-w-0">
      {/* `main` is the scroll container. It and the route wrapper are flex
          columns so a short page can fill the height and push a footer to the
          bottom (Settings does this with AboutFooter); other pages render at
          their natural height. */}
      {/* The page padding lives inside the scroll container. There is none
          at the bottom, so at the end of a scroll the last card ends level
          with the rail instead of 24px above it. */}
      <div key={location.pathname} className="glim-page-enter flex-1 flex flex-col p-6 pb-0">
        <Outlet />
      </div>
    </main>
  ) : (
    // Mobile: `main` carries the 16px gutter itself and the per-route wrapper
    // stays padding-free. No bottom padding, the same call the desktop branch
    // makes above: a scroller that keeps 16px under the column clamps a
    // sticky action bar 16px short of the bottom bar and leaves a strip of
    // scrolling page visible between the two. With the column ending on the
    // bar, the page's flush end reaches the nav exactly when its scroll does.
    //
    // The side paddings take the safe-area insets over the same 1rem floor,
    // the pattern the bottom bar's card uses: with viewport-fit=cover the
    // layout viewport runs under a landscape phone's side notch, and the
    // scroller is the surface whose content would slide beneath it. max()
    // with the 1rem gutter floor renders identically wherever the inset is
    // 0px, so plain phones and desktops are unchanged.
    //
    // scroll-padding-bottom keeps the sticky action bar out of the resting
    // place a keyboard scroll lands on: without it the browser parks a
    // focused control at the bottom edge, where an opaque bar is painted over
    // it (WCAG 2.2, focus not obscured).
    <main
      id="bv-main"
      className="flex-1 flex flex-col overflow-y-auto p-4 pb-0 scroll-pb-[var(--sticky-action-h)] pl-[max(1rem,var(--safe-area-left))] pr-[max(1rem,var(--safe-area-right))] min-w-0"
    >
      <div key={location.pathname} className="glim-page-enter flex-1 flex flex-col">
        <Outlet />
      </div>
    </main>
  );

  // The gutter sits on the frame around the rail and the content, so one value
  // spaces both from the window edge and from each other. It is GlimStone's
  // --page-gutter (1rem, `p-4`), the same in every app that uses the design
  // language. The content's 1.5rem padding is a separate distance on top.
  //
  // The shell root uses `h-dvh` so it tracks the visual viewport as mobile
  // browser chrome collapses and expands; on desktop dvh equals the viewport
  // height, so the desktop shell renders unchanged. The flex direction is the
  // chrome switch's other half: desktop is the historical row (rail | main)
  // plus the house gutter; mobile stacks the scroller over its normal-flow
  // bottom bar (the bar is a flex sibling of `main`, never a fixed overlay,
  // so the browser reserves its height and the scroller ends above it by
  // construction).
  return (
    // The desktop frame's side paddings take the same safe-area max() the
    // phone main does: the frame is the chrome a landscape phone renders once
    // the width breakpoint matches, and viewport-fit=cover puts its rail and
    // content under the side notch without it. On a real desktop env() reads
    // 0px and max() leaves the 1rem gutter, so the desktop rendering is
    // unchanged. The phone branch keeps its plain flex-col; its main and the
    // bottom bar's card each carry their own insets.
    <div
      ref={shellRef}
      className={`flex h-dvh overflow-hidden bg-carbon-background ${
        isDesktop
          ? "gap-4 p-4 pl-[max(1rem,var(--safe-area-left))] pr-[max(1rem,var(--safe-area-right))]"
          : "flex-col"
      }`}
    >
      {/* the one chrome switch; exactly one chrome surface renders at a time.
          The desktop branch is upstream's desktop tree verbatim (same Sidebar,
          same frame gutter, same wrapper padding); the mobile branch is the
          same scroller with the bottom bar as its flex sibling below it. The
          hidden surface is not rendered at all, never CSS-hidden: a
          display:none Sidebar would still run its subscriptions, dialogs and
          label engine below the breakpoint. What'sNewDialog renders outside
          the switch; it is a fixed-position dialog and belongs to both
          shells. */}
      {isDesktop ? (
        <>
          <Sidebar settings={settings} authEnabled={authEnabled} />
          {scroller}
        </>
      ) : (
        <>
          {scroller}
          <BottomNav
            settings={settings}
            authEnabled={authEnabled}
            scrollMainToTop={scrollMainToTop}
          />
        </>
      )}
      {whatsNewVersion && (
        <WhatsNewDialog version={whatsNewVersion} onClose={() => setWhatsNewVersion(null)} />
      )}
    </div>
  );
}
