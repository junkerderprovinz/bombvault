import { useState } from "react";

import { Badge } from "../../components/Badge";
import { Button } from "../../components/Button";
import { IconKey, IconUnlink } from "../../components/glyphs";
import { InfoBubble } from "../../components/InfoBubble";
import { IconPencil } from "../../components/navGlyphs";
import { releaseNode, type NodeView } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { relativeTime } from "../../lib/reltime";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { Facts, Figure, InstanceCard } from "./InstanceCard";
import { ReplaceWindow } from "./NodeCompose";
import { PlacementWindow } from "./NodePlacement";
import { NodeCardSchedule } from "./NodeSchedule";
import { isAdopted } from "../../lib/useNodes";

const PLACEMENT_LABEL = { receiver: "nodes.fig.receiver", location: "nodes.fig.location", node: "nodes.fig.onNode" } as const;

/** NodeBadge says whether a node is there, waits for its first start or is
 *  being released. */
export function NodeBadge({ node }: { node: NodeView }) {
  const { t } = useT();
  if (node.state === "releasing") {
    return (
      <Badge tone="warn" size="large">
        {t("nodes.releasing")}
        <InfoBubble tip={t("nodes.releasingTip")} onAccent />
      </Badge>
    );
  }
  if (node.adoptedAt === 0) {
    return (
      <Badge tone="neutral" size="large">
        {t("nodes.waiting")}
      </Badge>
    );
  }
  return (
    <Badge tone={node.online ? "ok" : "neutral"} size="large">
      {t(node.online ? "instances.connected" : "instances.inactive")}
    </Badge>
  );
}

/** NodeCards are the nodes this instance backs up through: servers that run
 *  BombVault without a web interface and take their orders from here. */
export function NodeCards({
  nodes,
  firstIndex,
  onChanged,
}: {
  nodes: NodeView[];
  firstIndex: number;
  onChanged: () => void;
}) {
  const { t } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [placing, setPlacing] = useState<NodeView | null>(null);
  const [replacing, setReplacing] = useState<NodeView | null>(null);

  async function release(node: NodeView) {
    const ask = t("nodes.releaseAsk").replace("{name}", () => node.name);
    if (!(await confirm(ask, { confirmKey: "nodes.release" }))) return;
    try {
      const res = await releaseNode(node.id);
      if (!res.ok) {
        push(res.error ?? t("settings.error"), "fail");
        return;
      }
      push(t(res.told ? "nodes.released" : "nodes.releaseLater").replace("{name}", () => node.name), res.told ? "success" : "warn");
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    }
    onChanged();
  }

  return (
    <>
      {nodes.map((node, i) => {
        const releasing = node.state === "releasing";
        const adopted = isAdopted(node);
        const away = adopted && !node.online;
        return (
          <InstanceCard
            key={node.id}
            cardKey={`node:${node.id}`}
            name={node.name}
            eyebrow={t("nodes.kind")}
            badges={<NodeBadge node={node} />}
            facts={
              <Facts
                items={[
                  node.version,
                  away && node.lastSeen && t("instances.lastSeen").replace("{time}", () => relativeTime(t, node.lastSeen!)),
                ]}
              />
            }
            figures={
              <Figure
                value={
                  node.placementTarget ? (
                    <bdi dir="ltr" className="wrap-anywhere">
                      {node.placementTarget}
                    </bdi>
                  ) : (
                    t("nodes.notPlaced")
                  )
                }
                label={t(node.placement ? PLACEMENT_LABEL[node.placement] : "nodes.fig.backupsTo")}
              />
            }
            actions={
              !releasing && (
                <>
                  <Button
                    label={t("nodes.editPlacement")}
                    labelKey="nodes.editPlacement"
                    glyph={<IconPencil />}
                    tone="neutral"
                    disabled={!adopted}
                    title={adopted ? undefined : t("nodes.placementWaits")}
                    onClick={() => setPlacing(node)}
                  />
                  <Button label={t("nodes.replace")} labelKey="nodes.replace" glyph={<IconKey />} tone="neutral" onClick={() => setReplacing(node)} />
                  <Button label={t("nodes.release")} labelKey="nodes.release" glyph={<IconUnlink />} tone="neutral" onClick={() => void release(node)} />
                </>
              )
            }
            index={firstIndex + i}
          >
            {!releasing && <NodeCardSchedule node={node} onSaved={onChanged} />}
          </InstanceCard>
        );
      })}
      {confirmDialog}
      {placing && <PlacementWindow node={placing} onClose={() => setPlacing(null)} onSaved={onChanged} />}
      {replacing && <ReplaceWindow node={replacing} onClose={() => setReplacing(null)} />}
    </>
  );
}
