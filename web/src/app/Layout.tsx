import { Outlet, useLocation } from "react-router-dom";
import { Sidebar } from "../components/Sidebar";
import { useEffect, useState, useCallback } from "react";
import { getSettings, getAuth, type Settings } from "../lib/api";
import { LoginPage } from "../pages/Login";

// Auth probe state: null = not yet fetched, false = auth off or authed,
// true = auth on AND not authed (show login).
type AuthGateState = "loading" | "pass" | "blocked";

export function Layout() {
  const [settings, setSettings] = useState<Settings | null>(null);
  const [authGate, setAuthGate] = useState<AuthGateState>("loading");
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
    </div>
  );
}
