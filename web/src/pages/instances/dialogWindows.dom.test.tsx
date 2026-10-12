// @vitest-environment jsdom
// The forms that open over the instances, folder sets and ZFS pages share the
// app's window: on a wide screen the buttons stay put while the form scrolls,
// and on a phone the form is a sheet from the bottom edge.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { I18nProvider, en, useT } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import type { FleetPeer } from "../../lib/api";
import { group } from "../../components/zfs/replica/replica.testsupport";

let wide = true;
vi.mock("../../lib/useMediaQuery", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/useMediaQuery")>()),
  useIsDesktop: () => wide,
}));

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  const empty = () => Promise.resolve({ ok: false });
  return {
    ...actual,
    getGroup: () => Promise.resolve(group()),
    getZFSReplicaKey: () => Promise.resolve({ publicKey: "" }),
    getReceiverServer: () => Promise.resolve({ ok: true, server: null, defaultPort: 8000, hostMountRoot: "/host/user" }),
    zfsHostDatasets: () =>
      Promise.resolve({
        ok: true,
        available: true,
        code: "ok",
        target: "root@192.168.1.10",
        datasets: [],
        hiddenLegacy: 0,
        unusedZvols: 0,
        notInItem: 0,
        truncated: false,
        maxNameLength: 219,
      }),
    listRepos: () => Promise.resolve({ ok: true, repos: [] }),
    getCloudCredSets: () => Promise.resolve({ ok: true, sets: [] }),
    listFleetPeers: () => Promise.resolve({ ok: true, peers: [] }),
    browse: empty,
  };
});

const { ReceiverDialog } = await import("./ReceivedRepoCard");
const { PullDialog } = await import("./PullSourceCard");
const { ProposeMeshDialog } = await import("./MeshOffers");
const { ReceiverServerWindow } = await import("./ReceiverServerWindow");
const { RoleDialog } = await import("./RoleDialog");
const { FileSetDialog } = await import("../../components/files/FileSetDialog");
const { ZFSAddDialog } = await import("../../components/zfs/ZFSAddDialog");
const { AddReplicaServerDialog } = await import("../../components/zfs/replica/AddReplicaServerDialog");

const peer = { id: "p1", name: "barn" } as FleetPeer;
const none = () => undefined;

/** Each form, with the name of a button in its foot. */
const FORMS: [string, () => ReactNode, string][] = [
  ["the received repository", () => <WithT render={(t) => <ReceiverDialog initial={null} t={t} onClose={none} onSaved={none} />} />, en["files.cancel"]],
  ["the pull source", () => <WithT render={(t) => <PullDialog initial={null} t={t} onClose={none} onSaved={none} />} />, en["common.cancel"]],
  ["the storage offer", () => <WithT render={(t) => <ProposeMeshDialog peer={peer} t={t} onClose={none} />} />, en["files.cancel"]],
  ["the receiving server", () => <WithT render={(t) => <ReceiverServerWindow t={t} lead="lead" onClose={none} />} />, en["common.cancel"]],
  [
    "a role",
    () => <RoleDialog title="Keeper" hint="hint" lead="lead" footer={<button type="button">{en["common.cancel"]}</button>} onClose={none} />,
    en["common.cancel"],
  ],
  [
    "the folder set",
    () => <WithT render={(t) => <FileSetDialog initial={null} presetSeed={null} hostMountRoot="/host/user" t={t} onClose={none} onSaved={none} />} />,
    en["files.cancel"],
  ],
  ["the ZFS dataset", () => <ZFSAddDialog onClose={none} onAdded={none} />, en["common.cancel"]],
  ["the ZFS server", () => <AddReplicaServerDialog onClose={none} onSaved={none} />, en["common.cancel"]],
];

function WithT({ render: draw }: { render: (t: ReturnType<typeof useT>["t"]) => ReactNode }) {
  const { t } = useT();
  return <>{draw(t)}</>;
}

async function open(form: () => ReactNode) {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>{form()}</ToastProvider>
      </I18nProvider>,
    );
  });
}

/** Whether the element moves with the scrolling body of its window. */
function scrollsWithBody(el: HTMLElement): boolean {
  for (let n: HTMLElement | null = el.parentElement; n; n = n.parentElement) {
    if (n.classList.contains("overflow-y-auto")) return true;
    if (n.getAttribute("role") === "dialog") return false;
  }
  return false;
}

/** The topmost window, which is the form for a form opened from a window. */
function topDialog(): HTMLElement {
  const all = screen.getAllByRole("dialog");
  return all[all.length - 1];
}

beforeEach(() => {
  localStorage.clear();
  window.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
});

afterEach(cleanup);

describe.each(FORMS)("the form for %s", (_, form, button) => {
  it("keeps its buttons in view while the form scrolls on a wide screen", async () => {
    wide = true;
    await open(form);
    const dialog = topDialog();
    const buttons = Array.from(dialog.querySelectorAll("button")).filter((b) => b.textContent?.includes(button));
    expect(buttons.length).toBeGreaterThan(0);
    expect(scrollsWithBody(buttons[buttons.length - 1])).toBe(false);
  });

  it("is a sheet from the bottom edge on a phone", async () => {
    wide = false;
    await open(form);
    expect(topDialog().className).toContain("bottom-0");
  });
});

describe("the explanations in the forms", () => {
  const BUBBLED: [string, () => ReactNode, (keyof typeof en)[]][] = [
    [
      "the received repository",
      FORMS[0][1],
      ["receiver.repoLocationHint", "receiver.deadManHoursHint", "pairing.memberHint"],
    ],
    ["the storage offer", FORMS[2][1], ["fleet.mesh.baseUrlHint"]],
    ["the folder set", FORMS[5][1], ["files.excludesHint"]],
  ];

  it.each(BUBBLED)("sit in info bubbles in the form for %s", async (_, form, keys) => {
    wide = true;
    await open(form);
    for (const key of keys) {
      expect(screen.queryByText(en[key])).toBeNull();
      expect(screen.getByLabelText(en[key])).toBeTruthy();
    }
  });

  it("put the folder's rules in the bubble beside the folder", async () => {
    wide = true;
    await open(FORMS[5][1]);
    expect(screen.queryByText(en["files.pathHint"])).toBeNull();
    expect(screen.queryByText(en["files.pathChangeHint"])).toBeNull();
    expect(screen.getByLabelText(`${en["files.pathHint"]} ${en["files.pathChangeHint"]}`)).toBeTruthy();
  });
});
