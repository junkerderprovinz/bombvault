// Where BombVault's pages start: "/" at the root of its address, or the
// prefix a proxy in front or the fortress serves it under. The server names it
// in index.html's <base>, so every relative address of the page starts there,
// and the paths the code writes from the root are put under it here. Outside a
// page, as in the tests, the paths stay as they are.

const base = typeof document === "undefined" ? "/" : new URL(".", document.baseURI).pathname;

/** basePath is the path the pages start at, with a slash at its end. */
export function basePath(): string {
  return base;
}

/** withinBase puts one of BombVault's own paths, such as /api/health, under
 *  the base. Any other address stays as it is. */
export function withinBase(path: string): string {
  return path.startsWith("/") && !path.startsWith("//") ? base + path.slice(1) : path;
}

/** routerBase is the base as the router's basename. */
export function routerBase(): string {
  return base === "/" ? "/" : base.slice(0, -1);
}
