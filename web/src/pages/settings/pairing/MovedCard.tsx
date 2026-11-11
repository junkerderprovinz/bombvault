// MovedCard stands where the group's phrase was once the group moved into
// the fortress: its instances and phones pair with the fortress phrase, and
// the ones still on the old phrase follow by themselves.
import { listOf } from "@glimstone/core";
import type { GroupState } from "../../../lib/api";
import { useT } from "../../../lib/i18n";
import { Card } from "../shared";

export function MovedCard({ group, hueIndex }: { group: GroupState; hueIndex: number }) {
  const { t, lang } = useT();
  const left = group.oldPhrase ?? [];
  return (
    <Card title={t("pairing.movedTitle")} hueIndex={hueIndex}>
      <p className="text-sm text-carbon-text">{t("pairing.moved")}</p>
      {left.length > 0 && (
        <p className="text-sm text-carbon-textSub">{t("pairing.oldPhrase").replace("{names}", listOf(lang, left))}</p>
      )}
    </Card>
  );
}
