// ---------------------------------------------------------------------------
// Nav model; the one ordered navigation registry for the whole app.
//
// The mobile chrome (bottom bar, More sheet) derives its destinations from
// this list. The desktop rail still evaluates its own hand-written NavItem
// JSX (its render-order hue counter and inline settings gates are the part
// this registry cannot own), so the rail and the registry are
// two listings of one navigation, held equal by test rather than by
// construction: Sidebar.navModel.dom.test.tsx renders the rail and compares
// it against destinations(settings) filtered to enabled; any divergence on
// either side fails there instead of silently shipping a rail this list no
// longer describes. Extracted from Sidebar.tsx's inline destination JSX
// (main <nav> list + footer Settings row): same order, same gates.
//
// never assign hues here, ever. Sidebar's rainbow positions come from its own
// `nextHue()` render counter, incremented in actual JSX evaluation order,
// with each settings gate short-circuiting before the increment so a hidden
// tab never burns a palette slot (the long-standing documented semantics;
// see Sidebar.tsx's own header comment and the hueSeq/nextHue block comment
// at the extraction site). A hue index precomputed in this list would be
// frozen against the full nine-entry registry, so hiding a gated tab could no
// longer shift later tabs into earlier slots; the exact visible-rank
// behaviour Sidebar.tabColor.dom.test.tsx pins. Hue assignment therefore
// stays a render-time concern of each consumer; this module hands out data
// (route, label, icon, bar membership, enabled) and nothing else, and
// navModel.test.ts guards that no entry ever grows a hue/colour field.
//
// Pure and synchronous by contract: a plain function of already-loaded
// Settings; no fetch, no timer, no shared mutable state; so chrome built on
// it renders the moment Layout renders and performs no fetches of its own.
// Gates never pre-filter the list: `destinations()` always returns
// the full ordered registry with an `enabled` flag per entry, so a consumer
// that needs render-order semantics (Sidebar's counter) skips disabled
// entries itself, and a consumer that needs a concrete surface list uses the
// `barDestinations`/`moreDestinations` filter derivations below; one list,
// so the surfaces' relative order is structural, never maintained twice.
//
// One import exception, flagged here because it is load-bearing:
// this lib module imports the glyph components from ../components/navGlyphs.
// The registry must be the single source of label and icon data (that is the
// point of extracting it; the Sidebar footer lookup and the mobile chrome
// both read icons from here), a component reference needs no JSX, and lib
// modules that render UI already cross this seam in-repo (useConfirm.tsx ->
// ConfirmDialog, toast.tsx -> Toast, dashboardLayout.tsx -> IconTipButton).
// Nothing else comes from components/, and api.ts is read type-only.
// ---------------------------------------------------------------------------
import type { ComponentType } from "react";
import type { Settings } from "./api";
import type { TranslationKey } from "./i18n";
import {
  IconContainers,
  IconConfig,
  IconDashboard,
  IconFiles,
  IconFleet,
  IconFlash,
  IconGear,
  IconRecovery,
  IconVM,
} from "../components/navGlyphs";

/**
 * One navigation destination; the single source of route, label, icon and
 * gating data for every chrome surface that shows one (desktop Sidebar; the
 * mobile bottom bar and More sheet).
 */
export interface NavDestination {
  /** Route path; matches the frozen route table in app/router.tsx. */
  to: string;
  /** Existing `nav.*` i18n key (en table in lib/i18n.ts), reused verbatim;
   *  never a second copy of the label text. Typed as the translation-key
   *  union itself, so a key that is not in the en table is a compile error,
   *  not a runtime blank. */
  labelKey: TranslationKey;
  /** The destination's glyph, as a component reference (no JSX here, which is
   *  why this file stays .ts). Rendered by the consumer as `<d.icon />`. */
  icon: ComponentType;
  /** Bottom-bar membership: dashboard, containers, files, recovery; the four
   *  destinations a phone bar shows directly. Recovery rides the bar because
   *  it is what people open on a phone when something went wrong; Settings
   *  and every other destination reach mobile chrome through the More sheet.
   *  Never a render filter by itself; barDestinations() below applies
   *  `enabled` on top of this. */
  bar: boolean;
  /** Whether the destination is currently on. Gates flip this flag and
   *  nothing else: the full list is handed out unchanged so consumers decide
   *  what an off entry means (Sidebar: skip before burning a hue slot;
   *  bar/More: filtered out entirely). */
  enabled: boolean;
}

/**
 * The full ordered navigation registry, in desktop Sidebar order: the three
 * always-on destinations, then the gated tabs each gated by its settings
 * field, then Settings (always on; Sidebar renders it in its footer group).
 *
 * `settings` is Sidebar's own prop type, `Settings | null`: null (or any
 * missing gate field; Sidebar's dom tests pass partial fixtures) means the
 * gate is off, matching the `?? false` defaults the Sidebar applies to the
 * same fields.
 */
export function destinations(settings: Settings | null): NavDestination[] {
  return [
    { to: "/dashboard", labelKey: "nav.dashboard", icon: IconDashboard, bar: true, enabled: true },
    // Always visible: disaster recovery is a core, non-expert flow.
    { to: "/recovery", labelKey: "nav.recovery", icon: IconRecovery, bar: true, enabled: true },
    { to: "/containers", labelKey: "nav.containers", icon: IconContainers, bar: true, enabled: true },
    // The gated tabs appear only once their domain is enabled; the gate
    // computes `enabled`, it does not remove the entry (see this file's header).
    { to: "/vms", labelKey: "nav.vms", icon: IconVM, bar: false, enabled: settings?.vmsEnabled ?? false },
    { to: "/flash", labelKey: "nav.flash", icon: IconFlash, bar: false, enabled: settings?.flashEnabled ?? false },
    { to: "/files", labelKey: "nav.files", icon: IconFiles, bar: true, enabled: settings?.filesEnabled ?? false },
    { to: "/config", labelKey: "nav.config", icon: IconConfig, bar: false, enabled: settings?.configEnabled ?? false },
    // Everything about another instance lives behind one row now (upstream
    // jdp: "ein eintrag aber der name gefällt mir nicht. sollen wir ihn nicht
    // besser instanzen nennen"). Receiver, Fleet and Pull are still three
    // separate objects with three separate tables - see Instances.tsx for why
    // merging them would be wrong - but they were three rows answering one
    // question, and two of them wore the same glyph. The row appears as soon
    // as any of the three is switched on; the page then shows only the tabs
    // whose own setting is on.
    { to: "/instances", labelKey: "instances.title", icon: IconFleet, bar: false, enabled: settings?.receiverEnabled || settings?.fleetEnabled || settings?.pullEnabled || false },
    { to: "/settings", labelKey: "nav.settings", icon: IconGear, bar: false, enabled: true },
  ];
}

/**
 * The bottom bar's destination slots: the registry filtered to enabled `bar`
 * entries. A one-line derivation of the one list, so the bar's relative order
 * always equals the desktop Sidebar's by construction.
 *
 * When `filesEnabled` is false the Files slot disappears and the bar renders
 * 3 destination slots + More; matching desktop Sidebar gating exactly, which
 * is what makes "a settings-gated tab never appears in mobile chrome when the
 * desktop Sidebar hides it" hold by construction rather than by a special
 * case.
 */
export function barDestinations(settings: Settings | null): NavDestination[] {
  return destinations(settings).filter((d) => d.bar && d.enabled);
}

/**
 * The More sheet's destination list: the registry filtered to enabled
 * non-bar entries; the desktop Sidebar's order. Same derivation shape as
 * barDestinations, same structural order-parity guarantee. With Recovery on
 * the bar this list can be empty (a fresh DB has every gate off): the More
 * sheet still never reads as empty, because the Simple/Advanced view toggle
 * (and, with a password set, sign-out) render below the rows; which is also
 * why the bar's More trigger renders unconditionally rather than gating on
 * this list's length.
 */
export function moreDestinations(settings: Settings | null): NavDestination[] {
  return destinations(settings).filter((d) => !d.bar && d.enabled);
}
