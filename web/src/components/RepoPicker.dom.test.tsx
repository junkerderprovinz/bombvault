// @vitest-environment jsdom
// A ZFS item picks its repository here, and a place with a folder for ZFS
// datasets is one of the choices even before it holds a repository for them.
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { NamedRepo } from "../lib/api";
import type { Place } from "../lib/places";

const made: [string, string][] = [];
let refusal: { code: string; error: string } | null = null;

function repo(id: string, name: string, placeId = "", placeDomain = ""): NamedRepo {
  return {
    id, name, repo: `/mnt/${id}`, credsRef: "", storageClass: "", limitUpload: 0, limitDownload: 0,
    immutable: false, enabled: true, offPremises: false, inUse: 0, companionOf: "", companionLost: false,
    placeId, placeDomain,
  };
}

function place(id: string, name: string, folders: Record<string, string>, homeDomains: string[] = []): Place {
  return {
    id, name, provider: "unraid-folder", kind: "local", base: `/mnt/${id}`, folders, offPremises: false,
    storageClass: "", immutable: false, retentionKeepLast: 0, retentionKeepDaily: 0, retentionKeepWeekly: 0,
    retentionKeepMonthly: 0, limitUpload: 0, limitDownload: 0, growthBudgetGb: 0, enabled: true, sortOrder: 0,
    credsRef: "", usage: { homeDomains, defaults: [], copyDomains: [], items: 0, copies: 0, repositories: 0 },
    locked: {}, repository: false, creds: { shared: true, fields: {}, set: [] },
  };
}

vi.mock("../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/api")>()),
  listRepos: async () => ({
    ok: true,
    repos: [repo("r-archive", "Archive"), repo("r-nas", "NAS", "p-nas", "containers"), repo("r-cold", "Cold", "p-cold", "zfs")],
  }),
}));

vi.mock("../lib/places", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/places")>()),
  listPlaces: async () => ({
    ok: true,
    places: [
      place("p-nas", "NAS", { containers: "containers", zfs: "zfs" }),
      place("p-cold", "Cold", { zfs: "zfs" }),
      place("p-home", "Unraid", { zfs: "zfs" }, ["zfs"]),
      place("p-vms", "VM box", { vms: "vms" }),
    ],
    unplaced: [],
  }),
  ensurePlaceRepo: async (placeId: string, domain: string) => {
    made.push([placeId, domain]);
    return refusal ? { ok: false, ...refusal } : { ok: true, repoId: "r-new" };
  },
}));

const { RepoPicker } = await import("./RepoPicker");

afterEach(() => {
  cleanup();
  made.length = 0;
  refusal = null;
});

async function open(onChange: (next: string) => void) {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <RepoPicker value="" onChange={onChange} />
        </ToastProvider>
      </I18nProvider>,
    );
  });
  await act(async () => fireEvent.click(screen.getByRole("combobox", { name: en["zfs.repo"] })));
  return screen.getAllByRole("option").map((o) => o.textContent);
}

describe("the ZFS repository picker", () => {
  it("offers the places with a ZFS folder beside the repositories", async () => {
    const offered = await open(() => {});
    expect(offered).toEqual([en["zfs.repoPlaceholder"], "Archive", "Cold", "NAS"]);
  });

  it("makes the place's repository when a place is picked", async () => {
    const onChange = vi.fn();
    await open(onChange);
    await act(async () => fireEvent.click(screen.getByRole("option", { name: "NAS" })));
    expect(made).toEqual([["p-nas", "zfs"]]);
    expect(onChange).toHaveBeenCalledWith("r-new");
  });

  it("keeps the choice and says why when the place refuses", async () => {
    refusal = { code: "place-off", error: "the place is switched off" };
    const onChange = vi.fn();
    await open(onChange);
    await act(async () => fireEvent.click(screen.getByRole("option", { name: "NAS" })));
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByText(/switched off/i)).toBeTruthy();
  });
});
