// @vitest-environment jsdom
// A snapshot taken together with a database dump says so, at its home and at
// an off-site place, whose copy carries a new id and keeps the source id. The
// tags that make the pairing possible stay out of the row.
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";

class NoopEventSource {
  onmessage: ((e: MessageEvent) => void) | null = null;
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
(globalThis as unknown as { EventSource: unknown }).EventSource = NoopEventSource;

const TAGS = ["container:immich_postgres", "bvrun:run-7", "dbengine:postgres", "dbversion:16.4", "dbname:immich", "nightly"];

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getTimeline: () =>
      Promise.resolve({
        ok: true,
        places: [
          { place: "local", label: "", kind: "home", remote: false, enabled: true, appendOnly: false, state: "read" },
          { place: "offsite:t-b2", label: "B2", kind: "target", remote: false, enabled: true, appendOnly: false, state: "read" },
        ],
        rows: [
          {
            key: "a1b2c3d4e5f6",
            time: "2026-09-01T10:00:00Z",
            places: [
              { place: "local", snapshotIds: ["a1b2c3d4e5f6"], tags: TAGS },
              { place: "offsite:t-b2", snapshotIds: ["ffff0000ffff"], tags: TAGS },
            ],
          },
        ],
      }),
    listDbDumps: () =>
      Promise.resolve({
        ok: true,
        dumps: [
          {
            id: "9999dddd",
            time: "2026-09-01T09:59:00Z",
            engine: "postgres",
            image: "postgres:16",
            version: "16.4",
            databases: ["immich"],
            bytes: 1024,
            damaged: false,
            pairedSnapshotId: "a1b2c3d4e5f6",
          },
        ],
      }),
    listRuns: () => Promise.resolve({ ok: true, runs: [] }),
    getSettings: () => Promise.resolve({ ok: false }),
    listOffsiteTargets: () => Promise.resolve({ ok: true, targets: [] }),
  };
});

const { RestorePanel } = await import("./RestorePanel");

type PanelProps = Parameters<typeof RestorePanel>[0];
const t = ((key: string) => en[key as keyof typeof en] ?? key) as unknown as PanelProps["t"];

async function renderPanel(over: Partial<PanelProps> = {}) {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <RestorePanel name="immich_postgres" t={t} open isDatabase dbCoverage="live" containerRunning {...over} />
        </ToastProvider>
      </I18nProvider>
    );
  });
}

afterEach(cleanup);

it("marks the snapshot the dump belongs to", async () => {
  await renderPanel();
  expect(await screen.findByText(en["dbdump.pairedBadge"])).toBeTruthy();
});

it("marks an off-site copy of that snapshot too", async () => {
  await renderPanel();
  await act(async () => {
    fireEvent.click(await screen.findByRole("tab", { name: "B2" }));
  });
  expect(await screen.findByText(en["dbdump.pairedBadge"])).toBeTruthy();
});

it("keeps the pairing tags out of the row", async () => {
  await renderPanel();
  await screen.findByText(en["dbdump.pairedBadge"]);
  expect(screen.getByText("nightly")).toBeTruthy();
  for (const tag of ["bvrun:run-7", "dbengine:postgres", "dbversion:16.4", "dbname:immich"]) {
    expect(screen.queryByText(tag)).toBeNull();
  }
});

it("warns above the restore button that the files were copied while the server ran", async () => {
  await renderPanel();
  fireEvent.click(await screen.findByRole("button", { name: en["restore.open"] }));
  expect(await screen.findByText(en["dbdump.restoreLiveWarn"])).toBeTruthy();
});

it("warns that the data folder is in no backup at all", async () => {
  await renderPanel({ dbCoverage: "none" });
  fireEvent.click(await screen.findByRole("button", { name: en["restore.open"] }));
  expect(await screen.findByText(en["dbdump.restoreNoneWarn"])).toBeTruthy();
});

// jsdom lays nothing out, so this pins the class the phone layout rests on.
it("wraps a snapshot row with the badge on a phone instead of pushing its buttons out", async () => {
  await renderPanel();
  const badge = await screen.findByText(en["dbdump.pairedBadge"]);
  expect(badge.closest("span.flex")!.parentElement!.className).toContain("flex-wrap");
});
