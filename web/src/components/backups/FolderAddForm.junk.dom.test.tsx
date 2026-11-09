// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { ExcludePreset, FileSetPresetResponse } from "../../lib/api";
import { en } from "../../lib/i18n";

const created = vi.hoisted(() => vi.fn(async () => ({ ok: true, id: "s1" })));

vi.mock("../files/createPlacedFileSet", () => ({ createPlacedFileSet: created }));
vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  listRepos: vi.fn(async () => ({ ok: true, repos: [] })),
  browse: vi.fn(async () => ({ ok: true, dirs: [], status: "ok", truncated: false })),
}));

const { FolderAddForm, withJunk } = await import("./FolderAddForm");

const junk: ExcludePreset = {
  app: "systemJunk",
  entries: [
    { line: ".DS_Store", kind: "macMetadata", optional: false },
    { line: "Thumbs.db", kind: "windowsThumbnails", optional: false },
    { line: "*.tmp", kind: "tempFiles", optional: false },
  ],
};
const preset: FileSetPresetResponse = { ok: true, offered: false, name: "", path: "", excludes: [], junk };

function renderForm() {
  render(
    <FolderAddForm
      found={[{ name: "photos", path: "user/photos" }]}
      foundUnder="user"
      taken={new Set()}
      preset={preset}
      hostMountRoot="/host"
      hostRoot="/mnt"
      onClose={() => undefined}
      onAdded={() => undefined}
    >
      {({ body, forward }) => (
        <>
          {body}
          {forward}
        </>
      )}
    </FolderAddForm>
  );
}

async function create() {
  fireEvent.click(screen.getByText("photos"));
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: en["dest.folder.create"] }));
  });
}

afterEach(cleanup);

describe("system junk on a new folder set", () => {
  it("is excluded whole unless a line is switched off", async () => {
    renderForm();
    for (const kind of ["macMetadata", "windowsThumbnails", "tempFiles"] as const) {
      expect(screen.getByRole("switch", { name: en[`excludes.presetKind.${kind}`] }).getAttribute("aria-checked")).toBe("true");
    }
    await create();
    expect(created.mock.calls[0][0]).toMatchObject({ excludes: [".DS_Store", "Thumbs.db", "*.tmp"] });
  });

  it("leaves out a line the user switched off", async () => {
    created.mockClear();
    renderForm();
    fireEvent.click(screen.getByRole("switch", { name: en["excludes.presetKind.windowsThumbnails"] }));
    await create();
    expect(created.mock.calls[0][0]).toMatchObject({ excludes: [".DS_Store", "*.tmp"] });
  });
});

describe("withJunk", () => {
  it("puts the junk lines behind the typed ones and names each line once", () => {
    expect(withJunk("cache/\n\n *.tmp \n", ["*.tmp", ".DS_Store"])).toEqual(["cache/", "*.tmp", ".DS_Store"]);
  });
});
