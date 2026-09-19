// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("../../lib/api", () => ({
  listRepos: () =>
    Promise.resolve({
      ok: true,
      repos: [
        {
          id: "r1",
          name: "NAS Keller",
          repo: "/mnt/remotes/nas/bv",
          credsRef: "",
          storageClass: "",
          limitUpload: 0,
          limitDownload: 0,
          immutable: false,
          enabled: true,
          inUse: 0,
        },
      ],
    }),
  createRepo: vi.fn(),
  updateRepo: vi.fn(),
  deleteRepo: vi.fn(),
}));

import { ReposCard } from "./ReposCard";

afterEach(cleanup);

describe("ReposCard", () => {
  it("shows each switch caption once and keeps it as the switch's name", async () => {
    render(<ReposCard />);
    await screen.findByText("NAS Keller");
    for (const caption of ["Append-only", "Available"]) {
      expect(screen.getAllByText(caption)).toHaveLength(1);
      expect(screen.getByRole("switch", { name: caption })).toBeTruthy();
    }
  });
});
