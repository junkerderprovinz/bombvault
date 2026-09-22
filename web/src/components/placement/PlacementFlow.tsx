import { useT } from "../../lib/i18n";
import { formatList } from "../../lib/placement";
import { useHostLabel } from "../../lib/useHostLabel";
import { offsiteTargetLabel, useOffsiteTargets } from "../../lib/useOffsiteTargets";

/** PlacementFlow is the one line Flash and self-backup get instead of a bar:
 *  this server, and the enabled targets it copies to. Neither domain has a
 *  home to choose, so there is nothing else to show. */
export function PlacementFlow({ domain }: { domain: "flash" | "config" }) {
  const { t, lang } = useT();
  const host = useHostLabel();
  const targets = useOffsiteTargets(domain);
  if (targets.length === 0) return null;
  return (
    <p className="text-xs text-carbon-textSub">
      {t("placement.flow")
        .replace("{from}", () => host)
        .replace("{to}", () => formatList(lang, targets.map(offsiteTargetLabel)))}
    </p>
  );
}
