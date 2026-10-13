import { Badge } from "../../components/Badge";
import { REACH_BADGE, type AppServer } from "../../lib/appServers";
import type { useT } from "../../lib/i18n";
import { Card } from "./Card";

type T = ReturnType<typeof useT>["t"];

/**
 * InstancesCard lists the other servers the Android app knows, with how each
 * answers, so a tap switches to one without going back to the app's list.
 */
export function InstancesCard({
  t,
  servers,
  onOpen,
  hueIndex,
}: {
  t: T;
  servers: AppServer[];
  onOpen: (id: string) => void;
  hueIndex?: number;
}) {
  return (
    <Card title={t("instances.title")} hint={t("overview.instances.hint")} hueIndex={hueIndex}>
      <ul data-new="overview-instances" className="flex flex-col glim-content-fade">
        {servers.map((server) => {
          const badge = server.reach && REACH_BADGE[server.reach];
          return (
            <li key={server.id} className="border-t border-carbon-border first:border-t-0">
              <button
                type="button"
                data-testid="instance-row"
                onClick={() => onOpen(server.id)}
                className="-mx-2 flex min-h-11 w-[calc(100%+1rem)] items-center gap-3 rounded-control px-2 py-3 text-start hover:bg-carbon-hover glim-field-focus"
              >
                <span className="min-w-0 flex-1 truncate text-sm font-semibold text-carbon-text">{server.name}</span>
                {badge && <Badge tone={badge.tone}>{t(badge.key)}</Badge>}
              </button>
            </li>
          );
        })}
      </ul>
    </Card>
  );
}
