import { Fragment } from "react";

import type { ZFSDatasetView } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { STATUS_TONE_CLASS, zfsObservedLine } from "../../lib/placement";

/** ZFSSitesLine is a ZFS entry's 3-2-1 line, in the words the placement line
 *  of the other domains uses. */
export function ZFSSitesLine({ item }: { item: Pick<ZFSDatasetView, "sites" | "rule321" | "lastBackup"> }) {
  const { t } = useT();
  return (
    <p className="text-xs">
      {zfsObservedLine(t, item).map((line, i) => (
        <Fragment key={i}>
          {i > 0 && <span className="text-carbon-textMuted"> · </span>}
          <span className={STATUS_TONE_CLASS[line.tone]}>{line.text}</span>
        </Fragment>
      ))}
    </p>
  );
}
