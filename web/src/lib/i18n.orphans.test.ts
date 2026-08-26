// ---------------------------------------------------------------------------
// Orphaned translation keys — the guard that makes a dead key cost something.
//
// A key nobody renders is not free here. The parity test requires all 42 tables
// to carry the SAME key set, so every orphan is 42 dead strings, and every
// locale added later pays to translate it again. Five of them survived this
// branch's own card and sweep rework — the Sidebar's theme toggle moved into
// ThemeCard (theme.toggle), the image-cleanup card was retitled
// (settings.imageCleanupTitle/-Hint), the rainbow switch's label became
// settings.rainbow (settings.rainbowOn) and the reconcile card lost its
// separate heading (settings.reconcileTitle) — and each was translated 42 times
// over on the way out.
//
// The scan is deliberately CONSERVATIVE: it reads every .ts/.tsx file under
// src/ except the tables themselves, TEST FILES INCLUDED. A guard that
// false-positives gets switched off, so a key some test still names counts as
// used; the case worth catching is a key with no reference anywhere at all.
//
// Dynamically composed keys are resolved rather than guessed at: every
// t(`…${…}…`) template in the tree contributes a pattern built from its static
// chunks, so t(`integrity.${a.key}Hint`) marks integrity.<anything>Hint used
// without marking the whole integrity.* namespace used.
// ---------------------------------------------------------------------------
import { describe, expect, it } from "vitest";
import { readFileSync, readdirSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { en } from "./i18n";

const SRC = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const TABLES = [join(SRC, "lib", "i18n.ts"), join(SRC, "lib", "locales")];

/** Every .ts/.tsx file under src/, minus the translation tables themselves. */
function sourceFiles(dir: string): string[] {
  const out: string[] = [];
  for (const e of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, e.name);
    if (TABLES.some((t) => p === t || p.startsWith(t + "\\") || p.startsWith(t + "/"))) continue;
    if (e.isDirectory()) out.push(...sourceFiles(p));
    else if (e.name.endsWith(".ts") || e.name.endsWith(".tsx")) out.push(p);
  }
  return out;
}

const CORPUS = sourceFiles(SRC)
  .map((p) => readFileSync(p, "utf8"))
  .join("\n");

/** t(`a.${x}b`) → /^a\..+b$/ — the keys that template can actually produce. */
function dynamicKeyPatterns(corpus: string): RegExp[] {
  const out: RegExp[] = [];
  for (const m of corpus.matchAll(/\bt\(`([^`]*\$\{[^`]*)`/g)) {
    const chunks = m[1].split(/\$\{[^}]*\}/);
    if (chunks.length < 2) continue;
    const body = chunks.map((c) => c.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")).join(".+");
    out.push(new RegExp("^" + body + "$"));
  }
  return out;
}

const DYNAMIC = dynamicKeyPatterns(CORPUS);

// KNOWN_ORPHANS is a RATCHET, not an approval. These 58 keys were already dead
// before this branch, and clearing them is its own piece of work; what matters
// here is that the list cannot GROW. The assertion is one-directional on
// purpose: deleting one of these is free (nothing requires them to still
// exist), adding a new one is not.
const KNOWN_ORPHANS = new Set([
  "dashboard.recentRuns",
  "dashboard.spikeStatus",
  "dashboard.spikeLink",
  "dashboard.hostIntegrationCheck",
  "dashboard.blockBackups",
  "spike.probeFailed",
  "containers.colName",
  "containers.colImage",
  "containers.colStatus",
  "containers.colAppdata",
  "containers.colActions",
  "containers.backupStarted",
  "containers.noDestination",
  "containers.schedule",
  "snapshots.colId",
  "snapshots.colTime",
  "snapshots.colTags",
  "snapshots.colSize",
  "snapshots.files",
  "files.restore",
  "files.restored",
  "restore.confirmTitle",
  "restore.confirmBody",
  "restore.preview",
  "restore.toFolder",
  "stack.restored",
  "stack.memberRestored",
  "stack.memberStarted",
  "run.statusSuccess",
  "run.statusFailed",
  "run.colKind",
  "run.colStatus",
  "run.colStarted",
  "run.colFinished",
  "run.colContainer",
  "settings.scheduleOff",
  "settings.retentionLocal",
  "settings.retentionOffsite",
  "folders.customPlaceholder",
  "folders.save",
  "excludes.save",
  "excludes.resolvedTo",
  "folder.browse",
  "config.schedule",
  "config.offsiteSchedule",
  "receiver.readDataPercentHint",
  "state.created",
  "state.running",
  "state.paused",
  "state.restarting",
  "state.removing",
  "state.exited",
  "state.dead",
  "state.shutoff",
  "state.inshutdown",
  "state.crashed",
  "state.pmsuspended",
  "state.notInstalled",
]);

describe("translation keys", () => {
  it("finds the app's dynamic key templates (so the scan below is not silently blind)", () => {
    expect(DYNAMIC.length).toBeGreaterThan(0);
    expect(DYNAMIC.some((r) => r.test("integrity.unlockHint"))).toBe(true);
    expect(DYNAMIC.some((r) => r.test("settings.shape.round"))).toBe(true);
    // …and is not so loose that it excuses everything under a namespace.
    expect(DYNAMIC.some((r) => r.test("integrity.title"))).toBe(false);
  });

  it("adds no en key that nothing in the tree renders", () => {
    const orphans = Object.keys(en).filter(
      (key) =>
        !KNOWN_ORPHANS.has(key) &&
        !CORPUS.includes(`"${key}"`) &&
        !CORPUS.includes(`'${key}'`) &&
        !DYNAMIC.some((r) => r.test(key))
    );
    expect(
      orphans,
      `these keys are carried by all 42 locale tables and rendered by nothing: ${orphans.join(", ")}. ` +
        `Delete them from web/src/lib/i18n.ts and every file in web/src/lib/locales/.`
    ).toEqual([]);
  });
});
