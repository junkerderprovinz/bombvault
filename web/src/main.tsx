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
import { applyStoredLabelModes } from "./lib/controls";
import { sync as syncDisplayPrefs } from "./lib/displayPrefs";

// Apply persisted preferences before first paint (flash prevention).
applyStoredTheme();
applyStoredLanguage();
applyStoredAccent();
applyStoredRainbow();
applyStoredShape();
applyStoredMotionIntensity();
applyStoredLabelModes();

// Then reconcile with the server, which is where the look actually lives
// (#191). Deliberately AFTER the calls above and deliberately not awaited: the
// point of those is that they are synchronous, so the page paints in the right
// theme instead of flashing the default one, and no network round trip can be.
// On a browser that still has its cache this changes nothing; on one that was
// cleared it brings the stored look back a moment after paint.
void syncDisplayPrefs();

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
