// @vitest-environment jsdom
// The add window hands the folder browser the roots a provider may use:
// /mnt/remotes for a NAS share, the user shares and the disks and pools for a
// folder on this server. It must start inside them, stay inside them, and
// still make a new folder where it stands.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { I18nProvider, en, type TranslationKey } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";

const tree: Record<string, string[]> = {
  "": ["user", "disk1", "remotes"],
  user: ["appdata", "backups"],
  "user/backups": [],
  disk1: ["cold"],
  remotes: ["syno"],
  "remotes/syno": ["bombvault"],
};
const made: { path: string; name: string }[] = [];

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    browse: (path = "") =>
      Promise.resolve({
        ok: true,
        path,
        dirs: (tree[path] ?? []).map((name) => ({ name, path: path ? `${path}/${name}` : name })),
      }),
    createFolder: (path: string, name: string) => {
      made.push({ path, name });
      const created = path ? `${path}/${name}` : name;
      tree[path] = [...(tree[path] ?? []), name];
      tree[created] = [];
      return Promise.resolve({ ok: true, path: created, name });
    },
  };
});

const { FolderBrowser } = await import("./FolderBrowser");

const HERE: { path: string; labelKey: TranslationKey }[] = [
  { path: "user", labelKey: "places.root.shares" },
  { path: "", labelKey: "places.root.disks" },
];

beforeEach(() => {
  made.length = 0;
});
afterEach(cleanup);

async function openBrowser(value: string, roots?: { path: string; labelKey: TranslationKey }[], onChange = vi.fn()) {
  render(
    <I18nProvider>
      <ToastProvider>
        <FolderBrowser label="Folder" value={value} hostMountRoot="/mnt" onChange={onChange} inDialog roots={roots} />
      </ToastProvider>
    </I18nProvider>
  );
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: en["folder.browseTitle"] }));
  });
  return onChange;
}

const rows = () => screen.queryAllByRole("button").map((b) => b.textContent);

describe("FolderBrowser roots", () => {
  it("starts in the first root when the value lies outside every root", async () => {
    await openBrowser("", [{ path: "remotes", labelKey: "places.root.remotes" }]);
    expect(rows()).toContain("syno");
    expect(rows()).not.toContain("..");
    // A single root needs no picker.
    expect(screen.queryByRole("toolbar", { name: en["folder.roots"] })).toBeNull();
  });

  it("starts in the value's folder when it lies under a root", async () => {
    await openBrowser("remotes/syno", [{ path: "remotes", labelKey: "places.root.remotes" }]);
    expect(rows()).toContain("bombvault");
    expect(rows()).toContain("..");
  });

  it("offers the roots as a picker and never climbs above the root it is in", async () => {
    await openBrowser("", HERE);
    const picker = screen.getByRole("toolbar", { name: en["folder.roots"] });
    expect(within(picker).getAllByRole("button").map((b) => b.textContent)).toEqual([
      en["places.root.shares"],
      en["places.root.disks"],
    ]);
    expect(rows()).toContain("appdata");
    expect(rows()).not.toContain("..");

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "backups" }));
    });
    expect(rows()).toContain("..");
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: ".." }));
    });
    expect(rows()).not.toContain("..");

    await act(async () => {
      fireEvent.click(within(picker).getByRole("button", { name: en["places.root.disks"] }));
    });
    expect(rows()).toEqual(expect.arrayContaining(["disk1", "remotes"]));
    // At the top of /mnt the path line names the mount root.
    expect(screen.getByText("/mnt/")).toBeTruthy();
  });

  it("makes a new folder where it stands and moves into it", async () => {
    const onChange = await openBrowser("", HERE);
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "backups" }));
    });
    fireEvent.change(screen.getByPlaceholderText(en["folder.newFolderPlaceholder"]), { target: { value: "bombvault" } });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["folder.newFolder"] }));
    });
    expect(made).toEqual([{ path: "user/backups", name: "bombvault" }]);
    fireEvent.click(screen.getByRole("button", { name: en["folder.use"] }));
    expect(onChange).toHaveBeenCalledWith("user/backups/bombvault");
  });

  it("takes Escape before the window around it", async () => {
    const aroundIt = vi.fn();
    document.addEventListener("keydown", aroundIt);
    // No roots: this is about the key alone, which every browser shares.
    await openBrowser("user");
    expect(rows()).toContain("appdata");
    fireEvent.keyDown(screen.getByRole("button", { name: "appdata" }), { key: "Escape" });
    expect(rows()).not.toContain("appdata");
    expect(aroundIt).not.toHaveBeenCalled();
    document.removeEventListener("keydown", aroundIt);
  });

  it("draws only filled glyphs", async () => {
    await openBrowser("", HERE);
    const glyphs = document.querySelectorAll(".glim-btn-glyph svg");
    expect(glyphs.length).toBeGreaterThan(0);
    for (const svg of glyphs) expect(svg.querySelector("[stroke]")).toBeNull();
  });
});
