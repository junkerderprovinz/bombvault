import { useEffect, useRef, useState } from "react";

import { runZFSReplica } from "../../../lib/api";
import { useT } from "../../../lib/i18n";
import { useToast } from "../../../lib/toast";
import { zfsCodeSentence } from "../../../lib/zfsCodes";
import { canReplicate, isRunning, targetName } from "./replicaModel";
import { useGroup, useReplica, useReplicaServers } from "./replicaStore";

/**
 * useReplicateNow starts one item's replica run and follows it: a run started
 * here ends in a sentence about it once the replica is read again. The card
 * and the entry page's action both use it, so they behave the same.
 */
export function useReplicateNow(itemId: string, name: string) {
  const { t } = useT();
  const { push } = useToast();
  const { replica, reload, progressActive, restoreActive } = useReplica(itemId);
  const { servers } = useReplicaServers();
  const group = useGroup();
  const [starting, setStarting] = useState(false);
  const watched = useRef<{ first: boolean; seen: boolean } | null>(null);

  const running = replica ? isRunning(replica.state, progressActive) : false;
  const shownName = replica ? targetName(replica, servers, group?.members ?? []) : "";

  useEffect(() => {
    const w = watched.current;
    if (!w || !replica) return;
    if (running) {
      w.seen = true;
      return;
    }
    if (!w.seen) return;
    watched.current = null;
    if (replica.state !== "ok") return;
    push(
      w.first
        ? t("zfs.replica.firstDone").replace("{name}", () => name).replace("{target}", () => shownName)
        : t("zfs.replica.upToDate").replace("{name}", () => name),
      "success",
    );
  }, [running, replica, name, shownName, push, t]);

  async function replicate() {
    setStarting(true);
    try {
      const res = await runZFSReplica(itemId);
      if (!res.ok) {
        push(res.code ? zfsCodeSentence(t, res.code) : (res.error ?? t("settings.error")), "fail");
        return;
      }
      watched.current = { first: replica?.state !== "ok", seen: false };
      reload();
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    } finally {
      setStarting(false);
    }
  }

  return {
    replica,
    reload,
    running,
    restoreActive,
    shownName,
    offered: replica ? canReplicate(replica) : false,
    busy: running || starting || restoreActive,
    replicate,
  };
}
