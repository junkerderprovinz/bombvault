import { useState } from "react";

import { Badge } from "../../components/Badge";
import { BottomSheet } from "../../components/mobile/BottomSheet";
import { REACH_BADGE, type AppServer } from "../../lib/appServers";
import type { useT } from "../../lib/i18n";
import { leaveLayers } from "../../lib/useBackCloses";

type T = ReturnType<typeof useT>["t"];

const NAME = "min-w-0 truncate text-lg font-semibold tracking-tight text-carbon-text";
const ROW = "flex min-h-[3.25rem] w-full items-center gap-3 rounded-control px-3 text-start text-body";

/**
 * InstanceSwitch names the server on screen at the top of the overview inside
 * the Android app. Once the app has named other servers, a tap on the name
 * lists them all with how each answers, and opens the one picked.
 */
export function InstanceSwitch({
  t,
  name,
  servers,
  onOpen,
}: {
  t: T;
  name: string;
  /** The other servers the app knows, null until it says. */
  servers: AppServer[] | null;
  onOpen: (id: string) => void;
}) {
  const [open, setOpen] = useState(false);

  // The sheet's Back entry is dropped before the app is asked: a step back
  // still on its way once the other server loads takes the WebView back here.
  const pick = (id: string) =>
    leaveLayers(() => {
      setOpen(false);
      onOpen(id);
    });

  if (!servers || servers.length === 0) {
    return <span className={NAME}>{name}</span>;
  }

  return (
    <>
      <button
        type="button"
        data-new="overview-instances"
        aria-label={`${name}, ${t("instances.title")}`}
        aria-haspopup="dialog"
        aria-expanded={open}
        onClick={() => setOpen(true)}
        className="-mx-1 flex min-h-11 min-w-0 items-center gap-1 rounded-control px-1 select-none motion-safe:active:scale-[var(--motion-press-scale)] glim-field-focus"
      >
        <span className={NAME}>{name}</span>
        <svg
          aria-hidden="true"
          data-testid="instance-chevron"
          width="16"
          height="16"
          viewBox="0 0 16 16"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          className="shrink-0 text-carbon-textMuted"
        >
          <path d="m4 6 4 4 4-4" />
        </svg>
      </button>
      <BottomSheet open={open} onClose={() => setOpen(false)} title={t("instances.title")} hint={t("overview.instances.hint")}>
        <ul className="flex flex-col gap-1 py-2">
          <li aria-current="true" data-testid="instance-current" className={ROW}>
            <span className="min-w-0 flex-1 truncate font-semibold text-carbon-text">{name}</span>
            <Badge tone="active">{t("instances.thisInstance")}</Badge>
          </li>
          {servers.map((server) => {
            const badge = server.reach && REACH_BADGE[server.reach];
            return (
              <li key={server.id}>
                <button
                  type="button"
                  data-testid="instance-row"
                  onClick={() => pick(server.id)}
                  className={`${ROW} hover:bg-carbon-hover glim-field-focus`}
                >
                  <span className="min-w-0 flex-1 truncate font-semibold text-carbon-text">{server.name}</span>
                  {badge && <Badge tone={badge.tone}>{t(badge.key)}</Badge>}
                </button>
              </li>
            );
          })}
        </ul>
      </BottomSheet>
    </>
  );
}
