// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, screen } from "@testing-library/react";
import type { StorageLocation, TimelinePlace, TimelineRow } from "../../lib/api";
import { LocationsContext } from "../../lib/attention";
import { countText, en } from "../../lib/i18n";
import { renderWithProviders } from "../../lib/placement.testsupport";
import { BackupPlaces, type TimelineState } from "./BackupPlaces";

function place(over: Partial<TimelinePlace>): TimelinePlace {
  return { place: "local", label: "", kind: "home", remote: false, enabled: true, appendOnly: false, state: "read", ...over };
}

function timeline(places: TimelinePlace[], rows: TimelineRow[] = []): TimelineState {
  return { domain: "files", places, rows, loading: false, error: null, reading: new Set(), loadPlace: async () => {}, reload: () => {} };
}

function folder(name: string, domain: "files" | "containers"): StorageLocation {
  return {
    id: `path:${name}`,
    object: "path",
    kind: "local",
    provider: "",
    backend: "local",
    name,
    where: `user/${name}`,
    enabled: true,
    offPremises: false,
    sections: [
      {
        domain,
        use: "home",
        where: `user/${name}/${domain}`,
        enabled: true,
        immutable: false,
        retention: { keepLast: 0, keepDaily: 7, keepWeekly: 4, keepMonthly: 6, keepYearly: 1 },
        compression: "auto",
        limitUpload: 0,
        limitDownload: 0,
        own: [],
      },
    ],
    protection: { immutable: false, testable: false },
    capacity: {},
  };
}

// An entry whose section switched Local off: it writes to the direct
// repository of Primary, and its old backups stay in the section's folder.
const LEFT_LOCAL = [
  place({ label: "", directOf: "Primary", remote: true }),
  place({ place: "folder", kind: "earlier" }),
];

describe("BackupPlaces", () => {
  afterEach(cleanup);

  it("calls the folder an entry left Local, as the storage pages do", () => {
    renderWithProviders(<BackupPlaces timeline={timeline(LEFT_LOCAL)} hueIndex={0} />);
    expect(screen.getByText("Primary · direct")).toBeTruthy();
    const earlier = screen.getByText(en["entry.places.earlier"]).parentElement!;
    expect(earlier.firstChild?.textContent).toBe(en["placement.segLocal"]);
    expect(screen.queryByText(/Host|Unraid/)).toBeNull();
  });

  it("names the folder after itself where the storage pages list several", () => {
    renderWithProviders(
      <LocationsContext.Provider value={[folder("bombvault", "containers"), folder("cache", "files")]}>
        <BackupPlaces timeline={timeline(LEFT_LOCAL)} hueIndex={0} />
      </LocationsContext.Provider>
    );
    expect(screen.getByText(en["entry.places.earlier"]).parentElement!.firstChild?.textContent).toBe("cache");
  });
});

describe("BackupPlaces of an entry that only replicates", () => {
  afterEach(cleanup);

  // The section's own folder on a bucket, read and empty, and a switched-off
  // off-site location nobody has checked.
  const SWITCHED_OFF = [
    place({ remote: true }),
    place({ place: "offsite:t1", label: "Wasabi", kind: "target", remote: true, enabled: false, state: "unchecked" }),
  ];
  const HELD: TimelineRow = { key: "20261020T030000Z", time: "2026-10-20T03:00:00Z", places: [{ place: "local", snapshotIds: ["a1"], tags: [] }] };
  const noBackups = countText(en["entry.places.count"], "en", 0);

  it("does not list the places it no longer backs up to", () => {
    renderWithProviders(<BackupPlaces timeline={timeline(SWITCHED_OFF)} hueIndex={0} />);
    expect(screen.getByText(noBackups)).toBeTruthy();
    cleanup();
    renderWithProviders(<BackupPlaces timeline={timeline(SWITCHED_OFF)} replicaOnly hueIndex={0} />);
    expect(screen.queryByText(noBackups)).toBeNull();
    expect(screen.queryByText("Wasabi")).toBeNull();
    expect(screen.queryByText(en["storage.title"])).toBeNull();
  });

  it("keeps a place that still holds backups of it", () => {
    renderWithProviders(<BackupPlaces timeline={timeline(SWITCHED_OFF, [HELD])} replicaOnly hueIndex={0} />);
    expect(screen.getByText(countText(en["entry.places.count"], "en", 1))).toBeTruthy();
    expect(screen.queryByText("Wasabi")).toBeNull();
  });
});
