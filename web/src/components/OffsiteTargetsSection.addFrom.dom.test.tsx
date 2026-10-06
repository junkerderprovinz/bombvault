// @vitest-environment jsdom
// Flash, self-backup and ZFS have no placement row, so their section is the
// only place a destination can be added to them.
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Destination } from "../lib/api";
import { en } from "../lib/i18n";

const qnap: Destination = {
  id: "dest-qnap", name: "QNAP", provider: "s3", repo: "s3:http://nas:9000/bv", credsRef: "", storageClass: "",
  immutable: false, createdAt: 1, domains: ["containers"],
};
const destinationForDomain = vi.fn();

vi.mock("../lib/api", () => ({
  PLACEMENT_DOMAINS: ["containers", "vms", "files"],
  listOffsiteTargets: () => Promise.resolve({ ok: true, targets: [] }),
  updateOffsiteTarget: vi.fn(),
  deleteOffsiteTarget: vi.fn(),
  listRepos: () => Promise.resolve({ ok: true, repos: [] }),
  createOffsiteTarget: vi.fn(),
  testOffsiteTarget: vi.fn(),
  getCloudCredSets: () => Promise.resolve({ ok: true, sets: [] }),
  listDestinations: () => Promise.resolve({ ok: true, destinations: [qnap] }),
  destinationForDomain: (...a: unknown[]) => destinationForDomain(...a),
}));

vi.mock("../lib/toast", () => ({
  useToast: () => ({ push: () => {} }),
}));

const { OffsiteTargetsSection } = await import("./OffsiteTargetsSection");

const t = ((key: string) => (en as Record<string, string>)[key] ?? key) as Parameters<typeof OffsiteTargetsSection>[0]["t"];
const addFromQnap = en["offsite.targets.addFrom"].replace("{name}", "QNAP");

afterEach(() => {
  cleanup();
  destinationForDomain.mockReset();
});

describe("adding a destination to a domain without a placement row", () => {
  it("creates the flash target under the destination", async () => {
    destinationForDomain.mockResolvedValue({ ok: true, created: true });
    render(<OffsiteTargetsSection domain="flash" t={t} />);
    fireEvent.click(await screen.findByRole("button", { name: addFromQnap }));
    await waitFor(() => expect(destinationForDomain).toHaveBeenCalledWith("dest-qnap", "flash"));
  });

  it("leaves a domain with a placement row to that row", async () => {
    render(<OffsiteTargetsSection domain="vms" t={t} />);
    await screen.findByRole("button", { name: en["offsite.targets.add"] });
    expect(screen.queryByRole("button", { name: addFromQnap })).toBeNull();
  });
});
