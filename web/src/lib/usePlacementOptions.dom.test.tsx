// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";

const fake = await vi.hoisted(async () => (await import("./placement.testsupport")).createPlacementApi());

vi.mock("./api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("./api")>()),
  ...fake.api,
}));

const { usePlacementOptions } = await import("./usePlacementOptions");
const { placementChanged } = await import("./placementEvents");
const { reposChanged } = await import("./useNamedRepos");
const { offsiteTargetsChanged } = await import("./useOffsiteTargets");

function Card({ id }: { id: string }) {
  const { options } = usePlacementOptions("vms");
  return <span data-testid={id}>{options ? `homes: ${options.homes.length}` : "loading"}</span>;
}

describe("usePlacementOptions", () => {
  beforeEach(() => fake.reset());
  afterEach(cleanup);

  it("asks the server once for every card of a domain", async () => {
    render(
      <>
        <Card id="a" />
        <Card id="b" />
        <Card id="c" />
      </>
    );
    await waitFor(() => expect(screen.getByTestId("c").textContent).toBe("homes: 2"));
    expect(fake.callsTo("getPlacementOptions")).toEqual([["vms"]]);
  });

  it("reads again after a write to targets, repositories or placement", async () => {
    render(<Card id="a" />);
    await waitFor(() => expect(screen.getByTestId("a").textContent).toBe("homes: 2"));
    act(() => reposChanged());
    act(() => placementChanged());
    act(() => offsiteTargetsChanged());
    await waitFor(() => expect(fake.callsTo("getPlacementOptions")).toHaveLength(4));
  });

  it("starts afresh once the last card of the domain is gone", async () => {
    render(<Card id="a" />);
    await waitFor(() => expect(screen.getByTestId("a").textContent).toBe("homes: 2"));
    cleanup();
    render(<Card id="a" />);
    expect(screen.getByTestId("a").textContent).toBe("loading");
    await waitFor(() => expect(fake.callsTo("getPlacementOptions")).toHaveLength(2));
  });
});
