// @vitest-environment jsdom
// The first steps on the overview of a new install: which of the three is
// next, what it asks for, and when the card goes.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { useLocation } from "react-router-dom";

import type { BackupItem, Settings } from "../lib/api";
import { NIGHTLY } from "../lib/firstSections";
import { en } from "../lib/i18n";
import { Dashboard } from "./Dashboard";
import { FakeEventSource, installMatchMedia, item, location, resetServer, server } from "./overview/overview.testsupport";
import { inApp } from "./overview/overviewApp.testsupport";

vi.mock("../lib/api", async (importOriginal) => {
  const { stagedApi } = await import("./overview/overview.testsupport");
  return stagedApi(await importOriginal<typeof import("../lib/api")>());
});

vi.mock("../components/storage/AddLocationWindow", () => ({
  AddLocationWindow: ({ domains, onClose }: { domains: string[]; onClose: () => void }) => (
    <div role="dialog" aria-label="add a location" data-domains={domains.join(" ")}>
      <button onClick={onClose}>close</button>
    </div>
  ),
}));

const CHOSEN = "bombvault.firstSteps.sectionsChosen";

const SECTIONS = {
  containersEnabled: true,
  vmsEnabled: false,
  flashEnabled: false,
  filesEnabled: false,
  zfsEnabled: false,
  configEnabled: false,
  containersSchedule: "off",
  vmsSchedule: "off",
  flashSchedule: "off",
  filesSchedule: "off",
  zfsSchedule: "off",
  configSchedule: "off",
  everythingSchedule: "off",
} as Partial<Settings>;

const unscheduled = { kind: "none", spec: "", reason: "schedule-off" } as const;

/** A container that was found, is in no schedule and has never been backed up. */
const found = (key: string, over: Parameters<typeof item>[0] = {}) =>
  item({ key, name: key, included: false, lastBackup: 0, lastRunStatus: "", effectiveSchedule: unscheduled, ...over });

const storageBox = location({
  id: "destination:1",
  object: "destination",
  kind: "offsite",
  name: "Storage Box",
  sections: [],
});

function Where() {
  const at = useLocation();
  return <p data-testid="where">{at.pathname + at.search}</p>;
}

async function renderPage() {
  render(
    inApp(
      <>
        <Dashboard />
        <Where />
      </>
    )
  );
  await act(async () => {});
}

const card = () => screen.queryByTestId("first-steps");
const steps = () => within(card()!).getAllByRole("listitem").filter((li) => li.hasAttribute("data-state"));
const states = () => steps().map((li) => li.getAttribute("data-state"));
const section = (kind: string) => document.querySelector<HTMLElement>(`[data-section='${kind}']`)!;
const flip = (kind: string) => within(section(kind)).getByRole("switch");
const saved = () => server.calls.filter((call) => call.name === "putSettings").map((call) => call.args[0] as Settings);
const called = () => server.calls.map((call) => call.name);
const next = () => within(steps()[0]).getByRole("button", { name: en["zfs.replica.add.next"] });
const where = () => screen.getByTestId("where").textContent;

beforeEach(() => {
  resetServer();
  installMatchMedia();
  vi.stubGlobal("EventSource", FakeEventSource);
  window.localStorage.clear();
  window.history.replaceState(null, "", "/dashboard");
  server.settings = { ...SECTIONS };
  server.items = [found("plex"), found("gitea"), found("win11", { kind: "vm", kindDisabled: true })];
  server.locations = [
    location({
      sections: [{ domain: "containers", use: "home", enabled: true } as never, { domain: "vms", use: "home", enabled: false } as never],
    }),
  ];
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("the first steps card", () => {
  it("stands on the overview of an install without a backup, with the sections as the next step", async () => {
    await renderPage();

    expect(within(card()!).getByText(en["firstSteps.title"])).toBeTruthy();
    expect(states()).toEqual(["next", "open", "open"]);
    expect(steps()[0].getAttribute("aria-current")).toBe("step");
  });

  it("is absent once an entry has a backup", async () => {
    server.items = [found("gitea"), item()];
    await renderPage();

    expect(card()).toBeNull();
  });

  it("is absent while the entries are not read, and when they cannot be", async () => {
    server.items = "pending";
    await renderPage();
    expect(card()).toBeNull();

    cleanup();
    server.items = "refused";
    await renderPage();
    expect(card()).toBeNull();
  });

  it("goes when the first backup has finished", async () => {
    await renderPage();
    expect(card()).toBeTruthy();

    server.items = [item(), found("gitea")];
    act(() => {
      window.dispatchEvent(new Event("bv:settings-changed"));
    });

    await waitFor(() => expect(card()).toBeNull());
  });

  it("leaves the page its action and names nothing that needs the person", async () => {
    server.kitDue = true;
    await renderPage();

    expect(screen.getByRole("button", { name: en["backups.backUpEverything"] })).toBeTruthy();
    expect(document.querySelector("[data-grid-block='needs']")).toBeNull();
    expect(within(screen.getByTestId("status-head")).getByText(en["backups.status.waiting"])).toBeTruthy();
  });
});

describe("the first step, choosing the sections", () => {
  it("says what was found and lists every section with its switch", async () => {
    await renderPage();

    expect(within(steps()[0]).getByText(/BombVault found 2 containers, 1 VM, and the flash drive\./)).toBeTruthy();
    expect(["container", "vm", "flash", "files", "zfs", "config"].map((kind) => flip(kind).getAttribute("aria-checked"))).toEqual([
      "true",
      "false",
      "false",
      "false",
      "false",
      "false",
    ]);
    expect(within(section("container")).getByText("2 found")).toBeTruthy();
    expect(within(section("vm")).getByText("1 found")).toBeTruthy();
    expect(within(section("config")).getByText(en["firstSteps.ownSettings"])).toBeTruthy();
    // Nothing is said about a kind the server reports nothing of.
    expect(section("zfs").textContent).toBe(en["nav.zfs"]);
  });

  it("switches a section on in the settings and keeps everything else as the server has it", async () => {
    await renderPage();
    server.settings = { ...SECTIONS, defaultLanguage: "fr" };

    fireEvent.click(flip("flash"));
    await act(async () => {});

    expect(saved()).toHaveLength(1);
    expect(saved()[0]).toMatchObject({ ...SECTIONS, flashEnabled: true, flashSchedule: NIGHTLY, defaultLanguage: "fr" });
    expect(flip("flash").getAttribute("aria-checked")).toBe("true");
  });

  it("gives a section it switches on a nightly schedule in the same save", async () => {
    await renderPage();

    fireEvent.click(flip("config"));
    await act(async () => {});

    expect(saved()).toHaveLength(1);
    expect(saved()[0]).toMatchObject({ configEnabled: true, configSchedule: NIGHTLY, containersSchedule: "off" });
  });

  it("keeps the schedule of a section that has one", async () => {
    server.settings = { ...SECTIONS, filesSchedule: "weekly Sun 04:00" };
    await renderPage();

    fireEvent.click(flip("files"));
    await act(async () => {});

    expect(saved()[0]).toMatchObject({ filesEnabled: true, filesSchedule: "weekly Sun 04:00" });
  });

  it("takes the nightly schedule back when the section is switched off again", async () => {
    await renderPage();

    fireEvent.click(flip("flash"));
    await act(async () => {});
    fireEvent.click(flip("flash"));
    await act(async () => {});

    expect(saved()).toHaveLength(2);
    expect(saved()[1]).toMatchObject({ flashEnabled: false, flashSchedule: "off" });
  });

  it("leaves a schedule somebody set alone when the section is switched off", async () => {
    server.settings = { ...SECTIONS, containersSchedule: "weekly Sun 04:00" };
    await renderPage();

    fireEvent.click(flip("container"));
    await act(async () => {});

    expect(saved()[0]).toMatchObject({ containersEnabled: false, containersSchedule: "weekly Sun 04:00" });
  });

  it("says in a bubble that what is switched on runs every night and where that is changed", async () => {
    await renderPage();

    const line = en["firstSteps.nightly"].replace("{where}", `${en["nav.settings"]} › ${en["settings.schedule"]}`);
    expect(within(steps()[0]).getByLabelText(line)).toBeTruthy();
    expect(within(steps()[0]).queryByText(line)).toBeNull();
  });

  it("puts a switch back and says why when the server refuses", async () => {
    server.saveRefused = "no SSH connection to the VM host";
    await renderPage();

    fireEvent.click(flip("vm"));
    await act(async () => {});

    expect(flip("vm").getAttribute("aria-checked")).toBe("false");
    expect(screen.getByText("no SSH connection to the VM host")).toBeTruthy();
  });

  it("schedules the sections that are on when Next ticks the step off", async () => {
    await renderPage();

    fireEvent.click(within(steps()[0]).getByRole("button", { name: en["zfs.replica.add.next"] }));
    await act(async () => {});

    expect(saved()).toHaveLength(1);
    expect(saved()[0]).toMatchObject({ containersSchedule: NIGHTLY, vmsSchedule: "off", flashSchedule: "off" });
    expect(states()).toEqual(["done", "next", "open"]);
  });

  it("saves nothing with Next where the sections that are on have a schedule", async () => {
    server.settings = { ...SECTIONS, containersSchedule: "daily 04:30" };
    await renderPage();

    fireEvent.click(within(steps()[0]).getByRole("button", { name: en["zfs.replica.add.next"] }));
    await act(async () => {});

    expect(saved()).toHaveLength(0);
    expect(states()).toEqual(["done", "next", "open"]);
  });

  it("stays on the first step and says why when the schedule cannot be saved", async () => {
    server.saveRefused = "the settings could not be written";
    await renderPage();

    fireEvent.click(within(steps()[0]).getByRole("button", { name: en["zfs.replica.add.next"] }));
    await act(async () => {});

    expect(states()).toEqual(["next", "open", "open"]);
    expect(screen.getByText("the settings could not be written")).toBeTruthy();
  });

  it("is ticked off with Next, shows what is switched on and stays ticked off after a reload", async () => {
    await renderPage();

    fireEvent.click(within(steps()[0]).getByRole("button", { name: en["zfs.replica.add.next"] }));
    await act(async () => {});

    expect(states()).toEqual(["done", "next", "open"]);
    expect(steps()[0].textContent).toContain(en["nav.containers"]);
    expect(document.querySelector("[data-section]")).toBeNull();

    cleanup();
    await renderPage();
    expect(states()).toEqual(["done", "next", "open"]);
  });
});

describe("the entries the first step takes into the schedule", () => {
  it("are the found containers when Next ticks the step off, after the schedule is saved", async () => {
    await renderPage();

    fireEvent.click(next());
    await act(async () => {});

    expect(called()).toEqual(["putSettings", "setIncludeAll"]);
    expect(server.calls[1].args).toEqual([true]);
    expect(states()).toEqual(["done", "next", "open"]);
  });

  it("are the VMs too once their section is on", async () => {
    server.settings = { ...SECTIONS, vmsEnabled: true };
    server.items = [found("plex"), found("win11", { kind: "vm" })];
    await renderPage();

    fireEvent.click(next());
    await act(async () => {});

    expect(called()).toEqual(["putSettings", "setIncludeAll", "setVMIncludeAll"]);
  });

  it("are left as they are where somebody has taken one in or left one out already", async () => {
    server.items = [found("plex", { included: true }), found("gitea", { included: false })];
    await renderPage();

    fireEvent.click(next());
    await act(async () => {});

    expect(called()).toEqual(["putSettings"]);
    expect(states()).toEqual(["done", "next", "open"]);
  });

  it("are none on a server that reports no container", async () => {
    server.items = [];
    await renderPage();

    fireEvent.click(next());
    await act(async () => {});

    expect(called()).toEqual(["putSettings"]);
  });

  it("keep the step open and say why when they cannot be taken in", async () => {
    server.includeRefused = "list containers: the Docker socket did not answer";
    await renderPage();

    fireEvent.click(next());
    await act(async () => {});

    expect(states()).toEqual(["next", "open", "open"]);
    expect(screen.getByText("list containers: the Docker socket did not answer")).toBeTruthy();
  });

  it("are not touched by a switch", async () => {
    await renderPage();

    fireEvent.click(flip("container"));
    await act(async () => {});
    fireEvent.click(flip("container"));
    await act(async () => {});

    expect(called()).toEqual(["putSettings", "putSettings"]);
  });
});

describe("choosing single entries from the first step", () => {
  const choose = (kind: string) => within(section(kind)).queryByRole("button", { name: new RegExp("^" + en["firstSteps.choose"]) });

  it("is offered for a section that is on and has found entries", async () => {
    await renderPage();

    expect(choose("container")).toBeTruthy();
    expect(choose("vm")).toBeNull();
    expect(choose("flash")).toBeNull();
    expect(choose("config")).toBeNull();
  });

  it("takes the entries into the schedule first and then opens their list", async () => {
    await renderPage();

    fireEvent.click(choose("container")!);
    await act(async () => {});

    expect(called()).toEqual(["putSettings", "setIncludeAll"]);
    expect(where()).toBe("/backups?kind=container");
  });

  it("does not take the entries in again with Next after every one of them was paused in the list", async () => {
    await renderPage();
    fireEvent.click(choose("container")!);
    await act(async () => {});
    expect(called()).toEqual(["putSettings", "setIncludeAll"]);

    server.items = (server.items as BackupItem[]).map((entry) => ({ ...entry, included: false }));
    act(() => {
      window.dispatchEvent(new Event("bv:settings-changed"));
    });
    await act(async () => {});
    fireEvent.click(next());
    await act(async () => {});

    expect(called()).toEqual(["putSettings", "setIncludeAll"]);
    expect(states()).toEqual(["done", "next", "open"]);
  });

  it("stays on the overview when the entries cannot be taken in", async () => {
    server.includeRefused = "the Docker socket did not answer";
    await renderPage();

    fireEvent.click(choose("container")!);
    await act(async () => {});

    expect(where()).toBe("/dashboard");
    expect(screen.getByText("the Docker socket did not answer")).toBeTruthy();
  });
});

describe("the second step, adding a storage location", () => {
  beforeEach(() => window.localStorage.setItem(CHOSEN, "1"));

  it("is the next one while only the folder the install came with exists", async () => {
    await renderPage();

    expect(states()).toEqual(["done", "next", "open"]);
    expect(within(steps()[1]).getByText(en["firstSteps.locationHint"])).toBeTruthy();
  });

  it("opens the window that adds one, for the sections that are switched on", async () => {
    await renderPage();

    fireEvent.click(within(steps()[1]).getByRole("button", { name: en["storage.add.title"] }));

    const dialog = screen.getByRole("dialog", { name: "add a location" });
    expect(dialog.getAttribute("data-domains")).toBe("containers");

    fireEvent.click(within(dialog).getByRole("button", { name: "close" }));
    expect(screen.queryByRole("dialog", { name: "add a location" })).toBeNull();
  });

  it("is ticked off by a location somebody added, and names it", async () => {
    server.locations = [location(), storageBox];
    await renderPage();

    expect(states()).toEqual(["done", "done", "next"]);
    expect(steps()[1].textContent).toContain("Storage Box");
  });
});

describe("the third step, the first backup", () => {
  beforeEach(() => {
    window.localStorage.setItem(CHOSEN, "1");
    server.locations = [location(), storageBox];
  });

  it("counts what waits and what no schedule reaches, and lists the entries", async () => {
    server.items = [
      found("plex", { included: true, effectiveSchedule: { kind: "domain", spec: "daily 03:00" } }),
      found("gitea"),
      found("immich"),
    ];
    await renderPage();

    const backup = within(steps()[2]);
    expect(backup.getByText("1 waiting for the first backup")).toBeTruthy();
    expect(backup.getByText("2 not protected")).toBeTruthy();
    const rows = backup.getAllByTestId("first-steps-entry");
    expect(rows.map((row) => within(row).getByRole("link").textContent)).toEqual(["plex", "gitea", "immich"]);
    expect(within(rows[0]).getByText(en["backups.status.waiting"])).toBeTruthy();
    expect(within(rows[1]).getByText(en["backups.status.notProtected"])).toBeTruthy();
    expect(within(rows[0]).getByRole("link").getAttribute("href")).toBe("/backups/container/plex");
  });

  it("says an entry that only replicates waits for its replica", async () => {
    const scheduled = { included: true, effectiveSchedule: { kind: "domain", spec: "daily 03:00" } } as const;
    server.items = [
      found("plex", scheduled),
      found("tank", { ...scheduled, kind: "zfs", replica: { lastAt: 0, lastStatus: "", only: true } }),
    ];
    await renderPage();

    const backup = within(steps()[2]);
    expect(backup.getByText("1 waiting for the first backup")).toBeTruthy();
    expect(backup.getByText("1 waiting for the first replica")).toBeTruthy();
    const rows = backup.getAllByTestId("first-steps-entry");
    expect(within(rows[1]).getByText(en["backups.status.waitingReplica"])).toBeTruthy();
  });

  it("heads the page with the replica when only replicas are awaited", async () => {
    server.items = [
      found("tank", {
        kind: "zfs",
        included: true,
        effectiveSchedule: { kind: "domain", spec: "daily 03:00" },
        replica: { lastAt: 0, lastStatus: "", only: true },
      }),
    ];
    await renderPage();

    expect(within(screen.getByTestId("status-head")).getByText(en["backups.status.waitingReplica"])).toBeTruthy();
  });

  it("shows four entries and counts the rest", async () => {
    server.items = ["a", "b", "c", "d", "e", "f"].map((key) => found(key));
    await renderPage();

    const backup = within(steps()[2]);
    expect(backup.getAllByTestId("first-steps-entry")).toHaveLength(4);
    expect(backup.getByText("and 2 more")).toBeTruthy();
  });

  it("lists nothing on a server with nothing to back up", async () => {
    server.items = [];
    await renderPage();

    expect(states()).toEqual(["done", "done", "next"]);
    expect(within(steps()[2]).queryByTestId("first-steps-entry")).toBeNull();
    expect(steps()[2].querySelector("p, ul")).toBeNull();
  });

  it("has no button of its own: the page's action starts the backup", async () => {
    await renderPage();

    expect(within(steps()[2]).queryByRole("button")).toBeNull();
    expect(within(screen.getByTestId("page-actions")).getByRole("button", { name: en["backups.backUpEverything"] })).toBeTruthy();
  });
});
