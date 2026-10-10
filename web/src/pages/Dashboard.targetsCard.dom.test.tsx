// @vitest-environment jsdom
// The card of the storage locations: each with how full it is and when it
// would be full, and a list that never arrived must not read as an empty one.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";

import { useAttention } from "../lib/attention";
import { en } from "../lib/i18n";
import { FakeEventSource, location, resetServer, server } from "./overview/overview.testsupport";
import { inApp } from "./overview/overviewApp.testsupport";
import { TargetsCard } from "./overview/TargetsCard";

vi.mock("../lib/api", async (importOriginal) => {
  const { stagedApi } = await import("./overview/overview.testsupport");
  return stagedApi(await importOriginal<typeof import("../lib/api")>());
});

const t = ((key: string, n?: number) => {
  const text = (en as Record<string, string>)[key] ?? key;
  if (n === undefined || !text.includes("|")) return text;
  const forms = Object.fromEntries(text.split("|").map((form) => form.split("=") as [string, string]));
  return (n === 1 ? forms.one : forms.other).replace("{n}", String(n));
}) as Parameters<typeof TargetsCard>[0]["t"];

const GB = 1024 ** 3;

beforeEach(() => {
  resetServer();
  vi.stubGlobal("EventSource", FakeEventSource);
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

/** The card with the list the page hands it. */
function Card() {
  return <TargetsCard t={t} list={useAttention().locations} />;
}

async function renderCard() {
  render(inApp(<Card />));
  await act(async () => {});
}

const rows = () => screen.getAllByTestId("target-row").map((row) => within(row));

describe("TargetsCard", () => {
  it("lists every location with its growth and when it would be full", async () => {
    server.locations = [location({ id: "path:1", name: "bombvault" }), location({ id: "destination:2", name: "Storage Box", kind: "offsite" })];
    await renderCard();

    const [local, box] = rows();
    expect(local.getByRole("link", { name: "bombvault" }).getAttribute("href")).toBe("/storage/path%3A1");
    expect(local.getByText(/Growing 10\.0 GB\/week/)).toBeTruthy();
    expect(local.getByText("full in about 33 weeks")).toBeTruthy();
    expect(box.getByRole("link", { name: "Storage Box" })).toBeTruthy();
  });

  it("warns when a location is full within weeks", async () => {
    server.locations = [
      location({ capacity: { usedBytes: 950 * GB, totalBytes: 1000 * GB, growthBytesPerWeek: 20 * GB, weeksToFull: 3 } }),
    ];
    await renderCard();

    expect(rows()[0].getByText("full in about 3 weeks").className).toContain("text-statusWarn");
  });

  it("says that the free space is unknown where the backend reports none", async () => {
    server.locations = [location({ capacity: { unsupported: true, storedBytes: 2 * GB, growthBytesPerWeek: GB } })];
    await renderCard();

    expect(rows()[0].getByText(en["dashboard.forecastFreeUnknown"])).toBeTruthy();
    expect(rows()[0].queryByText(/full in/)).toBeNull();
  });

  it("shows a switched-off location as off, without a forecast", async () => {
    server.locations = [location({ enabled: false })];
    await renderCard();

    expect(rows()[0].getByText(en["storage.off"])).toBeTruthy();
    expect(rows()[0].queryByText(/Growing/)).toBeNull();
  });

  it("says so when there is no storage location yet", async () => {
    await renderCard();

    expect(screen.getByText(en["storage.empty"])).toBeTruthy();
  });

  it("reports a list that did not load, and reads it again on request", async () => {
    server.locations = "refused";
    await renderCard();
    expect(screen.getByText(en["storage.loadFailed"])).toBeTruthy();
    expect(screen.queryByText(en["storage.empty"])).toBeNull();

    server.locations = [location()];
    fireEvent.click(screen.getByRole("button", { name: en["anomaly.retry"] }));
    await act(async () => {});

    expect(rows()).toHaveLength(1);
  });
});
