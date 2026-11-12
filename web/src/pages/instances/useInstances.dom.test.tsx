// @vitest-environment jsdom
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { renderHook, waitFor } from "@testing-library/react";
import { ApiError } from "../../lib/api";
import { I18nProvider } from "../../lib/i18n";

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  getGroup: vi.fn(async () => ({ ok: true, name: "tower", selfAddress: "https://tower:3443", members: [], away: [], settling: false })),
  getHealth: vi.fn(async () => ({ ok: true, version: "10.5.3" })),
  getStatus: vi.fn(async () => {
    throw new ApiError(500, "database is locked");
  }),
}));

const { useInstances } = await import("./useInstances");

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn(async () => new Response("{}", { status: 404 })));
});

afterEach(() => {
  vi.unstubAllGlobals();
});

it("shows this server when its status cannot be read", async () => {
  const wrapper = ({ children }: { children: ReactNode }) => <I18nProvider>{children}</I18nProvider>;
  const { result } = renderHook(() => useInstances({ fleet: false, receiver: false, pull: false, zfs: false }), { wrapper });

  await waitFor(() => expect(result.current.self).not.toBeNull());
  expect(result.current.self).toMatchObject({ name: "tower", version: "10.5.3", domains: [] });
});
