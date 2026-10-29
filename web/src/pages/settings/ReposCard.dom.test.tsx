// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { NamedRepo, StorageLocation } from "../../lib/api";
import { en } from "../../lib/i18n";

const api = vi.hoisted(() => ({
  listRepos: vi.fn(),
  createRepo: vi.fn(),
  listStorageLocations: vi.fn(),
}));
const pushed = vi.hoisted(() => [] as { message: string; severity?: string }[]);

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  ...api,
}));
vi.mock("../../lib/toast", () => ({
  useToast: () => ({
    push: (message: string, severity?: string) => pushed.push({ message, severity }),
    quiet: false,
    setQuiet: () => {},
  }),
}));

const { ReposCard } = await import("./ReposCard");

function repo(over: Partial<NamedRepo>): NamedRepo {
  return {
    id: "n1", name: "NAS", repo: "backups/nas", credsRef: "", storageClass: "", limitUpload: 0, limitDownload: 0,
    immutable: false, enabled: true, offPremises: false, inUse: 0, compression: "auto", companionOf: "", companionLost: false,
    defaultFor: [], ...over,
  };
}

function location(r: NamedRepo): StorageLocation {
  return {
    id: `repo:${r.id}`, object: "repo", kind: "local", provider: "", backend: "local", name: r.name, where: r.repo,
    enabled: r.enabled, offPremises: false, sections: [], protection: { immutable: r.immutable, testable: false }, capacity: {},
  };
}

function serve(...repos: NamedRepo[]) {
  api.listRepos.mockResolvedValue({ ok: true, repos });
  api.listStorageLocations.mockResolvedValue({ ok: true, locations: repos.filter((r) => r.companionOf === "").map(location) });
}

function renderCard() {
  return render(
    <MemoryRouter>
      <ReposCard />
    </MemoryRouter>
  );
}

beforeEach(() => {
  pushed.length = 0;
  for (const fn of Object.values(api)) fn.mockReset();
  serve();
});
afterEach(cleanup);

describe("the Repositories card", () => {
  it("lists each repository made here as one row that opens its own page", async () => {
    serve(repo({ id: "n1", name: "NAS", inUse: 3 }), repo({ id: "n2", name: "Cold storage", repo: "backups/cold" }));
    renderCard();

    const nas = await screen.findByRole("link", { name: "NAS" });
    expect(nas.getAttribute("href")).toBe("/storage/repo%3An1");
    expect(screen.getByRole("link", { name: "Cold storage" }).getAttribute("href")).toBe("/storage/repo%3An2");
    expect(screen.getByText("3 in use")).toBeTruthy();
    expect(screen.getByText(en["repos.unused"])).toBeTruthy();
    expect(screen.queryAllByRole("switch")).toHaveLength(0);
    expect(screen.queryByRole("button", { name: en["offsite.targets.remove"] })).toBeNull();
  });

  it("leaves a direct repository to the page of its target's storage location", async () => {
    serve(repo({ id: "n1", name: "NAS" }), repo({ id: "d1", name: "", directOf: "Mani", companionOf: "t-mani" }));
    renderCard();

    await screen.findByRole("link", { name: "NAS" });
    expect(screen.queryByText("Mani · direct")).toBeNull();
    expect(screen.getAllByRole("listitem")).toHaveLength(1);
  });

  it("says when there are none made here yet", async () => {
    serve(repo({ id: "d1", name: "", directOf: "Mani", companionOf: "t-mani" }));
    renderCard();
    expect(await screen.findByText(en["repos.empty"])).toBeTruthy();
  });

  it("labels a repository whose target an import removed", async () => {
    serve(repo({ companionLost: true }));
    renderCard();
    expect(await screen.findByText(`${en["repos.unused"]} · ${en["repos.companionLost"]}`)).toBeTruthy();
  });

  it("adds a repository and announces it to the rest of the page", async () => {
    const { subscribeRepos } = await import("../../lib/useNamedRepos");
    const seen = vi.fn();
    const off = subscribeRepos(seen);
    api.createRepo.mockResolvedValue({ ok: true, repo: repo({}) });
    renderCard();
    const form = await screen.findByLabelText(en["repos.name"]);

    fireEvent.change(form, { target: { value: "NAS" } });
    fireEvent.change(screen.getByLabelText(en["repos.location"]), { target: { value: "backups/nas" } });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["repos.add"] }));
    });

    await waitFor(() => expect(api.createRepo).toHaveBeenCalledWith({ name: "NAS", repo: "backups/nas", enabled: true }));
    expect(seen).toHaveBeenCalledTimes(1);
    off();
  });

  it("says why the server refused a new repository", async () => {
    api.createRepo.mockResolvedValue({ ok: false, error: "that place is taken" });
    renderCard();
    fireEvent.change(await screen.findByLabelText(en["repos.name"]), { target: { value: "NAS" } });
    fireEvent.change(screen.getByLabelText(en["repos.location"]), { target: { value: "backups/nas" } });
    await act(async () => {
      fireEvent.click(within(document.body).getByRole("button", { name: en["repos.add"] }));
    });
    expect(pushed).toContainEqual({ message: "that place is taken", severity: "fail" });
  });
});
