// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { StorageLocation } from "../../lib/api";
import { en } from "../../lib/i18n";
import type { LocationSetting } from "../../lib/storageWrites";
import type { LocationEdit } from "./useLocationEdit";

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  listRepos: vi.fn(async () => ({ ok: true, repos: [] })),
}));

const { ConnectionCard } = await import("./ConnectionCard");

const box: StorageLocation = {
  id: "destination:b1", object: "destination", kind: "offsite", provider: "box", backend: "rclone", name: "Box",
  where: "rclone:box:BombVault", enabled: true, offPremises: true, sections: [], connections: 0,
  protection: { immutable: false, testable: false }, capacity: {},
};

const save = vi.fn(async () => true);

function edit(location: StorageLocation): LocationEdit {
  return {
    location,
    records: { targets: [] },
    can: (setting: LocationSetting) => setting === "connections",
    save,
    follow: () => undefined,
    saveSection: async () => true,
    followAll: () => undefined,
    saveConnection: async () => ({ ok: true }),
    busy: false,
  };
}

function field(): HTMLInputElement {
  return screen.getByRole("spinbutton", { name: en["storage.connections"] }) as HTMLInputElement;
}

beforeEach(() => {
  vi.useFakeTimers();
  save.mockClear();
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("simultaneous connections", () => {
  it("starts empty on restic's default and saves a typed count", async () => {
    render(<ConnectionCard edit={edit(box)} hueIndex={0} />);
    expect(field().value).toBe("");
    expect(field().placeholder).toBe("5");

    fireEvent.change(field(), { target: { value: "12" } });
    await act(async () => {
      await vi.runAllTimersAsync();
    });
    expect(save).toHaveBeenCalledWith({ connections: 12 });
  });

  it("marks a count past the range and does not save it", async () => {
    render(<ConnectionCard edit={edit(box)} hueIndex={0} />);
    fireEvent.change(field(), { target: { value: "40" } });
    await act(async () => {
      await vi.runAllTimersAsync();
    });
    expect(save).not.toHaveBeenCalled();
    expect(field().getAttribute("aria-invalid")).toBe("true");
  });

  it("goes back to restic's default when the field is cleared", async () => {
    render(<ConnectionCard edit={edit({ ...box, connections: 8 })} hueIndex={0} />);
    expect(field().value).toBe("8");
    fireEvent.change(field(), { target: { value: "" } });
    await act(async () => {
      await vi.runAllTimersAsync();
    });
    expect(save).toHaveBeenCalledWith({ connections: 0 });
  });

  it("is not offered where the backend has no such setting", () => {
    render(<ConnectionCard edit={edit({ ...box, backend: "local", where: "/mnt/disks/usb", connections: undefined })} hueIndex={0} />);
    expect(screen.queryByRole("spinbutton", { name: en["storage.connections"] })).toBeNull();
  });
});
