import React from "react";
import ReactDOM from "react-dom/client";
import "./index.css";
import { applyStoredTheme } from "./lib/theme";
import { applyStoredLanguage } from "./lib/i18n";

// Apply persisted preferences before first paint.
applyStoredTheme();
applyStoredLanguage();

// Minimal placeholder — router wired in the next commit (Task 16).
ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <div className="min-h-screen bg-carbon-background flex items-center justify-center">
      <div className="bg-carbon-surface rounded-card border border-carbon-border p-8 text-center">
        <h1 className="text-xl font-semibold text-carbon-text">BombVault</h1>
        <p className="mt-2 text-sm text-carbon-textSub">Initialising…</p>
      </div>
    </div>
  </React.StrictMode>
);
