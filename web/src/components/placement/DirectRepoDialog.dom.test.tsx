// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { renderWithProviders } from "../../lib/placement.testsupport";

const fake = await vi.hoisted(async () => (await import("../../lib/placement.testsupport")).createPlacementApi());

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  ...fake.api,
}));

const { DirectRepoDialog } = await import("./DirectRepoDialog");

const target = { id: "t-b2", name: "B2" };

function renderDialog(mode: "create" | "remember" = "create") {
  const onDone = vi.fn();
  const onClose = vi.fn();
  renderWithProviders(<DirectRepoDialog target={target} mode={mode} onDone={onDone} onClose={onClose} />);
  return { onDone, onClose };
}

describe("DirectRepoDialog", () => {
  beforeEach(() => fake.reset());
  afterEach(cleanup);

  it("starts from the place the server suggests beside the target", async () => {
    renderDialog();
    expect(await screen.findByDisplayValue("b2:bucket:containers-direct")).toBeTruthy();
    expect(screen.getByText("Direct repository at B2")).toBeTruthy();
    expect(fake.callsTo("getDirectRepo")).toEqual([["t-b2"]]);
  });

  it.each([
    ["bucket-root", "B2 sits at the root of its bucket, so every path in that bucket lies inside it. Choose another bucket."],
    ["path-needed", "This rest-server keeps private repositories. Enter the path for the direct repository."],
  ])("explains a %s suggestion", async (note, text) => {
    fake.reply("getDirectRepo", { ok: true, repo: null, suggestion: { location: "b2:other-direct", note } });
    renderDialog();
    expect(await screen.findByText(text)).toBeTruthy();
  });

  it("says why the suggestion could not be loaded", async () => {
    fake.reply("getDirectRepo", { ok: false, error: "unknown target" });
    renderDialog();
    expect(await screen.findByText("unknown target")).toBeTruthy();
  });

  it("keeps a location typed before the suggestion arrives", async () => {
    const release = fake.hold("getDirectRepo");
    renderDialog();
    fireEvent.change(screen.getByLabelText("Location"), { target: { value: "custom/path" } });
    await act(async () => release());
    expect(screen.getByDisplayValue("custom/path")).toBeTruthy();
    expect(screen.queryByDisplayValue("b2:bucket:containers-direct")).toBeNull();
  });

  it("tests the place without creating anything", async () => {
    renderDialog();
    await screen.findByDisplayValue("b2:bucket:containers-direct");
    fireEvent.click(screen.getByRole("button", { name: "Test connection" }));
    expect(await screen.findByText("Reachable, empty")).toBeTruthy();
    expect(fake.callsTo("testDirectLocation")).toEqual([["t-b2", "b2:bucket:containers-direct"]]);
    expect(fake.callsTo("createDirectRepo")).toEqual([]);
  });

  it("says why a place is not reachable", async () => {
    fake.reply("testDirectLocation", { ok: false, error: "connection refused" });
    renderDialog();
    await screen.findByDisplayValue("b2:bucket:containers-direct");
    fireEvent.click(screen.getByRole("button", { name: "Test connection" }));
    expect(await screen.findByText("Not reachable: connection refused")).toBeTruthy();
  });

  it("leaves nothing behind when cancelled", async () => {
    const { onClose } = renderDialog();
    await screen.findByDisplayValue("b2:bucket:containers-direct");
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onClose).toHaveBeenCalled();
    expect(fake.callsTo("createDirectRepo")).toEqual([]);
  });

  it("creates the repository with the name left to the server and hands it back", async () => {
    const { onDone } = renderDialog();
    await screen.findByDisplayValue("b2:bucket:containers-direct");
    fireEvent.click(screen.getByRole("button", { name: "Create and use" }));
    await waitFor(() => expect(onDone).toHaveBeenCalled());
    expect(fake.callsTo("createDirectRepo")).toEqual([["t-b2", "", "b2:bucket:containers-direct"]]);
    expect(onDone.mock.calls[0][0]).toMatchObject({ kind: "created", repo: { id: "repo-direct", companionOf: "t-b2" } });
  });

  it("only remembers the place for a folder set that does not exist yet", async () => {
    const { onDone } = renderDialog("remember");
    await screen.findByDisplayValue("b2:bucket:containers-direct");
    expect(screen.getByText("Created together with the folder set.")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Create and use" }));
    expect(onDone).toHaveBeenCalledWith({ kind: "remembered", name: "", location: "b2:bucket:containers-direct" });
    expect(fake.callsTo("createDirectRepo")).toEqual([]);
  });
});
