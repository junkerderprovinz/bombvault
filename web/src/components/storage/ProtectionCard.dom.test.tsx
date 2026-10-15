// @vitest-environment jsdom
// The server refuses to test the delete protection of a switched-off copy, so
// the card tests only the copies that are on and says so when none is.
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import type { StorageLocation, StorageLocationSection } from "../../lib/api";
import { I18nProvider, en } from "../../lib/i18n";
import type { LocationEdit } from "./useLocationEdit";

const tested: string[] = [];

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    tamperTestOffsiteTarget: (id: string) => {
      tested.push(id);
      return Promise.resolve({ ok: true, testable: true, protected: true, detail: "" });
    },
  };
});

const { ProtectionCard } = await import("./ProtectionCard");

const KEEP = { keepLast: 0, keepDaily: 0, keepWeekly: 0, keepMonthly: 0, keepYearly: 0 };

function copy(domain: StorageLocationSection["domain"], targetId: string, enabled: boolean): StorageLocationSection {
  return { domain, use: "copy", targetId, where: "", enabled, immutable: true, retention: KEEP, compression: "auto", limitUpload: 0, limitDownload: 0, own: [] };
}

function edit(sections: StorageLocationSection[]): LocationEdit {
  const location: StorageLocation = {
    id: "destination:box",
    object: "destination",
    kind: "offsite",
    provider: "rest",
    backend: "rest",
    name: "Box",
    where: "rest:http://box:8000/bv",
    enabled: true,
    offPremises: true,
    sections,
    protection: { immutable: true, testable: true },
    capacity: {},
  };
  return {
    location,
    records: { targets: [] },
    can: () => true,
    save: () => Promise.resolve(true),
    follow: () => {},
    saveSection: () => Promise.resolve(true),
    followAll: () => {},
    saveConnection: () => Promise.resolve({ ok: true }),
    busy: false,
  };
}

function show(sections: StorageLocationSection[]) {
  render(
    <I18nProvider>
      <ProtectionCard edit={edit(sections)} hueIndex={0} />
    </I18nProvider>
  );
}

afterEach(() => {
  cleanup();
  tested.length = 0;
});

it("tests only the copies that are switched on", async () => {
  show([copy("containers", "t-on", true), copy("vms", "t-off", false)]);
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: en["storage.lock.testNow"] }));
  });
  expect(tested).toEqual(["t-on"]);
});

it("says every copy is switched off instead of offering a test", () => {
  show([copy("containers", "t-off", false)]);
  expect(screen.getByText(en["storage.lock.testOff"])).toBeTruthy();
  expect(screen.queryByRole("button", { name: en["storage.lock.testNow"] })).toBeNull();
});
