// Vitest setup; guarded no-op window.ResizeObserver stub.
//
// jsdom does not implement ResizeObserver, and Selector observes its strip
// and segments the moment one mounts, which is nearly every suite that renders
// a page. Wired via test.setupFiles in vitest.config.ts.
//
// A no-op is the honest stub: jsdom reports every box as zero, and Selector
// lays out nothing in a box that measures nothing. Suites that drive a resize
// of their own install their own stub, and those keep winning because this one
// installs only where nothing is there yet. The guard also covers the node-env
// suites, where window itself is undefined.
if (typeof window !== "undefined" && typeof window.ResizeObserver !== "function") {
  window.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
}
