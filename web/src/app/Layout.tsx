import { Outlet, useLocation } from "react-router-dom";
import { Sidebar } from "../components/Sidebar";
import { useEffect, useState, useCallback } from "react";
import { getSettings, getAuth, getHealth, type Settings } from "../lib/api";
import { LoginPage } from "../pages/Login";
import { WhatsNewDialog } from "../components/WhatsNewDialog";

// Per-browser record of the last BombVault version this browser saw. When the
// running version differs, the "What's new" dialog (#48) is shown once.
const LAST_SEEN_VERSION_KEY = "bombvault.lastSeenVersion";

// releaseTag reduces a build version to its GitHub release tag. :latest builds
// carry SemVer build metadata (e.g. "v5.0.0+main.fcc0544", issue #22); both the
// release-notes lookup and the seen-version comparison want the plain tag
// "v5.0.0" — otherwise the dialog fetches a tag that doesn't exist (404) and the
// changing short SHA re-nags on every :latest rebuild (issue #48). Returns null
// for "dev" / "0.0.0" / anything without an x.y.z core, so those never nag.
function releaseTag(version: string): string | null {
  const m = version.match(/\d+\.\d+\.\d+/);
  if (!m || m[0] === "0.0.0") return null;
  return `v${m[0]}`;
}

// Auth probe state: null = not yet fetched, false = auth off or authed,
// true = auth on AND not authed (show login).
type AuthGateState = "loading" | "pass" | "blocked";

export function Layout() {
  const [settings, setSettings] = useState<Settings | null>(null);
  const [authGate, setAuthGate] = useState<AuthGateState>("loading");
  // The version to show the "What's new" dialog for (null = don't show).
  const [whatsNewVersion, setWhatsNewVersion] = useState<string | null>(null);
  const location = useLocation();

  // Check auth state; used on mount and after a successful login.
  const checkAuth = useCallback(() => {
    getAuth()
      .then((res) => {
        if (res.enabled && !res.authed) {
          setAuthGate("blocked");
        } else {
          setAuthGate("pass");
        }
      })
      .catch(() => {
        // If the auth check itself fails (network error, server down) treat as
        // pass so the app doesn't get stuck in a permanent login screen.
        setAuthGate("pass");
      });
  }, []);

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  // Load settings to drive the sidebar's domain tabs.
  const loadSettings = useCallback(() => {
    getSettings()
      .then((res) => {
        if (res.ok) setSettings(res.settings);
      })
      .catch(() => {
        // Non-fatal: sidebar simply won't reveal VMs/Flash tabs.
      });
  }, []);

  // Initial load once auth is cleared.
  useEffect(() => {
    if (authGate !== "pass") return;
    loadSettings();
  }, [authGate, loadSettings]);

  // Live-refresh when settings change elsewhere (e.g. enabling a domain on the
  // Settings page) so a newly-enabled tab appears immediately — no page reload.
  useEffect(() => {
    const onChange = () => loadSettings();
    window.addEventListener("bv:settings-changed", onChange);
    return () => window.removeEventListener("bv:settings-changed", onChange);
  }, [loadSettings]);

  // "What's new" detection (#48): once past the auth gate, compare the running
  // version against the last one this browser saw. Show the dialog when it
  // differs from a previously stored value; on a brand-new browser just record
  // the version silently (don't nag a first-time user). "dev"/unknown builds are
  // ignored. lastSeenVersion is updated the moment we decide to show it, so a
  // new version can never re-nag on the next mount.
  useEffect(() => {
    if (authGate !== "pass") return;
    let active = true;
    getHealth()
      .then((h) => {
        if (!active) return;
        // Compare + store the plain release tag, not the raw build string, so
        // the dialog looks up an existing GitHub tag and :latest's changing
        // short SHA doesn't re-nag on every rebuild (issue #48).
        const tag = h.version ? releaseTag(h.version) : null;
        if (!tag) return;
        let last: string | null = null;
        try {
          last = localStorage.getItem(LAST_SEEN_VERSION_KEY);
        } catch {
          /* localStorage unavailable — skip the dialog entirely */
          return;
        }
        if (last === null) {
          // First ever open on this browser: remember it, don't show the dialog.
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

  // While loading the auth state show nothing (avoids flash of app content).
  if (authGate === "loading") {
    return null;
  }

  // Auth is ON and not authenticated — show the login screen.
  if (authGate === "blocked") {
    return <LoginPage onLogin={checkAuth} />;
  }

  return (
    <div className="flex h-screen overflow-hidden bg-carbon-background">
      <Sidebar settings={settings} />
      <main className="flex-1 overflow-y-auto p-6 min-w-0">
        {/* Keyed on the route so the content re-mounts and plays a short
            fade-in-up on every navigation (Item 7c). */}
        <div key={location.pathname} className="bv-page-enter">
          <Outlet />
        </div>
      </main>
      {whatsNewVersion && (
        <WhatsNewDialog version={whatsNewVersion} onClose={() => setWhatsNewVersion(null)} />
      )}
    </div>
  );
}
