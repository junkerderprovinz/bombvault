import type { NodeView } from "../../lib/api";
import type { useT } from "../../lib/i18n";
import { relativeTime } from "../../lib/reltime";
import { isAdopted } from "../../lib/useNodes";
import { Figure } from "../instances/InstanceCard";
import { NodeBadge } from "../instances/NodeCards";
import { Card } from "./Card";

type T = ReturnType<typeof useT>["t"];

/** NodesCard shows each node at a glance: whether it is there, when its last
 *  backup finished, and what is failing or unusual on it. */
export function NodesCard({ t, hueIndex, nodes }: { t: T; hueIndex?: number; nodes: NodeView[] }) {
  return (
    <Card title={t("overview.nodes.title")} hint={t("overview.nodes.hint")} hueIndex={hueIndex}>
      {nodes.length === 0 ? (
        <p className="text-xs text-carbon-textMuted">{t("overview.nodes.none")}</p>
      ) : (
        <ul data-testid="overview-nodes" data-new="overview-nodes" className="flex flex-col divide-y divide-carbon-border/60">
          {nodes.map((node) => {
            const away = isAdopted(node) && !node.online;
            return (
              <li key={node.id} data-node={node.id} className="flex flex-col gap-3 py-3 first:pt-0 last:pb-0">
                <div className="flex flex-wrap items-start gap-x-2.5 gap-y-1.5">
                  <span className="min-w-0 max-w-full font-medium text-carbon-text wrap-anywhere">{node.name}</span>
                  <span className="ms-auto">
                    <NodeBadge node={node} />
                  </span>
                </div>
                {away && node.lastSeen && (
                  <p className="text-xs text-carbon-textMuted">{t("instances.lastSeen").replace("{time}", () => relativeTime(t, node.lastSeen!))}</p>
                )}
                <div className="flex flex-wrap items-baseline gap-x-7 gap-y-3">
                  <Figure value={node.lastBackup ? relativeTime(t, node.lastBackup) : t("containers.never")} label={t("containers.lastBackup")} />
                  <Figure value={<span className={node.failures > 0 ? "text-statusFail" : undefined}>{node.failures}</span>} label={t("dashboard.statErrors")} />
                  <Figure value={<span className={node.anomalies > 0 ? "text-statusWarn" : undefined}>{node.anomalies}</span>} label={t("nav.anomalies")} />
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </Card>
  );
}
