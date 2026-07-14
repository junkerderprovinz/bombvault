import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { Layout } from "./Layout";
import { Dashboard } from "../pages/Dashboard";
import { Containers } from "../pages/Containers";
import { VMs } from "../pages/VMs";
import { Flash } from "../pages/Flash";
import { Config } from "../pages/Config";
import { Files } from "../pages/Files";
import { SettingsPage } from "../pages/Settings";
import Recovery from "../pages/Recovery";
import { I18nProvider } from "../lib/i18n";

export function AppRouter() {
  return (
    <I18nProvider>
      <BrowserRouter>
        <Routes>
          <Route element={<Layout />}>
            <Route index element={<Navigate to="/dashboard" replace />} />
            <Route path="/dashboard" element={<Dashboard />} />
            <Route path="/containers" element={<Containers />} />
            <Route path="/vms" element={<VMs />} />
            <Route path="/flash" element={<Flash />} />
            <Route path="/config" element={<Config />} />
            <Route path="/files" element={<Files />} />
            <Route path="/recovery" element={<Recovery />} />
            {/* The Plans page was retired into Settings › Schedules; keep /jobs
                as a redirect so old links/bookmarks land on the Schedules tab. */}
            <Route path="/jobs" element={<Navigate to="/settings#schedules" replace />} />
            <Route path="/settings" element={<SettingsPage />} />
            <Route path="*" element={<Navigate to="/dashboard" replace />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </I18nProvider>
  );
}
