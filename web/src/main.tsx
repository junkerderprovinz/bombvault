import React from "react";
import ReactDOM from "react-dom/client";
import "@fontsource-variable/noto-sans";
import "@fontsource-variable/noto-sans-arabic";
import "@fontsource-variable/noto-sans-hebrew";
import "@fontsource-variable/noto-sans-thai";
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
import { applyStoredDisco } from "./lib/disco";
import { applyStoredLabelModes } from "./lib/controls";
import { ADOPTED_EVENT, sync as syncDisplayPrefs } from "./lib/displayPrefs";

// Apply stored preferences before the first paint, so the defaults never flash.
applyStoredTheme();
applyStoredLanguage();
applyStoredAccent();
applyStoredRainbow();
applyStoredShape();
applyStoredMotionIntensity();
applyStoredLabelModes();
// Disco only walks hued elements, so it needs the rainbow state applied first.
applyStoredDisco();

// When the server hands this browser a different look after boot, localStorage
// changes underneath the page and every axis has to be applied again, in the
// same order. Registered before the sync below, which can fire it. The rainbow
// does not animate here: this is the stored look arriving late, and animating
// it would sweep every hued element from the flat accent to its hue just after
// the first paint.
window.addEventListener(ADOPTED_EVENT, () => {
  applyStoredTheme();
  applyStoredAccent();
  applyStoredRainbow({ animate: false });
  applyStoredShape();
  applyStoredMotionIntensity();
  applyStoredLabelModes();
  applyStoredDisco();
});

// The server holds the look; localStorage is only a cache. Not awaited, so the
// synchronous calls above decide the first paint and a cleared browser gets its
// stored look back a moment later.
void syncDisplayPrefs();

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <AdvancedProvider>
      <AppRouter />
    </AdvancedProvider>
  </React.StrictMode>
);

// Shape changes animate only once the first render has settled. lib/shape.ts
// explains why that takes two frames.
requestAnimationFrame(() => {
  requestAnimationFrame(() => {
    armShapeTransitions();
  });
});
