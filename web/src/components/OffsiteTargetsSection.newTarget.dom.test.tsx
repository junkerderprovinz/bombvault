// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import type { OffsiteTarget } from "../lib/api";
import { en } from "../lib/i18n";
import { renderWithProviders, targetPreview } from "../lib/placement.testsupport";

const fake = await vi.hoisted(async () => (await import("../lib/placement.testsupport")).createPlacementApi());
const listed = vi.hoisted(() => ({ targets: [] as unknown[] }));

vi.mock("../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/api")>()),
  ...fake.api,
  listOffsiteTargets: () => Promise.resolve({ ok: true, targets: listed.targets }),
  getCloudCredSets: () => Promise.resolve({ ok: true, sets: [] }),
}));

const { OffsiteTargetsSection } = await import("./OffsiteTargetsSection");

const t = ((key: string) => (en as Record<string, string>)[key] ?? key) as Parameters<typeof OffsiteTargetsSection>[0]["t"];

const hetzner: OffsiteTarget = {
  id: "t-hz", domain: "containers", name: "Hetzner", repo: "sftp:u1@box:/containers", credsRef: "", storageClass: "",
  immutable: false, schedule: "", retentionKeepLast: 7, retentionKeepDaily: 0, retentionKeepWeekly: 0,
  retentionKeepMonthly: 0, limitUpload: 0, limitDownload: 0, growthBudgetGb: 0, enabled: true, createdAt: 1, sortOrder: 1,
};

function repoField(): HTMLInputElement {
  return screen.getByPlaceholderText(en["offsite.wizard.repoUrlPlaceholder"]) as HTMLInputElement;
}

function save() {
  fireEvent.click(screen.getByRole("button", { name: en["offsite.targets.save"] }));
}

describe("the off-site targets and a new location", () => {
  beforeEach(() => {
    fake.reset();
    listed.targets = [];
  });
  afterEach(cleanup);

  it("asks before adding a target and sends the chosen exclusions along", async () => {
    fake.reply("getNewTargetPreview", {
      ok: true,
      preview: targetPreview({ formerlyExcluded: [{ identity: "container:plex", skip: ["t-b2"] }] }),
    });
    renderWithProviders(<OffsiteTargetsSection domain="containers" t={t} />);
    fireEvent.click(await screen.findByRole("button", { name: en["offsite.targets.add"] }));
    fireEvent.change(repoField(), { target: { value: "b2:bucket:containers" } });
    save();
    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("At its first run b2:bucket:containers receives every item not set to Local.");
    fireEvent.click(within(dialog).getByRole("switch", { name: "Leave these out here too" }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Confirm" }));
    await waitFor(() => expect(fake.callsTo("createOffsiteTarget")).toHaveLength(1));
    expect(fake.callsTo("getNewTargetPreview")).toEqual([["containers", "b2:bucket:containers", undefined]]);
    expect(fake.callsTo("createOffsiteTarget")[0][1]).toEqual({ identities: ["container:plex"], default: false });
  });

  it("adds nothing when the question is cancelled", async () => {
    renderWithProviders(<OffsiteTargetsSection domain="containers" t={t} />);
    fireEvent.click(await screen.findByRole("button", { name: en["offsite.targets.add"] }));
    fireEvent.change(repoField(), { target: { value: "b2:bucket:containers" } });
    save();
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(fake.callsTo("createOffsiteTarget")).toEqual([]);
  });

  it("locks Save while the question is still being prepared", async () => {
    const answer = fake.hold("getNewTargetPreview");
    renderWithProviders(<OffsiteTargetsSection domain="containers" t={t} />);
    fireEvent.click(await screen.findByRole("button", { name: en["offsite.targets.add"] }));
    fireEvent.change(repoField(), { target: { value: "b2:bucket:containers" } });
    const saveButton = () => screen.getByRole("button", { name: en["offsite.targets.save"] });

    save();

    await waitFor(() => expect(fake.callsTo("getNewTargetPreview")).toHaveLength(1));
    expect(saveButton().hasAttribute("disabled")).toBe(true);

    answer();
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(saveButton().hasAttribute("disabled")).toBe(false));
  });

  it("asks about the whole history when a saved target moves", async () => {
    listed.targets = [hetzner];
    renderWithProviders(<OffsiteTargetsSection domain="containers" t={t} />);
    fireEvent.click(await screen.findByText(en["offsite.targets.edit"]));
    fireEvent.change(repoField(), { target: { value: "sftp:u1@box:/containers-new" } });
    save();
    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("The new location of Hetzner receives the whole history.");
    fireEvent.click(within(dialog).getByRole("button", { name: "Confirm" }));
    await waitFor(() => expect(fake.callsTo("updateOffsiteTarget")).toHaveLength(1));
    expect(fake.callsTo("getNewTargetPreview")).toEqual([["containers", "sftp:u1@box:/containers-new", "t-hz"]]);
    expect(fake.callsTo("updateOffsiteTarget")[0][2]).toBeUndefined();
  });

  it("says a refused create in the language the rest of the page speaks", async () => {
    fake.reply("createOffsiteTarget", { ok: false, code: "nested-location", error: "the server's own sentence" });
    renderWithProviders(<OffsiteTargetsSection domain="containers" t={t} />);
    fireEvent.click(await screen.findByRole("button", { name: en["offsite.targets.add"] }));
    fireEvent.change(repoField(), { target: { value: "b2:bucket:containers" } });
    save();
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Confirm" }));
    expect(await screen.findByText(en["placementCode.nestedLocation"])).toBeTruthy();
  });

  it("writes the exclusions on their own route when the target was saved without them", async () => {
    listed.targets = [hetzner];
    fake.reply("getNewTargetPreview", {
      ok: true,
      preview: targetPreview({ formerlyExcluded: [{ identity: "container:plex", skip: ["t-b2"] }] }),
    });
    fake.reply("updateOffsiteTarget", { ok: false, code: "exclusion-unsaved", error: "the server's own sentence" });
    renderWithProviders(<OffsiteTargetsSection domain="containers" t={t} />);
    fireEvent.click(await screen.findByText(en["offsite.targets.edit"]));
    fireEvent.change(repoField(), { target: { value: "sftp:u1@box:/containers-new" } });
    save();
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("switch", { name: "Leave these out here too" }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Confirm" }));
    await waitFor(() =>
      expect(fake.callsTo("excludeFromTarget")).toEqual([
        [{ domain: "containers", targetId: "t-hz", identities: ["container:plex"], default: false }],
      ])
    );
    expect(await screen.findByText(en["settings.saved"])).toBeTruthy();
    expect(screen.queryByPlaceholderText(en["offsite.wizard.repoUrlPlaceholder"])).toBeNull();
  });

  it("says so when the exclusions are refused on their own route too", async () => {
    listed.targets = [hetzner];
    fake.reply("getNewTargetPreview", {
      ok: true,
      preview: targetPreview({ formerlyExcluded: [{ identity: "container:plex", skip: ["t-b2"] }] }),
    });
    fake.reply("updateOffsiteTarget", { ok: false, code: "exclusion-unsaved", error: "the server's own sentence" });
    fake.reply("excludeFromTarget", { ok: false, code: "unknown-target", error: "no such target" });
    renderWithProviders(<OffsiteTargetsSection domain="containers" t={t} />);
    fireEvent.click(await screen.findByText(en["offsite.targets.edit"]));
    fireEvent.change(repoField(), { target: { value: "sftp:u1@box:/containers-new" } });
    save();
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("switch", { name: "Leave these out here too" }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Confirm" }));
    expect(await screen.findByText(en["placementCode.unknownTarget"])).toBeTruthy();
    expect(await screen.findByText(en["placementCode.exclusionUnsaved"])).toBeTruthy();
    expect(screen.queryByText(en["settings.saved"])).toBeNull();
  });

  it("says which half of the save went through when the exclusions failed", async () => {
    listed.targets = [hetzner];
    fake.reply("updateOffsiteTarget", { ok: false, code: "exclusion-unsaved", error: "the server's own sentence" });
    renderWithProviders(<OffsiteTargetsSection domain="containers" t={t} />);
    fireEvent.click(await screen.findByText(en["offsite.targets.edit"]));
    fireEvent.change(repoField(), { target: { value: "sftp:u1@box:/containers-new" } });
    save();
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Confirm" }));
    expect(await screen.findByText(en["placementCode.exclusionUnsaved"])).toBeTruthy();
  });

  it("does not ask when a saved target keeps its location", async () => {
    listed.targets = [hetzner];
    renderWithProviders(<OffsiteTargetsSection domain="containers" t={t} />);
    fireEvent.click(await screen.findByText(en["offsite.targets.edit"]));
    fireEvent.change(screen.getByPlaceholderText(en["offsite.targets.namePlaceholder"]), { target: { value: "Storage box" } });
    save();
    await waitFor(() => expect(fake.callsTo("updateOffsiteTarget")).toHaveLength(1));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(fake.callsTo("getNewTargetPreview")).toEqual([]);
  });
});
