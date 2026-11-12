// A page opened through the fortress runs in a sandbox with an origin of its
// own, where the browser answers localStorage, sessionStorage and
// document.cookie with a SecurityError. Every entry imports this first: where
// touching one throws, it is replaced by a stand-in that keeps what is written
// for as long as the page stays open. Where they work, nothing changes.

class MemoryStorage {
  private items = new Map<string, string>();

  get length(): number {
    return this.items.size;
  }

  clear(): void {
    this.items.clear();
  }

  getItem(key: string): string | null {
    return this.items.get(String(key)) ?? null;
  }

  key(index: number): string | null {
    return [...this.items.keys()][index] ?? null;
  }

  removeItem(key: string): void {
    this.items.delete(String(key));
  }

  setItem(key: string, value: string): void {
    this.items.set(String(key), String(value));
  }
}

function refused(touch: () => unknown): boolean {
  try {
    touch();
    return false;
  } catch {
    return true;
  }
}

/** guardStorage puts the stand-ins in place of what the browser refuses. */
export function guardStorage(): void {
  for (const name of ["localStorage", "sessionStorage"] as const) {
    if (refused(() => window[name].length)) {
      Object.defineProperty(window, name, { value: new MemoryStorage(), configurable: true });
    }
  }
  if (refused(() => document.cookie)) {
    const cookies = new Map<string, string>();
    Object.defineProperty(document, "cookie", {
      configurable: true,
      get: () => [...cookies].map(([name, value]) => `${name}=${value}`).join("; "),
      set: (line: string) => {
        const [pair, ...attributes] = String(line).split(";");
        const at = pair.indexOf("=");
        if (at < 1) return;
        const name = pair.slice(0, at).trim();
        const gone = attributes.some((a) => /^\s*max-age\s*=\s*(0|-)/i.test(a));
        if (gone) cookies.delete(name);
        else cookies.set(name, pair.slice(at + 1).trim());
      },
    });
  }
}

guardStorage();
