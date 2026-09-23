// Vitest setup; guarded no-op window.ResizeObserver stub.
//
// jsdom does not implement ResizeObserver, and Selector observes the row a
// pinned well sits in (see its rowFill header) the moment such a strip mounts,
// which is every suite that renders the appearance card. Wired via
// test.setupFiles in vitest.config.ts.
//
// A no-op is the honest stub: jsdom reports every box as zero, so a callback
// would only hand rowFill an empty row, which it already answers by keeping
// the pinned width. Suites that drive a resize of their own install their own
// stub, and those keep winning because this one installs only where nothing is
// there yet. The guard also covers the node-env suites, where window itself is
// undefined.
if (typeof window !== "undefined" && typeof window.ResizeObserver !== "function") {
  window.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
}
