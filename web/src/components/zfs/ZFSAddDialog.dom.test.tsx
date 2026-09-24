// @vitest-environment jsdom
// The dialog that turns the host's pools into items. It is the only place that
// decides what a new item will contain, so the tests pin what a child starts
// as, what never gets a switch at all and what the submit sends.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { AdvancedProvider } from "../../lib/advanced";
import { I18nProvider, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import type { ZFSCreateItem, ZFSCreateResult, ZFSHostDataset, ZFSHostResult } from "../../lib/api";

let host: ZFSHostResult;
let results: ZFSCreateResult[] = [];
const created: ZFSCreateItem[][] = [];
const calls: string[] = [];
let probeGate: (() => void) | null = null;

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    zfsHostDatasets: () => Promise.resolve(host),
    listRepos: () => Promise.resolve({ ok: true, repos: [] }),
    createZFSDatasets: (items: ZFSCreateItem[]) => {
      created.push(items);
      return Promise.resolve({ ok: true, results });
    },
    probeZFSDataset: (id: string) => {
      calls.push(`probe:${id}`);
      if (!probeGate) return Promise.resolve({ ok: true });
      return new Promise((resolve) => {
        probeGate = () => resolve({ ok: true });
      });
    },
  };
});

const { ZFSAddDialog } = await import("./ZFSAddDialog");

function entry(overrides: Partial<ZFSHostDataset>): ZFSHostDataset {
  return {
    dataset: "cache",
    type: "filesystem",
    hostMountpoint: "/mnt/cache",
    referenced: 1024,
    used: 4096,
    usedByDataset: 1024,
    mounted: true,
    encrypted: false,
    keyLoaded: true,
    visible: true,
    writable: true,
    memberCode: "",
    managedId: "",
    coveredBy: "",
    vmDisk: false,
    system: false,
    vmVolume: false,
    blockers: [],
    ...overrides,
  };
}

function hostResult(datasets: ZFSHostDataset[], extra: Partial<ZFSHostResult> = {}): ZFSHostResult {
  return {
    ok: true,
    available: true,
    code: "ok",
    target: "root@192.168.1.10",
    datasets,
    hiddenLegacy: 0,
    unusedZvols: 0,
    notInItem: 0,
    truncated: false,
    maxNameLength: 219,
    ...extra,
  };
}

const POOL = entry({});
const APPDATA = entry({ dataset: "cache/appdata", hostMountpoint: "/mnt/cache/appdata" });
const PLEX = entry({ dataset: "cache/appdata/plex", hostMountpoint: "/mnt/cache/appdata/plex" });
const MEDIA = entry({ dataset: "cache/appdata/media", hostMountpoint: "/mnt/cache/appdata/media" });
const TANK = entry({ dataset: "tank", hostMountpoint: "/mnt/tank" });
const TANK_MEDIA = entry({ dataset: "tank/media", hostMountpoint: "/mnt/tank/media" });

const onClose = vi.fn();
const onAdded = vi.fn();

function renderDialog() {
  return render(
    <I18nProvider>
      <AdvancedProvider>
        <ToastProvider>
          <ZFSAddDialog onClose={onClose} onAdded={onAdded} />
        </ToastProvider>
      </AdvancedProvider>
    </I18nProvider>,
  );
}

/** Renders and waits for the host listing to arrive. */
async function openDialog() {
  const view = renderDialog();
  await screen.findByRole("tree");
  return view;
}

/** The switch that makes a dataset an item of its own. */
function rootSwitch(dataset: string) {
  return screen.getByRole("switch", { name: `${en["zfs.add.asItem"]} ${dataset}` });
}

beforeEach(() => {
  host = hostResult([POOL, APPDATA, PLEX, MEDIA]);
  results = [];
  created.length = 0;
  calls.length = 0;
  probeGate = null;
  onClose.mockClear();
  onAdded.mockClear();
});

afterEach(cleanup);

describe("ZFS add dialog", () => {
  it("opens the subtree of a chosen root with a switch for every child", async () => {
    const { container } = await openDialog();
    expect(screen.queryByText("plex")).toBeNull();
    fireEvent.click(rootSwitch("cache/appdata"));
    expect(screen.getByRole("switch", { name: "cache/appdata/plex" })).toBeTruthy();
    expect(screen.getByRole("switch", { name: "cache/appdata/media" })).toBeTruthy();
    expect(container.querySelector('input[type="checkbox"]')).toBeNull();
  });

  it("leaves a VM disk and system data out of a new item and says why", async () => {
    host = hostResult([
      POOL,
      APPDATA,
      entry({ dataset: "cache/appdata/vmdisks", vmDisk: true }),
      entry({ dataset: "cache/appdata/libvirt", system: true }),
    ]);
    await openDialog();
    fireEvent.click(rootSwitch("cache/appdata"));
    expect(screen.getByRole("switch", { name: "cache/appdata/vmdisks" }).getAttribute("aria-checked")).toBe("false");
    expect(screen.getByRole("switch", { name: "cache/appdata/libvirt" }).getAttribute("aria-checked")).toBe("false");
    expect(screen.getByText(en["zfs.add.vmDisk"])).toBeTruthy();
    expect(screen.getByText(en["zfs.add.system"])).toBeTruthy();
  });

  it("marks a child the backup cannot read as skipped", async () => {
    host = hostResult([POOL, APPDATA, entry({ dataset: "cache/appdata/enc", memberCode: "key-not-loaded" })]);
    await openDialog();
    fireEvent.click(rootSwitch("cache/appdata"));
    expect(
      screen.getByText(en["zfs.add.willSkip"].replace("{reason}", en["zfs.code.key-not-loaded"])),
    ).toBeTruthy();
  });

  it("gives a blocked root its reason instead of a switch", async () => {
    host = hostResult([
      POOL,
      entry({ dataset: "cache/docker", blockers: ["docker-storage"] }),
      entry({ dataset: "tank", hostMountpoint: "/mnt/tank", blockers: ["overlaps-item"] }),
      entry({ dataset: "tank/data", hostMountpoint: "/mnt/tank/data", managedId: "z9" }),
    ]);
    await openDialog();
    expect(screen.getByText(en["zfs.code.docker-storage"])).toBeTruthy();
    expect(screen.queryByRole("switch", { name: `${en["zfs.add.asItem"]} cache/docker` })).toBeNull();
    expect(screen.getByText(en["zfs.code.overlaps-item"].replace("{names}", "tank/data"))).toBeTruthy();
    expect(screen.getByText(en["zfs.add.isItem"])).toBeTruthy();
  });

  it("states the name limit a root is too long for", async () => {
    host = hostResult([POOL, entry({ dataset: "cache/long", blockers: ["name-too-long"] })]);
    await openDialog();
    expect(
      screen.getByText(
        en["zfs.code.name-too-long"].replace("{max}", "219").replace("{names}", "cache/long"),
      ),
    ).toBeTruthy();
  });

  it("asks before it makes a whole pool one item", async () => {
    await openDialog();
    fireEvent.click(rootSwitch("cache"));
    const dialog = await screen.findByRole("dialog", { name: en["confirmDialog.title"] });
    expect(
      within(dialog).getByText(
        en["zfs.add.poolRootConfirm"].replace("{dataset}", "cache").replace("{names}", "cache/appdata"),
      ),
    ).toBeTruthy();
    fireEvent.click(within(dialog).getByRole("button", { name: en["zfs.add.asItem"] }));
    expect(await screen.findByText(en["zfs.add.selected"].replace("{n}", "1"))).toBeTruthy();
  });

  it("offers no switch for a volume and says what becomes of it", async () => {
    host = hostResult(
      [
        POOL,
        entry({ dataset: "cache/win11", type: "volume", hostMountpoint: "-", vmVolume: true }),
        entry({ dataset: "cache/iscsi", type: "volume", hostMountpoint: "-" }),
      ],
      { unusedZvols: 1 },
    );
    await openDialog();
    const volume = screen.getByText("win11").closest('[role="treeitem"]');
    expect(volume).toBeTruthy();
    expect(within(volume as HTMLElement).queryByRole("switch")).toBeNull();
    expect(within(volume as HTMLElement).getByText(en["zfs.add.vmVolume"])).toBeTruthy();
    expect(screen.getByText(en["zfs.add.unusedZvol"])).toBeTruthy();
    expect(screen.getByText(en["zfs.add.unusedZvols"].replace("{n}", "1"))).toBeTruthy();
    expect(screen.getByLabelText(en["zfs.unusedZvolsHint"])).toBeTruthy();
  });

  it("collapses the Docker layer datasets into one line under their pool", async () => {
    host = hostResult(
      [
        POOL,
        entry({ dataset: "cache/l1", hostMountpoint: "legacy", memberCode: "legacy-mount" }),
        entry({ dataset: "cache/l2", hostMountpoint: "legacy", memberCode: "legacy-mount" }),
      ],
      { hiddenLegacy: 2 },
    );
    await openDialog();
    expect(screen.getByText(en["zfs.add.hiddenLegacy"].replace("{n}", "2"))).toBeTruthy();
    expect(screen.queryByText("l1")).toBeNull();
  });

  it("warns that a share also lives on another pool", async () => {
    host = hostResult([
      POOL,
      APPDATA,
      entry({ dataset: "disk1", hostMountpoint: "/mnt/disk1" }),
      entry({ dataset: "disk1/appdata", hostMountpoint: "/mnt/disk1/appdata" }),
    ]);
    await openDialog();
    expect(
      screen.getByText(
        en["zfs.add.shareSplit"].replace("{dataset}", "appdata").replace("{names}", "disk1"),
      ),
    ).toBeTruthy();
  });

  it("sends the children that were switched off with the item", async () => {
    results = [{ dataset: "cache/appdata", id: "z1", code: "ok", detail: "" }];
    await openDialog();
    fireEvent.click(rootSwitch("cache/appdata"));
    fireEvent.click(screen.getByRole("switch", { name: "cache/appdata/plex" }));
    fireEvent.click(screen.getByRole("button", { name: en["zfs.add.submit"].replace("{n}", "1") }));
    await waitFor(() => expect(created).toHaveLength(1));
    expect(created[0]).toEqual([
      { dataset: "cache/appdata", excludedChildren: ["cache/appdata/plex"], repo: "" },
    ]);
  });

  it("names the item the server would not take", async () => {
    host = hostResult([POOL, APPDATA, TANK, TANK_MEDIA]);
    results = [
      { dataset: "cache/appdata", id: "z1", code: "ok", detail: "" },
      { dataset: "tank/media", id: "", code: "overlaps-item", detail: "overlaps-item: tank/old" },
    ];
    await openDialog();
    fireEvent.click(rootSwitch("cache/appdata"));
    fireEvent.click(rootSwitch("tank/media"));
    fireEvent.click(screen.getByRole("button", { name: en["zfs.add.submit"].replace("{n}", "2") }));
    expect(
      await screen.findByText(en["zfs.add.result.ok"].replace("{dataset}", "cache/appdata")),
    ).toBeTruthy();
    expect(
      screen.getByText(
        en["zfs.add.result.failed"]
          .replace("{dataset}", "tank/media")
          .replace("{reason}", en["zfs.code.overlaps-item"].replace("{names}", "tank/old")),
      ),
    ).toBeTruthy();
  });

  it("probes one added item after the other and stays open until they are done", async () => {
    host = hostResult([POOL, APPDATA, TANK, TANK_MEDIA]);
    results = [
      { dataset: "cache/appdata", id: "z1", code: "ok", detail: "" },
      { dataset: "tank/media", id: "z2", code: "ok", detail: "" },
    ];
    probeGate = () => undefined;
    await openDialog();
    fireEvent.click(rootSwitch("cache/appdata"));
    fireEvent.click(rootSwitch("tank/media"));
    fireEvent.click(screen.getByRole("button", { name: en["zfs.add.submit"].replace("{n}", "2") }));
    await waitFor(() => expect(calls).toEqual(["probe:z1"]));
    expect(
      screen.getByText(en["zfs.add.testing"].replace("{dataset}", "cache/appdata")),
    ).toBeTruthy();
    expect(screen.getByRole("button", { name: en["zfs.add.done"] }).hasAttribute("disabled")).toBe(true);
    probeGate?.();
    await waitFor(() => expect(calls).toEqual(["probe:z1", "probe:z2"]));
    probeGate?.();
    await waitFor(() =>
      expect(screen.getByRole("button", { name: en["zfs.add.done"] }).hasAttribute("disabled")).toBe(false),
    );
    fireEvent.click(screen.getByRole("button", { name: en["zfs.add.done"] }));
    expect(onAdded).toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
  });

  it("says the host could not be read and offers another try", async () => {
    host = hostResult([], { ok: false, available: false, code: "ssh-unreachable" });
    renderDialog();
    expect(await screen.findByText(en["zfs.add.unavailable"])).toBeTruthy();
    expect(
      screen.getByText(en["zfs.code.ssh-unreachable"].replace("{target}", "root@192.168.1.10")),
    ).toBeTruthy();
    expect(screen.getByRole("button", { name: en["zfs.add.retry"] })).toBeTruthy();
  });

  it("filters the tree down to what matches", async () => {
    await openDialog();
    fireEvent.change(screen.getByLabelText(en["zfs.add.filter"]), { target: { value: "plex" } });
    expect(screen.getByText("plex")).toBeTruthy();
    fireEvent.change(screen.getByLabelText(en["zfs.add.filter"]), { target: { value: "nothing" } });
    expect(screen.getByText(en["zfs.add.none"])).toBeTruthy();
  });
});
