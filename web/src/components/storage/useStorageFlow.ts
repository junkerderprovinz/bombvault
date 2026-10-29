import { useState } from "react";
import {
  ApiError,
  destinationForDomain,
  LOCAL_SWITCH_DOMAINS,
  listOffsiteTargets,
  PLACEMENT_DOMAINS,
  previewPlacementDefault,
  putPlacementDefault,
  setSectionLocal,
  updateOffsiteTarget,
  type DefaultChange,
  type DefaultImpact,
  type DefaultRow,
  type NamedRepo,
  type OffsiteDomain,
  type OkEnvelope,
  type PlacementDomain,
  type StorageLocation,
} from "../../lib/api";
import { useT } from "../../lib/i18n";
import { defaultView, isPlacementDomain, placementButtons } from "../../lib/placement";
import { placementErrorText, pushSaveWarnings } from "../../lib/placementCodes";
import { placementChanged } from "../../lib/placementEvents";
import { flowDomains, flowRows, type FlowAction, type FlowDefault, type FlowLine, type FlowRow } from "../../lib/storageFlow";
import { useToast } from "../../lib/toast";
import { destinationsChanged } from "../../lib/useDestinations";
import { offsiteTargetsChanged } from "../../lib/useOffsiteTargets";
import { storageLocationsChanged } from "../../lib/useStorageLocations";
import { usePlacementOptions } from "../../lib/usePlacementOptions";

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
  /** The key of the line whose switch the server is still working on. */
  pending: string | null;
  shake: number;
  toggle: (line: FlowLine) => void;
  /** The direct repository a click would make the home, waiting for the
   *  dialog that sets it up. */
  direct: PendingDirect | null;
  /** Makes the repository the dialog created the home. */
  directMade: (repo: NamedRepo) => void;
  directCancelled: () => void;
}

export interface PendingDirect {
  key: string;
  domain: PlacementDomain;
  target: { id: string; name: string };
  skip: string[];
  /** The target that is switched on with it. */
  enable?: string;
}

/** A write that failed, with the answer that says why. */
class Refused extends Error {
  constructor(readonly res: OkEnvelope) {
    super(res.error);
  }
}

function must<T extends OkEnvelope>(res: T): T {
  if (!res.ok) throw new Refused(res);
  return res;
}

// The numbers a write is based on move with every backup, so a write the
// server calls stale is asked again with its fresh numbers, a few times.
const STALE_ROUNDS = 3;

// A preview the server refused refuses the write as well. One that never came
// back leaves the numbers to the server, which answers a write without them
// as stale and names them.
async function previewOf(domain: PlacementDomain, change: DefaultChange): Promise<DefaultImpact | undefined> {
  try {
    return must(await previewPlacementDefault(domain, change)).impact;
  } catch (err) {
    if (err instanceof ApiError && err.status >= 400 && err.status < 500 && err.status !== 408) throw err;
    if (err instanceof Refused) throw err;
    return undefined;
  }
}

// Every reader of what a click wrote reads again, also after a click that
// failed halfway, so the picture shows what did land.
function announce(action: FlowAction) {
  switch (action.kind) {
    case "local":
      storageLocationsChanged();
      return;
    case "enable":
    case "create":
      offsiteTargetsChanged();
      destinationsChanged();
      return;
    case "destination":
      offsiteTargetsChanged();
      destinationsChanged();
      placementChanged();
      return;
    default:
      if (action.enable) {
        offsiteTargetsChanged();
        destinationsChanged();
      }
      placementChanged();
  }
}

/**
 * useStorageFlow reads which section is backed up where and switches one line
 * per click: a line only says where new backups go, and nothing already
 * stored moves or goes. Only a direct repository that does not exist yet is
 * set up in its dialog first. A section with placement changes its default;
 * flash, ZFS and the self-backup switch their own folder or a target.
 */
export function useStorageFlow(locations: StorageLocation[], defaults: PlacementDefaults): StorageFlow {
  const { t, lang } = useT();
  const { push } = useToast();
  const [pending, setPending] = useState<string | null>(null);
  const [direct, setDirect] = useState<PendingDirect | null>(null);
  const [shake, setShake] = useState(0);
  const options = {
    containers: usePlacementOptions("containers"),
    vms: usePlacementOptions("vms"),
    files: usePlacementOptions("files"),
  };
  const shown = flowDomains(locations);

  let ready = defaults.rows !== null || defaults.error !== null;
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
    read[domain] = { options: state.options, view: defaultView(row, state.options) };
  }

  async function saveDefault(domain: PlacementDomain, change: DefaultChange) {
    let impact = await previewOf(domain, change);
    for (let round = 0; ; round++) {
      const res = await putPlacementDefault(domain, change, impact);
      if (res.ok) return;
      if (res.code !== "stale" || !res.impact || round === STALE_ROUNDS) throw new Refused(res);
      impact = res.impact;
    }
  }

  async function setTarget(domain: OffsiteDomain, targetId: string, on: boolean) {
    const list = must(await listOffsiteTargets(domain));
    const target = list.targets?.find((x) => x.id === targetId);
    if (!target) throw new Refused(list);
    pushSaveWarnings(push, t, must(await updateOffsiteTarget(target.id, { ...target, enabled: on })).warnings);
  }

  // A target made for a section starts unticked, so the click that made it
  // also lights it: as a copy where backups are written somewhere copies are
  // made from, and as that place otherwise, once its direct repository is set
  // up.
  async function newTarget(line: FlowLine, destinationId: string) {
    const { domain } = line;
    const made = must(await destinationForDomain(destinationId, domain));
    const target = made.target;
    if (!target) throw new Refused(made);
    if (!target.enabled) await setTarget(domain, target.id, true);
    const def = read[domain];
    if (!isPlacementDomain(domain) || !def || def === "unreadable") return;
    const buttons = placementButtons(def.view, def.options);
    if (buttons.local || buttons.home !== null) {
      const known = [...def.options.targets.map((x) => x.id), target.id];
      const copies = [...buttons.ticked.filter((id) => id !== buttons.home), target.id];
      await saveDefault(domain, { skip: known.filter((id) => !copies.includes(id)) });
      return;
    }
    setDirect({ key: line.key, domain, target: { id: target.id, name: target.name || line.location.name }, skip: ["*"] });
  }

  async function apply(line: FlowLine) {
    const { domain, action } = line;
    switch (action.kind) {
      case "local": {
        const section = LOCAL_SWITCH_DOMAINS.find((d) => d === domain);
        if (section) must(await setSectionLocal(section, action.on));
        return;
      }
      case "enable":
        return setTarget(domain, action.targetId, action.on);
      case "create":
      case "destination":
        return newTarget(line, action.destinationId);
    }
    if (!isPlacementDomain(domain)) return;
    if (action.enable) await setTarget(domain, action.enable, true);
    if (action.kind === "default") return saveDefault(domain, action.change);
  }

  // A direct repository is set up in its dialog, and the click writes nothing
  // until the dialog made it.
  function askDirect(line: FlowLine): boolean {
    const { domain, action } = line;
    if (action.kind !== "direct" || !isPlacementDomain(domain)) return false;
    const def = read[domain];
    const name = def && def !== "unreadable" ? def.options.targets.find((x) => x.id === action.targetId)?.name : undefined;
    setDirect({ key: line.key, domain, target: { id: action.targetId, name: name ?? line.location.name }, skip: action.skip, enable: action.enable });
    return true;
  }

  function directMade(repo: NamedRepo) {
    const asked = direct;
    setDirect(null);
    if (!asked) return;
    setPending(asked.key);
    (async () => {
      if (asked.enable) await setTarget(asked.domain, asked.enable, true);
      await saveDefault(asked.domain, { home: repo.id, skip: asked.skip });
    })()
      .catch(fail)
      .finally(() => {
        if (asked.enable) offsiteTargetsChanged();
        placementChanged();
        setPending(null);
      });
  }

  function fail(err: unknown) {
    const res = err instanceof Refused ? err.res : { ok: false, error: err instanceof ApiError ? err.message : undefined };
    push(placementErrorText(t, lang, res, "settings.error"), "fail");
    setShake((n) => n + 1);
  }

  function toggle(line: FlowLine) {
    if (pending !== null || direct !== null || askDirect(line)) return;
    setPending(line.key);
    apply(line)
      .catch(fail)
      .finally(() => {
        announce(line.action);
        setPending(null);
      });
  }

  return {
    ready,
    rows: ready ? flowRows(locations, read) : [],
    paused: PLACEMENT_DOMAINS.filter((domain) => shown.includes(domain) && defaults.rows?.find((r) => r.domain === domain)?.paused),
    busy: pending !== null,
    pending,
    shake,
    toggle,
    direct,
    directMade,
    directCancelled: () => setDirect(null),
  };
}
