// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import type { Container, Snapshot } from "../lib/api";
import { placementView } from "../lib/placement.testsupport";
import type { TranslationKey } from "../lib/i18n";

class FakeEventSource {
  onmessage: ((ev: MessageEvent) => void) | null = null;
  close() {
    /* no-op */
  }
}
vi.stubGlobal("EventSource", FakeEventSource);
// Anything the page asks for that a test does not name answers with an empty ok.
vi.stubGlobal(
  "fetch",
  vi.fn(async () => new Response(JSON.stringify({ ok: true }), { status: 200 })),
);

const snap = (id: string): Snapshot => ({ id, time: "2026-09-01T00:00:00Z", paths: [], tags: [], hostname: "tower" });

vi.mock("../lib/api", async () => {
  const actual = await vi.importActual<typeof import("../lib/api")>("../lib/api");
  return {
    ...actual,
    listContainers: vi.fn(async () => ({ ok: true, containers: [] })),
    listRuns: vi.fn(async () => ({ ok: true, runs: [] })),
    listSnapshots: vi.fn(async () => ({ ok: true, snapshots: [snap("a"), snap("b"), snap("c")] })),
    takeOverContainer: vi.fn(async () => ({ ok: true })),
    unlinkContainerAlias: vi.fn(async () => ({ ok: true })),
  };
});

const { listContainers, listSnapshots, takeOverContainer, unlinkContainerAlias } = await import("../lib/api");
const { ContainerRow, Containers } = await import("./Containers");
const { countText, en } = await import("../lib/i18n");
const { ToastProvider } = await import("../lib/toast");

const t = ((key: TranslationKey, n?: number) => countText(en[key], "en", n)) as unknown as Parameters<typeof ContainerRow>[0]["t"];

const radarr: Container = {
  name: "radarr",
  image: "ghcr.io/hotio/radarr:latest",
  state: "running",
  status: "Up 2 hours",
  ip: "",
  installed: true,
  includeInSchedule: false,
  lastBackup: null,
  lastBackupStarted: null,
  preHook: "",
  postHook: "",
  stopContainers: [],
  excludes: [],
  lastUpdateCheck: 0,
  lastUpdateResult: "",
  stack: "",
  placement: placementView(),
};

type RowExtra = { onDeleted?: () => void; linkCandidates?: string[] };

function row(container: Container, extra: RowExtra = {}) {
  return (
    <ToastProvider>
      <ContainerRow
        container={container}
        installedContainers={[]}
        t={t}
        onDeleted={extra.onDeleted ?? (() => {})}
        linkCandidates={extra.linkCandidates}
        index={0}
      />
    </ToastProvider>
  );
}

function renderRow(container: Container, extra: RowExtra = {}) {
  return render(row(container, extra));
}

async function confirmDialog() {
  const dialog = await screen.findByRole("dialog");
  const buttons = within(dialog).getAllByRole("button");
  fireEvent.click(buttons[buttons.length - 1]);
}

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
});

afterEach(() => {
  vi.restoreAllMocks();
  cleanup();
});

describe("the rename suggestion on a container card", () => {
  const suggested: Container = { ...radarr, renameFrom: "radarr-movies", renameReason: "appdata-bind" };

  it("names the entry it looks like, the evidence, and how many backups that entry has", async () => {
    renderRow(suggested);
    expect(screen.getByText("Looks like radarr-movies")).toBeTruthy();
    expect(await screen.findByText("same appdata folders, 3 backups")).toBeTruthy();
    expect(listSnapshots).toHaveBeenCalledWith("radarr-movies");
  });

  it("counts the old entry's backups once, however often the card renders", async () => {
    const view = renderRow(suggested);
    await screen.findByText("same appdata folders, 3 backups");
    view.rerender(row({ ...suggested }));
    view.rerender(row({ ...suggested, status: "Up 3 hours" }));
    await act(async () => {});

    expect(listSnapshots).toHaveBeenCalledTimes(1);
  });

  it("does not count the backups of a pair that was dismissed", async () => {
    localStorage.setItem("bombvault.renameSuggestionsDismissed", JSON.stringify([["radarr-movies", "radarr"]]));
    renderRow(suggested);
    await act(async () => {});

    expect(screen.queryByText("Looks like radarr-movies")).toBeNull();
    expect(listSnapshots).not.toHaveBeenCalled();
  });

  it("takes the entry over once the dialog is confirmed, then reloads the list", async () => {
    const onDeleted = vi.fn();
    renderRow(suggested, { onDeleted });
    await screen.findByText("same appdata folders, 3 backups");

    fireEvent.click(screen.getByRole("button", { name: "Take over" }));
    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("The 3 backups of radarr-movies become the history of radarr.");
    expect(takeOverContainer).not.toHaveBeenCalled();
    fireEvent.click(within(dialog).getByRole("button", { name: "Take over" }));

    await waitFor(() => expect(takeOverContainer).toHaveBeenCalledWith("radarr", "radarr-movies"));
    await waitFor(() => expect(onDeleted).toHaveBeenCalled());
  });

  it("offers Undo after the takeover, which unlinks the old name again and reloads", async () => {
    const onDeleted = vi.fn();
    renderRow(suggested, { onDeleted });
    fireEvent.click(screen.getByRole("button", { name: "Take over" }));
    await confirmDialog();
    await waitFor(() => expect(onDeleted).toHaveBeenCalledTimes(1));

    fireEvent.click(await screen.findByRole("button", { name: "Undo" }));
    await waitFor(() => expect(unlinkContainerAlias).toHaveBeenCalledWith("radarr", "radarr-movies"));
    await waitFor(() => expect(onDeleted).toHaveBeenCalledTimes(2));
  });

  it("shows the server's reason when the takeover is refused", async () => {
    vi.mocked(takeOverContainer).mockResolvedValueOnce({ ok: false, error: "radarr already has backups of its own" });
    renderRow(suggested);
    fireEvent.click(screen.getByRole("button", { name: "Take over" }));
    await confirmDialog();

    expect(await screen.findByText("radarr already has backups of its own")).toBeTruthy();
  });

  it("shows the translated reason when the refusal carries a code", async () => {
    vi.mocked(takeOverContainer).mockResolvedValueOnce({
      ok: false,
      error: 'rename "radarr-movies": container:radarr: that name already has a copy rule',
      code: "copy-rule-taken",
    });
    renderRow(suggested);
    fireEvent.click(screen.getByRole("button", { name: "Take over" }));
    await confirmDialog();

    expect(await screen.findByText(en["placementCode.copyRuleTaken"])).toBeTruthy();
  });

  it("stays hidden for that pair after Not this one, across a fresh render", async () => {
    renderRow(suggested);
    fireEvent.click(screen.getByRole("button", { name: "Not this one" }));
    expect(screen.queryByText("Looks like radarr-movies")).toBeNull();

    cleanup();
    renderRow(suggested);
    expect(screen.queryByText("Looks like radarr-movies")).toBeNull();

    cleanup();
    renderRow({ ...suggested, renameFrom: "radarr-old" });
    expect(screen.getByText("Looks like radarr-old")).toBeTruthy();

    cleanup();
    renderRow({ ...suggested, name: "radarr-4k" });
    expect(screen.getByText("Looks like radarr-movies")).toBeTruthy();
  });

  it("follows a new suggestion on the same card, whatever was dismissed before", async () => {
    localStorage.setItem("bombvault.renameSuggestionsDismissed", JSON.stringify([["radarr-hd", "radarr"]]));
    const view = renderRow(suggested);
    fireEvent.click(screen.getByRole("button", { name: "Not this one" }));

    view.rerender(row({ ...suggested, renameFrom: "radarr-old" }));
    expect(screen.getByText("Looks like radarr-old")).toBeTruthy();

    view.rerender(row({ ...suggested, renameFrom: "radarr-hd" }));
    expect(screen.queryByText("Looks like radarr-hd")).toBeNull();
  });

  it("still shows and hides when the browser refuses storage", async () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("storage denied");
    });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("storage denied");
    });
    renderRow(suggested);
    expect(screen.getByText("Looks like radarr-movies")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Not this one" }));
    expect(screen.queryByText("Looks like radarr-movies")).toBeNull();
  });

  it("is not offered on a card that is not installed", () => {
    renderRow({ ...suggested, installed: false, state: "not-installed" });
    expect(screen.queryByText("Looks like radarr-movies")).toBeNull();
  });

  it("sends focus to the include-in-schedule switch once Not this one is clicked, with no link candidates", () => {
    renderRow(suggested);
    fireEvent.click(screen.getByRole("button", { name: "Not this one" }));

    expect(document.activeElement).toBe(screen.getByRole("switch", { name: "Include in schedule" }));
  });

  it("sends focus to the link picker once Not this one is clicked, when link candidates exist", () => {
    renderRow(suggested, { linkCandidates: ["lidarr-old"] });
    fireEvent.click(screen.getByRole("button", { name: "Not this one" }));

    expect(document.activeElement).toBe(screen.getByRole("button", { name: "Link to an entry…" }));
  });
});

describe("linking a container card to an entry by hand", () => {
  it("takes over the entry picked from the list", async () => {
    renderRow(radarr, { linkCandidates: ["radarr-movies", "lidarr-old"] });
    fireEvent.click(screen.getByRole("button", { name: "Link to an entry…" }));
    fireEvent.click(screen.getByRole("combobox", { name: "Former entry" }));
    fireEvent.click(screen.getByRole("option", { name: "lidarr-old" }));
    fireEvent.click(screen.getByRole("button", { name: "Take over" }));

    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("The 3 backups of lidarr-old become the history of radarr.");
    fireEvent.click(within(dialog).getByRole("button", { name: "Take over" }));
    await waitFor(() => expect(takeOverContainer).toHaveBeenCalledWith("radarr", "lidarr-old"));
  });

  it("takes over an entry still in the list when the one first shown has gone", async () => {
    const view = renderRow(radarr, { linkCandidates: ["radarr-movies", "lidarr-old"] });
    fireEvent.click(screen.getByRole("button", { name: "Link to an entry…" }));
    view.rerender(row(radarr, { linkCandidates: ["lidarr-old"] }));
    fireEvent.click(screen.getByRole("button", { name: "Take over" }));

    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("Take over lidarr-old?");
    fireEvent.click(within(dialog).getByRole("button", { name: "Take over" }));
    await waitFor(() => expect(takeOverContainer).toHaveBeenCalledWith("radarr", "lidarr-old"));
  });

  it("is not offered on a card that has backups of its own", () => {
    renderRow({ ...radarr, lastBackup: 1_757_000_000 }, { linkCandidates: ["radarr-movies"] });
    expect(screen.queryByRole("button", { name: "Link to an entry…" })).toBeNull();
  });

  // An entry rebuilt by Discover carries its former names before its first run.
  it("is not offered on a card with former names and no last backup", () => {
    renderRow({ ...radarr, aliases: ["radarr-old"] }, { linkCandidates: ["lidarr-old"] });
    expect(screen.queryByRole("button", { name: "Link to an entry…" })).toBeNull();
    expect(screen.getByRole("button", { name: "Unlink radarr-old" })).toBeTruthy();
  });

  it("focuses the select on open and returns focus to the opener on cancel", () => {
    renderRow(radarr, { linkCandidates: ["radarr-movies", "lidarr-old"] });
    fireEvent.click(screen.getByRole("button", { name: "Link to an entry…" }));
    expect(document.activeElement).toBe(screen.getByRole("combobox", { name: "Former entry" }));

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "Link to an entry…" }));
  });

  it("lists only the containers that are no longer installed", async () => {
    vi.mocked(listContainers).mockResolvedValue({
      ok: true,
      containers: [
        radarr,
        { ...radarr, name: "sonarr", lastBackup: 1_757_000_000 },
        { ...radarr, name: "radarr-movies", installed: false, state: "not-installed" },
        { ...radarr, name: "lidarr-old", installed: false, state: "not-installed" },
      ],
    });
    render(
      <ToastProvider>
        <Containers />
      </ToastProvider>,
    );
    fireEvent.click(await screen.findByRole("button", { name: "Link to an entry…" }));
    fireEvent.click(screen.getByRole("combobox", { name: "Former entry" }));

    const offered = screen.getAllByRole("option").map((o) => o.textContent);
    expect(offered.sort()).toEqual(["lidarr-old", "radarr-movies"]);
  });

  it("leaves out an entry whose name is another entry's former name", async () => {
    vi.mocked(listContainers).mockResolvedValue({
      ok: true,
      containers: [
        radarr,
        { ...radarr, name: "sonarr", lastBackup: 1_757_000_000, aliases: ["sonarr-old"] },
        { ...radarr, name: "sonarr-old", installed: false, state: "not-installed" },
        { ...radarr, name: "lidarr-old", installed: false, state: "not-installed" },
      ],
    });
    render(
      <ToastProvider>
        <Containers />
      </ToastProvider>,
    );
    fireEvent.click(await screen.findByRole("button", { name: "Link to an entry…" }));
    fireEvent.click(screen.getByRole("combobox", { name: "Former entry" }));

    expect(screen.getAllByRole("option").map((o) => o.textContent)).toEqual(["lidarr-old"]);
  });
});

describe("a container card that took over an entry", () => {
  const linked: Container = { ...radarr, lastBackup: 1_757_000_000, aliases: ["radarr-movies", "radarr-old"] };

  it("names its former names in the info bubble", () => {
    renderRow(linked);
    expect(screen.getByLabelText(/^Formerly: radarr-movies, radarr-old\./)).toBeTruthy();
  });

  it("unlinks a former name once confirmed, then reloads the list", async () => {
    const onDeleted = vi.fn();
    renderRow(linked, { onDeleted });
    fireEvent.click(screen.getByRole("button", { name: "Unlink radarr-old" }));
    await confirmDialog();

    await waitFor(() => expect(unlinkContainerAlias).toHaveBeenCalledWith("radarr", "radarr-old"));
    await waitFor(() => expect(onDeleted).toHaveBeenCalled());
  });

  it("shows the server's reason when the unlink is refused", async () => {
    vi.mocked(unlinkContainerAlias).mockResolvedValueOnce({ ok: false, error: "radarr-old stays linked" });
    renderRow(linked);
    fireEvent.click(screen.getByRole("button", { name: "Unlink radarr-old" }));
    await confirmDialog();

    expect(await screen.findByText("radarr-old stays linked")).toBeTruthy();
  });

  it("shows the translated reason when the unlink refusal carries a code", async () => {
    vi.mocked(unlinkContainerAlias).mockResolvedValueOnce({
      ok: false,
      error: 'unlink "radarr-old": container:radarr-old: that name already has a copy rule',
      code: "copy-rule-taken",
    });
    renderRow(linked);
    fireEvent.click(screen.getByRole("button", { name: "Unlink radarr-old" }));
    await confirmDialog();

    expect(await screen.findByText(en["placementCode.copyRuleTaken"])).toBeTruthy();
  });

  it("does not unlink when the dialog is cancelled", async () => {
    renderRow(linked);
    fireEvent.click(screen.getByRole("button", { name: "Unlink radarr-old" }));
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: en["common.cancel"] }));
    await act(async () => {});

    expect(unlinkContainerAlias).not.toHaveBeenCalled();
  });
});

describe("a container card whose former name is in use again", () => {
  const conflicted: Container = {
    ...radarr,
    lastBackup: 1_757_000_000,
    aliases: ["radarr-movies"],
    aliasConflicts: ["radarr-movies"],
  };

  it("says so, explains it, and offers no unlink for that name", () => {
    renderRow(conflicted);
    expect(screen.getByText("The old name radarr-movies is taken again")).toBeTruthy();
    expect(screen.getByLabelText(en["takeover.conflictHint"])).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Unlink radarr-movies" })).toBeNull();
  });

  it("treats every live former name as its own conflict and still unlinks the one that isn't", async () => {
    renderRow({
      ...conflicted,
      aliases: ["radarr-movies", "radarr-4k", "radarr-old"],
      aliasConflicts: ["radarr-4k", "radarr-movies"],
    });

    expect(screen.getByText("The old name radarr-movies is taken again")).toBeTruthy();
    expect(screen.getByText("The old name radarr-4k is taken again")).toBeTruthy();
    expect(screen.getAllByLabelText(en["takeover.conflictHint"])).toHaveLength(2);
    expect(screen.queryByRole("button", { name: "Unlink radarr-movies" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Unlink radarr-4k" })).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Unlink radarr-old" }));
    await confirmDialog();
    await waitFor(() => expect(unlinkContainerAlias).toHaveBeenCalledWith("radarr", "radarr-old"));
  });
});

describe("BombVault's own container card", () => {
  const own: Container = { ...radarr, self: true, renameFrom: "radarr-movies", renameReason: "appdata-bind" };

  it("hides the rename suggestion and the manual link picker", () => {
    renderRow(own, { linkCandidates: ["radarr-movies"] });
    expect(screen.queryByText("Looks like radarr-movies")).toBeNull();
    expect(screen.queryByRole("button", { name: "Link to an entry…" })).toBeNull();
  });
});
