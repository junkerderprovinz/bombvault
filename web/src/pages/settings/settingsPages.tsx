import type { ReactNode } from "react";
import type { TranslationKey } from "../../lib/i18n";
import {
  IconContainers,
  IconLive,
  IconNotifications,
  IconSchedules,
  IconTabApps,
  IconTabGeneral,
  IconTabIntegrity,
  IconTabLook,
  IconTabSecurity,
  IconTabOffsite,
  IconTabRetention,
  IconTabStorage,
  IconTabSystem,
} from "../../components/navGlyphs";
import { IconKey, IconLink } from "../../components/glyphs";
import { save as saveDisplayPrefs } from "../../lib/displayPrefs";

// Each Settings page lives at /settings/<id>. The ids are part of bookmarks
// and support answers, so a page that is renamed keeps its id.
export type SettingsPageId =
  | "general"
  | "look"
  | "storage"
  | "retention"
  | "schedules"
  | "containers"
  | "offsite"
  | "cloud"
  | "notifications"
  | "integrity"
  | "security"
  | "pairing"
  | "integrations"
  | "apps"
  | "system";

export interface SettingsPageDef {
  id: SettingsPageId;
  label: TranslationKey;
  icon: ReactNode;
}

/** The default order of the rail. A reader can drag the tiles into their own. */
export const SETTINGS_PAGES: SettingsPageDef[] = [
  { id: "general", label: "settings.tab.general", icon: <IconTabGeneral /> },
  { id: "look", label: "settings.tab.look", icon: <IconTabLook /> },
  { id: "storage", label: "settings.tab.storage", icon: <IconTabStorage /> },
  { id: "retention", label: "settings.tab.retention", icon: <IconTabRetention /> },
  { id: "schedules", label: "settings.tab.schedules", icon: <IconSchedules /> },
  { id: "containers", label: "nav.containers", icon: <IconContainers /> },
  { id: "offsite", label: "settings.tab.offsite", icon: <IconTabOffsite /> },
  { id: "cloud", label: "settings.tab.cloud", icon: <IconKey /> },
  { id: "notifications", label: "settings.tab.notifications", icon: <IconNotifications /> },
  { id: "integrity", label: "settings.tab.integrity", icon: <IconTabIntegrity /> },
  { id: "security", label: "settings.tab.security", icon: <IconTabSecurity /> },
  { id: "pairing", label: "pairing.title", icon: <IconLink /> },
  { id: "integrations", label: "settings.tab.integrations", icon: <IconLive /> },
  { id: "apps", label: "settings.tab.apps", icon: <IconTabApps /> },
  { id: "system", label: "settings.tab.system", icon: <IconTabSystem /> },
];

export const FALLBACK_PAGE: SettingsPageId = "general";

export function isSettingsPage(id: string | undefined): id is SettingsPageId {
  return SETTINGS_PAGES.some((p) => p.id === id);
}

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
  offsite: "offsite",
  notifications: "notifications",
  integrity: "integrity",
  system: "system",
  anomalies: "integrity#anomalies",
  pairing: "pairing",
};

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
  try {
    localStorage.setItem(ORDER_KEY, JSON.stringify(ids));
  } catch {
    // Without storage the order lasts until the page reloads.
    return;
  }
  saveDisplayPrefs();
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
