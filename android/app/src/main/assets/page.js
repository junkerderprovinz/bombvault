// Runs before every page the app shows. bombvaultApp is the app's message
// port; the app only listens to it from the launcher and from saved servers.
(() => {
  const app = window.bombvaultApp;
  if (!app) return;
  const send = (msg) => app.postMessage(JSON.stringify(msg));

  // The interface saves exports by clicking a link to a blob: URL and revoking
  // it right after. A WebView hands such a link to the app only after the
  // revoke, when there is nothing left to read, so the click is caught here
  // while the blob still exists.
  const blobs = new Map();
  const createObjectURL = URL.createObjectURL;
  URL.createObjectURL = (o) => {
    const url = createObjectURL(o);
    if (o instanceof Blob) blobs.set(url, o);
    return url;
  };
  const revokeObjectURL = URL.revokeObjectURL;
  URL.revokeObjectURL = (url) => {
    blobs.delete(url);
    revokeObjectURL(url);
  };
  const click = HTMLAnchorElement.prototype.click;
  HTMLAnchorElement.prototype.click = function () {
    const blob = blobs.get(this.href);
    if (!blob || !this.hasAttribute("download")) return click.call(this);
    const reader = new FileReader();
    reader.onload = () => {
      const data = String(reader.result);
      send({ op: "file", name: this.download || "download", type: blob.type, data: data.slice(data.indexOf(",") + 1) });
    };
    reader.readAsDataURL(blob);
  };

  // The system bars take the page's own colour, which follows its theme.
  const bars = () => {
    const meta = document.querySelector('meta[name="theme-color"]');
    if (meta) send({ op: "bars", color: meta.content });
  };
  document.addEventListener("DOMContentLoaded", () => {
    bars();
    new MutationObserver(bars).observe(document.head, { subtree: true, childList: true, attributes: true, attributeFilter: ["content"] });
  });
  // A page brought back from the cache by Back loads no DOM again.
  window.addEventListener("pageshow", (e) => {
    if (e.persisted) bars();
  });
})();
