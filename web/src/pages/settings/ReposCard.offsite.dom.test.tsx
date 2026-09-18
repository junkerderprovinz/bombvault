// @vitest-environment jsdom
/**
 * The "already off site" switch on the Repositories card (#204 follow-up; why
 * it exists is on alreadyOffSite in internal/api). A remote location gets the
 * reason in place of a switch.
 */
import { render, screen, cleanup, waitFor, fireEvent } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const listRepos = vi.fn();
const updateRepo = vi.fn();
vi.mock("../../lib/api", () => ({
  listRepos: (...a: unknown[]) => listRepos(...a),
  updateRepo: (...a: unknown[]) => updateRepo(...a),
  createRepo: vi.fn(),
  deleteRepo: vi.fn(),
}));

import { ReposCard } from "./ReposCard";

function repo(over: Record<string, unknown>) {
  return {
    id: "r1",
    name: "NAS",
    repo: "remotes/nas/cold",
    credsRef: "",
    storageClass: "",
    limitUpload: 0,
    limitDownload: 0,
    immutable: false,
    enabled: true,
    alreadyOffsite: false,
    inUse: 1,
    ...over,
  };
}

beforeEach(() => {
  listRepos.mockReset();
  updateRepo.mockReset();
  updateRepo.mockResolvedValue({ ok: true });
});
afterEach(cleanup);

describe("ReposCard off-site mark", () => {
  it("offers the switch on a local repository and saves it", async () => {
    listRepos.mockResolvedValue({ ok: true, repos: [repo({})] });
    render(<ReposCard />);

    const sw = await waitFor(() => screen.getByRole("switch", { name: /already off site/i }));
    expect(sw.getAttribute("aria-checked")).toBe("false");
    fireEvent.click(sw);
    await waitFor(() => expect(updateRepo).toHaveBeenCalledWith("r1", { alreadyOffsite: true }));
  });

  // Every switch on a row sits in a <label> that already prints its caption, so
  // the Toggle's own caption has to stay hidden. Without hideLabel each one read
  // "Append-only (i) Append-only", on every row, since the card was built.
  it("prints each switch caption once", async () => {
    listRepos.mockResolvedValue({ ok: true, repos: [repo({})] });
    render(<ReposCard />);

    await waitFor(() => expect(screen.getByText("NAS")).toBeTruthy());
    for (const caption of ["Already off site", "Append-only", "Available"]) {
      expect(screen.getAllByText(caption), caption).toHaveLength(1);
    }
  });

  it("gives a remote repository the reason instead of a switch", async () => {
    listRepos.mockResolvedValue({ ok: true, repos: [repo({ repo: "b2:bucket/cold" })] });
    render(<ReposCard />);

    await waitFor(() => expect(screen.getByText("NAS")).toBeTruthy());
    expect(screen.queryByRole("switch", { name: /already off site/i })).toBeNull();
    expect(screen.getByText(/^off site$/i)).toBeTruthy();
  });
});
