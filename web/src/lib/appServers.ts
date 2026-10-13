// A server's own interface, opened in the Android app, can ask the app which
// other servers it knows and have it open one of them. The app answers with
// ids and names only, and opens nothing but a server from its own list.
import { useEffect, useState } from "react";

import type { TranslationKey } from "./i18n";

/** How a server answered the app's last question about what runs there. */
export type Reach = "connected" | "signIn" | "certificate" | "offline";

/** The badge the app's list and the overview show for each answer. */
export const REACH_BADGE: Record<Reach, { tone: "ok" | "fail" | "warn"; key: TranslationKey }> = {
  connected: { tone: "ok", key: "instances.connected" },
  signIn: { tone: "warn", key: "launcher.signIn" },
  certificate: { tone: "warn", key: "launcher.checkCertificate" },
  offline: { tone: "fail", key: "instances.notConnected" },
};

/** reachOf reads the status the app got: 0 when the server was not reached,
 *  -1 when its certificate is not the one trusted for it. */
export function reachOf(status: number): Reach {
  if (status === 401 || status === 403) return "signIn";
  if (status === -1) return "certificate";
  return status === 200 ? "connected" : "offline";
}

export interface AppServer {
  id: string;
  name: string;
  /** Unknown until the app has asked the server. */
  reach?: Reach;
}

type Answer = { op: "servers"; servers: { id: string; name: string }[] } | { op: "reach"; id: string; status: number };

interface Port {
  postMessage(data: string): void;
  addEventListener(type: "message", fn: (e: { data: string }) => void): void;
  removeEventListener(type: "message", fn: (e: { data: string }) => void): void;
}

/** The app's message port, which only a page inside the app has. */
function appPort(): Port | null {
  return (window as unknown as { bombvaultApp?: Port }).bombvaultApp ?? null;
}

/** inApp says whether the page runs inside the Android app. */
export function inApp(): boolean {
  return appPort() !== null;
}

// The app asks every server it lists in turn, over the group where it has to.
const POLL_MS = 30000;

/**
 * useAppServers lists the other servers the Android app knows, with how each
 * answers, and opens one in the app. It is null in a browser, and in an app
 * too old to answer.
 */
export function useAppServers(): { servers: AppServer[]; open: (id: string) => void } | null {
  const [servers, setServers] = useState<AppServer[] | null>(null);

  useEffect(() => {
    const port = appPort();
    if (!port) return;
    const onAnswer = (e: { data: string }) => {
      const answer = JSON.parse(e.data) as Answer;
      if (answer.op === "servers") {
        setServers((known) =>
          answer.servers.map(({ id, name }) => ({ id, name, reach: known?.find((s) => s.id === id)?.reach }))
        );
      } else if (answer.op === "reach") {
        setServers((known) => known && known.map((s) => (s.id === answer.id ? { ...s, reach: reachOf(answer.status) } : s)));
      }
    };
    port.addEventListener("message", onAnswer);
    const ask = () => port.postMessage(JSON.stringify({ op: "servers" }));
    ask();
    const timer = setInterval(ask, POLL_MS);
    return () => {
      clearInterval(timer);
      port.removeEventListener("message", onAnswer);
    };
  }, []);

  if (!servers) return null;
  return { servers, open: (id) => appPort()?.postMessage(JSON.stringify({ op: "open", id })) };
}
