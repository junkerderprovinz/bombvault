// @vitest-environment jsdom
import { useState } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import type { PlacementView } from "./api";
import { droppedTarget, placementView, renderWithProviders } from "./placement.testsupport";

const fake = await vi.hoisted(async () => (await import("./placement.testsupport")).createPlacementApi());

vi.mock("./api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("./api")>()),
  ...fake.api,
}));

const { usePlacementSave } = await import("./usePlacementSave");

const item = { domain: "containers", key: "nginx" } as const;
const base = placementView();

function Harness({ onView }: { onView: (next: PlacementView) => void }) {
  const { shown, shake, save } = usePlacementSave(item, base, onView);
  const send = (skip: string[]) => save({ copies: { skip } }, { skip, copiesFollow: false });
  return (
    <div>
      <span data-testid="skip">{shown.skip.join(",") || "none"}</span>
      <span data-testid="shake">{shake}</span>
      <button type="button" onClick={() => send(["t-b2"])}>
        one
      </button>
      <button type="button" onClick={() => send([])}>
        two
      </button>
      <button type="button" onClick={() => send(["*"])}>
        three
      </button>
    </div>
  );
}

// A card whose page hands it a new view, the way a list reload does.
function Reloading({ first, later }: { first: PlacementView; later: PlacementView }) {
  const [view, setView] = useState(first);
  const { shown, save } = usePlacementSave(item, view, () => undefined);
  return (
    <div>
      <span data-testid="home">{shown.repoLabel || "none"}</span>
      <button type="button" onClick={() => save({ copies: { skip: ["t-b2"] } }, { skip: ["t-b2"], copiesFollow: false })}>
        chip
      </button>
      <button type="button" onClick={() => setView(later)}>
        list
      </button>
    </div>
  );
}

describe("usePlacementSave", () => {
  beforeEach(() => fake.reset());
  afterEach(cleanup);

  it("shows a change at once and keeps what the server answered", async () => {
    const release = fake.hold("setItemPlacement");
    const onView = vi.fn();
    renderWithProviders(<Harness onView={onView} />);
    fireEvent.click(screen.getByText("one"));
    expect(screen.getByTestId("skip").textContent).toBe("t-b2");
    await act(async () => release());
    await waitFor(() => expect(onView).toHaveBeenCalledTimes(1));
    expect(screen.getByTestId("skip").textContent).toBe("t-b2");
  });

  it("sends one change at a time and lets only the newest wait", async () => {
    const release = fake.hold("setItemPlacement");
    renderWithProviders(<Harness onView={vi.fn()} />);
    fireEvent.click(screen.getByText("one"));
    fireEvent.click(screen.getByText("two"));
    fireEvent.click(screen.getByText("three"));
    expect(screen.getByTestId("skip").textContent).toBe("*");
    await act(async () => release());
    await waitFor(() => expect(fake.callsTo("setItemPlacement")).toHaveLength(2));
    expect(fake.maxInFlight("setItemPlacement")).toBe(1);
    expect(fake.callsTo("setItemPlacement")[1]).toEqual([item, { copies: { skip: ["*"] } }]);
  });

  it("goes back and shakes when the server refuses", async () => {
    fake.reply("setItemPlacement", { ok: false, error: "busy", code: "domain-busy" });
    renderWithProviders(<Harness onView={vi.fn()} />);
    fireEvent.click(screen.getByText("one"));
    expect(await screen.findByText("A backup is running. Choose again once it has finished.")).toBeTruthy();
    expect(screen.getByTestId("skip").textContent).toBe("none");
    expect(screen.getByTestId("shake").textContent).toBe("1");
  });

  it("tells the page nothing once the card is gone", async () => {
    const release = fake.hold("setItemPlacement");
    const onView = vi.fn();
    const { unmount } = renderWithProviders(<Harness onView={onView} />);
    fireEvent.click(screen.getByText("one"));
    unmount();
    await act(async () => release());
    expect(onView).not.toHaveBeenCalled();
  });

  it("keeps the home it is on when an older list answer lands during a save", async () => {
    const release = fake.hold("setItemPlacement");
    const nas = placementView({ repo: "repo-nas", repoLabel: "NAS Keller", repoKind: "local" });
    renderWithProviders(<Reloading first={nas} later={base} />);
    fireEvent.click(screen.getByText("chip"));
    fireEvent.click(screen.getByText("list"));
    expect(screen.getByTestId("home").textContent).toBe("NAS Keller");
    await act(async () => release());
  });

  it("says what each dropped target keeps", async () => {
    fake.reply(
      "setItemPlacement",
      { ok: true, dropped: [droppedTarget()], placement: base },
      { ok: true, dropped: [droppedTarget({ copies: null })], placement: base },
      { ok: true, dropped: [droppedTarget({ appendOnly: true })], placement: base }
    );
    renderWithProviders(<Harness onView={vi.fn()} />);
    fireEvent.click(screen.getByText("one"));
    expect(
      await screen.findByText("B2 keeps its copies (14) and trims them to its own rules at the next run. No new ones are added.")
    ).toBeTruthy();
    fireEvent.click(screen.getByText("one"));
    expect(
      await screen.findByText("B2 keeps its copies and trims them to its own rules at the next run. No new ones are added.")
    ).toBeTruthy();
    fireEvent.click(screen.getByText("one"));
    expect(await screen.findByText("B2 is append-only and keeps every copy. No new ones are added.")).toBeTruthy();
  });
});
