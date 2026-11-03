// @vitest-environment jsdom
// The server refuses to test the delete protection of a switched-off copy, so
// the card tests only the copies that are on and says so when none is. Where
// there is no delete protection, the card lets the user accept that.
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

function edit(
  sections: StorageLocationSection[],
  protection: StorageLocation["protection"] = { immutable: true, testable: true },
  save: LocationEdit["save"] = () => Promise.resolve(true),
  backend = "rest"
): LocationEdit {
  const location: StorageLocation = {
    id: "destination:box",
    object: "destination",
    kind: "offsite",
    provider: "rest",
    backend,
    name: "Box",
    where: "rest:http://box:8000/bv",
    enabled: true,
    offPremises: true,
    sections,
    protection,
    capacity: {},
  };
  return {
    location,
    records: { targets: [] },
    can: () => true,
    save,
    follow: () => {},
    saveSection: () => Promise.resolve(true),
    followAll: () => {},
    saveConnection: () => Promise.resolve({ ok: true }),
    busy: false,
  };
}

function show(sections: StorageLocationSection[], location: LocationEdit = edit(sections)) {
  render(
    <I18nProvider>
      <ProtectionCard edit={location} hueIndex={0} />
    </I18nProvider>
  );
}

const unprotected = { immutable: false, testable: false };

function acceptSwitch() {
  return screen.queryByRole("switch", { name: en["storage.lock.accept"] });
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

it("offers to accept a location without delete protection and saves the choice", async () => {
  const save = vi.fn<LocationEdit["save"]>(() => Promise.resolve(true));
  const copies = [copy("containers", "t1", true)];
  show(copies, edit(copies, unprotected, save));
  const toggle = acceptSwitch();
  expect(toggle?.getAttribute("aria-checked")).toBe("false");
  await act(async () => {
    fireEvent.click(toggle!);
  });
  expect(save).toHaveBeenCalledWith({ riskAccepted: true });
});

it("shows an accepted location as accepted, so the choice can be taken back", () => {
  const copies = [copy("containers", "t1", true)];
  show(copies, edit(copies, { ...unprotected, riskAccepted: true }));
  expect(acceptSwitch()?.getAttribute("aria-checked")).toBe("true");
});

it("has nothing to accept while delete protection is on", () => {
  show([copy("containers", "t1", true)]);
  expect(acceptSwitch()).toBeNull();
});

it("says every copy is switched off instead of offering a test", () => {
  show([copy("containers", "t-off", false)]);
  expect(screen.getByText(en["storage.lock.testOff"])).toBeTruthy();
  expect(screen.queryByRole("button", { name: en["storage.lock.testNow"] })).toBeNull();
});

it("offers the acceptance for copies on a folder of this host, without a switch for delete protection", () => {
  const copies = [copy("containers", "t1", true)];
  show(copies, edit(copies, unprotected, undefined, "local"));
  expect(acceptSwitch()).toBeTruthy();
  expect(screen.queryByRole("switch", { name: en["storage.lock.toggle"] })).toBeNull();
});
