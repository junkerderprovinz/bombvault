import type { ReactNode } from "react";
import {
  destinationForDomain,
  listOffsiteTargets,
  PLACEMENT_DOMAINS,
  updateOffsiteTarget,
  type DefaultRow,
  type OffsiteDomain,
  type PlacementDomain,
  type StorageLocation,
} from "../../lib/api";
import { useT } from "../../lib/i18n";
import { isPlacementDomain } from "../../lib/placement";
import { pushSaveWarnings } from "../../lib/placementCodes";
import { flowDomains, flowRows, type FlowAction, type FlowDefault, type FlowLine, type FlowRow } from "../../lib/storageFlow";
import { useToast } from "../../lib/toast";
import { destinationsChanged } from "../../lib/useDestinations";
import { offsiteTargetsChanged } from "../../lib/useOffsiteTargets";
import { usePlacementOptions } from "../../lib/usePlacementOptions";
import { useDefaultWrites, type DefaultContext } from "../placement/useDefaultWrites";

export interface PlacementDefaults {
  rows: DefaultRow[] | null;
  error: string | null;
}

export interface StorageFlow {
  /** False until every default the lines are read off has answered. */
  ready: boolean;
  rows: FlowRow[];
  /** The sections whose off-site copies wait for their default to be confirmed. */
  paused: PlacementDomain[];
  busy: boolean;
  shake: number;
  dialogs: ReactNode;
  toggle: (line: FlowLine) => void;
}

/**
 * useStorageFlow reads which section is backed up where and switches one line
 * at a time. A section with placement changes its default, which asks first
 * where the change has an impact; flash, ZFS and the self-backup switch their
 * target.
 */
export function useStorageFlow(locations: StorageLocation[], defaults: PlacementDefaults): StorageFlow {
  const { t } = useT();
  const { push } = useToast();
  const writes = useDefaultWrites();
  const options = {
    containers: usePlacementOptions("containers"),
    vms: usePlacementOptions("vms"),
    files: usePlacementOptions("files"),
  };
  const shown = flowDomains(locations);

  let ready = defaults.rows !== null || defaults.error !== null;
  const contexts: Partial<Record<PlacementDomain, DefaultContext>> = {};
  const read: Partial<Record<OffsiteDomain, FlowDefault | "unreadable">> = {};
  for (const domain of PLACEMENT_DOMAINS) {
    if (!shown.includes(domain)) continue;
    const row = defaults.rows?.find((r) => r.domain === domain);
    const state = options[domain];
    if (state.options === null && state.error === null) ready = false;
    if (!row || !state.options || row.unreadable || state.options.unreadable) {
      read[domain] = "unreadable";
      continue;
    }
    const ctx = writes.context(row, state.options);
    contexts[domain] = ctx;
    read[domain] = { options: state.options, view: ctx.view };
  }

  async function switchTarget(domain: OffsiteDomain, action: Extract<FlowAction, { kind: "enable" | "create" }>) {
    if (action.kind === "create") {
      const made = await destinationForDomain(action.destinationId, domain);
      if (!made.ok) return writes.fail(made);
    } else {
      const list = await listOffsiteTargets(domain);
      const target = list.targets?.find((x) => x.id === action.targetId);
      if (!list.ok || !target) return writes.fail(list);
      const res = await updateOffsiteTarget(target.id, { ...target, enabled: action.on });
      if (!res.ok) return writes.fail(res);
      pushSaveWarnings(push, t, res.warnings);
    }
    offsiteTargetsChanged();
    destinationsChanged();
  }

  function toggle(line: FlowLine) {
    const action = line.action;
    if (!action || writes.busy) return;
    if (action.kind === "enable" || action.kind === "create") {
      void writes.guarded(() => switchTarget(line.domain, action));
      return;
    }
    const ctx = isPlacementDomain(line.domain) ? contexts[line.domain] : undefined;
    if (!ctx) return;
    if (action.kind === "step") writes.run(ctx, action.step);
    else if (action.kind === "default") void writes.change(ctx, action.change);
    else void writes.addDestination(ctx, action.destinationId);
  }

  return {
    ready,
    rows: ready ? flowRows(locations, read, t, writes.host) : [],
    paused: PLACEMENT_DOMAINS.filter((domain) => shown.includes(domain) && defaults.rows?.find((r) => r.domain === domain)?.paused),
    busy: writes.busy,
    shake: writes.shake,
    dialogs: writes.dialogs,
    toggle,
  };
}
