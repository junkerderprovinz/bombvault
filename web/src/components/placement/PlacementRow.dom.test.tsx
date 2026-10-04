// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import type { PlacementView } from "../../lib/api";
import { en } from "../../lib/i18n";
import {
  accentButtons,
  destination,
  homeOption,
  placementOptions,
  placementPlan,
  placementView,
  renderWithProviders,
  sendToOption,
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
const b2Direct = placementOptions({ sendTo: [sendToOption({ repoId: "repo-b2-direct" })] });
const onB2: PlacementView = placementView({
  segment: "offsite-only",
  repo: "repo-b2-direct",
  repoLabel: "B2 direct",
  repoKind: "direct",
  repoTarget: "t-b2",
  skip: ["*"],
});
const box = sendToOption({ kind: "remote", repoId: "repo-box", targetId: "", name: "Storagebox" });
const onBox: PlacementView = placementView({
  segment: "offsite-only",
  repo: "repo-box",
  repoLabel: "Storagebox",
  repoKind: "remote",
  skip: ["*"],
});

function renderRow(view: PlacementView) {
  renderWithProviders(<PlacementRow item={item} name="nginx" view={view} onView={vi.fn()} />);
}

async function button(name: string): Promise<HTMLButtonElement> {
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

  it("keeps a home fixed by the first backup and says why when it is asked to move", async () => {
    renderRow(placementView({ repo: "repo-nas", repoKind: "local", repoLabel: "NAS Keller", locked: true, lockReason: "first-backup" }));
    const local = await button("Local");
    expect(bubbleOf(local)).toBe("Fixed since the first backup: NAS Keller · mounted");
    expect(screen.getByText("fixed since the first backup")).toBeTruthy();
    fireEvent.click(local);
    expect((await screen.findByRole("alert")).textContent).toContain("Fixed since the first backup: NAS Keller · mounted");
    expect(fake.callsTo("setItemPlacement")).toEqual([]);
  });

  // jsdom lays nothing out, so this pins the class the phone layout rests on.
  it("gives the bar a line of its own under the label on a phone", async () => {
    renderRow(placementView());
    expect((await button("Local")).closest('[class*="max-md:basis-full"]')).not.toBeNull();
  });

  it("stands on Local alone when the domain has no target, and keeps it lit", async () => {
    fake.reply("getPlacementOptions", { ok: true, options: placementOptions({ targets: [], sendTo: [] }) });
    renderRow(placementView({ segment: "local", skip: ["*"] }));
    const local = await button("Local");
    expect(local.getAttribute("aria-pressed")).toBe("true");
    expect(within(screen.getByRole("group", { name: "Placement" })).getAllByRole("button")).toEqual([local]);
    fireEvent.click(local);
    expect(await screen.findByText(en["placement.lastButton"])).toBeTruthy();
    expect(fake.callsTo("setItemPlacement")).toEqual([]);
  });

  it("darkens the only target and leaves the item on Local without a copy", async () => {
    renderRow(placementView());
    const b2 = await button("B2");
    expect(b2.getAttribute("aria-pressed")).toBe("true");
    fireEvent.click(b2);
    await waitFor(() => expect(fake.callsTo("setItemPlacement")).toEqual([[item, { copies: { skip: ["*"] } }]]));
  });

  it("sends nothing while the arrow keys move along the bar", async () => {
    renderRow(placementView());
    const local = await button("Local");
    local.focus();
    const bar = screen.getByRole("group", { name: "Placement" });
    for (const key of ["ArrowRight", "ArrowRight", "ArrowLeft", "End", "Home"]) fireEvent.keyDown(bar, { key });
    expect(document.activeElement).toBe(local);
    expect(fake.callsTo("setItemPlacement")).toEqual([]);
  });

  it("chooses exactly once for Enter and the click the browser sends with it", async () => {
    renderRow(placementView());
    const b2 = await button("B2");
    b2.focus();
    fireEvent.keyDown(screen.getByRole("group", { name: "Placement" }), { key: "Enter" });
    fireEvent.click(b2);
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
    expect(accentButtons(dialog)).toEqual(["Set"]);
    fireEvent.click(within(dialog).getByRole("button", { name: "Set" }));
    await waitFor(() => expect(fake.callsTo("setItemPlacement")).toEqual([[item, { home: { repo: "repo-cold" } }]]));
  });

  it("goes back and shakes when the server refuses", async () => {
    fake.reply("setItemPlacement", { ok: false, error: "busy", code: "domain-busy" });
    renderRow(placementView());
    fireEvent.click(await button("B2"));
    expect(await screen.findByText("A backup is running. Choose again once it has finished.")).toBeTruthy();
    expect((await button("B2")).getAttribute("aria-pressed")).toBe("true");
    expect(screen.getByRole("group", { name: "Placement" }).closest(".glim-shake")).not.toBeNull();
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
    expect(accentButtons(dialog)).toEqual(["Confirm"]);
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
    expect((await button("Hetzner")).disabled).toBe(true);
    fireEvent.click(await button("Hetzner"));
    await act(async () => release());
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect((screen.getByRole("button", { name: "Hetzner" }) as HTMLButtonElement).disabled).toBe(false));
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
    expect(accentButtons(dialog)).toEqual(["Confirm"]);
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

  it("opens the direct repository window when Local goes dark and the target has none yet", async () => {
    renderRow(placementView());
    fireEvent.click(await button("Local"));
    expect(await screen.findByText("Direct repository at B2")).toBeTruthy();
    expect(fake.callsTo("setItemPlacement")).toEqual([]);
    await screen.findByDisplayValue("b2:bucket:containers-direct");
    fireEvent.click(screen.getByRole("button", { name: "Create and use" }));
    await waitFor(() =>
      expect(fake.callsTo("setItemPlacement")).toEqual([[item, { home: { repo: "repo-direct" }, copies: { skip: ["*"] } }]])
    );
  });

  it("moves the home onto a target's direct repository once asked", async () => {
    fake.reply("getPlacementOptions", { ok: true, options: b2Direct });
    renderRow(placementView());
    fireEvent.click(await button("Local"));
    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("Back up nginx to B2 · direct from now on?");
    fireEvent.click(within(dialog).getByRole("button", { name: "Set" }));
    await waitFor(() =>
      expect(fake.callsTo("setItemPlacement")).toEqual([[item, { home: { repo: "repo-b2-direct" }, copies: { skip: ["*"] } }]])
    );
  });

  it("brings an item on a direct repository home and keeps its target as a copy", async () => {
    fake.reply("getPlacementOptions", { ok: true, options: b2Direct });
    renderRow(onB2);
    const local = await button("Local");
    expect(local.getAttribute("aria-pressed")).toBe("false");
    expect((await button("B2")).getAttribute("aria-pressed")).toBe("true");
    fireEvent.click(local);
    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("Back up nginx to");
    fireEvent.click(within(dialog).getByRole("button", { name: "Set" }));
    await waitFor(() => expect(fake.callsTo("setItemPlacement")).toEqual([[item, { home: { repo: "" }, copies: { skip: [] } }]]));
  });

  it("keeps a direct home lit while no other button is", async () => {
    fake.reply("getPlacementOptions", { ok: true, options: b2Direct });
    renderRow(onB2);
    fireEvent.click(await button("B2"));
    expect(await screen.findByText(en["placement.lastButton"])).toBeTruthy();
    expect(fake.callsTo("setItemPlacement")).toEqual([]);
  });

  it("takes no target for a home on a remote repository and says why", async () => {
    fake.reply("getPlacementOptions", { ok: true, options: placementOptions({ sendTo: [box] }) });
    renderRow(onBox);
    expect((await button("Storagebox · remote")).getAttribute("aria-pressed")).toBe("true");
    fireEvent.click(await button("B2"));
    expect((await screen.findByRole("alert")).textContent).toContain("Has its own credentials and is already off the premises");
    expect(fake.callsTo("setItemPlacement")).toEqual([]);
  });

  it("gives the domain its target under a destination and lights it for this item alone", async () => {
    fake.reply("listDestinations", { ok: true, destinations: [destination()] });
    renderRow(placementView({ skip: ["*"], copiesFollow: false }));
    fireEvent.click(await button("Wasabi"));
    await waitFor(() => expect(fake.callsTo("setItemPlacement")).toEqual([[item, { copies: { skip: ["t-b2"] } }]]));
    expect(fake.callsTo("destinationForDomain")).toEqual([["dest-wasabi", "containers"]]);
  });

  it("says why a destination could not be added and saves nothing", async () => {
    fake.reply("listDestinations", { ok: true, destinations: [destination()] });
    fake.reply("destinationForDomain", { ok: false, error: "rclone did not answer" });
    renderRow(placementView({ skip: ["*"], copiesFollow: false }));
    fireEvent.click(await button("Wasabi"));
    expect(await screen.findByText("rclone did not answer")).toBeTruthy();
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
    const withBox = { ok: true, options: placementOptions({ sendTo: [box] }) };
    fake.reply("getPlacementOptions", withBox, withBox);
    for (const copiesFollow of [true, false]) {
      renderRow({ ...onBox, skip: [...onBox.skip], copiesFollow, locked: true, lockReason: "first-backup" });
      expect((await button("Storagebox · remote")).getAttribute("aria-pressed")).toBe("true");
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
    expect(screen.queryByRole("group", { name: "Placement" })).toBeNull();
  });

  it("shows only its sentence when the domain's options cannot be read", async () => {
    fake.reply("getPlacementOptions", { ok: false, error: "the repository list could not be read" });
    renderRow(placementView());
    expect(await screen.findByText("Placement could not be read")).toBeTruthy();
    expect(screen.queryByRole("group", { name: "Placement" })).toBeNull();
  });
});
