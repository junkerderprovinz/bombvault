// @vitest-environment jsdom
// The protection card and the window of one section. The card is a row per
// switched-on section with one dot per check, and the window says what stands
// behind the dots and starts a trial restore.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";

import type { DomainStatus } from "../lib/api";
import { en } from "../lib/i18n";
import { domain, item, resetServer, server } from "./overview/overview.testsupport";
import { inApp } from "./overview/overviewApp.testsupport";
import { ProtectionCard } from "./overview/ProtectionCard";
import { sectionsOf, type Section } from "./overview/protectionChecks";
import { ProtectionWindow } from "./overview/ProtectionWindow";
import type { TrialRestore } from "./overview/useTrialRestore";

vi.mock("../lib/api", async (importOriginal) => {
  const { stagedApi } = await import("./overview/overview.testsupport");
  return stagedApi(await importOriginal<typeof import("../lib/api")>());
});

const t = ((key: string) => (en as Record<string, string>)[key] ?? key) as Parameters<typeof ProtectionCard>[0]["t"];

const sections = (domains: DomainStatus[], items = [item()]) => sectionsOf(domains, items, {}, null);

function renderCard(list: Section[], over: Partial<Parameters<typeof ProtectionCard>[0]> = {}) {
  const onOpen = vi.fn();
  render(inApp(<ProtectionCard t={t} lang="en" sections={list} loading={false} failed={false} onOpen={onOpen} {...over} />));
  return onOpen;
}

const row = (name: string) => within(screen.getByRole("button", { name }).closest("li")!);

beforeEach(resetServer);
afterEach(cleanup);

describe("ProtectionCard", () => {
  it("gives every section four dots, one per check", () => {
    renderCard(sections([domain(), domain({ domain: "vms" })]));

    for (const name of [en["nav.containers"], en["nav.vms"]]) {
      const dots = row(name).getAllByRole("img");
      expect(dots.map((d) => d.getAttribute("data-check"))).toEqual(["local", "offsite", "lock", "drill"]);
    }
  });

  it("says what each dot means to a screen reader and under the pointer", () => {
    renderCard(sections([domain()]));

    const names = row(en["nav.containers"])
      .getAllByRole("img")
      .map((d) => d.getAttribute("aria-label"));
    expect(names).toEqual([
      en["overview.check.localCurrent"],
      en["overview.check.offsiteCurrent"],
      en["overview.check.lockChecked"],
      en["overview.check.drillPassed"],
    ]);
  });

  it("marks a section without an off-site copy, and every such section", () => {
    renderCard(sections([domain({ offsiteConfigured: false }), domain({ domain: "flash", offsiteConfigured: false })], []));

    expect(screen.getAllByRole("img", { name: en["overview.check.offsiteNone"] })).toHaveLength(2);
    for (const dot of screen.getAllByRole("img", { name: en["overview.check.offsiteNone"] })) {
      expect(dot.getAttribute("data-tone")).toBe("fail");
    }
  });

  it("stays quiet about off-site once every entry lives off the premises", () => {
    renderCard(sections([domain({ offsiteConfigured: false, offPremisesCovered: true })]));

    expect(screen.queryByRole("img", { name: en["overview.check.offsiteNone"] })).toBeNull();
  });

  it("colours the section's own dot by its worst check", () => {
    renderCard(sections([domain({ replicationState: "overdue" }), domain({ domain: "zfs" })], [item(), item({ kind: "zfs", key: "z" })]));

    const tone = (name: string) =>
      screen.getByRole("button", { name }).closest("li")!.querySelector("[data-tone]")!.getAttribute("data-tone");
    expect(tone(en["nav.containers"])).toBe("warn");
    expect(tone(en["nav.zfs"])).toBe("ok");
  });

  it("counts the section's entries as every other card does", () => {
    renderCard(sections([domain()], [item(), item({ key: "new", lastBackup: 0 }), item({ key: "old", installed: false })]));

    const ratio = en["coverage.ratio"].replace("{protected}", "1").replace("{total}", "2");
    expect(row(en["nav.containers"]).getByText(ratio)).toBeTruthy();
  });

  it("opens a section", () => {
    const list = sections([domain()]);
    const onOpen = renderCard(list);

    fireEvent.click(screen.getByRole("button", { name: en["nav.containers"] }));

    expect(onOpen).toHaveBeenCalledWith(list[0]);
  });

  it("says it is still checking, and that the status did not load", () => {
    renderCard([], { loading: true });
    expect(screen.getByText(en["dashboard.checking"])).toBeTruthy();
    cleanup();

    renderCard([], { failed: true });
    expect(screen.getByText(en["dashboard.statusLoadFailed"])).toBeTruthy();
  });
});

describe("ProtectionWindow", () => {
  const idle: TrialRestore = { running: null, refused: {}, start: vi.fn() };

  function renderWindow(status: DomainStatus, trial: TrialRestore = idle, items = [item()]) {
    const [section] = sections([status], items);
    render(inApp(<ProtectionWindow section={section} trial={trial} t={t} onClose={() => {}} />));
    return within(screen.getByRole("dialog"));
  }

  it("lists the four checks with encryption and the prune rule", () => {
    const window = renderWindow(domain({ encryptionOn: false, pruneStrategySet: false }));

    for (const key of [
      "overview.check.localCurrent",
      "overview.check.offsiteCurrent",
      "overview.check.lockChecked",
      "overview.check.drillPassed",
      "overview.check.encryptionOff",
      "overview.check.pruneMissing",
    ] as const) {
      expect(window.getByText(en[key])).toBeTruthy();
    }
  });

  it("leads to the off-site settings from a missing off-site copy", () => {
    const window = renderWindow(domain({ offsiteConfigured: false }));

    const link = window.getByText(en["overview.check.offsiteNone"]).closest("a")!;
    expect(link.getAttribute("href")).toBe("/settings/offsite");
  });

  it("starts a trial restore for the section", () => {
    const start = vi.fn();
    const window = renderWindow(domain({ domain: "zfs" }), { ...idle, start }, [item({ kind: "zfs", key: "z" })]);

    fireEvent.click(window.getByRole("button", { name: en["overview.drill.again"] }));

    expect(start).toHaveBeenCalledWith("zfs");
  });

  it("offers the first trial restore where none has run", () => {
    const window = renderWindow(domain({ lastDrDrillAt: 0, lastDrDrillOK: false, drillState: "never" }));

    expect(window.getByRole("button", { name: en["overview.drill.start"] })).toBeTruthy();
  });

  it("says why the last trial restore failed", () => {
    const window = renderWindow(domain({ lastDrDrillOK: false, drillState: "failed", drillDetail: "pack 00 not readable" }));

    expect(window.getByText(en["overview.check.drillFailed"])).toBeTruthy();
    expect(window.getByText("pack 00 not readable")).toBeTruthy();
  });

  it("says why a trial restore left no result", () => {
    const window = renderWindow(domain(), { ...idle, refused: { containers: "the repository is busy" } });

    expect(window.getByText("the repository is busy")).toBeTruthy();
  });

  it("offers no trial restore without an off-site copy or for a section that cannot be restored into a sandbox", () => {
    expect(renderWindow(domain({ offsiteConfigured: false })).queryByRole("button", { name: en["overview.drill.again"] })).toBeNull();
    cleanup();
    const vms = renderWindow(domain({ domain: "vms" }), idle, [item({ kind: "vm", key: "win" })]);
    expect(vms.queryByRole("button", { name: en["overview.drill.again"] })).toBeNull();
    expect(vms.getByText(en["overview.check.drillUnavailable"])).toBeTruthy();
  });

  it("lists the section's entries with how each one stands", () => {
    const window = renderWindow(domain(), idle, [
      item(),
      item({ key: "new", name: "new", lastBackup: 0 }),
      item({ kind: "vm", key: "win", name: "win" }),
    ]);

    const plex = within(window.getByText("plex").closest("a")!);
    expect(plex.getByText(en["ransomware.protGreen"])).toBeTruthy();
    const fresh = within(window.getByText("new").closest("a")!);
    expect(fresh.getByText(en["backups.status.waiting"])).toBeTruthy();
    expect(window.queryByText("win")).toBeNull();
  });
});

describe("a trial restore started from the page", () => {
  it("runs against the off-site copy and reads the status again when it is back", async () => {
    const { useTrialRestore } = await import("./overview/useTrialRestore");
    let trial!: TrialRestore;
    function Probe() {
      trial = useTrialRestore("failed");
      return null;
    }
    const changed = vi.fn();
    window.addEventListener("bv:settings-changed", changed);
    render(<Probe />);

    act(() => trial.start("containers"));
    expect(trial.running).toBe("containers");
    await act(async () => {});

    expect(server.calls.find((c) => c.name === "runDrill")!.args).toEqual(["containers", "offsite", "dr"]);
    expect(trial.running).toBeNull();
    expect(changed).toHaveBeenCalledOnce();
    window.removeEventListener("bv:settings-changed", changed);
  });
});
