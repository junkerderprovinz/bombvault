// The parts of the look that belong to this screen rather than to a server.
// Inside the Android app the phone supplies them; in a browser OLED is the
// browser's own switch.
import { watchAppDevice } from "./appDevice";
import { applyStoredTheme } from "./theme";

/** Starts listening to the device. Called once, before the first render. */
export function watchDeviceLook(): void {
  // The theme paints OLED and the bar colour that goes with it.
  watchAppDevice(() => applyStoredTheme());
}
