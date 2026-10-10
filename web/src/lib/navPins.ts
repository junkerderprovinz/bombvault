// Which Settings pages also have an entry in the sidebar. The choice is part of
// the look, so it is stored and synced with the other display preferences.

import { useMemo, useSyncExternalStore } from "react";
import type { Settings } from "./api";
import { ADOPTED_EVENT, save as saveDisplayPrefs } from "./displayPrefs";

const KEY = "bombvault.navPins";

const PINS = ["storage", "instances"] as const;

export type NavPin = (typeof PINS)[number];

export type NavPins = Record<NavPin, boolean>;

type InstanceModules = Pick<Settings, "receiverEnabled" | "fleetEnabled" | "pullEnabled">;

function readRaw(): string {
  try {
    return localStorage.getItem(KEY) ?? "";
  } catch {
    return "";
  }
}

/** parse reads what the person chose. A page that is missing follows its
 *  default, which is why a default is never written. */
function parse(raw: string): Partial<NavPins> {
  const chosen: Partial<NavPins> = {};
  try {
    const stored: unknown = raw ? JSON.parse(raw) : null;
    if (stored && typeof stored === "object") {
      for (const pin of PINS) {
        const value = (stored as Record<string, unknown>)[pin];
        if (typeof value === "boolean") chosen[pin] = value;
      }
    }
  } catch {
    // The value syncs through the server and can be anything an older or
    // hand-edited blob holds.
  }
  return chosen;
}

function instancesOn(settings: InstanceModules | null | undefined): boolean {
  return !!settings && (settings.receiverEnabled || settings.fleetEnabled || settings.pullEnabled);
}

function resolve(raw: string, modulesOn: boolean): NavPins {
  const chosen = parse(raw);
  return { storage: chosen.storage ?? true, instances: chosen.instances ?? modulesOn };
}

/**
 * readNavPins is the pins as the sidebar shows them. Storage locations is
 * pinned until someone unpins it. Instances follows the receiver, fleet and
 * pull modules until someone pins or unpins it, so switching a module on brings
 * the entry along and an unpinned entry stays away.
 */
export function readNavPins(settings: InstanceModules | null | undefined): NavPins {
  return resolve(readRaw(), instancesOn(settings));
}

const listeners = new Set<() => void>();

export function setNavPin(pin: NavPin, pinned: boolean): void {
  try {
    localStorage.setItem(KEY, JSON.stringify({ ...parse(readRaw()), [pin]: pinned }));
  } catch {
    return;
  }
  saveDisplayPrefs();
  for (const listener of listeners) listener();
}

/** subscribeNavPins calls the listener when the stored pins may have changed:
 *  here, in another tab, or by adopting the server's look. */
export function subscribeNavPins(listener: () => void): () => void {
  listeners.add(listener);
  window.addEventListener("storage", listener);
  window.addEventListener(ADOPTED_EVENT, listener);
  return () => {
    listeners.delete(listener);
    window.removeEventListener("storage", listener);
    window.removeEventListener(ADOPTED_EVENT, listener);
  };
}

export function useNavPins(settings: InstanceModules | null | undefined): NavPins {
  const raw = useSyncExternalStore(subscribeNavPins, readRaw);
  const modulesOn = instancesOn(settings);
  return useMemo(() => resolve(raw, modulesOn), [raw, modulesOn]);
}
