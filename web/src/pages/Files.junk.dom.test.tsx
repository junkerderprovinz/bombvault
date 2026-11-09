// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { FileSetView } from "../lib/api";
import type { TranslationKey } from "../lib/i18n";

const api = vi.hoisted(() => ({
  patchFileSet: vi.fn(async () => ({ ok: true })),
  getFileSetPreset: vi.fn(async () => ({
    ok: true,
    offered: false,
    name: "",
    path: "",
    excludes: [],
    junk: {
      app: "systemJunk",
      entries: [
        { line: ".DS_Store", kind: "macMetadata", optional: false },
        { line: "@eaDir", kind: "synologyIndex", optional: false },
      ],
    },
  })),
}));

vi.mock("../lib/api", async () => ({
  ...(await vi.importActual<typeof import("../lib/api")>("../lib/api")),
  ...api,
  listRepos: vi.fn(async () => ({ ok: true, repos: [] })),
  browse: vi.fn(async () => ({ ok: true, dirs: [], status: "ok", truncated: false })),
}));

const { FileSetDialog } = await import("../components/files/FileSetDialog");
const { countText, en } = await import("../lib/i18n");

const t = ((key: TranslationKey, n?: number) => countText(en[key], "en", n)) as unknown as Parameters<typeof FileSetDialog>[0]["t"];

const photos: FileSetView = {
  id: "set1",
  name: "Photos",
  path: "photos",
  excludes: ["cache/"],
  enabled: true,
  lastBackup: 1_757_000_000,
  pathExists: true,
};

async function renderDialog() {
  render(<FileSetDialog initial={photos} presetSeed={null} hostMountRoot="/host/user" t={t} onClose={() => {}} onSaved={() => {}} />);
  await act(async () => {});
}

async function save() {
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: en["settings.save"] }));
  });
}

afterEach(() => {
  cleanup();
  api.patchFileSet.mockClear();
});

describe("system junk on an existing folder set", () => {
  it("keeps the set's excludes as they are until the junk lines are applied", async () => {
    await renderDialog();
    expect(screen.getByRole("button", { name: en["excludes.junk.load"] })).toBeTruthy();
    await save();
    expect(api.patchFileSet).toHaveBeenCalledWith("set1", expect.objectContaining({ excludes: ["cache/"] }));
  });

  it("adds the lines the user applies", async () => {
    await renderDialog();
    fireEvent.click(screen.getByRole("button", { name: en["excludes.junk.load"] }));
    fireEvent.click(screen.getByRole("switch", { name: en["excludes.presetKind.synologyIndex"] }));
    fireEvent.click(screen.getByRole("button", { name: en["excludes.presetApply"] }));
    await save();
    expect(api.patchFileSet).toHaveBeenCalledWith("set1", expect.objectContaining({ excludes: ["cache/", ".DS_Store"] }));
  });
});
