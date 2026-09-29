// Which BombVault the app is pointed at: this one, or a paired member opened
// in remote view. Lives in the URL (?instance=id&instanceName=name), so a
// deep link from the Fleet card, a reload and the browser's Back button all
// agree on it, and leaving the page for anywhere outside remote view drops
// the scope with it.

import { createContext, useContext, useEffect, useMemo, type ReactNode } from "react";
import { useSearchParams } from "react-router-dom";
import { apiBase, setInstanceScope } from "./api";

export interface InstanceScope {
  /** "" is this instance; anything else is a member's group id. */
  instanceId: string;
  /** The member's display name, for the remote-view bar and the Open
   *  button's own label; "" when instanceId is "". */
  instanceName: string;
  /** True while instanceId names a member rather than this instance. */
  remote: boolean;
  /** Points every api.ts call at instanceId, "" to come back to this one. */
  open: (instanceId: string, instanceName: string) => void;
  /** Same as open("", ""), named for what a click on the remote-view bar means. */
  leave: () => void;
}

const PARAM_ID = "instance";
const PARAM_NAME = "instanceName";

// No default: falling back to "" would let a control outside <InstanceProvider>
// silently act on this instance while a member's page is shown.
const Ctx = createContext<InstanceScope | null>(null);

export function useInstanceScope(): InstanceScope {
  const scope = useContext(Ctx);
  if (!scope) throw new Error("useInstanceScope() used outside <InstanceProvider>");
  return scope;
}

export function InstanceProvider({ children }: { children: ReactNode }) {
  const [params, setParams] = useSearchParams();
  const instanceId = params.get(PARAM_ID) ?? "";
  const instanceName = instanceId ? (params.get(PARAM_NAME) ?? "") : "";

  // api.ts holds the scope fetchJSON reads, since a plain async function
  // cannot read React context; this keeps the two in step on every render
  // the URL itself drives, including a reload and Back/Forward.
  useEffect(() => {
    setInstanceScope(instanceId);
  }, [instanceId]);

  const value = useMemo<InstanceScope>(() => {
    const open = (nextId: string, nextName: string) => {
      setParams((prev) => {
        const p = new URLSearchParams(prev);
        if (nextId) {
          p.set(PARAM_ID, nextId);
          p.set(PARAM_NAME, nextName);
        } else {
          p.delete(PARAM_ID);
          p.delete(PARAM_NAME);
        }
        return p;
      });
    };
    return {
      instanceId,
      instanceName,
      remote: instanceId !== "",
      open,
      leave: () => open("", ""),
    };
  }, [instanceId, instanceName, setParams]);

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

/** apiBase for the active scope; most call sites want useInstanceScope()
 *  instead, this is for the handful of raw-fetch spots (downloads) that
 *  build their own URL and just need to know whether one applies. */
export function instanceApiBase(instanceId: string): string {
  return apiBase(instanceId);
}

/**
 * Carries the current ?instance=&instanceName= onto an internal nav link's
 * target, so a click between remote view's own pages (Dashboard to
 * Containers, say) stays scoped instead of silently landing back on this
 * instance. search is the linking page's own location.search; a target with
 * no active scope passes through unchanged.
 */
export function withInstanceScope(to: string, search: string): string {
  const current = new URLSearchParams(search);
  const id = current.get(PARAM_ID);
  if (!id) return to;
  const params = new URLSearchParams({ [PARAM_ID]: id, [PARAM_NAME]: current.get(PARAM_NAME) ?? "" });
  return `${to}?${params.toString()}`;
}
