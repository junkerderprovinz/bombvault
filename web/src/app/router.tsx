import { BrowserRouter, Routes, Route, Navigate, useLocation } from "react-router-dom";
import { Layout } from "./Layout";
import { Dashboard } from "../pages/Dashboard";
import { Welcome } from "../pages/Welcome";
import { Anomalies } from "../pages/Anomalies";
import { BackupEntryPage } from "../pages/BackupEntry";
import { Backups } from "../pages/Backups";
import { Instances } from "../pages/Instances";
import { KindRedirect } from "../pages/KindRedirect";
import { ZFSServer } from "../pages/ZFSServer";
import { StoragePage } from "../pages/Storage";
import { StorageLocationPage } from "../pages/StorageLocation";
import { SettingsPage } from "../pages/Settings";
import { Restore } from "../pages/Restore";
import { GlyphSheet } from "../pages/Glyphs";
import { OAuthConsent } from "../pages/OAuthConsent";
import { ENTRY_KIND, ENTRY_KINDS, ENTRY_LIST_PATH, ENTRY_ROUTE } from "../lib/backupEntry";
import { mcpShipped } from "../lib/mcpSwitch";
import { I18nProvider } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";

/** Sends an address that moved to its new place and keeps what the link asked for. */
function MovedTo({ to }: { to: string }) {
  const { search } = useLocation();
  return <Navigate to={`${to}${search}`} replace />;
}

export function AppRouter() {
  return (
    <I18nProvider>
      {/* Inside I18nProvider: the dismiss button's aria-label is translated. */}
      <ToastProvider>
        <BrowserRouter>
          <Routes>
            {/* The page an OAuth client sends the operator to. It stands on its
                own like the login screen, without the rail, and checks the
                session itself. */}
            {mcpShipped && <Route path="/oauth/authorize" element={<OAuthConsent />} />}
            <Route element={<Layout />}>
              {/* The app opens on the welcome page, which passes an install
                  that has a backup straight on to the overview. */}
              <Route index element={<Navigate to="/welcome" replace />} />
              <Route path="/welcome" element={<Welcome />} />
              <Route path="/dashboard" element={<Dashboard />} />
              <Route path="/anomalies" element={<Anomalies />} />
              <Route path={ENTRY_LIST_PATH} element={<Backups />} />
              <Route path={ENTRY_ROUTE} element={<BackupEntryPage />} />
              {/* A link to a kind's own page leads to the list of that kind,
                  or to the entry it names. */}
              {ENTRY_KINDS.map((kind) => (
                <Route key={kind} path={ENTRY_KIND[kind].legacyPath} element={<KindRedirect kind={kind} />} />
              ))}
              <Route path="/instances" element={<Instances />} />
              <Route path="/instances/zfs/:id" element={<ZFSServer />} />
              {/* Bookmarks, release notes and support answers link to
                  these three paths. */}
              <Route path="/receiver" element={<Navigate to="/instances#receiver" replace />} />
              <Route path="/pull" element={<Navigate to="/instances" replace />} />
              <Route path="/fleet" element={<Navigate to="/instances" replace />} />
              <Route path="/storage" element={<StoragePage />} />
              <Route path="/storage/:id" element={<StorageLocationPage />} />
              <Route path="/restore" element={<Restore />} />
              <Route path="/recovery" element={<MovedTo to="/restore" />} />
              {/* Schedules are a Settings page; /jobs stays for existing links. */}
              <Route path="/jobs" element={<Navigate to="/settings/schedules" replace />} />
              <Route path="/settings/:page?" element={<SettingsPage />} />
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
