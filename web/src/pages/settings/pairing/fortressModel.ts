// What the fortress page shows of the server's view, in GlimStone's shapes:
// the other apps with their logos and a row per place they run, and the
// permissions that involve BombVault in BombVault's own words.
import { fill, type FortressApp, type FortressPermission } from "@glimstone/core";
import type { FortressMember, FortressPermissionState } from "../../../lib/api";
import type { TranslationKey } from "../../../lib/i18n";

const SELF = "bombvault";

/** The actions BombVault has words for: its own and the ones it calls. */
const ACTION_TITLES: Record<string, TranslationKey> = {
  "backup.last": "fortress.action.backupLast",
  "backup.request": "fortress.action.backupRequest",
  "snapshots.browse": "fortress.action.snapshotsBrowse",
  "backup.export": "fortress.action.backupExport",
  "backup.import": "fortress.action.backupImport",
  "ui.open": "fortress.action.uiOpen",
  "updates.managed": "fortress.action.updatesManaged",
};

// Where a member says it runs: the server it names, else its device.
const placeOf = (m: FortressMember) => `${m.app} ${(m.host || m.device).toLowerCase()}`;

/** fortressApps puts each other app on one card. Every instance opens unless
 *  `opens` is false, as on a paired phone, whose session opens nothing. Two
 *  members that claim one app in one place are marked, since one of them may
 *  copy the other's sign-in. */
export function fortressApps(members: FortressMember[], ago: (unix: number) => string, opens = true): FortressApp[] {
  const claims = new Map<string, number>();
  for (const m of members) claims.set(placeOf(m), (claims.get(placeOf(m)) ?? 0) + 1);
  const apps = new Map<string, FortressApp>();
  for (const m of members) {
    if (m.app === SELF) continue;
    const app = apps.get(m.app) ?? { id: m.app, name: m.name || m.app, instances: [] };
    app.logo ??= m.logo;
    app.instances.push({
      id: m.id,
      device: m.device,
      deviceKind: m.kind === "mobile" ? "phone" : "server",
      form: m.deployment || undefined,
      version: m.version || undefined,
      presence: m.online ? "connected" : "away",
      relay: m.relay,
      lastSeen: !m.online && m.lastSeen ? ago(m.lastSeen) : undefined,
      open: opens && m.kind === "instance" ? {} : undefined,
      doubt: (claims.get(placeOf(m)) ?? 0) > 1 || undefined,
      print: m.id.slice(-8),
    });
    apps.set(m.app, app);
  }
  return [...apps.values()].sort((a, b) => a.name.localeCompare(b.name));
}

export function fortressPermissions(
  permissions: FortressPermissionState[],
  members: FortressMember[],
  t: (key: TranslationKey) => string,
): FortressPermission[] {
  const name = (id: string) => (id === SELF ? "BombVault" : (members.find((m) => m.app === id)?.name ?? id));
  // What an app may do at itself matters only between two of its instances.
  const places = (app: string) => members.filter((m) => m.app === app).length + (app === SELF ? 1 : 0);
  return permissions
    .filter((p) => p.from !== p.to || places(p.from) > 1)
    .map((p) => {
      const words = ACTION_TITLES[p.action];
      const names = { from: name(p.from), to: name(p.to) };
      return {
        id: `${p.from} ${p.to} ${p.action}`,
        from: p.from,
        to: [p.to],
        on: p.on,
        ...(words ? { title: fill(t(words), names) } : { title: p.action, detail: fill(t("fortress.permissionPair"), names) }),
      };
    });
}

/** The switch an id of fortressPermissions names. */
export function permissionOf(id: string, on: boolean): FortressPermissionState {
  const [from, to, action] = id.split(" ");
  return { from, to, action, on };
}
