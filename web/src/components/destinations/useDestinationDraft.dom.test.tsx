// @vitest-environment jsdom
import { expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { act, renderHook } from "@testing-library/react";
import type { Provider } from "../../lib/api";
import { I18nProvider } from "../../lib/i18n";

const garage: Provider = {
  id: "garage", name: "Garage", backend: "s3", group: "selfs3", route: "s3",
  preset: { provider: "Other" }, defaults: { region: "garage" },
};

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  getProviders: vi.fn(async () => ({ ok: true, providers: [garage], backends: {} })),
}));

const { useDestinationDraft } = await import("./useDestinationDraft");

it("starts the form with the region Garage signs for, which can still be changed", async () => {
  const wrapper = ({ children }: { children: ReactNode }) => <I18nProvider>{children}</I18nProvider>;
  const { result } = renderHook(() => useDestinationDraft(), { wrapper });

  act(() => result.current.pick(garage));
  expect(result.current.settings.region).toBe("garage");

  act(() => result.current.setSetting("region", "eu-home"));
  expect(result.current.settings.region).toBe("eu-home");
  expect(garage.defaults?.region).toBe("garage");
});
