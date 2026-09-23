import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { Layout } from "./Layout";
import { Dashboard } from "../pages/Dashboard";
import { Containers } from "../pages/Containers";
import { VMs } from "../pages/VMs";
import { Flash } from "../pages/Flash";
import { Config } from "../pages/Config";
import { Files } from "../pages/Files";
import { Instances } from "../pages/Instances";
import { SettingsPage } from "../pages/Settings";
import Recovery from "../pages/Recovery";
import { GlyphSheet } from "../pages/Glyphs";
import { I18nProvider } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";

export function AppRouter() {
  return (
    <I18nProvider>
      {/* Inside I18nProvider: the dismiss button's aria-label is translated. */}
      <ToastProvider>
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
              {/* Receiver, Pull and Fleet are tabs of Instances. Their own
                  paths stay as redirects because bookmarks, release notes and
                  support answers link to them. */}
              <Route path="/instances" element={<Instances />} />
              <Route path="/receiver" element={<Navigate to="/instances#receiver" replace />} />
              <Route path="/pull" element={<Navigate to="/instances#pull" replace />} />
              <Route path="/fleet" element={<Navigate to="/instances#fleet" replace />} />
              <Route path="/recovery" element={<Recovery />} />
              {/* Schedules are a Settings tab; /jobs stays for existing links. */}
              <Route path="/jobs" element={<Navigate to="/settings#schedules" replace />} />
              <Route path="/settings" element={<SettingsPage />} />
              {/* Every glyph at its real size with its measured fill, so a
                  mis-sized icon shows up before it reaches a card. Unlisted:
                  no nav entry and no translation. */}
              <Route path="/glyphs" element={<GlyphSheet />} />
              <Route path="*" element={<Navigate to="/dashboard" replace />} />
            </Route>
          </Routes>
        </BrowserRouter>
      </ToastProvider>
    </I18nProvider>
  );
}
