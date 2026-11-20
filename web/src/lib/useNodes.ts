import { useCallback, useEffect, useRef, useState } from "react";
import { listNodes, type NodeView } from "./api";

/** useNodes reads the nodes of this instance, and again every pollMs when it
 *  is given. A failed read keeps the list it had, since a node card that
 *  disappears for one poll reads like a release. */
export function useNodes(pollMs?: number): { nodes: NodeView[]; reload: () => Promise<void> } {
  const [nodes, setNodes] = useState<NodeView[]>([]);
  const read = useRef(0);

  const reload = useCallback(async () => {
    const n = ++read.current;
    try {
      const res = await listNodes();
      if (n === read.current && res.ok) setNodes(res.nodes ?? []);
    } catch {
      // The cards stay as they were until the next read.
    }
  }, []);

  useEffect(() => {
    void reload();
    if (!pollMs) return;
    const timer = window.setInterval(() => void reload(), pollMs);
    return () => window.clearInterval(timer);
  }, [reload, pollMs]);

  return { nodes, reload };
}

/** isAdopted is a node that answered and has not been let go: one home can
 *  send requests to. */
export function isAdopted(node: NodeView): boolean {
  return node.adoptedAt > 0 && node.state !== "releasing";
}
