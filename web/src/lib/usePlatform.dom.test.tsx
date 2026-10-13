// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";

const fake = await vi.hoisted(async () => (await import("./placement.testsupport")).createPlacementApi());

vi.mock("./api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("./api")>()),
  ...fake.api,
}));

const { usePlatform } = await import("./usePlatform");

function Platform() {
  return <span data-testid="platform">{usePlatform()}</span>;
}

describe("usePlatform", () => {
  afterEach(cleanup);

  it("is empty until the platform is known", async () => {
    render(<Platform />);
    expect(screen.getByTestId("platform").textContent).toBe("");
    await waitFor(() => expect(screen.getByTestId("platform").textContent).toBe("unraid"));
  });

  it("asks for the platform once per page", async () => {
    render(<Platform />);
    await waitFor(() => expect(screen.getByTestId("platform").textContent).toBe("unraid"));
    expect(fake.callsTo("getSettings")).toHaveLength(1);
  });
});
