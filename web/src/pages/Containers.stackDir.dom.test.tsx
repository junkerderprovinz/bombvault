// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import type { Container } from "../lib/api";
import { useT } from "../lib/i18n";
import { placementView, renderWithProviders, stubEventSource } from "../lib/placement.testsupport";

const fake = await vi.hoisted(async () => (await import("../lib/placement.testsupport")).createPlacementApi());

vi.mock("../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/api")>()),
  ...fake.api,
  listOffsiteTargets: () =>
    Promise.resolve({ ok: true, targets: [{ id: "t-b2", domain: "containers", name: "B2", repo: "b2:bucket:c", enabled: true }] }),
  listRuns: () => Promise.resolve({ ok: true, runs: [] }),
}));

const { StackCard } = await import("./Containers");

stubEventSource();

function member(name: string): Container {
  return {
    name,
    image: "",
    state: "exited",
    status: "",
    ip: "",
    installed: false,
    includeInSchedule: false,
    lastBackup: 1,
    lastBackupStarted: null,
    preHook: "",
    postHook: "",
    stopContainers: [],
    excludes: [],
    lastUpdateCheck: 0,
    lastUpdateResult: "",
    stack: "immich",
    placement: placementView(),
  };
}

function Card() {
  const { t } = useT();
  return (
    <StackCard group={{ project: "immich", members: [member("immich-db"), member("immich-server")] }} onRestored={() => {}} t={t} index={0} />
  );
}

async function startRestore(offsite: boolean) {
  renderWithProviders(<Card />);
  fireEvent.click(screen.getByRole("button", { name: "Restore stack…" }));
  if (offsite) fireEvent.click(screen.getByRole("tab", { name: "Off-site" }));
  fireEvent.click(screen.getAllByRole("button", { name: "Restore stack…" })[1]);
  fireEvent.click(await screen.findByRole("button", { name: "Confirm" }));
}

describe("stack restore and its project folder", () => {
  beforeEach(() => fake.reset());
  afterEach(cleanup);

  it("asks before taking the project folder from the local backup", async () => {
    fake.reply("getStackDir", { ok: true, found: false, time: "" });
    await startRestore(true);
    expect(await screen.findByText("The project folder is not in B2. Use the state from Unraid?")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Confirm" }));
    await waitFor(() => expect(fake.callsTo("restoreStack")).toEqual([["immich", true, true, "offsite", "local"]]));
    expect(fake.callsTo("getStackDir")).toEqual([["immich", "offsite"]]);
  });

  it("restores nothing when the answer is no", async () => {
    fake.reply("getStackDir", { ok: true, found: false, time: "" });
    await startRestore(true);
    await screen.findByText("The project folder is not in B2. Use the state from Unraid?");
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.queryByText("The project folder is not in B2. Use the state from Unraid?")).toBeNull());
    expect(fake.callsTo("restoreStack")).toEqual([]);
  });

  it("does not ask when the target holds the project folder", async () => {
    await startRestore(true);
    await waitFor(() => expect(fake.callsTo("restoreStack")).toEqual([["immich", true, true, "offsite", undefined]]));
  });

  it("does not look for the folder in a local restore", async () => {
    await startRestore(false);
    await waitFor(() => expect(fake.callsTo("restoreStack")).toEqual([["immich", true, true, "local", undefined]]));
    expect(fake.callsTo("getStackDir")).toEqual([]);
  });
});
