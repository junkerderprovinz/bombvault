// A backup that waits for its app reads as deferred, in amber, with its next
// try, everywhere an entry's state is shown: the activity log, the list of
// backups, the entry's own line and the overview's first card.
import { describe, expect, it } from "vitest";
import { buildLogLines } from "./activityLog";
import type { BackupItem, DeferredBackup, Run } from "./api";
import { protection, rowStatus, runStrip } from "./backupList";
import { deferralSentence, deferralTime } from "./deferral";
import { entryReason } from "./entry";
import { countText, de, en } from "./i18n";
import { needsOf } from "./needs";
import { formatClockTime } from "./reltime";
import { runReason } from "./runReason";
import { statusLabel, statusTone } from "./runDisplay";

const resolveName = (key: string, params?: Record<string, string>): string =>
  params
    ? `${key} ${Object.entries(params)
        .map(([k, v]) => `${k}=${v}`)
        .join(" ")}`
    : key;

const t = ((key: keyof typeof en, n?: number) => countText(en[key], "en", n)) as Parameters<typeof deferralSentence>[0];

const NOW = Math.floor(new Date(2026, 9, 24, 1, 0).getTime() / 1000);

const waiting: DeferredBackup = {
  domain: "containers",
  name: "plex",
  reason: "Plex has 2 active sessions",
  since: NOW,
  nextTry: NOW + 1800,
  deadline: NOW + 12 * 3600,
  atDeadline: "give-up",
  tries: 1,
};

function item(over: Partial<BackupItem> = {}): BackupItem {
  return {
    kind: "container",
    key: "plex",
    name: "plex",
    included: true,
    paused: false,
    effectiveSchedule: { kind: "domain", spec: "daily 03:00" },
    lastBackup: NOW - 86400,
    lastRunStatus: "deferred",
    sourceBytes: 1,
    installed: true,
    runs: [{ id: "r2", kind: "backup", status: "deferred", startedAt: NOW }, { id: "r1", kind: "backup", status: "success", startedAt: NOW - 86400 }],
    deferral: waiting,
    ...over,
  };
}

function run(over: Partial<Run>): Run {
  return {
    id: "r2",
    targetId: "c-1",
    kind: "backup",
    status: "deferred",
    startedAt: NOW,
    finishedAt: NOW + 2,
    snapshotId: "",
    bytes: 0,
    error: "Plex has 2 active sessions",
    acknowledged: false,
    target: "plex",
    domain: "container",
    ...over,
  };
}

describe("a deferred backup", () => {
  it("says deferred in amber in the activity log with its next try", () => {
    const [line] = buildLogLines([run({ retryAt: NOW + 1800 })], {}, [], resolveName, (NOW + 10) * 1000);
    expect(line.warn).toBe(true);
    expect(line.text).toBe(
      `activityLog.lineBackupDeferred name=plex error=runReason.deferredSessions time=${formatClockTime(NOW + 1800, false)}`
    );
  });

  it("still reads as deferred in the log before its next try is known", () => {
    const [line] = buildLogLines([run({ retryAt: null })], {}, [], resolveName, (NOW + 10) * 1000);
    expect(line.text).toBe("activityLog.lineBackupDeferredLater name=plex error=runReason.deferredSessions");
  });

  it("is amber in the list of backups and on its run strip", () => {
    expect(rowStatus(item(), 0)).toEqual({ is: "deferred", nextTry: NOW + 1800 });
    expect(runStrip(item(), () => false).slice(-2)).toEqual(["ok", "warn"]);
    expect(statusTone("deferred")).toBe("warn");
    expect(statusLabel("deferred", t)).toBe(en["run.statusDeferred"]);
  });

  it("leads the entry's line and keeps it protected", () => {
    expect(entryReason(item())).toEqual({ is: "deferred", nextTry: NOW + 1800 });
    expect(protection(item())).toBe("ok");
  });

  it("stands under Needs you after what needs a hand, unless its entry failed", () => {
    const needs = needsOf({ items: [item()], runs: [], locations: [], anomalies: [], kitDue: false });
    expect(needs).toEqual([{ is: "deferred", item: item(), deferral: waiting }]);
    const failed = run({ id: "r2", status: "failed", error: "boom" });
    const withFailure = needsOf({ items: [item({ runs: [{ id: "r2", kind: "backup", status: "failed", startedAt: NOW }] })], runs: [failed], locations: [], anomalies: [], kitDue: false });
    expect(withFailure.map((n) => n.is)).toEqual(["failedRun"]);
  });

  it("names its reason, next try and deadline in one sentence", () => {
    const text = deferralSentence(t, waiting, NOW * 1000);
    expect(text).toContain("Plex has 2 active sessions");
    expect(text).toContain(formatClockTime(NOW + 1800, false));
    expect(text).toMatch(/^[^{}]+$/);
    expect(text.startsWith("Deferred:")).toBe(true);
    expect(text).toContain("Gives up at");
  });

  it("puts the date in front of a time that is not today", () => {
    expect(deferralTime(NOW + 1800, NOW * 1000)).toBe(formatClockTime(NOW + 1800, false));
    expect(deferralTime(NOW + 30 * 3600, NOW * 1000)).not.toBe(formatClockTime(NOW + 30 * 3600, false));
  });
});

describe("why a backup waits", () => {
  const tIn = (table: Record<string, string>, lang: string) =>
    ((key: string, n?: number) => countText(table[key] ?? key, lang, n)) as Parameters<typeof runReason>[1];

  it("reads in the reader's language with the right plural", () => {
    const tEn = tIn(en, "en");
    const tDe = tIn(de, "de");
    expect(runReason("Plex has 1 active session", tEn)).toBe("Plex has 1 active session");
    expect(runReason("Plex has 3 active sessions", tDe)).toBe("Plex hat 3 aktive Sitzungen");
    expect(runReason("Jellyfin did not answer", tDe)).toBe("Jellyfin hat nicht geantwortet");
    expect(runReason("Jellyfin has no API key set", tDe)).toBe("für Jellyfin ist kein API-Schlüssel eingetragen");
    expect(runReason("the container sent 100 KiB/s or more within the last 1 minute", tEn)).toBe(
      "the container sent 100 KiB/s or more within the last minute"
    );
    expect(runReason("the container sent 100 KiB/s or more within the last 5 minutes", tDe)).toBe(
      "der Container hat in den letzten 5 Minuten 100 KiB/s oder mehr gesendet"
    );
    expect(runReason("BombVault has watched the container's traffic for less than 1 minute", tDe)).toBe(
      "BombVault beobachtet den Verkehr des Containers seit weniger als einer Minute"
    );
    expect(runReason("the pre-backup command says the app is busy: 2 streams", tDe)).toBe(
      "der Befehl vor der Sicherung meldet, dass die App beschäftigt ist: 2 streams"
    );
    expect(runReason("the pre-backup command says the app is busy", tDe)).toBe("der Befehl vor der Sicherung meldet, dass die App beschäftigt ist");
  });
});

describe("a backup that gave up at its deadline", () => {
  it("counts as not protected, as the coverage does", () => {
    expect(protection(item({ deferral: undefined, deferralGaveUp: true, lastRunStatus: "failed" }))).toBe("none");
  });
});
