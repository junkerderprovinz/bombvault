// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { NamedRepo, OffsiteTarget } from "../../lib/api";
import { en } from "../../lib/i18n";

const api = vi.hoisted(() => ({
  listRepos: vi.fn(),
  listOffsiteTargets: vi.fn(),
  createRepo: vi.fn(),
  updateRepo: vi.fn(),
  deleteRepo: vi.fn(),
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

const b2 = { id: "t-b2", domain: "containers", name: "B2", repo: "b2:bkt:containers" } as OffsiteTarget;

function repo(over: Partial<NamedRepo>): NamedRepo {
  return {
    id: "n1", name: "NAS", repo: "backups/nas", credsRef: "", storageClass: "", limitUpload: 0, limitDownload: 0,
    immutable: false, enabled: true, inUse: 0, companionOf: "", companionLost: false, ...over,
  };
}

beforeEach(() => {
  pushed.length = 0;
  for (const fn of Object.values(api)) fn.mockReset();
  api.listRepos.mockResolvedValue({ ok: true, repos: [] });
  api.listOffsiteTargets.mockResolvedValue({ ok: true, targets: [b2] });
});
afterEach(cleanup);

describe("the Repositories card", () => {
  it("shows each switch caption once and keeps it as the switch's name", async () => {
    api.listRepos.mockResolvedValue({
      ok: true,
      repos: [repo({ id: "r1", name: "NAS Keller", repo: "/mnt/remotes/nas/bv" })],
    });
    render(<ReposCard />);
    await screen.findByText("NAS Keller");
    for (const caption of ["Append-only", "Available"]) {
      expect(screen.getAllByText(caption)).toHaveLength(1);
      expect(screen.getByRole("switch", { name: caption })).toBeTruthy();
    }
  });

  it("shows a direct repository as its target's and locks what it takes from it", async () => {
    api.listRepos.mockResolvedValue({
      ok: true,
      repos: [repo({ id: "d1", name: "B2 direct", repo: "b2:bkt:containers-direct", companionOf: "t-b2", inUse: 3 }), repo({})],
    });
    render(<ReposCard />);
    await screen.findByText("belongs to B2 · items: 3");
    // Plain DOM property access, not toBeDisabled(); this repo carries no
    // @testing-library/jest-dom, see ColorPickerPopover.dom.test.tsx.
    const appendOnly = screen.getAllByRole("switch", { name: en["repos.immutable"] }) as HTMLButtonElement[];
    expect(appendOnly[0].disabled).toBe(true);
    expect(appendOnly[1].disabled).toBe(false);
    for (const available of screen.getAllByRole("switch", { name: en["repos.enabled"] }) as HTMLButtonElement[]) {
      expect(available.disabled).toBe(false);
    }
  });

  it("offers no way to remove a direct repository", async () => {
    api.listRepos.mockResolvedValue({
      ok: true,
      repos: [repo({ id: "d1", name: "B2 direct", companionOf: "t-b2" }), repo({})],
    });
    render(<ReposCard />);
    await screen.findByText("B2 direct");
    const remove = screen.getAllByRole("button", { name: en["offsite.targets.remove"] }) as HTMLButtonElement[];
    expect(remove[0].disabled).toBe(true);
    expect(remove[1].disabled).toBe(false);
    expect(screen.getByLabelText("Goes with B2. Remove that target to remove this repository.")).toBeTruthy();
  });

  it("labels a repository whose target an import removed", async () => {
    api.listRepos.mockResolvedValue({ ok: true, repos: [repo({ companionLost: true })] });
    render(<ReposCard />);
    await screen.findByText(en["repos.companionLost"]);
  });

  it("names the defaults when a delete is refused", async () => {
    api.listRepos.mockResolvedValue({ ok: true, repos: [repo({})] });
    api.deleteRepo.mockResolvedValue({ ok: false, error: "in use", code: "repo-in-use", items: 0, defaultDomains: ["containers"] });
    render(<ReposCard />);
    fireEvent.click(await screen.findByRole("button", { name: en["offsite.targets.remove"] }));
    await act(async () => {
      fireEvent.click(await screen.findByRole("button", { name: en["common.confirm"] }));
    });
    await waitFor(() =>
      expect(pushed).toContainEqual({
        message: "The default for Containers points at this repository. Change the default first.",
        severity: "fail",
      })
    );
  });
});
