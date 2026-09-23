// navigator.clipboard exists only in secure contexts (HTTPS with an accepted
// certificate), so over plain HTTP, or behind a proxy that downgrades the
// context, a Copy button relying on it does nothing. copyText falls back to
// execCommand("copy") on a temporary off-screen textarea, which works there.

/**
 * copyText copies `text` to the clipboard, trying the async Clipboard API
 * first and execCommand("copy") second. It returns true when either worked,
 * so callers show "copied" only then and can, say, select the source field
 * when it failed.
 */
export async function copyText(text: string): Promise<boolean> {
  const nav = (globalThis as { navigator?: { clipboard?: { writeText(t: string): Promise<void> } } }).navigator;
  if (nav?.clipboard?.writeText) {
    try {
      await nav.clipboard.writeText(text);
      return true;
    } catch {
      // Permission denied or a transient failure: fall through to execCommand.
    }
  }
  return execCommandCopy(text);
}

/** Legacy fallback: temporary textarea + document.execCommand("copy"). */
function execCommandCopy(text: string): boolean {
  const doc = (globalThis as {
    document?: {
      createElement(tag: string): {
        value: string;
        style: Record<string, string>;
        setAttribute(name: string, value: string): void;
        select(): void;
        remove(): void;
      };
      body: { appendChild(el: unknown): void };
      execCommand?(cmd: string): boolean;
    };
  }).document;
  if (!doc?.execCommand) return false;
  const ta = doc.createElement("textarea");
  ta.value = text;
  // Off-screen rather than display:none, which would make select() a no-op.
  ta.style.position = "fixed";
  ta.style.top = "-1000px";
  ta.style.opacity = "0";
  ta.setAttribute("readonly", "");
  doc.body.appendChild(ta);
  try {
    ta.select();
    return doc.execCommand("copy");
  } catch {
    return false;
  } finally {
    ta.remove();
  }
}
