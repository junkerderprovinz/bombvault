import type { useT } from "../../lib/i18n";
import { InfoBubble } from "../InfoBubble";

type T = ReturnType<typeof useT>["t"];

/** Properties only zfs create can take; internal/zfs/props.go keeps the same list. */
export const ZFS_CREATE_ONLY = ["casesensitivity", "normalization", "utf8only"];

/** Properties BombVault shows but never sets; internal/zfs/props.go keeps the same list. */
export const ZFS_NOT_APPLIED = ["mountpoint", "canmount", "readonly", "encryption", "keyformat", "keylocation", "pbkdf2iters"];

/** Where a stored property goes in a restore of the given kind. */
export function zfsPropertyFate(name: string, into: "new" | "existing"): "applied" | "createOnly" | "notApplied" {
  if (ZFS_NOT_APPLIED.includes(name)) return "notApplied";
  if (into === "existing" && ZFS_CREATE_ONLY.includes(name)) return "createOnly";
  return "applied";
}

// ZFSPropertyList shows the ZFS properties a backup stored for one dataset and
// what the chosen restore does with each of them.
export function ZFSPropertyList({
  properties,
  into,
  t,
}: {
  properties: Record<string, string>;
  into: "new" | "existing";
  t: T;
}) {
  const names = Object.keys(properties).sort();
  if (names.length === 0) return null;
  return (
    <div className="flex flex-col gap-1">
      <span className="flex items-center gap-1 text-xs text-carbon-textSub">
        {t("zfs.restore.propertiesTitle")}
        <InfoBubble tip={t(into === "new" ? "zfs.restore.propertiesNewHint" : "zfs.restore.propertiesExistingHint")} />
      </span>
      <ul className="flex flex-col gap-0.5 rounded-control bg-carbon-surface2 px-3 py-1.5 text-xs">
        {names.map((name) => {
          const fate = zfsPropertyFate(name, into);
          return (
            <li key={name} className="flex items-center gap-2">
              <span dir="ltr" className={`font-mono text-start ${fate === "applied" ? "text-carbon-text" : "text-carbon-textMuted"}`}>
                {name}={properties[name]}
              </span>
              {fate !== "applied" && (
                <span className="text-carbon-textMuted">
                  {t(fate === "createOnly" ? "zfs.restore.propertyCreateOnly" : "zfs.restore.propertyNotApplied")}
                </span>
              )}
            </li>
          );
        })}
      </ul>
    </div>
  );
}
