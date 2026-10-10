import { useEffect, useState, type ReactNode } from "react";
import type { TranslationKey } from "../../lib/i18n";
import {
  IconFleet,
  IconLive,
  IconNotifications,
  IconSchedules,
  IconTabApps,
  IconTabGeneral,
  IconTabIntegrity,
  IconTabLook,
  IconTabRecovery,
  IconTabSecurity,
  IconTabStorage,
  IconTabSystem,
} from "../../components/navGlyphs";
import { IconInfo, IconLink } from "../../components/glyphs";
import { ADOPTED_EVENT, save as saveDisplayPrefs } from "../../lib/displayPrefs";

// Each Settings page lives at /settings/<id>. The ids are part of bookmarks
// and support answers, so a page that is renamed keeps its id.
export type SettingsPageId =
  | "general"
  | "look"
  | "schedules"
  | "storage"
  | "restore"
  | "notifications"
  | "integrity"
  | "security"
  | "instances"
  | "pairing"
  | "integrations"
  | "apps"
  | "system"
  | "info";

export interface SettingsPageDef {
  id: SettingsPageId;
  label: TranslationKey;
  icon: ReactNode;
}

/** The default order of the rail. A reader can drag the tiles into their own. */
export const SETTINGS_PAGES: SettingsPageDef[] = [
  { id: "general", label: "settings.tab.general", icon: <IconTabGeneral /> },
  { id: "look", label: "settings.tab.look", icon: <IconTabLook /> },
  { id: "schedules", label: "settings.schedule", icon: <IconSchedules /> },
  { id: "storage", label: "storage.title", icon: <IconTabStorage /> },
  { id: "restore", label: "activityLog.typeRestore", icon: <IconTabRecovery /> },
  { id: "notifications", label: "settings.tab.notifications", icon: <IconNotifications /> },
  { id: "integrity", label: "activityLog.typeVerify", icon: <IconTabIntegrity /> },
  { id: "security", label: "settings.tab.security", icon: <IconTabSecurity /> },
  { id: "instances", label: "instances.title", icon: <IconFleet /> },
  { id: "pairing", label: "pairing.title", icon: <IconLink /> },
  { id: "integrations", label: "settings.tab.integrations", icon: <IconLive /> },
  { id: "apps", label: "settings.tab.apps", icon: <IconTabApps /> },
  { id: "system", label: "settings.tab.system", icon: <IconTabSystem /> },
  { id: "info", label: "activityLog.glyphInfo", icon: <IconInfo /> },
];

export const FALLBACK_PAGE: SettingsPageId = "general";

export function isSettingsPage(id: string | undefined): id is SettingsPageId {
  return SETTINGS_PAGES.some((p) => p.id === id);
}

/** A page that was dissolved, and the name its tile carried. */
export interface FormerPage {
  label: TranslationKey;
  /** Where its cards went, as a page id with an optional #anchor. */
  to: string;
}

/**
 * The pages earlier versions had and where their cards live. Their addresses
 * still open the right page, and the search finds them by their old names.
 */
export const FORMER_PAGES: Record<string, FormerPage> = {
  retention: { label: "settings.tab.retention", to: "storage#retention" },
  offsite: { label: "settings.tab.offsite", to: "storage#offsite" },
  cloud: { label: "settings.tab.cloud", to: "storage#cloud" },
  containers: { label: "nav.containers", to: "schedules#order" },
};

/** The names of pages that kept their id and changed their tile's name. */
export const FORMER_NAMES: { label: TranslationKey; page: SettingsPageId }[] = [
  { label: "settings.tab.storage", page: "storage" },
  { label: "settings.tab.schedules", page: "schedules" },
  { label: "settings.tab.integrity", page: "integrity" },
];

/**
 * Where the hashes of the tabbed Settings page lead: a tab to its page, a card
 * to the page holding it plus its anchor. Links in release notes, issue
 * answers and older builds still carry them.
 */
export const LEGACY_HASH: Record<string, string> = {
  general: "general",
  look: "look",
  storage: "storage",
  schedules: "schedules",
  offsite: FORMER_PAGES.offsite.to,
  notifications: "notifications",
  integrity: "integrity",
  system: "system",
  anomalies: "integrity#anomalies",
  pairing: "pairing",
};

/**
 * redirectFor says where an address that names no page of its own leads, as a
 * page id with an optional #anchor, and null for one that names a page.
 * /settings alone opens the card an old hash names, otherwise the page opened
 * last. A dissolved page leads to where its cards went.
 */
export function redirectFor(param: string | undefined, hash: string): string | null {
  if (isSettingsPage(param)) return null;
  if (param === undefined) return LEGACY_HASH[hash.replace(/^#/, "")] ?? readLastPage();
  return FORMER_PAGES[param]?.to ?? FALLBACK_PAGE;
}

/** orderPages lays the pages out in the reader's saved order. A page the saved
 *  order does not know yet goes after the ones it does, in its default place. */
export function orderPages(saved: readonly string[]): SettingsPageDef[] {
  const known = saved.filter(isSettingsPage);
  const rank = (id: SettingsPageId) => {
    const at = known.indexOf(id);
    return at === -1 ? known.length + SETTINGS_PAGES.findIndex((p) => p.id === id) : at;
  };
  return [...SETTINGS_PAGES].sort((a, b) => rank(a.id) - rank(b.id));
}

const ORDER_KEY = "bombvault.settingsOrder";
const ORDER_EVENT = "bv:settings-order";
const LAST_KEY = "bombvault.settingsPage";

export function readOrder(): string[] {
  try {
    const raw = localStorage.getItem(ORDER_KEY);
    const parsed: unknown = raw ? JSON.parse(raw) : [];
    return Array.isArray(parsed) ? parsed.filter((v): v is string => typeof v === "string") : [];
  } catch {
    return [];
  }
}

export function writeOrder(ids: string[]): void {
  window.dispatchEvent(new CustomEvent(ORDER_EVENT, { detail: ids }));
  try {
    localStorage.setItem(ORDER_KEY, JSON.stringify(ids));
  } catch {
    // Without storage the order lasts until the page reloads.
    return;
  }
  saveDisplayPrefs();
}

/** useSettingsOrder is the saved order of the tiles, kept in step with the
 *  rail and with the row that resets it. */
export function useSettingsOrder(): string[] {
  const [order, setOrder] = useState(readOrder);
  useEffect(() => {
    const changed = (e: Event) => setOrder((e as CustomEvent<string[]>).detail);
    const adopted = () => setOrder(readOrder());
    window.addEventListener(ORDER_EVENT, changed);
    window.addEventListener(ADOPTED_EVENT, adopted);
    return () => {
      window.removeEventListener(ORDER_EVENT, changed);
      window.removeEventListener(ADOPTED_EVENT, adopted);
    };
  }, []);
  return order;
}

export function readLastPage(): SettingsPageId {
  try {
    const last = localStorage.getItem(LAST_KEY) ?? undefined;
    return isSettingsPage(last) ? last : FALLBACK_PAGE;
  } catch {
    return FALLBACK_PAGE;
  }
}

export function writeLastPage(id: SettingsPageId): void {
  try {
    localStorage.setItem(LAST_KEY, id);
  } catch {
    // Without storage /settings opens General.
  }
}
