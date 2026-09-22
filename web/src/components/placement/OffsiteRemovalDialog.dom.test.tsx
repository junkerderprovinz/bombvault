// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { formatTs } from "../../lib/reltime";
import { removalPreview, renderWithProviders } from "../../lib/placement.testsupport";

const fake = await vi.hoisted(async () =>
  (await import("../../lib/placement.testsupport")).createPlacementApi()
);

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  ...fake.api,
}));

const { OffsiteRemovalDialog } = await import("./OffsiteRemovalDialog");

const item = { domain: "containers", key: "vaultwarden" } as const;

function open() {
  const onDone = vi.fn();
  const onClose = vi.fn();
  renderWithProviders(
    <OffsiteRemovalDialog item={item} name="vaultwarden" target={{ id: "t-b2", name: "B2" }} onDone={onDone} onClose={onClose} />
  );
  return { onDone, onClose };
}

function at(time: string): string {
  return formatTs(Date.parse(time) / 1000);
}

describe("OffsiteRemovalDialog", () => {
  beforeEach(() => fake.reset());
  afterEach(cleanup);

  it("asks with the count when nothing exists only at the target", async () => {
    const { onDone } = open();
    expect(await screen.findByText("Delete every copy of vaultwarden in B2? Copies: 14.")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Delete in B2" }));
    await waitFor(() => expect(onDone).toHaveBeenCalledWith(14));
    expect(fake.callsTo("deleteAtTarget")).toEqual([[item, "t-b2", [], ""]]);
  });

  it("keeps the button locked until the name is typed for snapshots found only there", async () => {
    fake.reply("getOffsiteRemoval", {
      ok: true,
      ...removalPreview({ onlyThere: [{ id: "b9aa", time: "2026-09-01T03:00:00Z" }] }),
    });
    const { onDone } = open();
    const button = (await screen.findByRole("button", { name: "Delete in B2" })) as HTMLButtonElement;
    expect(screen.getByText(at("2026-09-01T03:00:00Z"))).toBeTruthy();
    expect(button.disabled).toBe(true);
    const field = screen.getByLabelText("Type vaultwarden to confirm");
    fireEvent.change(field, { target: { value: "vault" } });
    expect(button.disabled).toBe(true);
    fireEvent.change(field, { target: { value: "vaultwarden" } });
    fireEvent.click(button);
    await waitFor(() => expect(onDone).toHaveBeenCalled());
    expect(fake.callsTo("deleteAtTarget")).toEqual([[item, "t-b2", ["b9aa"], "vaultwarden"]]);
  });

  it("says when the home could not be checked", async () => {
    fake.reply("getOffsiteRemoval", {
      ok: true,
      ...removalPreview({ homeUnreadable: true, onlyThere: [{ id: "b1aa", time: "2026-09-01T03:00:00Z" }] }),
    });
    open();
    expect(await screen.findByText("Whether NAS Keller still has them could not be checked.")).toBeTruthy();
  });

  it("shows the grown list in the same window and asks again", async () => {
    const first = [{ id: "b9aa", time: "2026-09-01T03:00:00Z" }];
    const grown = [...first, { id: "c7bb", time: "2026-09-02T03:00:00Z" }];
    fake.reply("getOffsiteRemoval", { ok: true, ...removalPreview({ onlyThere: first }) });
    fake.reply(
      "deleteAtTarget",
      { ok: false, code: "removal-grown", error: "grown", preview: removalPreview({ count: 15, onlyThere: grown }) },
      { ok: true, deleted: 15 }
    );
    const { onDone } = open();
    fireEvent.change(await screen.findByLabelText("Type vaultwarden to confirm"), { target: { value: "vaultwarden" } });
    fireEvent.click(screen.getByRole("button", { name: "Delete in B2" }));
    expect(await screen.findByText(at("2026-09-02T03:00:00Z"))).toBeTruthy();
    fireEvent.change(screen.getByLabelText("Type vaultwarden to confirm"), { target: { value: "vaultwarden" } });
    fireEvent.click(screen.getByRole("button", { name: "Delete in B2" }));
    await waitFor(() => expect(onDone).toHaveBeenCalledWith(15));
    expect(fake.callsTo("deleteAtTarget")[1]).toEqual([item, "t-b2", ["b9aa", "c7bb"], "vaultwarden"]);
  });

  it("closes with the translated refusal when the target is append-only", async () => {
    fake.reply("deleteAtTarget", { ok: false, code: "append-only", error: "server sentence" });
    const { onClose } = open();
    fireEvent.click(await screen.findByRole("button", { name: "Delete in B2" }));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(await screen.findByText("The target is append-only. Nothing here may delete from it.")).toBeTruthy();
  });
});
