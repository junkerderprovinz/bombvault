// @vitest-environment jsdom
/**
 * Links to a kind's own page.
 *
 * Bookmarks, release notes, notification texts and support answers carry
 * /containers, /vms, /files, /flash, /zfs and /config. Each has to land on
 * the list of its kind or on the entry it names, with what it asks for.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";

import { ENTRY_KIND, ENTRY_KINDS } from "../lib/backupEntry";
import { I18nProvider, en } from "../lib/i18n";
import { PHOTOS, SET_ID, TANK, ZFS_ID } from "./backupEntry.testsupport";

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return { ...actual, listItems: vi.fn() };
});

const { listItems } = vi.mocked(await import("../lib/api"));
const { KindRedirect } = await import("./KindRedirect");

function Landed() {
  const { pathname, search } = useLocation();
  return <p data-testid="landed">{`${pathname}${search}`}</p>;
}

async function follow(link: string) {
  await act(async () => {
    render(
      <I18nProvider>
        <MemoryRouter initialEntries={[link]}>
          <Routes>
            {ENTRY_KINDS.map((kind) => (
              <Route key={kind} path={ENTRY_KIND[kind].legacyPath} element={<KindRedirect kind={kind} />} />
            ))}
            <Route path="*" element={<Landed />} />
          </Routes>
        </MemoryRouter>
      </I18nProvider>
    );
  });
}

const landed = () => screen.getByTestId("landed").textContent;

beforeEach(() => {
  listItems.mockResolvedValue({ ok: true, items: [PHOTOS, TANK], unlisted: [] });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("a link without an entry", () => {
  it.each([
    ["/containers", "/backups?kind=container"],
    ["/vms", "/backups?kind=vm"],
    ["/files", "/backups?kind=files"],
    ["/zfs", "/backups?kind=zfs"],
    ["/flash", "/backups/flash/flash"],
    ["/config", "/backups/config/config"],
  ])("%s leads to %s without asking the server", async (link, target) => {
    await follow(link);
    expect(landed()).toBe(target);
    expect(listItems).not.toHaveBeenCalled();
  });
});

describe("a link that names an entry", () => {
  it("opens a container's page with the restore it asks for", async () => {
    await follow("/containers?restore=abc123&at=1700000000&item=immich&dump=1");
    expect(landed()).toBe("/backups/container/immich?restore=abc123&at=1700000000&dump=1");
    expect(listItems).not.toHaveBeenCalled();
  });

  it("opens a VM's page under its libvirt name", async () => {
    await follow("/vms?restore=abc123&at=1700000000&item=win11");
    expect(landed()).toBe("/backups/vm/win11?restore=abc123&at=1700000000");
  });

  it("keeps the restore request of the flash drive and of the self-backup", async () => {
    await follow("/flash?restore=abc123&at=1700000000");
    expect(landed()).toBe("/backups/flash/flash?restore=abc123&at=1700000000");
  });

  it("looks a folder set up by its name and opens the page of its id", async () => {
    await follow("/files?restore=abc123&at=1700000000&item=Photos");
    expect(landed()).toBe(`/backups/files/${SET_ID}?restore=abc123&at=1700000000`);
  });

  it("looks a ZFS item up by its root dataset and keeps the dataset the backup is of", async () => {
    await follow("/zfs?restore=abc123&at=1700000000&item=tank%2Fappdata&dataset=tank%2Fappdata%2Fimmich");
    expect(landed()).toBe(
      `/backups/zfs/${ZFS_ID}?restore=abc123&at=1700000000&dataset=tank%2Fappdata%2Fimmich`
    );
  });

  it("does not take a folder set for the ZFS item of the same name", async () => {
    listItems.mockResolvedValue({ ok: true, items: [{ ...PHOTOS, name: "tank/appdata" }], unlisted: [] });
    await follow("/zfs?item=tank%2Fappdata");
    expect(landed()).toBe("/backups?kind=zfs");
  });
});

describe("a link whose entry cannot be found", () => {
  it("leads to the list of its kind when the entry is gone", async () => {
    await follow("/files?restore=abc123&at=1700000000&item=Movies");
    expect(landed()).toBe("/backups?kind=files");
  });

  it("leads to the list of its kind when the server refuses the list", async () => {
    listItems.mockResolvedValue({ ok: false, items: [], unlisted: [] });
    await follow("/zfs?item=tank%2Fappdata");
    expect(landed()).toBe("/backups?kind=zfs");
  });

  it("leads to the list of its kind when the server does not answer", async () => {
    listItems.mockRejectedValue(new Error("offline"));
    await follow("/files?item=Photos");
    expect(landed()).toBe("/backups?kind=files");
  });

  it("says it is checking until the list arrives", async () => {
    listItems.mockReturnValue(new Promise(() => {}));
    await follow("/files?item=Photos");
    expect(screen.getByText(en["dashboard.checking"])).toBeTruthy();
    expect(screen.queryByTestId("landed")).toBeNull();
  });
});
