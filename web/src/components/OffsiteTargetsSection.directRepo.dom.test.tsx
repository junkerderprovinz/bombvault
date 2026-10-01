// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { NamedRepo, OffsiteTarget } from "../lib/api";
import { en } from "../lib/i18n";

const api = {
  listOffsiteTargets: vi.fn(),
  updateOffsiteTarget: vi.fn(),
  deleteOffsiteTarget: vi.fn(),
  listRepos: vi.fn(),
};
const pushed: { message: string; severity?: string }[] = [];

vi.mock("../lib/api", () => ({
  listOffsiteTargets: (...a: unknown[]) => api.listOffsiteTargets(...a),
  updateOffsiteTarget: (...a: unknown[]) => api.updateOffsiteTarget(...a),
  deleteOffsiteTarget: (...a: unknown[]) => api.deleteOffsiteTarget(...a),
  listRepos: () => api.listRepos(),
  createOffsiteTarget: vi.fn(),
  testOffsiteTarget: vi.fn(),
  getCloudCredSets: () => Promise.resolve({ ok: true, sets: [] }),
}));
vi.mock("../lib/toast", () => ({
  useToast: () => ({
    push: (message: string, severity?: string) => pushed.push({ message, severity }),
    quiet: false,
    setQuiet: () => {},
  }),
}));

const { OffsiteTargetsSection } = await import("./OffsiteTargetsSection");

const t = ((key: string) => (en as Record<string, string>)[key] ?? key) as Parameters<typeof OffsiteTargetsSection>[0]["t"];

const b2: OffsiteTarget = {
  id: "t-b2", domain: "containers", name: "B2", repo: "b2:bkt:containers", credsRef: "", storageClass: "",
  immutable: true, schedule: "", retentionKeepLast: 7, retentionKeepDaily: 0, retentionKeepWeekly: 0,
  retentionKeepMonthly: 0, limitUpload: 0, limitDownload: 0, growthBudgetGb: 0, enabled: true, createdAt: 1, sortOrder: 1,
};
const direct: NamedRepo = {
  id: "d1", name: "B2 direct", repo: "b2:bkt:containers-direct", credsRef: "", storageClass: "", limitUpload: 0,
  limitDownload: 0, immutable: true, enabled: true, offPremises: false, inUse: 2, companionOf: "t-b2", companionLost: false,
};

beforeEach(() => {
  pushed.length = 0;
  api.listOffsiteTargets.mockResolvedValue({ ok: true, targets: [b2] });
  api.listRepos.mockResolvedValue({ ok: true, repos: [direct] });
  api.updateOffsiteTarget.mockReset();
  api.updateOffsiteTarget.mockResolvedValue({ ok: true, target: b2, warnings: [] });
  api.deleteOffsiteTarget.mockReset();
});
afterEach(cleanup);

async function openEditor() {
  render(<OffsiteTargetsSection domain="containers" t={t} />);
  fireEvent.click(await screen.findByText(en["offsite.targets.edit"]));
  await screen.findByText("Also applies to B2 direct. Items whose only copy is there: 2.");
}

async function clickSave() {
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: en["offsite.targets.save"] }));
  });
}

describe("the target editor beside a used direct repository", () => {
  it("says the target's rules apply to its direct repository too", async () => {
    await openEditor();
  });

  it("asks before keeping less and saves nothing when told no", async () => {
    await openEditor();
    fireEvent.change(screen.getAllByRole("spinbutton")[0], { target: { value: "3" } });
    await clickSave();
    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain(
      "Items whose only copy is in B2 direct: 2. Keeping less deletes their older snapshots for good at the next prune. Save anyway?"
    );
    // Scoped to the dialog: the editor's own Cancel button (open behind it)
    // shares the same English text.
    await act(async () => {
      fireEvent.click(within(dialog).getByRole("button", { name: en["common.cancel"] }));
    });
    expect(api.updateOffsiteTarget).not.toHaveBeenCalled();
    await clickSave();
    await act(async () => {
      fireEvent.click(await screen.findByRole("button", { name: en["common.confirm"] }));
    });
    await waitFor(() => expect(api.updateOffsiteTarget).toHaveBeenCalledTimes(1));
    expect(api.updateOffsiteTarget.mock.calls[0][1]).toMatchObject({ retentionKeepLast: 3 });
  });

  it("asks before switching append-only off", async () => {
    await openEditor();
    fireEvent.click(screen.getByRole("switch", { name: en["offsite.immutable"] }));
    await clickSave();
    expect((await screen.findByRole("dialog")).textContent).toContain("Without append-only this box may delete from it.");
  });

  it("does not ask when the save keeps more", async () => {
    await openEditor();
    fireEvent.change(screen.getAllByRole("spinbutton")[0], { target: { value: "9" } });
    await clickSave();
    await waitFor(() => expect(api.updateOffsiteTarget).toHaveBeenCalledTimes(1));
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("shows the warnings the save answered with", async () => {
    api.updateOffsiteTarget.mockResolvedValue({
      ok: true,
      target: b2,
      warnings: [{ code: "direct-creds-kept", targetId: "t-b2", targetName: "B2", items: 2 }],
    });
    await openEditor();
    await clickSave();
    await waitFor(() =>
      expect(pushed).toContainEqual({ message: "B2 direct cannot be opened with the new key and keeps the old one.", severity: "warn" })
    );
  });

  it("explains a refused delete", async () => {
    api.deleteOffsiteTarget.mockResolvedValue({
      ok: false, error: "in use", code: "target-in-use", use: { directRepoId: "d1", items: 2, defaultDomains: [] },
    });
    render(<OffsiteTargetsSection domain="containers" t={t} />);
    fireEvent.click(await screen.findByText(en["offsite.targets.remove"]));
    await act(async () => {
      fireEvent.click(screen.getByText(en["offsite.targets.confirmRemove"]));
    });
    await waitFor(() =>
      expect(pushed).toContainEqual({
        message: "Items still back up to the direct repository of this target: 2. Point them somewhere else first.",
        severity: "fail",
      })
    );
  });
});
