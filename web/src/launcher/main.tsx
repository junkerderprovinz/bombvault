import React from "react";
import ReactDOM from "react-dom/client";
import "@fontsource-variable/noto-sans";
import "@fontsource-variable/noto-sans-arabic";
import "@fontsource-variable/noto-sans-hebrew";
import "@fontsource-variable/noto-sans-thai";
import "../index.css";
import { I18nProvider, applyStoredLanguage } from "../lib/i18n";
import { applyStoredTheme } from "../lib/theme";
import { applyStoredAccent } from "../lib/accent";
import { applyStoredShape } from "../lib/shape";
import { applyStoredRainbow } from "../lib/appearance";
import { applyStoredMotionIntensity } from "../lib/motion";
import { applyStoredDisco } from "../lib/disco";
import { appBridge } from "./bridge";
import { previewBridge } from "./preview";
import { Launcher } from "./Launcher";

// The look the settings page keeps, or the followed server's, before the
// first paint; on a fresh install the defaults.
applyStoredTheme();
applyStoredLanguage();
applyStoredAccent();
applyStoredRainbow();
applyStoredShape();
applyStoredMotionIntensity();
applyStoredDisco();

const bridge = appBridge() ?? (import.meta.env.DEV ? previewBridge() : null);

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <I18nProvider>{bridge && <Launcher bridge={bridge} />}</I18nProvider>
  </React.StrictMode>
);
