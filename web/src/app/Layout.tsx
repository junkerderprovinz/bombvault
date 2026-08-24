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
        let last: string | null;
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
      {/* `flex flex-col` added here (sticky-footer page-shell fix, jdp live
          review — "die Versionsnummer soll unterhalb der untersten Card
          stehen, nicht die Cards durchfahren lassen"): `main` is the actual
          scrollable viewport (overflow-y-auto, sized to exactly 100vh minus
          its own p-6 padding via the h-screen row's flex-stretch above) — a
          page that wants its own footer to sit flush with the BOTTOM of this
          box when its content is short, while still scrolling normally
          underneath it when content is tall, needs `main`'s direct child to
          become a flex item it can measure/fill against. Harmless for every
          OTHER route: a page that doesn't opt into filling that height (see
          `bv-page-enter` below) just renders at its own natural height with
          invisible blank flex space below it — no visible change. */}
      <main className="flex-1 flex flex-col overflow-y-auto p-6 min-w-0">
        {/* `flex-1 flex flex-col` added (same fix as above): makes this
            per-route wrapper fill `main`'s available height (a definite size,
            since it's now a flex item of a sized flex column) AND pass a flex
            column context down to whichever page Outlet renders — Settings.tsx
            is the one page that currently uses this to push its own
            AboutFooter to the bottom of the column instead of leaving it
            fixed to the viewport (see AboutFooter's own header comment for
            the full before/after). Every other page ignores the extra
            height exactly as described above. */}
        <div key={location.pathname} className="bv-page-enter flex-1 flex flex-col">
          <Outlet />
        </div>
      </main>
      {whatsNewVersion && (
        <WhatsNewDialog version={whatsNewVersion} onClose={() => setWhatsNewVersion(null)} />
      )}
    </div>
  );
}
