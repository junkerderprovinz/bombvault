// @vitest-environment jsdom
// A node's card says whether it is there, what it runs and where its backups
// go, and carries Edit placement, Replace and Release. A node that has not
// answered yet cannot be placed, and one being released has no actions.
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { I18nProvider, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import type { NodeView } from "../../lib/api";

const released: string[] = [];
const composed: string[] = [];
let told = true;

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    releaseNode: (id: string) => {
      released.push(id);
      return Promise.resolve({ ok: true, told });
    },
    nodeCompose: (id: string) => {
      composed.push(id);
      return Promise.resolve({ ok: true, compose: { MODE: "node", APP_KEY: "f".repeat(64), NODE_HOME: "bvhome1.abc", NODE_NAME: "vps" } });
    },
  };
});

const { NodeCards } = await import("./NodeCards");

const online: NodeView = {
  id: "n1",
  name: "vps-falkenstein",
  createdAt: 1_700_000_000,
  adoptedAt: 1_700_000_100,
  placement: "receiver",
  placementTarget: "https://backup.example.org",
  online: true,
  version: "v10.5.0",
};

let changed = 0;

async function renderCards(nodes: NodeView[]) {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <NodeCards nodes={nodes} firstIndex={1} onChanged={() => changed++} />
        </ToastProvider>
      </I18nProvider>,
    );
  });
}

const card = (id: string) => document.querySelector(`[data-instance="node:${id}"]`) as HTMLElement;

afterEach(() => {
  cleanup();
  released.length = 0;
  composed.length = 0;
  told = true;
  changed = 0;
  localStorage.clear();
});

describe("NodeCards", () => {
  it("shows an online node with its version and where its backups go", async () => {
    await renderCards([online]);
    const c = within(card("n1"));
    expect(c.getByText("vps-falkenstein")).toBeTruthy();
    expect(c.getByText(en["instances.connected"])).toBeTruthy();
    expect(c.getByText("v10.5.0")).toBeTruthy();
    expect(c.getByText("https://backup.example.org")).toBeTruthy();
    expect(c.getByText(en["nodes.fig.receiver"])).toBeTruthy();
    expect((c.getByRole("button", { name: en["nodes.editPlacement"] }) as HTMLButtonElement).disabled).toBe(false);
  });

  it("says when an away node was last seen", async () => {
    const lastSeen = Math.floor(Date.now() / 1000) - 7200;
    await renderCards([{ ...online, online: false, lastSeen, version: undefined }]);
    const c = within(card("n1"));
    expect(c.getByText(en["instances.inactive"])).toBeTruthy();
    expect(c.getByText(/^Last seen /)).toBeTruthy();
  });

  it("waits for a node that has not answered and cannot place it yet", async () => {
    await renderCards([{ ...online, adoptedAt: 0, placement: undefined, placementTarget: undefined, online: false }]);
    const c = within(card("n1"));
    expect(c.getByText(en["nodes.waiting"])).toBeTruthy();
    expect(c.getByText(en["nodes.notPlaced"])).toBeTruthy();
    expect((c.getByRole("button", { name: en["nodes.editPlacement"] }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("shows a node being released as such, without actions", async () => {
    await renderCards([{ ...online, state: "releasing", online: false }]);
    const c = within(card("n1"));
    expect(c.getByText(en["nodes.releasing"])).toBeTruthy();
    expect(c.queryByRole("button", { name: en["nodes.release"] })).toBeNull();
  });

  it("releases a node after asking and says when it is away", async () => {
    told = false;
    await renderCards([online]);
    fireEvent.click(within(card("n1")).getByRole("button", { name: en["nodes.release"] }));
    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("Release vps-falkenstein?");
    await act(async () => {
      fireEvent.click(within(dialog).getByRole("button", { name: en["nodes.release"] }));
    });
    expect(released).toEqual(["n1"]);
    expect(changed).toBe(1);
    expect(screen.getByText(en["nodes.releaseLater"].replace("{name}", "vps-falkenstein"))).toBeTruthy();
  });

  it("hands out the compose file again on Replace", async () => {
    await renderCards([online]);
    await act(async () => {
      fireEvent.click(within(card("n1")).getByRole("button", { name: en["nodes.replace"] }));
    });
    expect(composed).toEqual(["n1"]);
    const compose = screen.getByTestId("node-compose").textContent ?? "";
    expect(compose).toContain(`APP_KEY: "${"f".repeat(64)}"`);
    expect(compose).toContain('NODE_HOME: "bvhome1.abc"');
  });
});
