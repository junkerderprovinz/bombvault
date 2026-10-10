// The providers a page of the app stands in, for the tests that render one.
import { createElement, useEffect, useState, type ReactElement, type ReactNode } from "react";
import { MemoryRouter } from "react-router-dom";

import { getSettings, type Settings } from "../../lib/api";
import { AttentionProvider } from "../../lib/attentionProvider";
import { I18nProvider } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import { AnomalyProvider } from "../../lib/useAnomalies";

/** Reads the settings as the layout does and hands them down. */
function WithSettings({ children }: { children: ReactNode }) {
  const [settings, setSettings] = useState<Settings | null>(null);
  useEffect(() => {
    const load = () => void getSettings().then((res) => setSettings(res.settings));
    load();
    window.addEventListener("bv:settings-changed", load);
    return () => window.removeEventListener("bv:settings-changed", load);
  }, []);
  return createElement(AttentionProvider, { settings }, children);
}

/** Wraps a page in everything it reads from above. */
export function inApp(page: ReactElement, path = "/dashboard"): ReactElement {
  return createElement(
    MemoryRouter,
    { initialEntries: [path] },
    createElement(
      I18nProvider,
      null,
      createElement(ToastProvider, null, createElement(AnomalyProvider, null, createElement(WithSettings, null, page)))
    )
  );
}
