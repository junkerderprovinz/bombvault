// @vitest-environment jsdom
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import type { DestinationDraft, Provider } from "../../lib/api";
import { I18nProvider } from "../../lib/i18n";

const checks: DestinationDraft[] = [];
const listings: DestinationDraft[] = [];

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  getProviders: () => Promise.resolve({ ok: true, providers: [], backends: {} }),
  checkDraft: (d: DestinationDraft) => {
    checks.push(d);
    return Promise.resolve(d.dir ? { ok: true } : { ok: false, error: "AccessDenied: Access Denied" });
  },
  draftFolders: (d: DestinationDraft) => {
    listings.push(d);
    return Promise.resolve({ ok: true, folders: [] });
  },
}));

const { useDestinationDraft } = await import("./useDestinationDraft");

const r2: Provider = { id: "r2", name: "Cloudflare R2", backend: "s3", group: "storage", route: "s3", preset: { provider: "Cloudflare" } };

const wrapper = ({ children }: { children: ReactNode }) => <I18nProvider>{children}</I18nProvider>;

afterEach(() => {
  cleanup();
  checks.length = 0;
  listings.length = 0;
});

it("tests a key that may not list the buckets inside the bucket it names", async () => {
  const { result } = renderHook(() => useDestinationDraft(), { wrapper });
  await act(async () => result.current.pick(r2));

  let ok = await act(() => result.current.runTest());
  expect(ok).toBe(false);
  expect(result.current.test.verdict?.ok).toBe(false);
  expect(result.current.test.verdict && "reason" in result.current.test.verdict && result.current.test.verdict.reason).toContain(
    "type the bucket in"
  );

  await act(async () => result.current.setBucket("backups"));
  ok = await act(() => result.current.runTest());
  expect(ok).toBe(true);
  expect(checks.at(-1)?.dir).toBe("backups");
  expect(listings.at(-1)?.dir).toBe("backups");
  expect(result.current.dir).toEqual(["backups"]);
});
