// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import type { PlacementView } from "../../lib/api";
import {
  homeOption,
  placementOptions,
  placementPlan,
  placementView,
  renderWithProviders,
  targetOption,
  uploadEstimate,
  wheel,
} from "../../lib/placement.testsupport";

const fake = await vi.hoisted(async () => (await import("../../lib/placement.testsupport")).createPlacementApi());

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  ...fake.api,
}));

const { PlacementRow } = await import("./PlacementRow");

const item = { domain: "containers", key: "nginx" } as const;
const twoTargets = placementOptions({
  targets: [targetOption(), targetOption({ id: "t-hz", name: "Hetzner", primary: false })],
});

function renderRow(view: PlacementView) {
  renderWithProviders(<PlacementRow item={item} name="nginx" view={view} onView={vi.fn()} />);
}

async function segment(name: string): Promise<HTMLButtonElement> {
  return (await screen.findByRole("button", { name })) as HTMLButtonElement;
}

// A disabled button takes no pointer events, so its bubble answers on the wrapper.
function bubbleOf(el: HTMLButtonElement): string | null | undefined {
  fireEvent.mouseEnter(el.disabled ? (el.parentElement as HTMLElement) : el);
  return document.querySelector(".glim-bubble")?.textContent;
}

describe("PlacementRow", () => {
  beforeEach(() => fake.reset());
  afterEach(cleanup);

  it("keeps a locked segment on the bar and says why", async () => {
    renderRow(
      placementView({
        repo: "repo-nas",
        repoKind: "local",
        repoLabel: "NAS Keller",
        locked: true,
        lockReason: "first-backup",
        segmentLocks: { "offsite-only": "home-fixed" },
      })
    );
    const offsiteOnly = await segment("Off-site only");
    expect(offsiteOnly.disabled).toBe(true);
    expect(bubbleOf(offsiteOnly)).toBe("Fixed since the first backup: NAS Keller · mounted");
    expect(screen.getByText("fixed since the first backup")).toBeTruthy();
  });

  it("stands on Local with both other segments locked when the domain has no target", async () => {
    const locks = { "local-offsite": "no-target", "offsite-only": "no-target" } as const;
    fake.reply("getPlacementOptions", { ok: true, options: placementOptions({ targets: [], sendTo: [], segmentLocks: locks }) });
    renderRow(placementView({ segment: "local", skip: ["*"], segmentLocks: locks }));
    expect((await segment("Local")).getAttribute("aria-pressed")).toBe("true");
    expect((await segment("Local + off-site")).disabled).toBe(true);
    const offsiteOnly = await segment("Off-site only");
    expect(offsiteOnly.disabled).toBe(true);
    expect(bubbleOf(offsiteOnly)).toBe("No off-site target set up");
  });

  it("locks the only target's chip, since Local is how to copy nowhere", async () => {
    renderRow(placementView());
    const chip = (await screen.findByRole("button", { name: "B2" })) as HTMLButtonElement;
    expect(chip.getAttribute("aria-pressed")).toBe("true");
    expect(chip.disabled).toBe(true);
    expect(bubbleOf(chip)).toBe("Choose Local for no copy");
  });

  it("sends nothing while the arrow keys move along the bar", async () => {
    renderRow(placementView());
    const local = await segment("Local");
    local.focus();
    const bar = screen.getByRole("toolbar", { name: "Placement" });
    for (const key of ["ArrowRight", "ArrowRight", "ArrowLeft", "End", "Home"]) fireEvent.keyDown(bar, { key });
    expect(document.activeElement).toBe(local);
    expect(fake.callsTo("setItemPlacement")).toEqual([]);
  });

  it("chooses exactly once for Enter and the click the browser sends with it", async () => {
    renderRow(placementView());
    const local = await segment("Local");
    local.focus();
    fireEvent.keyDown(screen.getByRole("toolbar", { name: "Placement" }), { key: "Enter" });
    fireEvent.click(local);
    await waitFor(() => expect(fake.callsTo("setItemPlacement")).toEqual([[item, { copies: { skip: ["*"] } }]]));
  });

  it("sends nothing for five wheel notches until Set, and asks before it does", async () => {
    fake.reply("getPlacementOptions", {
      ok: true,
      options: placementOptions({
        homes: [
          homeOption(),
          homeOption({ id: "repo-nas", name: "NAS Keller", location: "/mnt/remotes/nas/bv", kind: "local" }),
          homeOption({ id: "repo-cold", name: "Cold", location: "/mnt/remotes/cold", kind: "local" }),
        ],
      }),
    });
    renderRow(placementView());
    wheel(await screen.findByRole("combobox", { name: "Stored on" }), 5);
    expect(fake.callsTo("setItemPlacement")).toEqual([]);
    fireEvent.click(screen.getByRole("button", { name: "Set" }));
    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("Back up nginx to Cold · mounted from now on? The location is fixed from the first backup on.");
    fireEvent.click(within(dialog).getByRole("button", { name: "Set" }));
    await waitFor(() => expect(fake.callsTo("setItemPlacement")).toEqual([[item, { home: { repo: "repo-cold" } }]]));
  });

  it("goes back and shakes when the server refuses", async () => {
    fake.reply("setItemPlacement", { ok: false, error: "busy", code: "domain-busy" });
    renderRow(placementView());
    fireEvent.click(await segment("Local"));
    expect(await screen.findByText("A backup is running. Choose again once it has finished.")).toBeTruthy();
    expect((await segment("Local + off-site")).getAttribute("aria-pressed")).toBe("true");
    expect(screen.getByRole("toolbar", { name: "Placement" }).closest(".glim-shake")).not.toBeNull();
  });

  it("warns when copies are on and every ticked target is switched off", async () => {
    fake.reply("getPlacementOptions", { ok: true, options: placementOptions({ targets: [targetOption({ enabled: false })] }) });
    renderRow(placementView());
    expect(await screen.findByText("No copy at the moment, switched off: B2. Tick a target or choose Local.")).toBeTruthy();
    expect(screen.getByRole("button", { name: "B2 (off)" }).getAttribute("aria-pressed")).toBe("true");
  });

  it("sends one change at a time and lets only the newest wait", async () => {
    fake.reply("getPlacementOptions", { ok: true, options: twoTargets });
    const release = fake.hold("setItemPlacement");
    renderRow(placementView());
    fireEvent.click(await screen.findByRole("button", { name: "B2" }));
    await waitFor(() => expect(fake.callsTo("setItemPlacement")).toHaveLength(1));
    fireEvent.click(screen.getByRole("button", { name: "B2" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "B2" }).getAttribute("aria-pressed")).toBe("true"));
    fireEvent.click(screen.getByRole("button", { name: "Hetzner" }));
    await act(async () => release());
    await waitFor(() => expect(fake.callsTo("setItemPlacement")).toHaveLength(2));
    expect(fake.maxInFlight("setItemPlacement")).toBe(1);
    expect(fake.callsTo("setItemPlacement")[1]).toEqual([item, { copies: { skip: ["t-hz"] } }]);
  });

  it("asks before a ticked target starts uploading history", async () => {
    fake.reply("getPlacementOptions", { ok: true, options: twoTargets });
    const added = { ok: true, added: [uploadEstimate({ uncheckable: ["NAS Keller"] })], dropped: [] };
    fake.reply("previewItemPlacement", added, added);
    renderRow(placementView({ skip: ["t-b2"], copiesFollow: false }));
    fireEvent.click(await screen.findByRole("button", { name: "B2" }));
    let dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("At the next run, snapshots of nginx are uploaded:");
    expect(dialog.textContent).toContain("B2: about 40");
    expect(dialog.textContent).toContain("Could not be checked: NAS Keller");
    expect(dialog.textContent).toContain("Uploads can cost money at the provider.");
    fireEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(fake.callsTo("setItemPlacement")).toEqual([]);
    fireEvent.click(screen.getByRole("button", { name: "B2" }));
    dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Confirm" }));
    await waitFor(() => expect(fake.callsTo("setItemPlacement")).toEqual([[item, { copies: { skip: [] } }]]));
  });

  it("takes no second choice while the question is being prepared", async () => {
    fake.reply("getPlacementOptions", { ok: true, options: twoTargets });
    const release = fake.hold("previewItemPlacement");
    fake.reply("previewItemPlacement", { ok: true, added: [uploadEstimate()], dropped: [] });
    renderRow(placementView({ skip: ["t-b2"], copiesFollow: false }));
    fireEvent.click(await screen.findByRole("button", { name: "B2" }));
    await waitFor(() => expect(fake.callsTo("previewItemPlacement")).toHaveLength(1));
    expect((await segment("Local")).disabled).toBe(true);
    fireEvent.click(await segment("Local"));
    await act(async () => release());
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect((screen.getByRole("button", { name: "Local" }) as HTMLButtonElement).disabled).toBe(false));
    expect(fake.callsTo("previewItemPlacement")).toHaveLength(1);
    expect(fake.callsTo("setItemPlacement")).toEqual([]);
  });

  it("still asks, with the reason, when the upload cannot be estimated", async () => {
    fake.reply("getPlacementOptions", { ok: true, options: twoTargets });
    fake.reply("previewItemPlacement", { ok: false, error: "unreadable", code: "placement-unreadable" });
    renderRow(placementView({ skip: ["t-b2"], copiesFollow: false }));
    fireEvent.click(await screen.findByRole("button", { name: "B2" }));
    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("It could not be worked out how much of nginx would be uploaded.");
    expect(dialog.textContent).not.toContain("At the next run, snapshots of nginx are uploaded:");
    expect(dialog.textContent).toContain("The placement rules could not be read, so nothing is copied until they can.");
    expect(dialog.textContent).toContain("Uploads can cost money at the provider.");
    fireEvent.click(within(dialog).getByRole("button", { name: "Confirm" }));
    await waitFor(() => expect(fake.callsTo("setItemPlacement")).toEqual([[item, { copies: { skip: [] } }]]));
  });

  it("still asks when the upload estimate does not answer at all", async () => {
    fake.reply("getPlacementOptions", { ok: true, options: twoTargets });
    fake.reply("previewItemPlacement", new Error("the server did not answer"));
    renderRow(placementView({ skip: ["t-b2"], copiesFollow: false }));
    fireEvent.click(await screen.findByRole("button", { name: "B2" }));
    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("It could not be worked out how much of nginx would be uploaded.");
    expect(dialog.textContent).toContain("the server did not answer");
    expect(dialog.textContent).toContain("Uploads can cost money at the provider.");
    fireEvent.click(within(dialog).getByRole("button", { name: "Confirm" }));
    await waitFor(() => expect(fake.callsTo("setItemPlacement")).toEqual([[item, { copies: { skip: [] } }]]));
  });

  it("opens the direct repository window instead of saving Off-site only", async () => {
    renderRow(placementView());
    fireEvent.click(await segment("Off-site only"));
    expect(await screen.findByText("Direct repository at B2")).toBeTruthy();
    expect(fake.callsTo("setItemPlacement")).toEqual([]);
  });

  it("says what the card follows and resets a location set before the first backup", async () => {
    renderRow(placementView({ repo: "repo-nas", repoKind: "local", repoLabel: "NAS Keller" }));
    expect(await screen.findByText("Location set: NAS Keller · mounted")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Reset" }));
    await waitFor(() =>
      expect(fake.callsTo("setItemPlacement")).toEqual([[item, { home: { follow: true }, copies: { follow: true } }]])
    );
  });

  it("takes no second reset while the first one's question is being prepared", async () => {
    const release = fake.hold("previewItemPlacement");
    fake.reply("previewItemPlacement", { ok: true, added: [uploadEstimate()], dropped: [] });
    renderRow(placementView({ repo: "repo-nas", repoKind: "local", repoLabel: "NAS Keller" }));
    fireEvent.click(await screen.findByRole("button", { name: "Reset" }));
    await waitFor(() => expect(fake.callsTo("previewItemPlacement")).toHaveLength(1));
    const reset = screen.getByRole("button", { name: "Reset" }) as HTMLButtonElement;
    expect(reset.disabled).toBe(true);
    fireEvent.click(reset);
    await act(async () => release());
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect((screen.getByRole("button", { name: "Reset" }) as HTMLButtonElement).disabled).toBe(false));
    expect(fake.callsTo("previewItemPlacement")).toHaveLength(1);
  });

  it("has no copies line once it backs up to a repository that is never copied", async () => {
    const onDirect = {
      segment: "offsite-only",
      repo: "repo-b2-direct",
      repoKind: "direct",
      repoLabel: "B2 direct",
      skip: ["*"],
      locked: true,
      lockReason: "first-backup",
      segmentLocks: { local: "home-fixed", "local-offsite": "at-target" },
    } as const;
    for (const copiesFollow of [true, false]) {
      renderRow(placementView({ ...onDirect, skip: [...onDirect.skip], copiesFollow }));
      expect((await segment("Off-site only")).getAttribute("aria-pressed")).toBe("true");
      expect(screen.queryByText("Copies follow the default")).toBeNull();
      expect(screen.queryByText("Own copies")).toBeNull();
      expect(screen.queryByRole("button", { name: "Reset" })).toBeNull();
      cleanup();
    }
  });

  it("follows the default while nothing is chosen", async () => {
    renderRow(placementView({ homeFollows: true }));
    expect(await screen.findByText("Follows the default")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Reset" })).toBeNull();
  });

  it("shows the pause of its domain once", async () => {
    renderRow(placementView({ paused: true, plan: placementPlan({ kind: "paused", warn: true }) }));
    expect(await screen.findAllByText("Off-site paused until the default is confirmed.")).toHaveLength(1);
  });

  it("shows only its sentence when the placement cannot be read", async () => {
    renderRow(placementView({ unreadable: true, segment: "", repoKind: "" }));
    expect(await screen.findByText("Placement could not be read")).toBeTruthy();
    expect(screen.queryByRole("toolbar")).toBeNull();
  });

  it("shows only its sentence when the domain's options cannot be read", async () => {
    fake.reply("getPlacementOptions", { ok: false, error: "the repository list could not be read" });
    renderRow(placementView());
    expect(await screen.findByText("Placement could not be read")).toBeTruthy();
    expect(screen.queryByRole("toolbar")).toBeNull();
  });
});
