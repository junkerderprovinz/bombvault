// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";

const fake = await vi.hoisted(async () => (await import("./placement.testsupport")).createPlacementApi());

vi.mock("./api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("./api")>()),
  ...fake.api,
}));

const { useHostLabel } = await import("./useHostLabel");

function Label() {
  return <span data-testid="host">{useHostLabel()}</span>;
}

describe("useHostLabel", () => {
  afterEach(cleanup);

  it("says Host until the platform is known, then names the product", async () => {
    render(<Label />);
    expect(screen.getByTestId("host").textContent).toBe("Host");
    await waitFor(() => expect(screen.getByTestId("host").textContent).toBe("Unraid"));
  });

  it("asks for the platform once per page", async () => {
    render(<Label />);
    await waitFor(() => expect(screen.getByTestId("host").textContent).toBe("Unraid"));
    expect(fake.callsTo("getSettings")).toHaveLength(1);
  });
});
