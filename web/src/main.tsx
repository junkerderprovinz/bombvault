import React from "react";
import ReactDOM from "react-dom/client";
import "./index.css";
import "flag-icons/css/flag-icons.min.css";
import { AppRouter } from "./app/router";
import { AdvancedProvider } from "./lib/advanced";
import { applyStoredTheme } from "./lib/theme";
import { applyStoredLanguage } from "./lib/i18n";
import { applyStoredAccent } from "./lib/accent";
import { applyStoredRainbow } from "./lib/appearance";

// Apply persisted preferences before first paint (flash prevention).
applyStoredTheme();
applyStoredLanguage();
applyStoredAccent();
applyStoredRainbow();

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <AdvancedProvider>
      <AppRouter />
    </AdvancedProvider>
  </React.StrictMode>
);
