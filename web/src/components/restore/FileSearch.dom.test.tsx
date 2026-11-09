// @vitest-environment jsdom
/**
 * The search for a file over every backup on the Restore page.
 *
 * It asks the server for a name, lists each file with every backup that holds
 * it, says which places it read, and hands a picked backup to the restore
 * window with the file in it.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";

import type { BackupItem, FileSearchResponse } from "../../lib/api";
import { I18nProvider, countText, en } from "../../lib/i18n";
import { IMMICH, WIN11, entryItem } from "../../pages/backupEntry.testsupport";

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return { ...actual, searchFiles: vi.fn() };
});

const api = await import("../../lib/api");
const searchFiles = vi.mocked(api.searchFiles);
const { FileSearch } = await import("./FileSearch");

const NEWER = "2026-11-08T03:00:00Z";
const OLDER = "2026-11-07T03:00:00Z";

function found(over: Partial<FileSearchResponse> = {}): FileSearchResponse {
  return {
    ok: true,
    files: [
      {
        domain: "containers",
        key: IMMICH.key,
        path: "/config/app.conf",
        type: "file",
        versions: [
          { at: 0, row: "r0", time: NEWER, place: "local", snapshot: "s0", size: 2048, modified: "2026-11-07T20:00:00Z" },
          { at: 0, row: "r1", time: OLDER, place: "local", snapshot: "s1", size: 1024, modified: "2026-11-06T20:00:00Z" },
        ],
      },
    ],
    places: [
      { label: "", domain: "containers", remote: false, state: "searched", backups: 2 },
      { label: "B2", domain: "containers", remote: true, state: "notAsked", backups: 0 },
    ],
    more: false,
    ...over,
  };
}

const onOpen = vi.fn();

async function show(items: BackupItem[] = [IMMICH]) {
  await act(async () => {
    render(
      <I18nProvider>
        <FileSearch items={items} onOpen={onOpen} />
      </I18nProvider>
    );
  });
}

async function search(name: string) {
  fireEvent.change(screen.getByRole("searchbox", { name: en["fileSearch.placeholder"] }), { target: { value: name } });
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: en["fileSearch.search"] }));
  });
}

beforeEach(() => {
  searchFiles.mockResolvedValue(found());
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("FileSearch", () => {
  it("lists every backup that holds the file and opens the one picked", async () => {
    await show();
    await search("app.conf");

    expect(searchFiles).toHaveBeenCalledWith("app.conf", false, expect.any(AbortSignal));
    const hit = screen.getByTestId("found-file");
    expect(within(hit).getByText(IMMICH.name)).toBeTruthy();
    expect(within(hit).getByText("/config/app.conf")).toBeTruthy();
    const versions = within(hit).getAllByRole("button", { name: /KB/ });
    expect(versions).toHaveLength(2);
    await act(async () => {
      fireEvent.click(versions[1]);
    });
    expect(onOpen).toHaveBeenCalledWith(IMMICH, {
      snapshot: "s1",
      at: Date.parse(OLDER) / 1000,
      place: "local",
      path: "/config/app.conf",
    });
  });

  it("reads the off-site places only when they are switched on", async () => {
    await show();
    await act(async () => {
      fireEvent.click(screen.getByRole("switch", { name: en["fileSearch.offsite"] }));
    });
    await search("app.conf");
    expect(searchFiles).toHaveBeenCalledWith("app.conf", true, expect.any(AbortSignal));
  });

  it("says which places it read and which it left alone", async () => {
    await show();
    await search("app.conf");
    const places = screen.getByRole("list", { name: en["fileSearch.places"] });
    expect(within(places).getByText(countText(en["fileSearch.place.searched"], "en", 2))).toBeTruthy();
    expect(within(places).getByText("B2")).toBeTruthy();
    expect(within(places).getByText(en["fileSearch.place.notAsked"])).toBeTruthy();
  });

  it("stops a search that runs", async () => {
    let signal: AbortSignal | undefined;
    searchFiles.mockImplementation((_name, _offsite, s) => {
      signal = s;
      return new Promise(() => {});
    });
    await show();
    await search("app.conf");
    expect(screen.getByText(en["fileSearch.running"])).toBeTruthy();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["fileSearch.stop"] }));
    });
    expect(signal?.aborted).toBe(true);
    expect(screen.queryByText(en["fileSearch.running"])).toBeNull();
    expect(screen.getByRole("button", { name: en["fileSearch.search"] })).toBeTruthy();
  });

  it("asks for a name without a folder before it asks the server", async () => {
    await show();
    await search("config/app.conf");
    expect(searchFiles).not.toHaveBeenCalled();
    expect(screen.getByText(en["fileSearch.noSlash"])).toBeTruthy();
  });

  it("says when no backup holds the name and when it lists only the first files", async () => {
    searchFiles.mockResolvedValue(found({ files: [] }));
    await show();
    await search("nothing");
    expect(screen.getByText(en["fileSearch.none"])).toBeTruthy();

    cleanup();
    searchFiles.mockResolvedValue(found({ more: true }));
    await show();
    await search("app");
    expect(screen.getByText(en["fileSearch.more"].replace("{n}", "1"))).toBeTruthy();
  });

  it("explains that the files inside a virtual machine are not searched", async () => {
    await show([IMMICH, WIN11]);
    expect(screen.getByText(en["fileSearch.vms"])).toBeTruthy();
    cleanup();
    await show([IMMICH, entryItem({ ...WIN11, lastBackup: 0 })]);
    expect(screen.queryByText(en["fileSearch.vms"])).toBeNull();
  });
});
