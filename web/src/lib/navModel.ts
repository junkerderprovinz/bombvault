// The one ordered list of the app's navigation. The sidebar, the phone's bottom
// bar and its More sheet are filters of it, so no two of them can name, order
// or draw a destination differently.
//
// The list carries data and no colour. A rainbow position is an entry's rank
// among the entries a surface shows, so each surface counts its rows as it
// renders them. A position stored here would stay put when an entry is hidden,
// and the rows after it would leave a gap in the palette.
//
// Settings and pins come in as arguments and nothing is fetched, so chrome
// built on the list renders together with the layout. For the same reason the
// module imports types and glyphs only and spells its routes out: the API
// client stays out of everything that merely draws the navigation.
import type { ComponentType } from "react";
import type { Settings } from "./api";
import type { TranslationKey } from "./i18n";
import type { NavPins } from "./navPins";
import { IconArchive } from "../components/glyphs";
import {
  IconAnomalies,
  IconDashboard,
  IconFleet,
  IconGear,
  IconTabRecovery,
  IconTabStorage,
} from "../components/navGlyphs";

/** The figures an entry can carry beside its name: what needs the person, and
 *  the open findings. */
export type NavCountKind = "attention" | "anomalies";

export interface NavDestination {
  to: string;
  labelKey: TranslationKey;
  icon: ComponentType;
  count?: NavCountKind;
  sidebar: boolean;
  /** Stands at the foot of the sidebar, below sign out, instead of in its list. */
  footer: boolean;
  phone: boolean;
  /** A slot of the phone's bottom bar. Every other phone entry waits in the
   *  More sheet. */
  bar: boolean;
  /** The caption under the slot's glyph, which has a fifth of a phone's width
   *  to stand in. */
  barLabelKey?: TranslationKey;
}

/**
 * The whole navigation in sidebar order. A gate only flips flags and never
 * removes an entry, so every surface filters the same list and the order
 * between any two entries is the same everywhere.
 */
export function destinations(settings: Settings | null, pins: NavPins): NavDestination[] {
  // Detection is a switch, not a place, so its entry goes when it is off.
  const detection = settings?.anomalyEnabled ?? false;
  return [
    { to: "/dashboard", labelKey: "nav.overview", icon: IconDashboard, count: "attention", sidebar: true, footer: false, phone: true, bar: true, barLabelKey: "nav.overviewShort" },
    { to: "/anomalies", labelKey: "nav.anomalies", icon: IconAnomalies, count: "anomalies", sidebar: detection, footer: false, phone: detection, bar: false },
    { to: "/backups", labelKey: "backups.title", icon: IconArchive, sidebar: true, footer: false, phone: true, bar: true, barLabelKey: "nav.backupsShort" },
    { to: "/restore", labelKey: "nav.restore", icon: IconTabRecovery, sidebar: true, footer: false, phone: true, bar: true, barLabelKey: "nav.restoreShort" },
    // The two Settings pages that can be pinned to the sidebar. A phone always
    // lists the storage locations, which keeps its More sheet from ever
    // opening empty.
    { to: "/storage", labelKey: "storage.title", icon: IconTabStorage, sidebar: pins.storage, footer: false, phone: true, bar: false },
    { to: "/instances", labelKey: "instances.title", icon: IconFleet, sidebar: pins.instances, footer: false, phone: pins.instances, bar: false },
    { to: "/settings", labelKey: "nav.settings", icon: IconGear, sidebar: true, footer: true, phone: true, bar: true, barLabelKey: "nav.settingsShort" },
  ];
}

export function sidebarDestinations(settings: Settings | null, pins: NavPins): NavDestination[] {
  return destinations(settings, pins).filter((d) => d.sidebar);
}

export function barDestinations(settings: Settings | null, pins: NavPins): NavDestination[] {
  return destinations(settings, pins).filter((d) => d.phone && d.bar);
}

export function moreDestinations(settings: Settings | null, pins: NavPins): NavDestination[] {
  return destinations(settings, pins).filter((d) => d.phone && !d.bar);
}

/** onDestination reports whether a path is the destination or a page below it,
 *  which is how a link to it decides that it is the current one. The welcome
 *  page of a new install stands in for the overview. */
export function onDestination(pathname: string, d: NavDestination): boolean {
  if (pathname === "/welcome") return d.to === "/dashboard";
  return pathname === d.to || pathname.startsWith(`${d.to}/`);
}
