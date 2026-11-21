// A node answers a fixed list of its own routes through home. A call made in a
// node's scope goes to that node when its route is on the list, and to home
// otherwise, so a page about a node can still read home's settings.
import { withinBase } from "./base";

/** One route of the list, as "METHOD /api/a/{b}" split into its parts. */
export interface NodeRoute {
  method: string;
  parts: string[];
}

let scope: string | null = null;

/**
 * inNode runs call with node as the scope of every request it starts. The
 * scope is read when a request starts, so it covers what call sends before
 * its first await, which is every request the functions in api.ts make.
 */
export function inNode<T>(node: string | null, call: () => T): T {
  if (node === null) return call();
  const outer = scope;
  scope = node;
  try {
    return call();
  } finally {
    scope = outer;
  }
}

/** The node the request being started is meant for, null for home. */
export function currentNode(): string | null {
  return scope;
}

export function parseRoutes(list: string[]): NodeRoute[] {
  return list.map((route) => {
    const [method, path] = route.split(" ");
    return { method, parts: path.split("/") };
  });
}

function matches(route: NodeRoute, method: string, parts: string[]): boolean {
  if (route.method !== method || route.parts.length !== parts.length) return false;
  return route.parts.every((part, i) => (part.startsWith("{") ? parts[i] !== "" : part === parts[i]));
}

/** nodeTarget is where a request for node goes, or null when the node does
 *  not take its route and it stays at home. */
export function nodeTarget(routes: NodeRoute[], node: string, method: string, path: string): string | null {
  const at = path.indexOf("?");
  const bare = at < 0 ? path : path.slice(0, at);
  const query = at < 0 ? "" : path.slice(at);
  const parts = bare.split("/");
  if (!routes.some((route) => matches(route, method.toUpperCase(), parts))) return null;
  return `/api/nodes/${encodeURIComponent(node)}/api/${bare.slice("/api/".length)}${query}`;
}

let routes: Promise<NodeRoute[]> | null = null;

/** The routes home passes to a node, read once. A read that fails is tried
 *  again with the next request. */
export function nodeRoutes(): Promise<NodeRoute[]> {
  routes ??= fetch(withinBase("/api/nodes/routes"))
    .then((res) => (res.ok ? (res.json() as Promise<{ routes?: string[] }>) : Promise.reject(new Error(String(res.status)))))
    .then((body) => parseRoutes(body.routes ?? []))
    .catch((err: unknown) => {
      routes = null;
      throw err;
    });
  return routes;
}
