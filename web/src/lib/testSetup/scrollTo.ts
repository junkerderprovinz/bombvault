// Vitest setup; guarded no-op Element.prototype.scrollTo stub.
//
// jsdom has window.scrollTo but none on elements, and Zag's select machine
// scrolls its list back to the top whenever it closes, so every suite that
// picks from a SelectField would end in a TypeError. Wired via
// test.setupFiles in vitest.config.ts.
//
// jsdom lays nothing out, so there is no scroll position to move and a no-op
// loses nothing. The guard covers the node-env suites, where Element is
// undefined.
if (typeof Element !== "undefined" && typeof Element.prototype.scrollTo !== "function") {
  Element.prototype.scrollTo = function scrollTo() {};
}
