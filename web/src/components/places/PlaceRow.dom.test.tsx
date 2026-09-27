// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { I18nProvider, countText, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import { PLACES_CHANGED, type Place } from "../../lib/places";

const calls: string[] = [];
let testAnswer: { ok: boolean; code?: string; error?: string } = { ok: true };
let deleteAnswer: Record<string, unknown> = { ok: true };

vi.mock("../../lib/places", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/places")>();
  return {
    ...actual,
    testPlace: (id: string) => {
      calls.push(`test ${id}`);
      return Promise.resolve(testAnswer);
    },
    deletePlace: (id: string) => {
      calls.push(`delete ${id}`);
      return Promise.resolve(deleteAnswer);
    },
    patchPlace: (id: string, body: Partial<Place>) => {
      calls.push(`patch ${id} ${JSON.stringify(body)}`);
      return Promise.resolve({ ok: true, place: { ...place(), ...body } });
    },
  };
});

const { PlaceRow } = await import("./PlaceRow");

function place(over: Partial<Place> = {}): Place {
  return {
    id: "p1",
    name: "NAS Keller",
    provider: "synology",
    kind: "local",
    base: "remotes/syno/bombvault",
    folders: { containers: "container" },
    offPremises: true,
    storageClass: "",
    immutable: false,
    retentionKeepLast: 0,
    retentionKeepDaily: 7,
    retentionKeepWeekly: 4,
    retentionKeepMonthly: 6,
    limitUpload: 0,
    limitDownload: 0,
    growthBudgetGb: 0,
    enabled: true,
    sortOrder: 0,
    usage: { homeDomains: ["containers", "vms"], defaults: [], copyDomains: ["flash"], items: 2, copies: 0, repositories: 0 },
    locked: { containers: true },
    repository: false,
    creds: { shared: false, fields: {}, set: [] },
    ...over,
  };
}

beforeEach(() => {
  calls.length = 0;
  testAnswer = { ok: true };
  deleteAnswer = { ok: true };
});
afterEach(cleanup);

function row(p: Place = place()) {
  render(
    <I18nProvider>
      <ToastProvider>
        <PlaceRow place={p} hueIndex={0} hostMountRoot="/mnt" onSaved={vi.fn()} />
      </ToastProvider>
    </I18nProvider>
  );
}

const button = (key: keyof typeof en) => screen.getByRole("button", { name: en[key] });

describe("PlaceRow", () => {
  it("shows the mark, name, kind, site and use of a place", () => {
    row(place({ lastTest: { at: Math.floor(Date.now() / 1000) - 120, ok: true, source: "run" } }));
    expect(document.querySelector("svg[data-mark='synology']")?.getAttribute("width")).toBe("32");
    expect(screen.getByText("NAS Keller")).toBeTruthy();
    expect(screen.getByText(`Synology · ${en["places.kind.local"]}`)).toBeTruthy();
    expect(screen.getByText(en["places.row.otherSite"])).toBeTruthy();
    expect(screen.getByText("Stores Containers and VMs · Copies of Flash · 2 items")).toBeTruthy();
    expect(screen.getByText("Last copy 2 minutes ago")).toBeTruthy();
  });

  it("says a place is unused and untested", () => {
    row(place({ offPremises: false, usage: { homeDomains: [], defaults: [], copyDomains: [], items: 0, copies: 0, repositories: 0 } }));
    expect(screen.getByText(en["places.row.unused"])).toBeTruthy();
    expect(screen.getByText(en["places.row.untested"])).toBeTruthy();
    expect(screen.queryByText(en["places.row.otherSite"])).toBeNull();
  });

  it("folds its details out and in", async () => {
    row();
    expect(screen.queryByText(en["places.details.retention"])).toBeNull();
    fireEvent.click(button("places.row.showDetails"));
    expect(screen.getByText(en["places.details.retention"])).toBeTruthy();
    await act(async () => {
      fireEvent.click(button("places.row.closeDetails"));
    });
    expect(screen.queryByText(en["places.details.retention"])).toBeNull();
  });

  it.each([
    ["saves it", "common.confirm", ['patch p1 {"retentionKeepDaily":3}']],
    ["puts it back", "common.cancel", []],
  ] as const)("asks about lower retention typed just before closing, then %s and closes", async (_, answer, sent) => {
    row();
    fireEvent.click(button("places.row.showDetails"));
    fireEvent.change(screen.getByLabelText(en["places.details.keepDaily"]), { target: { value: "3" } });
    await act(async () => {
      fireEvent.click(button("places.row.closeDetails"));
    });
    const ask = screen.getByRole("dialog");
    expect(within(ask).getByText(countText(en["places.details.retentionLowerAsk"], "en", 2))).toBeTruthy();
    expect(screen.getByText(en["places.details.retention"])).toBeTruthy();
    expect(calls).toEqual([]);

    await act(async () => {
      fireEvent.click(within(ask).getByRole("button", { name: en[answer] }));
    });
    expect(calls).toEqual(sent);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.queryByText(en["places.details.retention"])).toBeNull();
  });

  it("tests every address and says so, or shakes when one fails", async () => {
    const changed = vi.fn();
    window.addEventListener(PLACES_CHANGED, changed);
    row();
    await act(async () => {
      fireEvent.click(button("places.row.test"));
    });
    expect(calls).toEqual(["test p1"]);
    expect(screen.getByText(en["places.row.testOk"].replace("{name}", "NAS Keller"))).toBeTruthy();
    expect(changed).toHaveBeenCalledTimes(1);

    testAnswer = { ok: false, code: "place-probe-failed", error: "the folder holds files but no restic repository" };
    await act(async () => {
      fireEvent.click(button("places.row.test"));
    });
    expect(screen.getByText("The connection test failed: the folder holds files but no restic repository")).toBeTruthy();
    expect(button("places.row.test").className).toContain("glim-shake");
    window.removeEventListener(PLACES_CHANGED, changed);
  });

  it("asks before it removes, naming the copies that stay", async () => {
    row(place({ usage: { homeDomains: [], defaults: [], copyDomains: ["flash"], items: 0, copies: 4, repositories: 0 } }));
    await act(async () => {
      fireEvent.click(button("places.row.remove"));
    });
    const ask = screen.getByRole("dialog");
    expect(within(ask).getByText(countText(en["places.row.removeAskCopies"], "en", 4).replace("{name}", "NAS Keller"))).toBeTruthy();
    await act(async () => {
      fireEvent.click(within(ask).getByRole("button", { name: en["places.row.remove"] }));
    });
    expect(calls).toEqual(["delete p1"]);
  });

  it("says what still holds a place it cannot remove", async () => {
    deleteAnswer = {
      ok: false,
      code: "place-in-use",
      error: "place in use",
      holders: { homeDomains: ["containers"], defaults: [], items: [], directInUse: [] },
    };
    row();
    await act(async () => {
      fireEvent.click(button("places.row.remove"));
    });
    await act(async () => {
      fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: en["places.row.remove"] }));
    });
    expect(screen.getByText("This place is still in use: where Containers are stored. Change that first.")).toBeTruthy();
    expect(button("places.row.remove").className).toContain("glim-shake");
  });
});
