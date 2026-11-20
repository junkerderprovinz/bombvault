// @vitest-environment jsdom
// A node's section reads its containers through the node's own routes and
// leads each row to the entry's page under that node, and adds and edits its
// folder sets through them too. A node that is not adopted yet gets no section.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import type { NodeView } from "../../lib/api";
import type { NodeEntryKind } from "../../lib/nodeEntries";
import { NodeSections } from "./NodeSections";

const NODE: NodeView = { id: "n1", name: "vps-falkenstein", createdAt: 1, adoptedAt: 2, online: true };
const ANSWERS: Record<string, unknown> = {
  "/api/nodes/routes": {
    ok: true,
    routes: ["GET /api/containers", "GET /api/files", "GET /api/browse", "POST /api/files/sets", "PATCH /api/files/sets/{id}"],
  },
  "/api/nodes/n1/api/files": {
    ok: true,
    fileSets: [{ id: "s1", name: "Website uploads", path: "srv/uploads", excludes: [], enabled: true, lastBackup: 0, pathExists: true }],
  },
  "/api/nodes/n1/api/browse": { ok: true, root: "/host/user", path: "", dirs: [{ name: "srv", path: "srv" }], status: "ok", truncated: false },
  "/api/nodes/n1/api/containers": {
    ok: true,
    containers: [
      { name: "nextcloud", image: "nextcloud:29", state: "running", ip: "", installed: true, includeInSchedule: true, lastBackup: 1_700_000_000 },
      { name: "bombvault-node", image: "bombvault", state: "running", ip: "", installed: true, includeInSchedule: false, lastBackup: null, self: true },
    ],
  },
};

const fetchMock = vi.fn((url: string, init?: RequestInit) => {
  const body = init?.method === "POST" || init?.method === "PATCH" ? { ok: true, id: "s2" } : ANSWERS[url.split("?")[0]];
  return Promise.resolve({ ok: body !== undefined, status: body ? 200 : 404, json: () => Promise.resolve(body ?? { ok: false }) });
});

function sent(method: string) {
  return fetchMock.mock.calls
    .filter(([, init]) => init?.method === method)
    .map(([url, init]) => ({ url, body: JSON.parse(String(init?.body)) as unknown }));
}

beforeEach(() => vi.stubGlobal("fetch", fetchMock));
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  fetchMock.mockClear();
  localStorage.clear();
});

async function renderSections(nodes: NodeView[], kinds: NodeEntryKind[] = ["container"]) {
  await act(async () => {
    render(
      <MemoryRouter>
        <I18nProvider>
          <ToastProvider>
            <NodeSections nodes={nodes} kinds={kinds} firstHue={1} />
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

  it("adds a folder set on the node in the window this instance uses for its own", async () => {
    await renderSections([NODE], ["files"]);
    const section = screen.getByRole("region", { name: NODE.name });
    fireEvent.click(within(section).getByRole("button", { name: en["files.addSet"] }));

    const box = await screen.findByRole("dialog", { name: en["nodes.addSet"].replace("{name}", NODE.name) });
    expect(within(box).queryByRole("group", { name: en["storage.title"] })).toBeNull();
    fireEvent.change(within(box).getByPlaceholderText("documents"), { target: { value: "Photos" } });
    fireEvent.change(within(box).getByPlaceholderText("user/appdata"), { target: { value: "srv/photos" } });
    expect(within(box).getByText("→ /host/user/srv/photos")).toBeTruthy();
    fireEvent.click(within(box).getByRole("button", { name: en["folder.browseTitle"] }));
    expect(await within(box).findByText("srv")).toBeTruthy();
    expect(within(box).queryByRole("button", { name: en["folder.newFolder"] })).toBeNull();
    fireEvent.click(within(box).getByRole("button", { name: en["settings.save"] }));

    await waitFor(() =>
      expect(sent("POST")).toEqual([
        { url: "/api/nodes/n1/api/files/sets", body: { name: "Photos", path: "srv/photos", excludes: [], enabled: true } },
      ]),
    );
    expect(fetchMock.mock.calls.map(([url]) => url)).not.toContain("/api/browse");
  });

  it("edits a node's folder set from its menu through the node", async () => {
    await renderSections([NODE], ["files"]);
    const section = screen.getByRole("region", { name: NODE.name });
    fireEvent.contextMenu(await within(section).findByRole("link", { name: "Website uploads" }));
    fireEvent.click(screen.getByRole("menuitem", { name: en["common.edit"] }));

    const box = await screen.findByRole("dialog", { name: en["nodes.editSet"].replace("{name}", NODE.name) });
    fireEvent.change(within(box).getByDisplayValue("Website uploads"), { target: { value: "Uploads" } });
    fireEvent.click(within(box).getByRole("button", { name: en["settings.save"] }));

    await waitFor(() =>
      expect(sent("PATCH")).toEqual([
        { url: "/api/nodes/n1/api/files/sets/s1", body: { name: "Uploads", path: "srv/uploads", excludes: [], enabled: true } },
      ]),
    );
  });

  it("leaves out a node that has not answered yet", async () => {
    await renderSections([{ ...NODE, adoptedAt: 0 }]);
    expect(document.querySelector("[data-node-section]")).toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
