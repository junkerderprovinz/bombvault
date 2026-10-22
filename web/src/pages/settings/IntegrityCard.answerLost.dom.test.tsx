// @vitest-environment jsdom
/**
 * A check behind a proxy that gives up waiting goes on on the server. The card
 * says so instead of reporting the check as failed.
 */
import { cleanup, render, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

const checkDomain = vi.fn();
const push = vi.fn();

vi.mock("../../lib/api", async (importOriginal) => {
  const { domain } = await import("../overview/overview.testsupport");
  return {
    ...(await importOriginal<typeof import("../../lib/api")>()),
    checkDomain: (...a: unknown[]) => checkDomain(...a),
    getStatus: vi.fn().mockResolvedValue({ ok: true, domains: [domain({ domain: "containers" })] }),
    listItems: vi.fn().mockResolvedValue({ ok: true, items: [] }),
    openedThroughRelay: () => false,
  };
});

vi.mock("../../lib/toast", () => ({
  useToast: () => ({ push, quiet: false, setQuiet: () => {} }),
}));

import { ApiError } from "../../lib/api";
import { en } from "../../lib/i18n";
import { IntegrityCard } from "./IntegrityCard";
import { pickAction } from "./IntegrityCard.testsupport";

class FakeEventSource {
  onmessage: ((ev: MessageEvent) => void) | null = null;
  close() {
    /* no-op */
  }
}
vi.stubGlobal("EventSource", FakeEventSource);

const t = ((key: string) => (en as Record<string, string>)[key] ?? key) as unknown as Parameters<typeof IntegrityCard>[0]["t"];

beforeEach(() => {
  checkDomain.mockReset();
  push.mockReset();
});
afterEach(cleanup);

it("says a check whose answer a proxy gave up on carries on", async () => {
  checkDomain.mockRejectedValue(new ApiError(504, "gateway timeout"));
  render(<IntegrityCard t={t} />);
  await pickAction("containers", en["integrity.checkReadable"]);
  await waitFor(() => expect(push).toHaveBeenCalled());
  expect(push).toHaveBeenCalledWith(en["integrity.answerLost"].replace("{section}", en["nav.containers"]), "warn");
  expect(push.mock.calls.some(([, tone]) => tone === "fail")).toBe(false);
});

it("still reports a check the server refused", async () => {
  checkDomain.mockRejectedValue(new ApiError(500, "restic check failed"));
  render(<IntegrityCard t={t} />);
  await pickAction("containers", en["integrity.checkReadable"]);
  await waitFor(() => expect(push).toHaveBeenCalledWith("restic check failed", "fail"));
});
