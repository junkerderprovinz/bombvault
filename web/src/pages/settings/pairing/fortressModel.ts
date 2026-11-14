// What the fortress page shows of the server's view, in GlimStone's shapes:
// the other apps with their logos and a row per place they run, and the
// permissions that involve BombVault in BombVault's own words.
import { fill, type FortressApp, type FortressPermission } from "@glimstone/core";
import type { FortressMember, FortressPermissionState } from "../../../lib/api";
import type { TranslationKey } from "../../../lib/i18n";

const SELF = "bombvault";

// The cards of members whose app the fortress has not heard yet, one each,
// named after their device.
const UNHEARD = "member:";

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

// GlimStone calls a phone app an Android app, and writes the v of a version
// itself.
const formOf = (m: FortressMember) => (m.deployment === "phone" ? "android" : m.deployment || undefined);
const versionOf = (m: FortressMember) => m.version.replace(/^v/i, "") || undefined;

// Where a member says it runs: the server it names, else its device.
const placeOf = (m: FortressMember) => `${m.app} ${(m.host || m.device).toLowerCase()}`;

/** unheard says the card stands for a member whose app is unknown here,
 *  which nothing can open or name beyond its device. */
export const unheard = (app: FortressApp) => app.id.startsWith(UNHEARD);

/** onOldPhrase names the members still on the phrase before a change, by
 *  `named` where their app is known and by their device otherwise. */
export function onOldPhrase(members: FortressMember[], named: (app: string, device: string) => string): string[] {
  return members.filter((m) => m.oldPhrase && m.app !== SELF).map((m) => (m.app ? named(m.name || m.app, m.device) : m.device));
}

/** fortressApps puts each other app on one card. Every instance opens unless
 *  `opens` is false, as on a paired phone, whose session opens nothing. Two
 *  members that claim one app in one place are marked, since one of them may
 *  copy the other's sign-in. A member whose app is unknown gets a card of its
 *  own under its device's name, and opens nothing. */
export function fortressApps(members: FortressMember[], ago: (unix: number) => string, opens = true): FortressApp[] {
  const claims = new Map<string, number>();
  for (const m of members) claims.set(placeOf(m), (claims.get(placeOf(m)) ?? 0) + 1);
  const apps = new Map<string, FortressApp>();
  for (const m of members) {
    if (m.app === SELF) continue;
    const id = m.app || UNHEARD + m.id;
    const app = apps.get(id) ?? { id, name: m.app ? m.name || m.app : m.device, instances: [] };
    app.logo ??= m.logo;
    app.instances.push({
      id: m.id,
      device: m.device,
      deviceKind: m.kind === "mobile" ? "phone" : "server",
      form: formOf(m),
      version: versionOf(m),
      presence: m.online ? "connected" : "away",
      relay: m.relay,
      lastSeen: !m.online && m.lastSeen ? ago(m.lastSeen) : undefined,
      open: opens && m.app && m.kind === "instance" ? {} : undefined,
      doubt: (!!m.app && (claims.get(placeOf(m)) ?? 0) > 1) || undefined,
      print: m.id.slice(-8),
    });
    apps.set(id, app);
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
