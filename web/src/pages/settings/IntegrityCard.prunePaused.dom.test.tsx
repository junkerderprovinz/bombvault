// @vitest-environment jsdom
/**
 * A manual prune that skipped items an open anomaly is holding has to say so.
 * Their backups were kept on purpose, and a plain success would read as the
 * policy having been applied everywhere.
 */
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

const pruneDomain = vi.fn();
const push = vi.fn();

vi.mock("../../lib/api", () => ({
  pruneDomain: (...a: unknown[]) => pruneDomain(...a),
  unlockDomain: vi.fn().mockResolvedValue({ ok: true }),
  checkDomain: vi.fn().mockResolvedValue({ ok: true }),
  runDrill: vi.fn().mockResolvedValue({ ok: true }),
  tamperTest: vi.fn().mockResolvedValue({ ok: true }),
  getDrills: vi.fn().mockResolvedValue({ ok: true, drills: [], latest: null }),
  getStatus: vi.fn().mockResolvedValue({ ok: true }),
  listContainers: vi.fn().mockResolvedValue({ containers: [] }),
  listVMs: vi.fn().mockResolvedValue({ vms: [] }),
}));

vi.mock("../../lib/toast", () => ({
  useToast: () => ({ push, quiet: false, setQuiet: () => {} }),
}));

import { IntegrityCard } from "./IntegrityCard";
import { AdvancedProvider } from "../../lib/advanced";
import { en } from "../../lib/i18n";

const t = ((key: string) => (en as Record<string, string>)[key] ?? key) as unknown as Parameters<typeof IntegrityCard>[0]["t"];
const settings = { drDrillTarget: "", drDrillTargetVm: "" } as never;

async function prune() {
  render(
    <AdvancedProvider>
      <IntegrityCard t={t} settings={settings} setSettings={() => {}} save={async () => true} />
    </AdvancedProvider>
  );
  const button = (await screen.findAllByRole("button", { name: /^prune$/i }))[0];
  await act(async () => {
    fireEvent.click(button);
  });
  const dialog = await screen.findByRole("dialog");
  await act(async () => {
    fireEvent.click(within(dialog).getByRole("button", { name: /^confirm$/i }));
  });
  await waitFor(() => expect(pruneDomain).toHaveBeenCalled());
}

beforeEach(() => {
  localStorage.setItem("bombvault.advanced", "1");
  pruneDomain.mockReset();
  push.mockReset();
});
afterEach(() => {
  cleanup();
  localStorage.clear();
});

it("names the items whose backups a prune kept", async () => {
  pruneDomain.mockResolvedValue({ ok: true, paused: ["container:nextcloud", "dbdump:immich_postgres"] });
  await prune();
  await waitFor(() =>
    expect(push).toHaveBeenCalledWith(
      en["anomaly.prunePaused"].replace("{names}", "nextcloud, Database dump of immich_postgres"),
      "warn"
    )
  );
});

it("stays quiet about anomalies when nothing was held", async () => {
  pruneDomain.mockResolvedValue({ ok: true });
  await prune();
  expect(push).not.toHaveBeenCalled();
});
