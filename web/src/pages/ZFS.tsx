// ZFS backs up a dataset together with the datasets below it, from one
// recursive snapshot taken on the host over SSH.

import { PAGE_SHELL } from "../lib/pageShell";
import { OffsiteIndicator } from "../components/OffsiteIndicator";
import { useT } from "../lib/i18n";

export function ZFS() {
  const { t } = useT();

  return (
    <div className={PAGE_SHELL}>
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-2xl font-semibold text-carbon-text">{t("zfs.title")}</h1>
          <p className="mt-1 text-sm text-carbon-textSub">{t("zfs.subtitle")}</p>
          <div className="mt-2"><OffsiteIndicator domain="zfs" /></div>
        </div>
      </div>
    </div>
  );
}
