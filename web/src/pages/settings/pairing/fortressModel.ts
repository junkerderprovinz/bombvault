// What the fortress page shows of the server's view, in GlimStone's shapes:
// the other apps with a row per place they run, and the permissions that
// involve BombVault in BombVault's own words.
import { fill, type FortressApp, type FortressPermission } from "@glimstone/core";
import type { FortressMember, FortressPermissionState } from "../../../lib/api";
import type { TranslationKey } from "../../../lib/i18n";

const SELF = "bombvault";

/** The actions BombVault has words for: its own and the one it calls. */
const ACTION_TITLES: Record<string, TranslationKey> = {
  "backup.last": "fortress.action.backupLast",
  "backup.request": "fortress.action.backupRequest",
  "snapshots.browse": "fortress.action.snapshotsBrowse",
  "backup.export": "fortress.action.backupExport",
};

export function fortressApps(members: FortressMember[], ago: (unix: number) => string): FortressApp[] {
  const apps = new Map<string, FortressApp>();
  for (const m of members) {
    if (m.app === SELF) continue;
    const app = apps.get(m.app) ?? { id: m.app, name: m.name || m.app, instances: [] };
    app.instances.push({
      id: m.id,
      device: m.device,
      deviceKind: m.kind === "mobile" ? "phone" : "server",
      form: m.deployment || undefined,
      version: m.version || undefined,
      presence: m.online ? "connected" : "away",
      relay: m.relay,
      lastSeen: !m.online && m.lastSeen ? ago(m.lastSeen) : undefined,
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
  return permissions.map((p) => {
    const words = ACTION_TITLES[p.action];
    return {
      id: `${p.from} ${p.to} ${p.action}`,
      from: p.from,
      to: [p.to],
      on: p.on,
      title: words ? t(words) : p.action,
      detail: fill(t("fortress.permissionPair"), { from: name(p.from), to: name(p.to) }),
    };
  });
}

/** The switch an id of fortressPermissions names. */
export function permissionOf(id: string, on: boolean): FortressPermissionState {
  const [from, to, action] = id.split(" ");
  return { from, to, action, on };
}
