// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import type { FileSetView } from "../lib/api";
import { useT } from "../lib/i18n";
import {
  defaultRow,
  destination,
  placementOptions,
  placementView,
  renderWithProviders,
  targetOption,
  wheel,
} from "../lib/placement.testsupport";

const fake = await vi.hoisted(async () => (await import("../lib/placement.testsupport")).createPlacementApi());
const patchFileSet = vi.hoisted(() => vi.fn(async () => ({ ok: true })));

vi.mock("../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/api")>()),
  ...fake.api,
  patchFileSet,
  browse: () => Promise.resolve({ ok: true, status: "ok", truncated: false, dirs: [] }),
}));

const { FileSetDialog } = await import("./Files");

const BASE = { name: "Photos", path: "photos", excludes: [], enabled: true };

function Harness({ initial }: { initial: FileSetView | null }) {
  const { t } = useT();
  return (
    <FileSetDialog initial={initial} presetSeed={null} hostMountRoot="/host/user" t={t} onClose={() => {}} onSaved={() => {}} />
  );
}

async function openAdd() {
  renderWithProviders(<Harness initial={null} />);
  await screen.findByRole("group", { name: "Placement" });
}

function fillAndSave() {
  fireEvent.change(screen.getByPlaceholderText("documents"), { target: { value: BASE.name } });
  fireEvent.change(screen.getByPlaceholderText("user/appdata"), { target: { value: BASE.path } });
  fireEvent.click(screen.getByRole("button", { name: "Save" }));
}

describe("adding a folder set", () => {
  beforeEach(() => {
    fake.reset();
    patchFileSet.mockClear();
  });
  afterEach(cleanup);

  it("creates an untouched set open, with neither location nor copies", async () => {
    await openAdd();
    fillAndSave();
    await waitFor(() => expect(fake.callsTo("createFileSet")).toEqual([[BASE]]));
  });

  it("sends the copies that were changed and nothing about the location", async () => {
    fake.reply("getPlacementOptions", {
      ok: true,
      options: placementOptions({ targets: [targetOption(), targetOption({ id: "t-hz", name: "Hetzner", primary: false })] }),
    });
    await openAdd();
    fireEvent.click(screen.getByRole("button", { name: "Hetzner" }));
    fillAndSave();
    await waitFor(() => expect(fake.callsTo("createFileSet")).toEqual([[{ ...BASE, copies: { skip: ["t-hz"] } }]]));
  });

  it("lights a destination's new target for this set alone", async () => {
    fake.reply("getPlacementOptions", { ok: true, options: placementOptions({ default: defaultRow({ domain: "files", skip: ["*"] }) }) });
    fake.reply("listDestinations", { ok: true, destinations: [destination()] });
    await openAdd();
    fireEvent.click(await screen.findByRole("button", { name: "Wasabi" }));
    await waitFor(() => expect(fake.callsTo("destinationForDomain")).toEqual([["dest-wasabi", "files"]]));
    await act(async () => {});
    fillAndSave();
    await waitFor(() => expect(fake.callsTo("createFileSet")).toEqual([[{ ...BASE, copies: { skip: ["t-b2"] } }]]));
  });

  it("sends a location only once it is set, without asking", async () => {
    await openAdd();
    wheel(screen.getByRole("combobox", { name: "Stored on" }), 1);
    fireEvent.click(screen.getByRole("button", { name: "Set" }));
    expect(screen.queryByText(/from now on\?/)).toBeNull();
    fillAndSave();
    await waitFor(() => expect(fake.callsTo("createFileSet")).toEqual([[{ ...BASE, repo: "repo-nas" }]]));
  });

  it("creates the remembered direct repository before the set once Local goes dark", async () => {
    await openAdd();
    fireEvent.click(screen.getByRole("button", { name: "Local" }));
    expect(await screen.findByText("Created together with the folder set.")).toBeTruthy();
    await screen.findByDisplayValue("b2:bucket:containers-direct");
    fireEvent.click(screen.getByRole("button", { name: "Create and use" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Local" }).getAttribute("aria-pressed")).toBe("false"));
    expect(screen.getByRole("button", { name: "B2" }).getAttribute("aria-pressed")).toBe("true");
    expect(fake.callsTo("createDirectRepo")).toEqual([]);
    fillAndSave();
    await waitFor(() => expect(fake.callsTo("createFileSet")).toHaveLength(1));
    expect(fake.calls.map((c) => c.fn).filter((fn) => fn === "createDirectRepo" || fn === "createFileSet")).toEqual([
      "createDirectRepo",
      "createFileSet",
    ]);
    expect(fake.callsTo("createDirectRepo")).toEqual([["t-b2", "", "b2:bucket:containers-direct"]]);
    expect(fake.callsTo("createFileSet")).toEqual([[{ ...BASE, repo: "repo-direct", copies: { skip: ["*"] } }]]);
  });

  it("keeps the direct repository when the set fails and does not make a second one", async () => {
    fake.reply("createFileSet", { ok: false, error: "name taken" }, { ok: true, id: "set-new" });
    await openAdd();
    fireEvent.click(screen.getByRole("button", { name: "Local" }));
    await screen.findByDisplayValue("b2:bucket:containers-direct");
    fireEvent.click(screen.getByRole("button", { name: "Create and use" }));
    fillAndSave();
    expect(await screen.findByText("name taken")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(fake.callsTo("createFileSet")).toHaveLength(2));
    expect(fake.callsTo("createDirectRepo")).toHaveLength(1);
    expect(fake.callsTo("createFileSet")[1]).toEqual([{ ...BASE, repo: "repo-direct", copies: { skip: ["*"] } }]);
  });
});

describe("editing a folder set", () => {
  beforeEach(() => {
    fake.reset();
    patchFileSet.mockClear();
  });
  afterEach(cleanup);

  it("has no placement field and never sends a location", async () => {
    const set = { id: "set1", ...BASE, lastBackup: 0, pathExists: true, placement: placementView() } as FileSetView;
    renderWithProviders(<Harness initial={set} />);
    expect(screen.queryByRole("group", { name: "Placement" })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(patchFileSet).toHaveBeenCalledTimes(1));
    expect(patchFileSet.mock.calls[0]).toEqual(["set1", BASE]);
  });
});
