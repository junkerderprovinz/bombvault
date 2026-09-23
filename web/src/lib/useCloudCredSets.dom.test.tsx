// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";

const getCloudCredSets = vi.fn();
vi.mock("./api", () => ({
  getCloudCredSets: () => getCloudCredSets(),
}));

const { credSetsChanged, useCloudCredSets } = await import("./useCloudCredSets");

function Reader({ tag }: { tag: string }) {
  const sets = useCloudCredSets();
  return <span data-testid={tag}>{sets.map((s) => s.name).join(",")}</span>;
}

beforeEach(() => {
  getCloudCredSets.mockReset();
});

afterEach(() => {
  cleanup();
});

async function settle() {
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
}

describe("credential-set invalidation", () => {
  it("re-reads every mounted reader when a write announces a change", async () => {
    getCloudCredSets.mockResolvedValue({ ok: true, sets: [{ id: "a", name: "Hetzner" }] });

    // The editor card and a target's credential picker, both on the Off-site tab.
    render(
      <>
        <Reader tag="editor" />
        <Reader tag="picker" />
      </>
    );
    await settle();
    expect(screen.getByTestId("picker").textContent).toBe("Hetzner");

    getCloudCredSets.mockResolvedValue({
      ok: true,
      sets: [{ id: "a", name: "Hetzner" }, { id: "b", name: "Backblaze" }],
    });
    expect(screen.getByTestId("picker").textContent).toBe("Hetzner");

    await act(async () => {
      credSetsChanged();
    });
    await settle();

    expect(screen.getByTestId("picker").textContent).toBe("Hetzner,Backblaze");
    expect(screen.getByTestId("editor").textContent).toBe("Hetzner,Backblaze");
  });

  it("keeps the last good list when a refetch fails", async () => {
    getCloudCredSets.mockResolvedValue({ ok: true, sets: [{ id: "a", name: "Hetzner" }] });
    render(<Reader tag="picker" />);
    await settle();

    getCloudCredSets.mockRejectedValue(new Error("network"));
    await act(async () => {
      credSetsChanged();
    });
    await settle();

    expect(screen.getByTestId("picker").textContent).toBe("Hetzner");
  });

  it("stops listening once a reader unmounts", async () => {
    getCloudCredSets.mockResolvedValue({ ok: true, sets: [] });
    const { unmount } = render(<Reader tag="picker" />);
    await settle();
    const callsWhileMounted = getCloudCredSets.mock.calls.length;

    unmount();
    await act(async () => {
      credSetsChanged();
    });
    await settle();

    expect(getCloudCredSets.mock.calls.length).toBe(callsWhileMounted);
  });
});
