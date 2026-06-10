import { Outlet } from "react-router-dom";
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

  // Load settings once auth is cleared to drive sidebar feature flags.
  useEffect(() => {
    if (authGate !== "pass") return;
    getSettings()
      .then((res) => {
        if (res.ok) setSettings(res.settings);
      })
      .catch(() => {
        // Non-fatal: sidebar will show VMs/Flash as disabled.
      });
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
        <Outlet />
      </main>
    </div>
  );
}
