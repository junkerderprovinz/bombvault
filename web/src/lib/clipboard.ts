// ---------------------------------------------------------------------------
// clipboard — one robust copy helper for every "Copy" button (#112).
//
// navigator.clipboard exists ONLY in secure contexts (HTTPS with an accepted
// certificate). Reaching BombVault over plain HTTP or through a proxy that
// downgrades the context leaves it undefined — the old per-button handlers then
// threw inside their try, swallowed the error, and the button silently did
// nothing (issue #112). This helper adds the classic execCommand("copy")
// fallback (a temporary off-screen textarea), which works in non-secure
// contexts, and reports success so callers only show "copied" when the text
// actually reached the clipboard.
// ---------------------------------------------------------------------------

/**
 * copyText copies `text` to the clipboard. Tries the async Clipboard API
 * first (secure contexts), then falls back to execCommand("copy"). Returns
 * true when either path succeeded — callers show their "copied" feedback only
 * then, and can e.g. select the source field instead when it failed.
 */
export async function copyText(text: string): Promise<boolean> {
  const nav = (globalThis as { navigator?: { clipboard?: { writeText(t: string): Promise<void> } } }).navigator;
  if (nav?.clipboard?.writeText) {
    try {
      await nav.clipboard.writeText(text);
      return true;
    } catch {
      // Permission denied or transient failure — fall through to execCommand.
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
  // Off-screen but focusable — display:none would make select() a no-op.
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
