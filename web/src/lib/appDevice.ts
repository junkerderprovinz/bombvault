// Inside the Android app the phone decides what belongs to its screen: OLED,
// and the colour Material You hands out. The app sets both as
// window.bombvaultDevice before any script of the page runs, so the first frame
// is already right, and posts them again as a "device" message when they
// change. The launcher and every server's page read them the same way, and
// neither value goes to a server.

export interface AppDevice {
  oled: boolean;
  /** Material You's colour for each theme, absent below Android 12. */
  accent?: { light: string; dark: string };
}

interface Port {
  postMessage(data: string): void;
  addEventListener(type: "message", fn: (e: { data: string }) => void): void;
  removeEventListener(type: "message", fn: (e: { data: string }) => void): void;
}

function port(): Port | null {
  return (window as unknown as { bombvaultApp?: Port }).bombvaultApp ?? null;
}

const HEX = /^#[0-9a-fA-F]{6}$/;

function parse(raw: unknown): AppDevice | undefined {
  if (!raw || typeof raw !== "object") return undefined;
  const { oled, accent } = raw as { oled?: unknown; accent?: { light?: unknown; dark?: unknown } };
  const tone = (v: unknown): v is string => typeof v === "string" && HEX.test(v);
  const device: AppDevice = { oled: oled === true };
  if (accent && tone(accent.light) && tone(accent.dark)) device.accent = { light: accent.light, dark: accent.dark };
  return device;
}

// Undefined until first read, null for a page outside the app or an app too
// old to say.
let device: AppDevice | null | undefined;

/** What the app says about the phone's screen, or undefined outside the app. */
export function appDevice(): AppDevice | undefined {
  if (device === undefined) {
    device = port() ? (parse((window as unknown as { bombvaultDevice?: unknown }).bombvaultDevice) ?? null) : null;
  }
  return device ?? undefined;
}

/**
 * Calls `onChange` with every change the app reports, after asking it to
 * report them to this page. Returns the function that stops listening.
 */
export function watchAppDevice(onChange: (device: AppDevice) => void): () => void {
  const p = port();
  if (!p) return () => {};
  const fn = (e: { data: string }) => {
    let msg: unknown;
    try {
      msg = JSON.parse(e.data);
    } catch {
      return;
    }
    if ((msg as { op?: unknown } | null)?.op !== "device") return;
    const next = parse(msg);
    if (!next) return;
    device = next;
    onChange(next);
  };
  p.addEventListener("message", fn);
  p.postMessage(JSON.stringify({ op: "device" }));
  return () => p.removeEventListener("message", fn);
}

/** Has the app keep OLED as the phone's choice, for the launcher and every server alike. */
export function tellAppOled(on: boolean): void {
  const p = port();
  const now = appDevice();
  if (!p || !now) return;
  device = { ...now, oled: on };
  p.postMessage(JSON.stringify({ op: "oled", on }));
}
