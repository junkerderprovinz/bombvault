// @vitest-environment jsdom
// Overview carries the count of what needs the person. It wears the entry's
// colour like the count on Anomalies, and keeps out of the way of a centred
// glyph when the rail shows no labels.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";

import { Sidebar } from "./Sidebar";
import { AttentionProvider } from "../lib/attentionProvider";
import { setLabelMode } from "../lib/controls";
import { I18nProvider, en } from "../lib/i18n";
import type { BackupItem, Run, Settings } from "../lib/api";

vi.mock("../lib/api", async () => {
  const actual = await vi.importActual<typeof import("../lib/api")>("../lib/api");
  return { ...actual, listRuns: vi.fn(), listItems: vi.fn() };
});

const api = await import("../lib/api");
const listRuns = vi.mocked(api.listRuns);
const listItems = vi.mocked(api.listItems);

function failedRun(targetId: string): Run {
  return {
    id: `run-${targetId}`,
    targetId,
    kind: "backup",
    status: "failed",
    startedAt: 1_700_000_000,
    finishedAt: 1_700_000_500,
    snapshotId: "",
    bytes: 0,
    error: "restic exited 1",
    acknowledged: false,
    target: targetId,
    domain: "container",
  };
}

/** A container no schedule reaches and that has no backup. */
const paperless: BackupItem = {
  kind: "container",
  key: "paperless",
  name: "paperless",
  included: false,
  paused: false,
  effectiveSchedule: { kind: "none", spec: "", reason: "excluded" },
  lastBackup: 0,
  lastRunStatus: "",
  sourceBytes: 1,
  installed: true,
  runs: [],
};

/** A container with a backup, which makes the install one that is in use. */
const plex: BackupItem = { ...paperless, key: "plex", name: "plex", lastBackup: 1_700_000_000 };

const kitDue = { encryptionEnabled: true, recoveryKitAck: false } as Settings;
const calm = { encryptionEnabled: true, recoveryKitAck: true } as Settings;

function renderRail(settings: Settings, path = "/") {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <I18nProvider>
        <AttentionProvider settings={settings}>
          <Sidebar settings={settings} authEnabled={false} />
        </AttentionProvider>
      </I18nProvider>
    </MemoryRouter>
  );
}

const label = (n: number) => en["nav.attentionCountAria"].replace("{n}", String(n));

async function overviewCount(n: number): Promise<HTMLElement> {
  const entry = screen.getByRole("link", { name: new RegExp(`^${en["nav.overview"]}`) });
  return within(entry).findByLabelText(label(n));
}

beforeEach(() => {
  localStorage.clear();
  listRuns.mockReset().mockResolvedValue({ ok: true, runs: [failedRun("gitea"), failedRun("immich")] });
  listItems.mockReset().mockResolvedValue({ ok: true, items: [paperless, plex], unlisted: [] });
});

afterEach(cleanup);

describe("the count on Overview", () => {
  it("adds up the failures, the unprotected entries and the recovery kit reminder", async () => {
    renderRail(kitDue);
    expect((await overviewCount(4)).textContent).toBe("4");
  });

  it("is absent while nothing needs the person", async () => {
    listRuns.mockResolvedValue({ ok: true, runs: [] });
    listItems.mockResolvedValue({ ok: true, items: [], unlisted: [] });
    renderRail(calm);
    await vi.waitFor(() => expect(listItems).toHaveBeenCalled());
    expect(screen.getByRole("link", { name: en["nav.overview"] }).textContent).toBe(en["nav.overview"]);
  });

  it("is absent before the first backup, whatever waits", async () => {
    listItems.mockResolvedValue({ ok: true, items: [paperless], unlisted: [] });
    renderRail(kitDue);
    await vi.waitFor(() => expect(listItems).toHaveBeenCalled());
    await act(async () => {});
    expect(screen.getByRole("link", { name: en["nav.overview"] }).textContent).toBe(en["nav.overview"]);
  });

  it("takes its colours from the entry it stands on, with none of its own", async () => {
    renderRail(calm);
    const count = await overviewCount(3);
    const row = count.closest("a")!;
    // index.css paints `.glim-nav-count` from the accent the row's hue hands
    // down, and turns the pair round under `.glim-active`.
    expect(row.className).toContain("glim-hue");
    expect(count.className).toContain("glim-nav-count");
    expect(count.className).not.toContain("glim-nav-count--quiet");
    expect(count.className).not.toMatch(/status|\bbg-|\btext-(?!dense|caption)/);
  });

  it("stands inside the filled row on the current entry", async () => {
    renderRail(calm, "/dashboard");
    const count = await overviewCount(3);
    expect(count.closest(".glim-active")).toBe(count.closest("a"));
  });
});

describe("the count and the label modes", () => {
  it.each(["text", "textGlyph"] as const)("stands at the end of the row in %s mode", async (mode) => {
    setLabelMode("sidebar", mode);
    renderRail(calm);
    const count = await overviewCount(3);
    expect(count.className).toContain("ms-auto");
    expect(count.className).not.toContain("absolute");
  });

  it("hangs on the corner of the narrow rail, out of the flow that centres the glyph", async () => {
    setLabelMode("sidebar", "glyph");
    renderRail(calm);
    const count = await overviewCount(3);
    expect(count.className).toContain("absolute");
    expect(count.className).not.toContain("ms-auto");
    expect(count.parentElement?.className).toContain("relative");
    expect(count.parentElement?.className).toContain("justify-center");
  });

  it("stays at the end of the reactive row without moving its glyph off centre", async () => {
    setLabelMode("sidebar", "reactive");
    renderRail(calm);
    const count = await overviewCount(3);
    expect(count.className).toContain("absolute");
    expect(count.className).toContain("end-3");
  });
});
