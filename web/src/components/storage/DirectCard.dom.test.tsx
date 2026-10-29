// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { NamedRepo, StorageLocation, StorageLocationSection } from "../../lib/api";
import { en } from "../../lib/i18n";

const api = vi.hoisted(() => ({ listRepos: vi.fn(), deleteRepo: vi.fn() }));

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  ...api,
}));

const { DirectCard } = await import("./DirectCard");

const KEEP = { keepLast: 0, keepDaily: 7, keepWeekly: 4, keepMonthly: 6, keepYearly: 1 };

function section(over: Partial<StorageLocationSection>): StorageLocationSection {
  return {
    domain: "containers", use: "copy", where: "", enabled: true, immutable: false, retention: KEEP,
    compression: "auto", limitUpload: 0, limitDownload: 0, own: [], ...over,
  };
}

function repo(over: Partial<NamedRepo>): NamedRepo {
  return {
    id: "d1", name: "", repo: "s3:mani/containers-direct", credsRef: "", storageClass: "", limitUpload: 0, limitDownload: 0,
    immutable: false, enabled: true, offPremises: false, inUse: 0, compression: "auto", companionOf: "t1", directOf: "Mani",
    companionLost: false, defaultFor: [], ...over,
  };
}

const mani: StorageLocation = {
  id: "destination:m1", object: "destination", kind: "offsite", provider: "", backend: "s3", name: "Mani",
  where: "s3:mani", enabled: true, offPremises: true, protection: { immutable: false, testable: false }, capacity: {},
  sections: [
    section({ targetId: "t1" }),
    section({ use: "home", targetId: "t1", repoId: "d1" }),
    section({ domain: "files", targetId: "t2" }),
    section({ domain: "files", use: "home", targetId: "t2", repoId: "d2" }),
  ],
};

beforeEach(() => {
  for (const fn of Object.values(api)) fn.mockReset();
  api.listRepos.mockResolvedValue({
    ok: true,
    repos: [repo({ id: "d1", inUse: 4 }), repo({ id: "d2", companionOf: "t2", repo: "s3:mani/files-direct" })],
  });
});
afterEach(cleanup);

describe("the sections that back up straight to a storage location", () => {
  it("shows each one with how many entries back up there and no switches of its own", async () => {
    render(<DirectCard location={mani} hueIndex={0} />);

    expect(await screen.findByText("4 in use")).toBeTruthy();
    expect(screen.getByText(en["nav.containers"])).toBeTruthy();
    expect(screen.queryAllByRole("switch")).toHaveLength(0);
  });

  it("offers to remove only the one that nothing backs up to any more", async () => {
    render(<DirectCard location={mani} hueIndex={0} />);

    await screen.findByText(en["repos.directIdle"]);
    const remove = screen.getAllByRole("button", { name: en["offsite.targets.remove"] });
    expect(remove).toHaveLength(1);
    expect(remove[0].closest("li")?.textContent).toContain(en["nav.files"]);
  });

  it("removes the unused one once confirmed", async () => {
    api.deleteRepo.mockResolvedValue({ ok: true });
    render(<DirectCard location={mani} hueIndex={0} />);

    fireEvent.click(await screen.findByRole("button", { name: en["offsite.targets.remove"] }));
    await act(async () => {
      fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: en["offsite.targets.remove"] }));
    });

    await waitFor(() => expect(api.deleteRepo).toHaveBeenCalledWith("d2"));
  });

  it("says so when the section's default backs up there, instead of calling it unused", async () => {
    api.listRepos.mockResolvedValue({ ok: true, repos: [repo({ id: "d1", defaultFor: ["containers"] })] });
    render(<DirectCard location={{ ...mani, sections: mani.sections.slice(0, 2) }} hueIndex={0} />);

    expect(await screen.findByText(en["repos.byDefault"].replace("{section}", en["nav.containers"]))).toBeTruthy();
    expect(screen.queryByText(en["repos.unused"])).toBeNull();
    expect(screen.queryByRole("button", { name: en["offsite.targets.remove"] })).toBeNull();
  });

  it("is not there for a location nothing backs up to straight", () => {
    const { container } = render(<DirectCard location={{ ...mani, sections: [section({ targetId: "t1" })] }} hueIndex={0} />);
    expect(container.textContent).toBe("");
  });
});
