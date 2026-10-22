import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Settings } from "./api";
import { putSettingsPatch } from "./settingsPatch";
import { rebaseSettings } from "../pages/settings/settingsStore";

const calls: { body: Settings; revision?: string }[] = [];
let answers: { ok: boolean; code?: string; error?: string; revision?: string }[] = [];
let onServer: Settings;

vi.mock("./api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./api")>();
  return {
    ...actual,
    putSettings: (body: Settings, revision?: string) => {
      calls.push({ body, revision });
      return Promise.resolve(answers.shift() ?? { ok: true });
    },
    getSettings: () => Promise.resolve({ ok: true, settings: onServer, hostMountRoot: "/host/user", platform: "unraid", revision: "r2" }),
  };
});

const tabCopy = { containersPath: "backups/containers", containersOffsite: "rest:http://box:8000/c", catchUpMissed: false } as Settings;

beforeEach(() => {
  calls.length = 0;
  answers = [];
  onServer = { ...tabCopy, containersOffsite: "" };
});

describe("putSettingsPatch", () => {
  it("lays the patch on a fresh read when the copy was stale", async () => {
    answers = [{ ok: false, code: "settings-stale", error: "stale" }, { ok: true, revision: "r3" }];
    const out = await putSettingsPatch(tabCopy, "r1", { catchUpMissed: true });
    expect(calls.map((c) => c.revision)).toEqual(["r1", "r2"]);
    expect(calls[1].body).toEqual({ ...onServer, catchUpMissed: true });
    expect(out.res.ok).toBe(true);
    expect(out.sent.containersOffsite).toBe("");
    expect(out.revision).toBe("r3");
    expect(out.fresh).toEqual(onServer);
  });

  it("sends once when the copy is current", async () => {
    answers = [{ ok: true, revision: "r2" }];
    const out = await putSettingsPatch(tabCopy, "r1", { catchUpMissed: true });
    expect(calls).toHaveLength(1);
    expect(out.revision).toBe("r2");
    expect(out.fresh).toBeUndefined();
  });

  it("does not try again after another refusal", async () => {
    answers = [{ ok: false, error: "bad path" }];
    const out = await putSettingsPatch(tabCopy, "r1", { containersPath: "/etc" });
    expect(calls).toHaveLength(1);
    expect(out.res.error).toBe("bad path");
    expect(out.revision).toBe("r1");
  });
});

describe("rebaseSettings", () => {
  it("takes what changed elsewhere and keeps an edit of the page", () => {
    const live = { ...tabCopy, containersPath: "backups/typed" } as Settings;
    const next = rebaseSettings(live, tabCopy, onServer);
    expect(next.containersOffsite).toBe("");
    expect(next.containersPath).toBe("backups/typed");
  });
});
