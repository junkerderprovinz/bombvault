// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Destination, OffsiteTarget } from "../lib/api";
import { en } from "../lib/i18n";

const box: Destination = {
  id: "dest-box", name: "Box", provider: "box", repo: "rclone:box:BombVault", credsRef: "", storageClass: "",
  immutable: false, createdAt: 1, domains: ["vms"],
};
const fromBox: OffsiteTarget = {
  id: "t-box", domain: "vms", name: "Box", repo: "rclone:box:BombVault/vms", credsRef: "", storageClass: "",
  immutable: false, schedule: "", retentionKeepLast: 7, retentionKeepDaily: 0, retentionKeepWeekly: 0,
  retentionKeepMonthly: 0, retentionKeepYearly: 0, compression: "auto", limitUpload: 0, limitDownload: 0, growthBudgetGb: 0, enabled: true, createdAt: 1, sortOrder: 1,
  destinationId: "dest-box", provider: "box",
};

vi.mock("../lib/api", () => ({
  PLACEMENT_DOMAINS: ["containers", "vms", "files"],
  listOffsiteTargets: () => Promise.resolve({ ok: true, targets: [fromBox] }),
  updateOffsiteTarget: vi.fn(),
  deleteOffsiteTarget: vi.fn(),
  listRepos: () => Promise.resolve({ ok: true, repos: [] }),
  createOffsiteTarget: vi.fn(),
  testOffsiteTarget: vi.fn(),
  getCloudCredSets: () => Promise.resolve({ ok: true, sets: [] }),
  listDestinations: () => Promise.resolve({ ok: true, destinations: [box] }),
}));

const { OffsiteTargetsSection } = await import("./OffsiteTargetsSection");

const t = ((key: string) => (en as Record<string, string>)[key] ?? key) as Parameters<typeof OffsiteTargetsSection>[0]["t"];

afterEach(cleanup);

describe("a target made from a destination", () => {
  it("names the destination it came from", async () => {
    render(<OffsiteTargetsSection domain="vms" t={t} />);
    expect(await screen.findByText("Made from the destination Box")).toBeTruthy();
  });

  it("leaves only the domain's own settings editable", async () => {
    render(<OffsiteTargetsSection domain="vms" t={t} />);
    fireEvent.click(await screen.findByText(en["offsite.targets.edit"]));
    expect(screen.getByDisplayValue("rclone:box:BombVault/vms")).toHaveProperty("disabled", true);
    expect(screen.getByDisplayValue("Box")).toHaveProperty("disabled", true);
    expect(screen.getByRole("switch", { name: en["offsite.immutable"] })).toHaveProperty("disabled", true);
    expect(screen.getAllByRole("spinbutton")[0]).toHaveProperty("disabled", false);
  });
});
