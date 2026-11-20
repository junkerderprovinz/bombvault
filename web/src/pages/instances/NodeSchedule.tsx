import { useEffect, useState } from "react";

import { InfoBubble } from "../../components/InfoBubble";
import { PlanButton } from "../../components/PlanButton";
import { setNodeSchedule, type NodeSection, type NodeView } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";

const NODE_SECTIONS: NodeSection[] = ["containers", "files"];

/** What the window offers a new node, half an hour apart so the two sections
 *  do not wait for each other. */
export const DEFAULT_NODE_SCHEDULE: Record<NodeSection, string> = { containers: "daily 02:00", files: "daily 02:30" };

function scheduleOf(node: NodeView): Record<NodeSection, string> {
  return { containers: node.containersCadence, files: node.filesCadence };
}

/** NodeSchedule is when this instance starts the backups of each section of a
 *  node, as one schedule button per section. */
export function NodeSchedule({
  value,
  onChange,
}: {
  value: Record<NodeSection, string>;
  onChange: (section: NodeSection, cadence: string) => void;
}) {
  const { t } = useT();
  return (
    <div data-testid="node-schedule" className="flex flex-col gap-2">
      <span className="flex items-center gap-1.5 text-sm text-carbon-text">
        {t("nodes.when")}
        <InfoBubble tip={t("nodes.whenTip")} />
      </span>
      <div className="flex flex-wrap gap-x-5 gap-y-2">
        {NODE_SECTIONS.map((section) => (
          <div key={section} className="flex min-w-0 max-w-full flex-col items-start gap-1">
            <span className="text-dense uppercase tracking-[.14em] text-carbon-textMuted">{t(`nav.${section}`)}</span>
            <PlanButton label={t(`nav.${section}`)} value={value[section]} onChange={(next) => onChange(section, next)} />
          </div>
        ))}
      </div>
    </div>
  );
}

/** NodeCardSchedule saves a change on a node's card as soon as it is made,
 *  and goes back to what was stored when the save fails. */
export function NodeCardSchedule({ node, onSaved }: { node: NodeView; onSaved: () => void }) {
  const { t } = useT();
  const { push } = useToast();
  const [value, setValue] = useState(() => scheduleOf(node));
  const stored = `${node.containersCadence}\n${node.filesCadence}`;
  useEffect(() => {
    setValue(scheduleOf(node));
    // The stored schedule is what the effect follows, not the node object a
    // list read hands out anew each time.
  }, [stored]); // eslint-disable-line react-hooks/exhaustive-deps

  async function change(section: NodeSection, cadence: string) {
    const before = value;
    setValue({ ...value, [section]: cadence });
    try {
      const res = await setNodeSchedule(node.id, { [section]: cadence });
      if (!res.ok) throw new Error(res.error ?? t("settings.error"));
      onSaved();
    } catch (err) {
      setValue(before);
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    }
  }

  return <NodeSchedule value={value} onChange={(section, cadence) => void change(section, cadence)} />;
}
