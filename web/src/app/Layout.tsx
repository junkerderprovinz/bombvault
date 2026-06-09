import { Outlet } from "react-router-dom";
import { Sidebar } from "../components/Sidebar";
import { TopBar } from "../components/TopBar";
import { useEffect, useState } from "react";
import { getSettings, type Settings } from "../lib/api";

export function Layout() {
  const [settings, setSettings] = useState<Settings | null>(null);

  // Load settings once on mount to drive sidebar feature flags (vmsEnabled etc.)
  useEffect(() => {
    getSettings()
      .then((res) => {
        if (res.ok) setSettings(res.settings);
      })
      .catch(() => {
        // Non-fatal: sidebar will show VMs/Flash as disabled.
      });
  }, []);

  return (
    <div className="flex h-screen overflow-hidden bg-carbon-background">
      <Sidebar settings={settings} />
      <div className="flex flex-col flex-1 overflow-hidden min-w-0">
        <TopBar />
        <main className="flex-1 overflow-y-auto p-6">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
