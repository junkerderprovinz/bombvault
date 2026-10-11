// @vitest-environment jsdom
/**
 * Taking over the entry a renamed container or VM left behind.
 *
 * Three controls share one takeover: the suggestion under an entry that looks
 * renamed, the picker that links an entry by hand, and the list of former
 * names with the way to unlink one. Each asks before it writes, sends the
 * name the routes take and says why the server refused.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import type { ReactNode } from "react";
import type { Snapshot } from "../lib/api";
import type { TranslationKey } from "../lib/i18n";

const snap = (id: string): Snapshot => ({ id, time: "2026-09-01T00:00:00Z", paths: [], tags: [], hostname: "tower" });

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listSnapshots: vi.fn(async () => ({ ok: true, snapshots: [snap("a"), snap("b"), snap("c")] })),
    takeOverContainer: vi.fn(async () => ({ ok: true })),
    unlinkContainerAlias: vi.fn(async () => ({ ok: true })),
    listVMSnapshots: vi.fn(async () => ({ ok: true, snapshots: [snap("a"), snap("b")] })),
    takeOverVM: vi.fn(async () => ({ ok: true })),
    unlinkVMAlias: vi.fn(async () => ({ ok: true })),
  };
});

const { listSnapshots, takeOverContainer, unlinkContainerAlias, listVMSnapshots, takeOverVM, unlinkVMAlias } =
  await import("../lib/api");
const { countText, en } = await import("../lib/i18n");
const { ToastProvider } = await import("../lib/toast");
const { containerTakeover, vmTakeover } = await import("../lib/useTakeOver");
const { RenameTakeoverRow } = await import("./RenameTakeoverRow");
const { LinkEntryPicker } = await import("./LinkEntryPicker");
const { FormerNames } = await import("./FormerNames");

const t = ((key: TranslationKey, n?: number) => countText(en[key], "en", n)) as unknown as Parameters<
  typeof RenameTakeoverRow
>[0]["t"];

const radarr = { name: "radarr", displayName: "radarr", api: containerTakeover };
// On TrueNAS the name a user reads and the libvirt name differ, and only the
// libvirt name reaches a route.
const win11 = { name: "1_win11", displayName: "win11", api: vmTakeover };

const DISMISSED = "bombvault.renameSuggestionsDismissed";

function suggestion(over: { from?: string; entry?: typeof radarr; onDone?: () => void; after?: ReactNode } = {}) {
  const from = over.from ?? "radarr-movies";
  return (
    <ToastProvider>
      <div>
        {/* Keyed by the old name, as on the entry's page. */}
        <RenameTakeoverRow
          key={from}
          from={from}
          reason="appdata-bind"
          entry={over.entry ?? radarr}
          onDone={over.onDone ?? (() => {})}
          t={t}
        />
        {over.after}
      </div>
    </ToastProvider>
  );
}

function picker(candidates: string[], entry = radarr) {
  return (
    <ToastProvider>
      <LinkEntryPicker candidates={candidates} entry={entry} onDone={() => {}} t={t} />
    </ToastProvider>
  );
}

function formerNames(aliases: string[], conflicts: string[] = [], onDone: () => void = () => {}, entry = radarr) {
  return (
    <ToastProvider>
      <FormerNames aliases={aliases} conflicts={conflicts} entry={entry} onDone={onDone} t={t} />
    </ToastProvider>
  );
}

async function confirmDialog() {
  const dialog = await screen.findByRole("dialog");
  const buttons = within(dialog).getAllByRole("button");
  fireEvent.click(buttons[buttons.length - 1]);
}

const press = (name: string) => fireEvent.click(screen.getByRole("button", { name }));

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
});

afterEach(() => {
  vi.restoreAllMocks();
  cleanup();
});

describe("the rename suggestion", () => {
  it("names the entry it looks like, the evidence, and how many backups that entry has", async () => {
    render(suggestion());
    expect(screen.getByText("Looks like radarr-movies")).toBeTruthy();
    expect(await screen.findByText("same appdata folders, 3 backups")).toBeTruthy();
    expect(listSnapshots).toHaveBeenCalledWith("radarr-movies");
  });

  it("counts the old entry's backups once, however often it renders", async () => {
    const view = render(suggestion());
    await screen.findByText("same appdata folders, 3 backups");
    view.rerender(suggestion());
    view.rerender(suggestion({ after: <p>more</p> }));
    await act(async () => {});
    expect(listSnapshots).toHaveBeenCalledTimes(1);
  });

  it("does not count the backups of a pair that was dismissed", async () => {
    localStorage.setItem(DISMISSED, JSON.stringify([["radarr-movies", "radarr"]]));
    render(suggestion());
    await act(async () => {});
    expect(screen.queryByText("Looks like radarr-movies")).toBeNull();
    expect(listSnapshots).not.toHaveBeenCalled();
  });

  it("takes the entry over once the dialog is confirmed, then says it is done", async () => {
    const onDone = vi.fn();
    render(suggestion({ onDone }));
    await screen.findByText("same appdata folders, 3 backups");

    press("Take over");
    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("The 3 backups of radarr-movies become the history of radarr.");
    expect(takeOverContainer).not.toHaveBeenCalled();
    fireEvent.click(within(dialog).getByRole("button", { name: "Take over" }));

    await waitFor(() => expect(takeOverContainer).toHaveBeenCalledWith("radarr", "radarr-movies"));
    await waitFor(() => expect(onDone).toHaveBeenCalled());
  });

  it("offers Undo after the takeover, which unlinks the old name again", async () => {
    const onDone = vi.fn();
    render(suggestion({ onDone }));
    press("Take over");
    await confirmDialog();
    await waitFor(() => expect(onDone).toHaveBeenCalledTimes(1));

    fireEvent.click(await screen.findByRole("button", { name: "Undo" }));
    await waitFor(() => expect(unlinkContainerAlias).toHaveBeenCalledWith("radarr", "radarr-movies"));
    await waitFor(() => expect(onDone).toHaveBeenCalledTimes(2));
  });

  it("shows the server's reason when the takeover is refused", async () => {
    vi.mocked(takeOverContainer).mockResolvedValueOnce({ ok: false, error: "radarr already has backups of its own" });
    render(suggestion());
    press("Take over");
    await confirmDialog();
    expect(await screen.findByText("radarr already has backups of its own")).toBeTruthy();
  });

  it("shows the translated reason when the refusal carries a code", async () => {
    vi.mocked(takeOverContainer).mockResolvedValueOnce({
      ok: false,
      error: 'rename "radarr-movies": container:radarr: that name already has a copy rule',
      code: "copy-rule-taken",
    });
    render(suggestion());
    press("Take over");
    await confirmDialog();
    expect(await screen.findByText(en["placementCode.copyRuleTaken"])).toBeTruthy();
  });

  it("stays hidden for that pair after Not this one, and for no other pair", async () => {
    render(suggestion());
    press("Not this one");
    expect(screen.queryByText("Looks like radarr-movies")).toBeNull();

    cleanup();
    render(suggestion());
    expect(screen.queryByText("Looks like radarr-movies")).toBeNull();

    cleanup();
    render(suggestion({ from: "radarr-old" }));
    expect(screen.getByText("Looks like radarr-old")).toBeTruthy();

    cleanup();
    render(suggestion({ entry: { ...radarr, name: "radarr-4k" } }));
    expect(screen.getByText("Looks like radarr-movies")).toBeTruthy();
  });

  it("follows a new suggestion for the same entry, whatever was dismissed before", async () => {
    localStorage.setItem(DISMISSED, JSON.stringify([["radarr-hd", "radarr"]]));
    const view = render(suggestion());
    press("Not this one");

    view.rerender(suggestion({ from: "radarr-old" }));
    expect(screen.getByText("Looks like radarr-old")).toBeTruthy();

    view.rerender(suggestion({ from: "radarr-hd" }));
    expect(screen.queryByText("Looks like radarr-hd")).toBeNull();
  });

  it("still shows and hides when the browser refuses storage", async () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("storage denied");
    });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("storage denied");
    });
    render(suggestion());
    expect(screen.getByText("Looks like radarr-movies")).toBeTruthy();

    press("Not this one");
    expect(screen.queryByText("Looks like radarr-movies")).toBeNull();
  });

  it("hands focus to the control after it once Not this one is pressed", () => {
    render(suggestion({ after: <button type="button">next on the page</button> }));
    press("Not this one");
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "next on the page" }));
  });

  it("names the evidence and the backups of a VM, and takes it over under its libvirt name", async () => {
    render(
      <ToastProvider>
        <RenameTakeoverRow from="1_windows-11" reason="libvirt-uuid" entry={win11} onDone={() => {}} t={t} />
      </ToastProvider>
    );
    expect(screen.getByText("Looks like 1_windows-11")).toBeTruthy();
    expect(await screen.findByText("same VM (UUID), 2 backups")).toBeTruthy();
    expect(listVMSnapshots).toHaveBeenCalledWith("1_windows-11");

    press("Take over");
    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("become the history of win11.");
    fireEvent.click(within(dialog).getByRole("button", { name: "Take over" }));
    await waitFor(() => expect(takeOverVM).toHaveBeenCalledWith("1_win11", "1_windows-11"));
    expect(takeOverVM).not.toHaveBeenCalledWith("win11", expect.anything());
  });
});

describe("linking an entry by hand", () => {
  it("takes over the entry picked from the list", async () => {
    render(picker(["radarr-movies", "lidarr-old"]));
    press("Link to an entry…");
    fireEvent.click(screen.getByRole("combobox", { name: "Former entry" }));
    fireEvent.click(screen.getByRole("option", { name: "lidarr-old" }));
    press("Take over");

    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("The 3 backups of lidarr-old become the history of radarr.");
    fireEvent.click(within(dialog).getByRole("button", { name: "Take over" }));
    await waitFor(() => expect(takeOverContainer).toHaveBeenCalledWith("radarr", "lidarr-old"));
  });

  it("takes over an entry still in the list when the one first shown has gone", async () => {
    const view = render(picker(["radarr-movies", "lidarr-old"]));
    press("Link to an entry…");
    view.rerender(picker(["lidarr-old"]));
    press("Take over");

    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("Take over lidarr-old?");
    fireEvent.click(within(dialog).getByRole("button", { name: "Take over" }));
    await waitFor(() => expect(takeOverContainer).toHaveBeenCalledWith("radarr", "lidarr-old"));
  });

  it("focuses the select on open and returns focus to the opener on cancel", () => {
    render(picker(["radarr-movies", "lidarr-old"]));
    press("Link to an entry…");
    expect(document.activeElement).toBe(screen.getByRole("combobox", { name: "Former entry" }));

    press("Cancel");
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "Link to an entry…" }));
  });

  it("links a VM under its libvirt name", async () => {
    render(picker(["1_windows-11"], win11));
    press("Link to an entry…");
    press("Take over");
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Take over" }));
    await waitFor(() => expect(takeOverVM).toHaveBeenCalledWith("1_win11", "1_windows-11"));
  });
});

describe("the former names of an entry", () => {
  const aliases = ["radarr-movies", "radarr-old"];

  it("stand in the info bubble", () => {
    render(formerNames(aliases));
    expect(screen.getByLabelText(/^Formerly: radarr-movies, radarr-old\./)).toBeTruthy();
  });

  it("are unlinked one at a time, once confirmed", async () => {
    const onDone = vi.fn();
    render(formerNames(aliases, [], onDone));
    press("Unlink radarr-old");
    await confirmDialog();
    await waitFor(() => expect(unlinkContainerAlias).toHaveBeenCalledWith("radarr", "radarr-old"));
    await waitFor(() => expect(onDone).toHaveBeenCalled());
  });

  it("show the server's reason when the unlink is refused", async () => {
    vi.mocked(unlinkContainerAlias).mockResolvedValueOnce({ ok: false, error: "radarr-old stays linked" });
    render(formerNames(aliases));
    press("Unlink radarr-old");
    await confirmDialog();
    expect(await screen.findByText("radarr-old stays linked")).toBeTruthy();
  });

  it("show the translated reason when the unlink refusal carries a code", async () => {
    vi.mocked(unlinkContainerAlias).mockResolvedValueOnce({
      ok: false,
      error: 'unlink "radarr-old": container:radarr-old: that name already has a copy rule',
      code: "copy-rule-taken",
    });
    render(formerNames(aliases));
    press("Unlink radarr-old");
    await confirmDialog();
    expect(await screen.findByText(en["placementCode.copyRuleTaken"])).toBeTruthy();
  });

  it("stay linked when the dialog is cancelled", async () => {
    render(formerNames(aliases));
    press("Unlink radarr-old");
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: en["common.cancel"] }));
    await act(async () => {});
    expect(unlinkContainerAlias).not.toHaveBeenCalled();
  });

  it("are unlinked under the libvirt name of a VM", async () => {
    render(formerNames(["1_windows-11"], [], () => {}, win11));
    press("Unlink 1_windows-11");
    await confirmDialog();
    await waitFor(() => expect(unlinkVMAlias).toHaveBeenCalledWith("1_win11", "1_windows-11"));
  });
});

describe("a former name that is in use again", () => {
  it("is named, explained, and cannot be unlinked", () => {
    render(formerNames(["radarr-movies"], ["radarr-movies"]));
    expect(screen.getByText("The old name radarr-movies is taken again")).toBeTruthy();
    expect(screen.getByLabelText(en["takeover.conflictHint"])).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Unlink radarr-movies" })).toBeNull();
  });

  it("is its own conflict beside others, and leaves the free name to unlink", async () => {
    render(formerNames(["radarr-movies", "radarr-4k", "radarr-old"], ["radarr-4k", "radarr-movies"]));
    expect(screen.getByText("The old name radarr-movies is taken again")).toBeTruthy();
    expect(screen.getByText("The old name radarr-4k is taken again")).toBeTruthy();
    expect(screen.getAllByLabelText(en["takeover.conflictHint"])).toHaveLength(2);
    expect(screen.queryByRole("button", { name: "Unlink radarr-movies" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Unlink radarr-4k" })).toBeNull();

    press("Unlink radarr-old");
    await confirmDialog();
    await waitFor(() => expect(unlinkContainerAlias).toHaveBeenCalledWith("radarr", "radarr-old"));
  });
});
