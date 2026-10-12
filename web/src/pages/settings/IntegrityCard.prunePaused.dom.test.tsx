// @vitest-environment jsdom
/**
 * A manual prune that skipped items an open anomaly is holding has to say so.
 * Their backups were kept on purpose, and a plain success would read as the
 * policy having been applied everywhere. A prune the server refuses over the
 * login says why in a sentence of its own.
 */
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

const pruneDomain = vi.fn();
const push = vi.fn();

vi.mock("../../lib/api", async () => {
  const { domain } = await import("../overview/overview.testsupport");
  return {
  pruneDomain: (...a: unknown[]) => pruneDomain(...a),
  unlockDomain: vi.fn().mockResolvedValue({ ok: true }),
  checkDomain: vi.fn().mockResolvedValue({ ok: true }),
  runDrill: vi.fn().mockResolvedValue({ ok: true }),
  tamperTest: vi.fn().mockResolvedValue({ ok: true }),
  getStatus: vi.fn().mockResolvedValue({ ok: true, domains: [domain({ domain: "containers", offsiteConfigured: false })] }),
  listItems: vi.fn().mockResolvedValue({ ok: true, items: [] }),
  };
});

vi.mock("../../lib/toast", () => ({
  useToast: () => ({ push, quiet: false, setQuiet: () => {} }),
}));

import { IntegrityCard } from "./IntegrityCard";
import { en } from "../../lib/i18n";
import { pickAction } from "./IntegrityCard.testsupport";

// The card follows live progress, and jsdom has no EventSource.
class FakeEventSource {
  onmessage: ((ev: MessageEvent) => void) | null = null;
  close() {
    /* no-op */
  }
}
vi.stubGlobal("EventSource", FakeEventSource);

const t = ((key: string) => (en as Record<string, string>)[key] ?? key) as unknown as Parameters<typeof IntegrityCard>[0]["t"];

async function prune() {
  render(<IntegrityCard t={t} />);
  await pickAction("containers", en["integrity.prune"]);
  const dialog = await screen.findByRole("dialog");
  await act(async () => {
    fireEvent.click(within(dialog).getByRole("button", { name: en["integrity.prune"] }));
  });
  await waitFor(() => expect(pruneDomain).toHaveBeenCalled());
}

beforeEach(() => {
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

it("names a VM once with its disks and the flash drive by its backup type", async () => {
  pruneDomain.mockResolvedValue({ ok: true, paused: ["vm:win11", "vm:win11:zvol:vdb", "flash"] });
  await prune();
  await waitFor(() =>
    expect(push).toHaveBeenCalledWith(
      en["anomaly.prunePaused"].replace("{names}", `win11, ${en["dashboard.domainFlash"]}`),
      "warn"
    )
  );
});

it("stays quiet about anomalies when nothing was held", async () => {
  pruneDomain.mockResolvedValue({ ok: true });
  await prune();
  await waitFor(() => expect(push).toHaveBeenCalled());
  expect(push.mock.calls.every(([, tone]) => tone !== "warn")).toBe(true);
});

it("says that pruning takes a login password when none is set", async () => {
  pruneDomain.mockResolvedValue({ ok: false, code: "password-not-set", error: "set a login password before pruning backups" });
  await prune();
  await waitFor(() => expect(push).toHaveBeenCalledWith(en["secrets.deletePasswordNotSet"], "fail"));
});

it("offers a paired phone the sign-in instead of a failed prune", async () => {
  pruneDomain.mockResolvedValue({ ok: false, code: "own-login-needed", error: "sign in at this instance's web interface" });
  await prune();
  const ask = await screen.findByRole("dialog");
  expect(within(ask).getByText(en["secrets.ownLoginNeeded"])).toBeTruthy();
  expect(within(ask).getByRole("button", { name: en["secrets.signInHere"] })).toBeTruthy();
  expect(push.mock.calls.some(([, tone]) => tone === "fail")).toBe(false);
});
