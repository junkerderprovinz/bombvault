import React from "react";
import ReactDOM from "react-dom/client";
import "./index.css";
import { AppRouter } from "./app/router";
import { applyStoredTheme } from "./lib/theme";
import { applyStoredLanguage } from "./lib/i18n";

// Apply persisted preferences before first paint.
applyStoredTheme();
applyStoredLanguage();

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <AppRouter />
  </React.StrictMode>
);
