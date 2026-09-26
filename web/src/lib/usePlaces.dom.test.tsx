// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import type { Place } from "./places";

const listPlaces = vi.fn();
vi.mock("./places", async (importOriginal) => ({
  ...(await importOriginal<typeof import("./places")>()),
  listPlaces: () => listPlaces(),
}));

const { usePlaces } = await import("./usePlaces");
const { placesChanged } = await import("./places");

function Names() {
  return <span data-testid="names">{usePlaces().map((p) => p.name).join(",")}</span>;
}

const named = (...names: string[]) => ({ ok: true, places: names.map((name) => ({ name }) as Place), unplaced: [] });

afterEach(() => {
  cleanup();
  listPlaces.mockReset();
});

describe("usePlaces", () => {
  it("reads the places again after a place write", async () => {
    listPlaces.mockResolvedValue(named("Unraid"));
    await act(async () => {
      render(<Names />);
    });
    expect(screen.getByTestId("names").textContent).toBe("Unraid");
    listPlaces.mockResolvedValue(named("Unraid", "B2"));
    await act(async () => {
      placesChanged();
    });
    expect(screen.getByTestId("names").textContent).toBe("Unraid,B2");
  });

  it("keeps the last list when a read fails", async () => {
    listPlaces.mockResolvedValue(named("Unraid"));
    await act(async () => {
      render(<Names />);
    });
    listPlaces.mockRejectedValue(new Error("network"));
    await act(async () => {
      placesChanged();
    });
    expect(screen.getByTestId("names").textContent).toBe("Unraid");
  });
});
