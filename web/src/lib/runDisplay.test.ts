// runTargetText's naming contract for the Backup Everything parent run.
//
// The backend names the parent run's target with its own server-side
// literal, which is English text no matter the UI language; every other run
// kind resolves its target through a translated key or a user-chosen name.
// The everything pass is one action with one name (settings.everythingTitle,
// the same key the effective-schedule sentence uses), so the run sheet's
// title reads Gesamt-Backup in German instead of gluing the untranslated
// literal onto the translated kind. The fake t returns the key itself, so
// the assertion names the exact key the text must come from.
import { describe, expect, it } from "vitest";
import type { Run } from "./api";
import { runTargetText } from "./runDisplay";

const t = ((key: string) => key) as unknown as Parameters<typeof runTargetText>[0];

function makeRun(over: Partial<Run>): Run {
  return {
    id: "run-1",
    targetId: "everything",
    kind: "backup",
    status: "success",
    startedAt: 1,
    finishedAt: 2,
    snapshotId: "abc12345",
    bytes: 0,
    error: "",
    acknowledged: false,
    target: "Backup Everything",
    domain: "everything",
    ...over,
  };
}

describe("runTargetText", () => {
  it("names the everything parent run from settings.everythingTitle, never the raw server name", () => {
    expect(runTargetText(t, makeRun({}))).toBe("settings.everythingTitle");
  });

  it("leaves ordinary runs on their own target name", () => {
    expect(runTargetText(t, makeRun({ domain: "container", target: "plex" }))).toBe("plex");
  });
});
