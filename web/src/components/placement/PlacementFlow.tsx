import { useT } from "../../lib/i18n";
import { formatList } from "../../lib/placement";
import { useLocalName } from "../../lib/useLocalName";
import { offsiteTargetLabel, useOffsiteTargets } from "../../lib/useOffsiteTargets";

/** PlacementFlow is the one line Flash and self-backup get instead of a bar:
 *  the section's own folder, and the enabled targets it copies to. Neither
 *  domain has a home to choose, so there is nothing else to show. */
export function PlacementFlow({ domain }: { domain: "flash" | "config" }) {
  const { t, lang } = useT();
  const local = useLocalName(domain);
  const targets = useOffsiteTargets(domain);
  if (targets.length === 0) return null;
  return (
    <p className="text-xs text-carbon-textSub">
      {t("placement.flow")
        .replace("{from}", () => local)
        .replace("{to}", () => formatList(lang, targets.map(offsiteTargetLabel)))}
    </p>
  );
}
