// ---------------------------------------------------------------------------
// Routed pages — the coverage guard for bombvault/page-uses-page-shell.
//
// That lint rule can only govern what it is pointed at: files matching
// src/pages/*.tsx, and inside each one, the component named for the file
// (`Config`, `SettingsPage`, or the default export). Both halves of that are
// conventions in their own right, and both are invisible to the rule itself —
// a page added under src/components/, or a page component named something
// unrelated to its file, is simply never looked at. It would pass lint while
// rendering at whatever width it liked, which is exactly the state
// lib/pageShell.ts was written to end.
//
// So this test asserts the rule's scope still matches reality:
//   1. every component used as a <Route element={…}> comes from ../pages/…
//   2. every page module the router imports actually exists
//   3. each one exports a component the rule's name lookup will find
//   4. the exception list in eslint.config.js names only real files
//
// Node environment, no DOM: this reads source text, it does not render.
// ---------------------------------------------------------------------------
import { readFileSync, existsSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const HERE = dirname(fileURLToPath(import.meta.url));
const SRC = join(HERE, "..");
const WEB = join(SRC, "..");

const router = readFileSync(join(HERE, "router.tsx"), "utf8");

/** `<Route path="/x" element={<Fleet />} />` → "Fleet". `<Navigate>` is not a page. */
function routedComponents(): string[] {
  const names = new Set<string>();
  for (const m of router.matchAll(/element=\{<\s*([A-Za-z][A-Za-z0-9_]*)\b/g)) {
    if (m[1] === "Navigate" || m[1] === "Layout") continue;
    names.add(m[1]);
  }
  return [...names].sort();
}

/** `import { Fleet } from "../pages/Fleet";` / `import Recovery from "…"`. */
function importSource(name: string): string | undefined {
  const named = new RegExp(`import\\s*\\{[^}]*\\b${name}\\b[^}]*\\}\\s*from\\s*"([^"]+)"`);
  const def = new RegExp(`import\\s+${name}\\s+from\\s*"([^"]+)"`);
  return (named.exec(router) ?? def.exec(router))?.[1];
}

/** The names bombvault/page-uses-page-shell will look for in `<stem>.tsx`. */
function ruleWillFind(stem: string, source: string): boolean {
  return (
    new RegExp(`export\\s+function\\s+(${stem}|${stem}Page)\\b`).test(source) ||
    new RegExp(`export\\s+const\\s+(${stem}|${stem}Page)\\b`).test(source) ||
    /export\s+default\s+(function|\(|[A-Za-z])/.test(source)
  );
}

const ROUTED = routedComponents();

describe("routed pages stay inside the page-shell rule's scope", () => {
  it("finds the routes at all (guards against this test silently matching nothing)", () => {
    expect(ROUTED.length).toBeGreaterThanOrEqual(9);
  });

  it.each(ROUTED)("%s is imported from src/pages", (name) => {
    const source = importSource(name);
    expect(
      source,
      `router.tsx routes to <${name} /> but no import for it was found. Every routed page must be imported from "../pages/<Name>".`
    ).toBeDefined();
    expect(
      source,
      `<${name} /> is routed from "${source}". bombvault/page-uses-page-shell only governs src/pages/*.tsx, so a page anywhere else silently escapes the shared page shell — move it to src/pages/.`
    ).toMatch(/^\.\.\/pages\//);
  });

  it.each(ROUTED)("%s's module exists and exposes a component the rule can see", (name) => {
    const rel = importSource(name);
    if (!rel) return; // already reported by the test above
    const stem = rel.slice(rel.lastIndexOf("/") + 1);
    const file = join(SRC, "pages", `${stem}.tsx`);
    expect(existsSync(file), `${file} does not exist, but router.tsx imports it.`).toBe(true);
    const text = readFileSync(file, "utf8");
    expect(
      ruleWillFind(stem, text),
      `src/pages/${stem}.tsx has no export named "${stem}" or "${stem}Page" and no default export. ` +
        `bombvault/page-uses-page-shell locates a page component by those names, so this page would never be checked. ` +
        `Rename the exported component to match its file.`
    ).toBe(true);
  });
});

describe("the page-shell exception list is real", () => {
  const config = readFileSync(join(WEB, "eslint.config.js"), "utf8");

  it("declares the exceptions in eslint.config.js rather than inferring them", () => {
    expect(config).toContain("bombvault/page-uses-page-shell");
    expect(config).toContain("exceptions:");
  });

  it("names only files that exist", () => {
    const block = /exceptions:\s*\{([\s\S]*?)\n\s{10}\},/.exec(config)?.[1] ?? "";
    const named = [...block.matchAll(/"([A-Za-z0-9_.]+\.tsx)"\s*:/g)].map((m) => m[1]);
    expect(named.length, "no exception entries were parsed out of eslint.config.js").toBeGreaterThan(0);
    for (const f of named) {
      expect(
        existsSync(join(SRC, "pages", f)),
        `eslint.config.js exempts src/pages/${f} from the page shell, but that file does not exist. ` +
          `A stale exception quietly exempts nothing and hides that the list is out of date — remove it.`
      ).toBe(true);
    }
  });
});
