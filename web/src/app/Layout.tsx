import { Outlet, useLocation } from "react-router-dom";
import { Sidebar } from "../components/Sidebar";
import { useEffect, useState, useCallback } from "react";
import { getSettings, getAuth, getHealth, type Settings } from "../lib/api";
import { LoginPage } from "../pages/Login";
import { WhatsNewDialog } from "../components/WhatsNewDialog";
import { AnomalyProvider } from "../lib/useAnomalies";
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

  if (authGate === "blocked") {
    return <LoginPage onLogin={checkAuth} />;
  }

  // The gutter sits on the frame around the rail and the content, so one value
  // spaces both from the window edge and from each other. It is GlimStone's
  // --page-gutter (1rem, `p-4`), the same in every app that uses the design
  // language. The content's 1.5rem padding is a separate distance on top.
  // Inside the gate, so a browser that may not enter polls nothing. The rail's
  // count and the dashboard card read the same summary from here.
  return (
    <AnomalyProvider>
      <div className="flex h-screen overflow-hidden bg-carbon-background gap-4 p-4">
        <Sidebar settings={settings} authEnabled={authEnabled} />
        {/* `main` is the scroll container. It and the route wrapper are flex
            columns so a short page can fill the height and push a footer to the
            bottom (Settings does this with AboutFooter); other pages render at
            their natural height. */}
        <main className="flex-1 flex flex-col overflow-y-auto min-w-0">
          {/* The page padding lives inside the scroll container. There is none
              at the bottom, so at the end of a scroll the last card ends level
              with the rail instead of 24px above it. */}
          <div key={location.pathname} className="glim-page-enter flex-1 flex flex-col p-6 pb-0">
            <Outlet />
          </div>
        </main>
        {whatsNewVersion && (
          <WhatsNewDialog version={whatsNewVersion} onClose={() => setWhatsNewVersion(null)} />
        )}
      </div>
    </AnomalyProvider>
  );
}
