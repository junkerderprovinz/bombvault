// @vitest-environment jsdom
// A node's section reads its containers through the node's own routes and
// leads each row to the entry's page under that node. A node that is not
// adopted yet gets no section.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import type { NodeView } from "../../lib/api";
import { NodeSections } from "./NodeSections";

const NODE: NodeView = { id: "n1", name: "vps-falkenstein", createdAt: 1, adoptedAt: 2, online: true };
const ANSWERS: Record<string, unknown> = {
  "/api/nodes/routes": { ok: true, routes: ["GET /api/containers", "GET /api/files"] },
  "/api/nodes/n1/api/containers": {
    ok: true,
    containers: [
      { name: "nextcloud", image: "nextcloud:29", state: "running", ip: "", installed: true, includeInSchedule: true, lastBackup: 1_700_000_000 },
      { name: "bombvault-node", image: "bombvault", state: "running", ip: "", installed: true, includeInSchedule: false, lastBackup: null, self: true },
    ],
  },
};

const fetchMock = vi.fn((url: string) =>
  Promise.resolve({ ok: url in ANSWERS, status: url in ANSWERS ? 200 : 404, json: () => Promise.resolve(ANSWERS[url] ?? { ok: false }) }),
);

beforeEach(() => vi.stubGlobal("fetch", fetchMock));
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  fetchMock.mockClear();
  localStorage.clear();
});

async function renderSections(nodes: NodeView[]) {
  await act(async () => {
    render(
      <MemoryRouter>
        <I18nProvider>
          <ToastProvider>
            <NodeSections nodes={nodes} kinds={["container"]} firstHue={1} />
          </ToastProvider>
        </I18nProvider>
      </MemoryRouter>,
    );
  });
}

describe("NodeSections", () => {
  it("lists a node's containers through the node and links them under it", async () => {
    await renderSections([NODE]);
    const asked = fetchMock.mock.calls.map(([url]) => url);
    expect(asked).toContain("/api/nodes/n1/api/containers");
    expect(asked).not.toContain("/api/containers");

    const link = (await screen.findByRole("link", { name: "nextcloud" })) as HTMLAnchorElement;
    expect(link.getAttribute("href")).toBe("/nodes/n1/backups/container/nextcloud");
    expect(screen.queryByRole("link", { name: "bombvault-node" })).toBeNull();
  });

  it("folds a section away and keeps it folded", async () => {
    await renderSections([NODE]);
    const toggle = screen.getByRole("button", { name: en["nodes.sectionToggle"].replace("{name}", "vps-falkenstein") });
    expect(toggle.getAttribute("aria-expanded")).toBe("true");
    fireEvent.click(toggle);
    expect(toggle.getAttribute("aria-expanded")).toBe("false");
    expect(screen.queryByRole("link", { name: "nextcloud" })).toBeNull();
    expect(JSON.parse(localStorage.getItem("bv-node-sections") ?? "{}")).toEqual({ n1: false });
  });

  it("leaves out a node that has not answered yet", async () => {
    await renderSections([{ ...NODE, adoptedAt: 0 }]);
    expect(document.querySelector("[data-node-section]")).toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
