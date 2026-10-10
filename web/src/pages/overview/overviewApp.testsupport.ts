// The providers a page of the app stands in, for the tests that render one.
import { createElement, type ReactElement } from "react";
import { MemoryRouter } from "react-router-dom";

import { I18nProvider } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import { AnomalyProvider } from "../../lib/useAnomalies";

/** Wraps a page in everything it reads from above. */
export function inApp(page: ReactElement, path = "/dashboard"): ReactElement {
  return createElement(
    MemoryRouter,
    { initialEntries: [path] },
    createElement(I18nProvider, null, createElement(ToastProvider, null, createElement(AnomalyProvider, null, page)))
  );
}
