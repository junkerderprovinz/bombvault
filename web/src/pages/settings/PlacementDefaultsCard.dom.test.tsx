// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import type { DefaultRow } from "../../lib/api";
import {
  defaultImpact,
  defaultRow,
  namedRepo,
  placementOptions,
  renderWithProviders,
  targetImpact,
  targetOption,
  targetPreview,
  uploadEstimate,
  wheel,
} from "../../lib/placement.testsupport";

const fake = await vi.hoisted(async () => (await import("../../lib/placement.testsupport")).createPlacementApi());

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  ...fake.api,
}));

const { PlacementDefaultsCard } = await import("./PlacementDefaultsCard");
const { placementChanged } = await import("../../lib/placementEvents");

function withContainers(over: Partial<DefaultRow>) {
  fake.reply("listPlacementDefaults", {
    ok: true,
    defaults: [defaultRow(over), defaultRow({ domain: "vms" }), defaultRow({ domain: "files" })],
  });
}

// The card has one row per domain, each with its own bar; a test acts on one.
async function containersRow(): Promise<HTMLElement> {
  renderWithProviders(<PlacementDefaultsCard />);
  const row = await screen.findByRole("region", { name: "Containers" });
  await within(row).findByRole("toolbar", { name: "Placement" });
  return row;
}

describe("PlacementDefaultsCard", () => {
  beforeEach(() => fake.reset());
  afterEach(cleanup);

  it("counts what follows, what has its own choice and what has no location yet", async () => {
    withContainers({ counts: { follow: 14, own: 2, open: 3, chosenNoRun: 1 } });
    const row = await containersRow();
    expect(within(row).getByText("Following the default: 14 · Own choice: 2 · No location yet: 3 · Location set, no backup: 1")).toBeTruthy();
  });

  it("names what a target stops getting and what stays there before Local is saved", async () => {
    const impact = defaultImpact({ dropped: [targetImpact({ items: 15, snapshots: 210 })] });
    fake.reply("previewPlacementDefault", { ok: true, impact });
    const row = await containersRow();
    fireEvent.click(within(row).getByRole("button", { name: "Local" }));
    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("Items and project folders that B2 no longer gets: 15. Copies that stay there: 210.");
    fireEvent.click(within(dialog).getByRole("button", { name: "Confirm" }));
    await waitFor(() => expect(fake.callsTo("putPlacementDefault")).toEqual([["containers", { skip: ["*"] }, impact]]));
    expect(fake.callsTo("previewPlacementDefault")).toEqual([["containers", { skip: ["*"] }]]);
  });

  it("asks again with the new numbers when they changed in the meantime", async () => {
    const first = defaultImpact({ dropped: [targetImpact({ items: 15, snapshots: 210 })] });
    const second = defaultImpact({ dropped: [targetImpact({ items: 16, snapshots: 230 })] });
    fake.reply("previewPlacementDefault", { ok: true, impact: first });
    fake.reply("putPlacementDefault", { ok: false, error: "stale", code: "stale", impact: second }, { ok: true, default: defaultRow() });
    const row = await containersRow();
    fireEvent.click(within(row).getByRole("button", { name: "Local" }));
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Confirm" }));
    expect(await screen.findByText(/no longer gets: 16\. Copies that stay there: 230\./)).toBeTruthy();
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Confirm" }));
    await waitFor(() =>
      expect(fake.callsTo("putPlacementDefault")).toEqual([
        ["containers", { skip: ["*"] }, first],
        ["containers", { skip: ["*"] }, second],
      ])
    );
  });

  it("names the new location and the items that take it before the location changes", async () => {
    const impact = defaultImpact({ openTakeHome: 3 });
    fake.reply("previewPlacementDefault", { ok: true, impact });
    const row = await containersRow();
    wheel(within(row).getByRole("combobox", { name: "Stored on" }), 1);
    fireEvent.click(within(row).getByRole("button", { name: "Set" }));
    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain(
      "New items in Containers take NAS Keller · mounted at their first backup. Items with a location keep it."
    );
    expect(dialog.textContent).toContain("Items without a location that take NAS Keller · mounted at their first backup: 3.");
    fireEvent.click(within(dialog).getByRole("button", { name: "Set" }));
    await waitFor(() => expect(fake.callsTo("putPlacementDefault")).toEqual([["containers", { home: "repo-nas" }, impact]]));
  });

  it("saves a change without effect and without a new location unasked", async () => {
    const two = placementOptions({ targets: [targetOption(), targetOption({ id: "t-hz", name: "Hetzner", primary: false })] });
    fake.reply("getPlacementOptions", { ok: true, options: two }, { ok: true, options: two }, { ok: true, options: two });
    const row = await containersRow();
    fireEvent.click(within(row).getByRole("button", { name: "Hetzner" }));
    await waitFor(() =>
      expect(fake.callsTo("putPlacementDefault")).toEqual([["containers", { skip: ["t-hz"] }, defaultImpact()]])
    );
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("writes nothing when the one open segment of a domain without a target is clicked", async () => {
    const locks = { "local-offsite": "no-target", "offsite-only": "no-target" } as const;
    const noTarget = { ok: true, options: placementOptions({ targets: [], sendTo: [], segmentLocks: locks }) };
    fake.reply("getPlacementOptions", noTarget, noTarget, noTarget);
    const row = await containersRow();
    const local = within(row).getByRole("button", { name: "Local" });
    expect(local.getAttribute("aria-pressed")).toBe("true");
    fireEvent.click(local);
    expect(fake.callsTo("previewPlacementDefault")).toEqual([]);
    expect(fake.callsTo("putPlacementDefault")).toEqual([]);
  });

  it("opens the direct repository window for Off-site only and moves the default there", async () => {
    const row = await containersRow();
    fireEvent.click(within(row).getByRole("button", { name: "Off-site only" }));
    await screen.findByDisplayValue("b2:bucket:containers-direct");
    fireEvent.click(screen.getByRole("button", { name: "Create and use" }));
    expect(await screen.findByText(/New items in Containers take/)).toBeTruthy();
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Set" }));
    await waitFor(() =>
      expect(fake.callsTo("putPlacementDefault")).toEqual([["containers", { home: "repo-direct" }, defaultImpact()]])
    );
    expect(fake.callsTo("createDirectRepo")).toEqual([["t-b2", "", "b2:bucket:containers-direct"]]);
  });

  it("confirms a paused default and leaves out the ticked names", async () => {
    withContainers({ paused: true, confirmedAt: 0 });
    fake.reply("getConfirmPreview", {
      ok: true,
      paused: true,
      targets: [{ targetId: "t-b2", name: "B2", preview: targetPreview() }],
      unmatched: [{ identity: "container:plex", snapshots: 12 }],
    });
    const row = await containersRow();
    expect(within(row).getByText("Off-site paused after a rebuild")).toBeTruthy();
    expect(row.textContent).toContain("Nothing from Containers is copied off-site until this default is confirmed.");
    fireEvent.click(within(row).getByRole("button", { name: "Confirm default" }));
    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("At the next run, this is copied:");
    expect(dialog.textContent).toContain("Items and project folders: 15");
    expect(dialog.textContent).toContain("Names in the backups without an entry here. Ticked ones are left out:");
    fireEvent.click(within(dialog).getByRole("switch", { name: "plex, snapshots: 12" }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Confirm default" }));
    await waitFor(() => expect(fake.callsTo("confirmPlacementDefault")).toEqual([["containers", ["container:plex"]]]));
    expect(await screen.findByText("Default confirmed. Off-site copies resume at the next run.")).toBeTruthy();
  });

  it("applies the default to the items without backups and names those it keeps", async () => {
    fake.reply("getApplyDefaultPreview", {
      ok: true,
      reset: [
        { key: "nginx", label: "nginx", losesHome: true, losesRule: false, uploads: [uploadEstimate({ snapshots: 12 })] },
        { key: "redis", label: "redis", losesHome: false, losesRule: false, uploads: [uploadEstimate({ snapshots: 3 })] },
      ],
      kept: [{ key: "plex", label: "plex", reason: "has-backups" }],
    });
    fake.reply("applyPlacementDefault", { ok: true, reset: ["nginx"], kept: [{ key: "redis", label: "redis", reason: "changed" }] });
    const row = await containersRow();
    fireEvent.click(within(row).getByRole("button", { name: "Apply to items without backups" }));
    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("Location and copies go back to the default for these items: 2.");
    expect(dialog.textContent).toContain("Losing their own choice: nginx");
    expect(dialog.textContent).toContain("Stay as they are, they have backups: plex");
    expect(dialog.textContent).toContain("B2: about 15");
    fireEvent.click(within(dialog).getByRole("button", { name: "Confirm" }));
    await waitFor(() => expect(fake.callsTo("applyPlacementDefault")).toEqual([["containers", ["nginx", "redis"]]]));
    expect(await screen.findByText("Changed in the meantime and left alone: redis")).toBeTruthy();
  });

  it("lets the apply button give up its width floor so a narrow card can shrink it", async () => {
    const row = await containersRow();
    const apply = within(row).getByRole("button", { name: "Apply to items without backups" });
    expect(apply.className).toContain("glim-btn-elastic");
  });

  it("says to apply again once a running backup has finished", async () => {
    fake.reply("getApplyDefaultPreview", {
      ok: true,
      reset: [{ key: "nginx", label: "nginx", losesHome: false, losesRule: false, uploads: [] }],
      kept: [],
    });
    fake.reply("applyPlacementDefault", { ok: false, error: "a backup is running", code: "domain-busy" });
    const row = await containersRow();
    fireEvent.click(within(row).getByRole("button", { name: "Apply to items without backups" }));
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Confirm" }));
    expect(await screen.findByText("A backup is running. Apply again once it has finished.")).toBeTruthy();
  });

  it("says so when no item without backups differs from the default", async () => {
    const row = await containersRow();
    fireEvent.click(within(row).getByRole("button", { name: "Apply to items without backups" }));
    expect(await screen.findByText("No item without backups differs from the default.")).toBeTruthy();
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("marks a default whose repository is switched off", async () => {
    const cold = { ok: true, repos: [namedRepo({ id: "repo-cold", name: "Cold", enabled: false })] };
    fake.reply("listRepos", cold, cold, cold);
    withContainers({ home: "repo-cold", homeKind: "local", homeOff: true });
    const row = await containersRow();
    expect(
      await within(row).findByText(
        "Cold is switched off. Items without a location are not backed up until it is on again or the default changes."
      )
    ).toBeTruthy();
  });

  it("says so when the list cannot be read again, and keeps the rows it has", async () => {
    const row = await containersRow();
    fake.reply("listPlacementDefaults", { ok: false, error: "database is locked" });
    act(() => placementChanged());
    expect(await screen.findByText("Placement could not be read")).toBeTruthy();
    expect(within(row).getByRole("toolbar", { name: "Placement" })).toBeTruthy();
  });

  it("shows only its sentence for a default that cannot be read", async () => {
    withContainers({ unreadable: true });
    renderWithProviders(<PlacementDefaultsCard />);
    const row = await screen.findByRole("region", { name: "Containers" });
    expect(await within(row).findByText("Placement could not be read")).toBeTruthy();
    expect(within(row).queryByRole("toolbar")).toBeNull();
  });
});
