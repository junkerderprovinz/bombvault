import { useT } from "../../../lib/i18n";

/** RollbackLine names the snapshot to roll a replica's target back to. The
 *  name comes from the host, so it stays left to right on any page. */
export function RollbackLine({ snapshot, className = "" }: { snapshot: string; className?: string }) {
  const { t } = useT();
  const [before, after = ""] = t("zfs.replica.rollbackTo").split("{snapshot}");
  return (
    <span className={`block ${className}`}>
      {before}
      <bdi dir="ltr" className="font-mono wrap-anywhere">
        {snapshot}
      </bdi>
      {after}
    </span>
  );
}
