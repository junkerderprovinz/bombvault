// @vitest-environment jsdom
// Adding a node: the name makes its APP_KEY, the placement step refuses plain
// http to a public address in place, the last step shows the compose file and
// waits for the node, and once it answers the node is told where its backups go.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { I18nProvider, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import type { NodePlacement, NodeView, ReceiverServer } from "../../lib/api";

const NODE: NodeView = { id: "n1", name: "vps-falkenstein", createdAt: 1, adoptedAt: 0, online: false, containersCadence: "daily 02:30", filesCadence: "daily 02:30", failures: 0, anomalies: 0 };
const COMPOSE = { MODE: "node", APP_KEY: "a".repeat(64), NODE_HOME: "bvhome1.xyz", NODE_NAME: "vps-falkenstein" };

const receiver = { url: "http://192.168.1.20:8000", nodeUrl: "", nodeCa: false } as ReceiverServer;
const added: string[] = [];
const addresses: string[] = [];
const placed: [string, NodePlacement][] = [];
const scheduled: [string, unknown][] = [];
let refusal: { ok: false; code: string } | null = null;
let adoptedAt = 0;

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    getReceiverServer: () => Promise.resolve({ ok: true, server: receiver }),
    listDestinations: () => Promise.resolve({ ok: true, destinations: [] }),
    getCloudCredSets: () => Promise.resolve({ ok: true, sets: [] }),
    addNode: (name: string) => {
      added.push(name);
      return Promise.resolve(refusal ?? { ok: true, node: { ...NODE, name }, compose: COMPOSE });
    },
    listNodes: () => Promise.resolve({ ok: true, nodes: [{ ...NODE, adoptedAt }] }),
    setReceiverNodeAddress: (url: string) => {
      addresses.push(url);
      return Promise.resolve({ ok: true, nodeUrl: url });
    },
    setNodeSchedule: (id: string, schedule: unknown) => {
      scheduled.push([id, schedule]);
      return Promise.resolve({ ok: true });
    },
    placeNode: (id: string, placement: NodePlacement) => {
      placed.push([id, placement]);
      return Promise.resolve({ ok: true, reach: [{ domain: "containers", reached: true }], warning: "" });
    },
  };
});

const { ADOPT_POLL_MS, AddNodeWindow } = await import("./AddNodeWindow");

async function renderWindow() {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <AddNodeWindow onClose={() => undefined} onChanged={() => undefined} />
        </ToastProvider>
      </I18nProvider>,
    );
  });
}

const next = () => screen.getByRole("button", { name: en["nodes.next"] }) as HTMLButtonElement;

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  added.length = 0;
  addresses.length = 0;
  placed.length = 0;
  scheduled.length = 0;
  refusal = null;
  adoptedAt = 0;
});

describe("AddNodeWindow", () => {
  it("adds a node with its start time, places it on the receiver once it answers, and ticks green", async () => {
    await renderWindow();
    expect(next().disabled).toBe(true);
    fireEvent.change(screen.getByLabelText(en["nodes.step.name"]), { target: { value: " vps-falkenstein " } });
    await act(async () => {
      fireEvent.click(next());
    });
    expect(added).toEqual(["vps-falkenstein"]);

    const address = screen.getByLabelText(en["nodes.address"]);
    fireEvent.change(address, { target: { value: "http://backup.example.org:8000" } });
    expect(screen.getByRole("alert").textContent).toBe(en["nodes.addressPlain"]);
    expect(next().disabled).toBe(true);

    fireEvent.change(address, { target: { value: "https://backup.example.org" } });
    expect(screen.queryByRole("alert")).toBeNull();
    const startAt = screen.getByRole("button", { name: `${en["nodes.startAt"]}: 02:30` });
    fireEvent.click(startAt);
    fireEvent.click(within(screen.getByRole("listbox", { name: en["timePicker.hour"] })).getByRole("option", { name: "04" }));
    await act(async () => {
      fireEvent.click(next());
    });
    expect(addresses).toEqual(["https://backup.example.org"]);
    expect(scheduled).toEqual([["n1", { containers: "daily 04:30", files: "daily 04:30" }]]);

    expect(screen.getByTestId("node-compose").textContent).toContain(`APP_KEY: "${COMPOSE.APP_KEY}"`);
    expect(screen.getByTestId("node-adoption").textContent).toContain(en["nodes.waitingFor"].replace("{name}", "vps-falkenstein"));
    expect(placed).toEqual([]);

    adoptedAt = 2;
    await act(async () => {
      await vi.advanceTimersByTimeAsync(ADOPT_POLL_MS);
    });
    const status = screen.getByTestId("node-adoption").textContent;
    expect(status).toContain(en["nodes.online"].replace("{name}", "vps-falkenstein"));
    expect(status).toContain(en["nodes.placed"].replace("{target}", "https://backup.example.org"));
    expect(placed).toEqual([["n1", { kind: "receiver" }]]);
  });

  it("keeps the address and the certificate apart by room alone, with no line between", async () => {
    await renderWindow();
    fireEvent.change(screen.getByLabelText(en["nodes.step.name"]), { target: { value: "vps" } });
    await act(async () => {
      fireEvent.click(next());
    });
    const row = (label: string) => screen.getByLabelText(label).closest(".glim-form-row")!;
    const address = row(en["nodes.address"]);
    expect(address.parentElement).toBe(row(en["nodes.certificate"]).parentElement);
    expect(address.parentElement!.className).not.toMatch(/divide-/);
  });

  it("lets a node keep its own backups at the address this instance fetches them from", async () => {
    await renderWindow();
    fireEvent.change(screen.getByLabelText(en["nodes.step.name"]), { target: { value: "vps" } });
    await act(async () => {
      fireEvent.click(next());
    });
    fireEvent.click(screen.getByRole("radio", { name: new RegExp(`^${en["nodes.toNode"]}`) }));
    expect(screen.getByText(en["nodes.portNote"])).toBeTruthy();
    expect(next().disabled).toBe(true);

    const address = screen.getByLabelText(en["nodes.nodeAddress"]);
    fireEvent.change(address, { target: { value: "http://203.0.113.7:8000" } });
    expect(screen.getByRole("alert").textContent).toBe(en["nodes.fetchPlain"]);
    expect(next().disabled).toBe(true);
    fireEvent.change(address, { target: { value: "https://203.0.113.7:8443/backups" } });
    expect(screen.getByRole("alert").textContent).toBe(en["nodes.addressForm"]);
    fireEvent.change(address, { target: { value: "https://203.0.113.7:8443" } });
    expect(screen.queryByRole("alert")).toBeNull();
    await act(async () => {
      fireEvent.click(next());
    });
    expect(addresses).toEqual([]);
    expect(screen.getByTestId("node-adoption").textContent).toContain(en["nodes.placementLaterOnNode"].replace("{target}", "https://203.0.113.7:8443"));

    adoptedAt = 2;
    await act(async () => {
      await vi.advanceTimersByTimeAsync(ADOPT_POLL_MS);
    });
    expect(screen.getByTestId("node-adoption").textContent).toContain(en["nodes.placedOnNode"].replace("{target}", "https://203.0.113.7:8443"));
    expect(placed).toEqual([["n1", { kind: "node", address: "https://203.0.113.7:8443" }]]);
  });

  it("says why it cannot add a node while no login password is set", async () => {
    refusal = { ok: false, code: "password-not-set" };
    await renderWindow();
    fireEvent.change(screen.getByLabelText(en["nodes.step.name"]), { target: { value: "vps" } });
    await act(async () => {
      fireEvent.click(next());
    });
    expect(screen.getByRole("alert").textContent).toBe(en["secrets.passwordNotSet"]);
    expect(screen.getByLabelText(en["nodes.step.name"])).toBeTruthy();
  });
});
