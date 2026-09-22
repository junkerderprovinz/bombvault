// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import type { NewTargetAnswer, NewTargetQuestion } from "./NewTargetQuestion";
import { renderWithProviders, targetPreview } from "../../lib/placement.testsupport";

const fake = await vi.hoisted(async () => (await import("../../lib/placement.testsupport")).createPlacementApi());

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  ...fake.api,
}));

const { useNewTargetQuestion } = await import("./NewTargetQuestion");

const b2: NewTargetQuestion = { domain: "containers", location: "b2:bucket:containers", name: "B2", moved: false };

function Harness({ q, onAnswer }: { q: NewTargetQuestion; onAnswer: (a: NewTargetAnswer) => void }) {
  const { ask, dialog } = useNewTargetQuestion();
  return (
    <>
      <button type="button" onClick={() => void ask(q).then(onAnswer)}>
        ask
      </button>
      {dialog}
    </>
  );
}

function askWith(q: NewTargetQuestion = b2) {
  const onAnswer = vi.fn();
  renderWithProviders(<Harness q={q} onAnswer={onAnswer} />);
  fireEvent.click(screen.getByText("ask"));
  return onAnswer;
}

describe("the new target question", () => {
  beforeEach(() => fake.reset());
  afterEach(cleanup);

  it("names what the first run receives and leaves out a size nobody measured", async () => {
    askWith();
    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("At its first run B2 receives every item not set to Local.");
    expect(dialog.textContent).toContain("Items and project folders: 15");
    expect(dialog.textContent).toContain("Snapshots: up to 420");
    expect(dialog.textContent).not.toContain("Size:");
    expect(fake.callsTo("getNewTargetPreview")).toEqual([["containers", "b2:bucket:containers", undefined]]);
  });

  it("gives the size when the sources were measured", async () => {
    fake.reply("getNewTargetPreview", { ok: true, preview: targetPreview({ bytes: 5 * 1024 ** 3 }) });
    askWith();
    expect((await screen.findByRole("dialog")).textContent).toContain("Size: up to 5.0 GB");
  });

  it("asks about the whole history when a target moves", async () => {
    askWith({ domain: "vms", location: "b2:new", targetId: "t1", name: "B2", moved: true });
    expect((await screen.findByRole("dialog")).textContent).toContain("The new location of B2 receives the whole history.");
    expect(fake.callsTo("getNewTargetPreview")).toEqual([["vms", "b2:new", "t1"]]);
  });

  it("sends the chosen exclusions along", async () => {
    fake.reply("getNewTargetPreview", {
      ok: true,
      preview: targetPreview({ formerlyExcluded: [{ identity: "container:plex", skip: ["t-hz"] }], defaultExcludes: true }),
    });
    const onAnswer = askWith();
    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("Left out of other targets so far: plex");
    fireEvent.click(within(dialog).getByRole("switch", { name: "Leave these out here too" }));
    fireEvent.click(within(dialog).getByRole("switch", { name: "Leave B2 out of the default too" }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Confirm" }));
    await waitFor(() =>
      expect(onAnswer).toHaveBeenCalledWith({ go: true, alsoExclude: { identities: ["container:plex"], default: true } })
    );
  });

  it("goes ahead without exclusions when none is ticked", async () => {
    fake.reply("getNewTargetPreview", {
      ok: true,
      preview: targetPreview({ formerlyExcluded: [{ identity: "container:plex", skip: ["t-hz"] }] }),
    });
    const onAnswer = askWith();
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Confirm" }));
    await waitFor(() => expect(onAnswer).toHaveBeenCalledWith({ go: true, alsoExclude: null }));
  });

  it("answers no when cancelled", async () => {
    const onAnswer = askWith();
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(onAnswer).toHaveBeenCalledWith({ go: false }));
  });

  it("still asks, with the reason, when the preview cannot be read", async () => {
    fake.reply("getNewTargetPreview", { ok: false, error: "unreadable", code: "placement-unreadable" });
    const onAnswer = askWith();
    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("The placement rules could not be read, so nothing is copied until they can.");
    fireEvent.click(within(dialog).getByRole("button", { name: "Confirm" }));
    await waitFor(() => expect(onAnswer).toHaveBeenCalledWith({ go: true, alsoExclude: null }));
  });

  it.each(["flash", "config"] as const)("does not ask for %s", async (domain) => {
    const onAnswer = askWith({ ...b2, domain });
    await waitFor(() => expect(onAnswer).toHaveBeenCalledWith({ go: true, alsoExclude: null }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(fake.callsTo("getNewTargetPreview")).toEqual([]);
  });
});
