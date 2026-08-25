// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// useCloudCredSets — the cross-component invalidation contract (#173).
//
// The reported bug: a credential set created in Settings' CloudCredSetsCard
// was not selectable in an off-site target's credential picker until the
// browser was reloaded. Both live on the SAME page (the Off-site tab renders
// one OffsiteTargetsSection per domain plus the credential card), and each
// held its own fetched-once copy of the list, so the picker never learned that
// the list had grown.
//
// This asserts the mechanism that fixes it: a second, independently mounted
// reader re-reads the list when a write announces a change — which is exactly
// the picker's situation, one component reacting to another's write.
// ---------------------------------------------------------------------------
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";

const getCloudCredSets = vi.fn();
vi.mock("./api", () => ({
  getCloudCredSets: () => getCloudCredSets(),
}));

const { credSetsChanged, useCloudCredSets } = await import("./useCloudCredSets");

/** A reader that renders whatever names the hook currently reports. */
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

/** Let the mocked fetch's promise chain settle inside act(). */
async function settle() {
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
}

describe("credential-set invalidation", () => {
  it("re-reads every mounted reader when a write announces a change", async () => {
    getCloudCredSets.mockResolvedValue({ ok: true, sets: [{ id: "a", name: "Hetzner" }] });

    // Two independently mounted readers = the editor card and a target's
    // credential picker, both on the Off-site tab.
    render(
      <>
        <Reader tag="editor" />
        <Reader tag="picker" />
      </>
    );
    await settle();
    expect(screen.getByTestId("picker").textContent).toBe("Hetzner");

    // A set is created elsewhere; the server now returns two.
    getCloudCredSets.mockResolvedValue({
      ok: true,
      sets: [{ id: "a", name: "Hetzner" }, { id: "b", name: "Backblaze" }],
    });

    // Before the announcement the picker still shows the old list — this is
    // precisely the stale state the reporter had to reload to clear.
    expect(screen.getByTestId("picker").textContent).toBe("Hetzner");

    await act(async () => {
      credSetsChanged();
    });
    await settle();

    // ...and no reload was involved.
    expect(screen.getByTestId("picker").textContent).toBe("Hetzner,Backblaze");
    expect(screen.getByTestId("editor").textContent).toBe("Hetzner,Backblaze");
  });

  it("keeps the last good list when a refetch fails", async () => {
    getCloudCredSets.mockResolvedValue({ ok: true, sets: [{ id: "a", name: "Hetzner" }] });
    render(<Reader tag="picker" />);
    await settle();

    // An empty picker would silently drop the target's current selection, so a
    // failed refetch must not blank the list.
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
