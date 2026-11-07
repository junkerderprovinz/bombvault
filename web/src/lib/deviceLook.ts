// The parts of the look that belong to this screen rather than to a server:
// OLED and the device's own colour. Inside the Android app the phone supplies
// both. In a browser OLED is the browser's own switch, and the colour comes
// from AccentColor wherever the browser fills it with the system's.
import { setSystemAccent } from "./accent";
import { appDevice, watchAppDevice } from "./appDevice";
import { inApp } from "./appServers";
import { watchBrowserAccent } from "./browserAccent";
import { applyStoredTheme } from "./theme";

/** Starts listening to the device's sources. Called once, before the first render. */
export function watchDeviceLook(): void {
  if (inApp()) {
    setSystemAccent(appDevice()?.accent);
    watchAppDevice((device) => {
      // The theme paints OLED and the bar colour that goes with it.
      applyStoredTheme();
      setSystemAccent(device.accent);
    });
    return;
  }
  watchBrowserAccent((hex) => setSystemAccent(hex ? { light: hex, dark: hex } : undefined));
}
