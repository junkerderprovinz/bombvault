// Every en key has to be referenced somewhere in src. The parity test makes all
// 42 tables carry the same key set, so a dead key is 42 dead strings.
//
// The scan reads every .ts/.tsx file under src/ except the tables, tests
// included: a key some test still names counts as used, because a guard that
// raises false alarms gets switched off. Each t(`…${…}…`) template adds a
// pattern built from its static parts, so t(`integrity.${a.key}Hint`) marks
// integrity.<anything>Hint as used without covering all of integrity.*.
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

/** dynamicKeyPatterns turns each t(`a.${x}b`) template into /^a\..+b$/, the
 *  keys that template can produce. */
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

// KNOWN_ORPHANS lists keys that are already dead. It is a ratchet: removing an
// entry is free, since nothing requires them to exist, but it must not grow.
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
  it("finds the app's dynamic key templates", () => {
    expect(DYNAMIC.length).toBeGreaterThan(0);
    expect(DYNAMIC.some((r) => r.test("integrity.unlockHint"))).toBe(true);
    expect(DYNAMIC.some((r) => r.test("settings.shape.round"))).toBe(true);
    // A template must not excuse its whole namespace.
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
