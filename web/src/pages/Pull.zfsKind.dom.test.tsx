// @vitest-environment jsdom
// A new pull source asks first what it fetches. ZFS datasets swap the restic
// fields for the source's ZFS items and the pool and root they land under here.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { PullSourceInput, PullSourceView, ZFSHostDataset } from "../lib/api";
import { group } from "../components/zfs/replica/replica.testsupport";

const created: PullSourceInput[] = [];
let rows: PullSourceView[] = [];

function pool(dataset: string): ZFSHostDataset {
  return { dataset, type: "filesystem" } as ZFSHostDataset;
}

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listPullSources: () => Promise.resolve({ ok: true, sources: rows }),
    getGroup: () => Promise.resolve(group()),
    memberRepos: () => Promise.resolve({ ok: true, repos: [] }),
    getCloudCredSets: () => Promise.resolve({ ok: true, sets: [] }),
    zfsHostDatasets: () =>
      Promise.resolve({ ok: true, available: true, datasets: [pool("tank"), pool("tank/media"), pool("cache")] }),
    createPullSource: (body: PullSourceInput) => {
      created.push(body);
      return Promise.resolve({ ok: true });
    },
  };
});

const { Pull } = await import("./Pull");

async function openDialog() {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <Pull />
        </ToastProvider>
      </I18nProvider>,
    );
  });
  fireEvent.click(await screen.findByRole("button", { name: en["pull.addSource"] }));
}

async function choose(combobox: string, option: string) {
  fireEvent.click(await screen.findByRole("combobox", { name: combobox }));
  fireEvent.click(screen.getByRole("option", { name: option }));
}

beforeEach(() => {
  created.length = 0;
  rows = [];
  localStorage.clear();
});

afterEach(cleanup);

describe("a ZFS pull source", () => {
  it("starts as a restic source with its repository field", async () => {
    await openDialog();
    const kind = screen.getByRole("radio", { name: en["zfs.replica.pull.restic"] });
    expect(kind.getAttribute("aria-checked")).toBe("true");
    expect(screen.getByText(en["pull.repoLocation"])).toBeTruthy();
    expect(screen.queryByLabelText(en["zfs.replica.pull.datasets"])).toBeNull();
  });

  it("asks for the source's ZFS items and where they land, then saves them", async () => {
    await openDialog();
    await choose(en["pull.member"], `tower-2 (${en["pairing.direct"]})`);
    fireEvent.click(screen.getByRole("radio", { name: en["zfs.title"] }));
    expect(screen.queryByText(en["pull.repoLocation"])).toBeNull();
    expect(screen.getByDisplayValue("ZFS from tower-2")).toBeTruthy();
    expect(screen.getByText(en["zfs.replica.pull.asks"].replace("{peer}", "tower-2"))).toBeTruthy();

    const save = screen.getByRole("button", { name: en["settings.save"] }) as HTMLButtonElement;
    expect(save.disabled).toBe(true);
    expect(screen.getByText(en["zfs.replica.pull.needDataset"])).toBeTruthy();

    fireEvent.change(screen.getByLabelText(en["zfs.replica.pull.datasets"]), {
      target: { value: "cache/appdata\n\ncache/domains " },
    });
    await choose(en["zfs.replica.pull.pool"], "tank");
    expect((screen.getByLabelText(en["zfs.replica.server.root"]) as HTMLInputElement).value).toBe("tank/bombvault-replica");

    fireEvent.click(screen.getByRole("radio", { name: en["zfs.replica.keep.short"] }));
    fireEvent.click(save);
    await waitFor(() => expect(created).toHaveLength(1));
    expect(created[0]).toMatchObject({
      name: "ZFS from tower-2",
      memberId: "peer-1",
      kind: "zfs",
      domain: "zfs",
      repo: "",
      datasets: ["cache/appdata", "cache/domains"],
      pool: "tank",
      root: "tank/bombvault-replica",
      keep: { preset: "short", own: [0, 7, 3, 0, 0] },
    });
  });

  it("shows a source still waiting for the other side to allow it", async () => {
    rows = [
      {
        id: "p1",
        name: "ZFS from tower-2",
        repo: "",
        credsRef: "",
        domain: "zfs",
        cadence: "",
        limitDownload: 0,
        limitUpload: 0,
        lastPullAt: 0,
        lastPullOk: null,
        lastPullError: "",
        snapshotsPulled: 0,
        enabled: true,
        createdAt: 1_600_000_000,
        sortOrder: 0,
        memberId: "peer-1",
        needsPairing: false,
        kind: "zfs",
        datasets: ["cache/appdata"],
        pool: "tank",
        root: "tank/bombvault-replica",
        state: "asked",
      },
    ];
    await act(async () => {
      render(
        <I18nProvider>
          <ToastProvider>
            <Pull />
          </ToastProvider>
        </I18nProvider>,
      );
    });
    expect(await screen.findByText("Waiting for tower-2 to allow it")).toBeTruthy();
    expect(screen.getByText("cache/appdata → tank/bombvault-replica")).toBeTruthy();
  });
});
