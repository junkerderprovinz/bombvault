// A server's own interface, opened in the Android app, reaches the app over a
// message port only a page inside the app has. The app's launcher shows how
// each server answers with the badges below.
import type { TranslationKey } from "./i18n";

/** How a server answered the app's last question about what runs there. */
export type Reach = "connected" | "signIn" | "certificate" | "offline";

interface ReachBadge {
  tone: "ok" | "fail" | "warn" | "neutral";
  key: TranslationKey;
}

/** The badge the app's list shows for each answer. */
export const REACH_BADGE: Record<Reach, ReachBadge> = {
  connected: { tone: "ok", key: "instances.connected" },
  signIn: { tone: "warn", key: "launcher.signIn" },
  certificate: { tone: "warn", key: "launcher.checkCertificate" },
  offline: { tone: "fail", key: "instances.notConnected" },
};

/** serverBadge is the badge of one server in the app's list. A member of the
 *  group that does not answer is away, as on the Instances page, which is no
 *  fault; a server that has not answered yet is being checked. */
export function serverBadge(reach: Reach | undefined, member: boolean): ReachBadge {
  if (reach === undefined) return { tone: "neutral", key: "instances.checking" };
  if (reach === "offline" && member) return { tone: "neutral", key: "instances.inactive" };
  return REACH_BADGE[reach];
}

/** reachOf reads the status the app got: 0 when the server was not reached,
 *  -1 when its certificate is not the one trusted for it. */
export function reachOf(status: number): Reach {
  if (status === 401 || status === 403) return "signIn";
  if (status === -1) return "certificate";
  return status === 200 ? "connected" : "offline";
}

type Port = { postMessage(data: string): void };

function appPort(): Port | null {
  return (window as unknown as { bombvaultApp?: Port }).bombvaultApp ?? null;
}

/** inApp says whether the page runs inside the Android app. */
export function inApp(): boolean {
  return appPort() !== null;
}

/** goHome has the app leave this server for its own overview. */
export function goHome() {
  appPort()?.postMessage(JSON.stringify({ op: "home" }));
}
