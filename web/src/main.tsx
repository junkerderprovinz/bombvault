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
import { applyStoredShape, armShapeTransitions } from "./lib/shape";
import { applyStoredMotionIntensity } from "./lib/motion";

// Apply persisted preferences before first paint (flash prevention).
applyStoredTheme();
applyStoredLanguage();
applyStoredAccent();
applyStoredRainbow();
applyStoredShape();
applyStoredMotionIntensity();

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <AdvancedProvider>
      <AppRouter />
    </AdvancedProvider>
  </React.StrictMode>
);

// GlimStone motion-engine, animation 1 (shape-morph) — armed two animation
// frames after the render call above, never before: see armShapeTransitions()'s
// own comment in lib/shape.ts for why two frames, not zero or one.
requestAnimationFrame(() => {
  requestAnimationFrame(() => {
    armShapeTransitions();
  });
});
