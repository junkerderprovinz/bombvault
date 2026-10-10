import { useCallback, useEffect, useRef, useState } from "react";
import {
  getSettings,
  listContainers,
  listFileSets,
  listItemRuns,
  listItems,
  listVMs,
  listZFSDatasets,
  type PlacementView,
  type Run,
} from "./api";
import type { EntryKind } from "./backupEntry";
import { itemProgressKey } from "./backupList";
import { withPlacement, type Entry, type EntryDetail } from "./entry";
import { subscribePlacement } from "./placementEvents";
import { useProgress } from "./progress";
import { subscribeRepos } from "./useNamedRepos";

export type EntryStatus = "loading" | "ready" | "missing" | "failed";

class ReadFailed extends Error {}

/**
 * linkable picks the entries a live one can take over by hand: those whose
 * container or VM is gone. One that carries another entry's former name is
 * left out, because the server refuses to move it. The older backups under
 * that name belong to the other entry.
 */
function linkable<T extends { aliases?: string[] }>(
  entries: T[],
  gone: (e: T) => boolean,
  nameOf: (e: T) => string
): string[] {
  const former = new Set(entries.flatMap((e) => e.aliases ?? []));
  return entries
    .filter(gone)
    .map(nameOf)
    .filter((name) => !former.has(name));
}

/** readDetail reads the list of the entry's kind and picks the entry out of
 *  it. The server has no route for one entry. */
async function readDetail(kind: EntryKind, key: string): Promise<EntryDetail | null> {
  switch (kind) {
    case "container": {
      const res = await listContainers();
      if (!res.ok) throw new ReadFailed();
      const containers = res.containers ?? [];
      const container = containers.find((c) => c.name === key);
      if (!container) return null;
      return {
        kind,
        container,
        installed: containers.filter((c) => c.installed),
        linkCandidates: linkable(containers, (c) => !c.installed, (c) => c.name),
      };
    }
    case "vm": {
      const res = await listVMs();
      if (!res.ok) throw new ReadFailed();
      const vms = res.vms ?? [];
      const vm = vms.find((v) => v.libvirtName === key);
      if (!vm) return null;
      return {
        kind,
        vm,
        linkCandidates: linkable(vms, (v) => v.state === "not-installed", (v) => v.libvirtName),
      };
    }
    case "files": {
      const res = await listFileSets();
      if (!res.ok) throw new ReadFailed();
      const set = (res.fileSets ?? []).find((s) => s.id === key);
      return set ? { kind, set } : null;
    }
    case "zfs": {
      const res = await listZFSDatasets();
      if (!res.ok) throw new ReadFailed();
      const dataset = (res.datasets ?? []).find((d) => d.id === key);
      return dataset ? { kind, dataset } : null;
    }
    default:
      return { kind };
  }
}

async function readEntry(kind: EntryKind, key: string): Promise<Entry | null> {
  const [items, detail, settings] = await Promise.all([listItems(), readDetail(kind, key), getSettings()]);
  if (!items.ok || !settings.ok) throw new ReadFailed();
  const item = items.items.find((i) => i.kind === kind && i.key === key);
  if (!item || !detail) return null;
  return { item, detail, settings: settings.settings, hostMountRoot: settings.hostMountRoot };
}

// Enough for the two weeks of the history and the list of recent runs.
const RUN_LIMIT = 120;

/**
 * useEntry reads one entry and its runs, and reads both again when a backup
 * or restore of it ends and when its placement changes somewhere else. A read
 * that fails after the first one keeps what is on the page.
 */
export function useEntry(kind: EntryKind, key: string) {
  const [entry, setEntry] = useState<Entry | null>(null);
  const [runs, setRuns] = useState<Run[]>([]);
  const [status, setStatus] = useState<EntryStatus>("loading");
  // Only the newest read may land, or a slow answer would undo what a later
  // one brought.
  const read = useRef(0);

  const reload = useCallback(async () => {
    const n = ++read.current;
    try {
      const [next, runList] = await Promise.all([
        readEntry(kind, key),
        listItemRuns(kind, key, RUN_LIMIT).catch(() => null),
      ]);
      if (n !== read.current) return;
      if (runList?.ok) setRuns(runList.runs ?? []);
      setEntry(next);
      setStatus(next ? "ready" : "missing");
    } catch {
      if (n === read.current) setStatus((was) => (was === "ready" ? was : "failed"));
    }
  }, [kind, key]);

  useEffect(() => {
    void reload();
    const offs = [subscribeRepos(() => void reload()), subscribePlacement(() => void reload())];
    return () => offs.forEach((off) => off());
  }, [reload]);

  const progress = useProgress();
  const active = entry ? progress[itemProgressKey(entry.item)]?.active === true : false;
  const wasActive = useRef(false);
  useEffect(() => {
    if (wasActive.current && !active) void reload();
    wasActive.current = active;
  }, [active, reload]);

  const setPlacement = useCallback((placement: PlacementView) => {
    setEntry((was) => (was ? withPlacement(was, placement) : was));
  }, []);

  return { entry, runs, status, reload, setPlacement };
}
