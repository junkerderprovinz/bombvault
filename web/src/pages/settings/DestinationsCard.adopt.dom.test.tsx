// @vitest-environment jsdom
// A target typed in by hand under a destination is offered for takeover,
// and nothing changes until the takeover is confirmed.
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { en } from "../../lib/i18n";
import type { Destination } from "../../lib/api";

const listDestinations = vi.fn();
const adoptIntoDestination = vi.fn();
const confirm = vi.fn();

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    listDestinations: () => listDestinations(),
    adoptIntoDestination: (...a: unknown[]) => adoptIntoDestination(...a),
  };
});

vi.mock("../../lib/toast", () => ({
  useToast: () => ({ push: () => {} }),
}));

vi.mock("../../lib/useConfirm", () => ({
  useConfirm: () => ({ confirm: (...a: unknown[]) => confirm(...a), confirmDialog: null }),
}));

const { DestinationsCard } = await import("./DestinationsCard");

const qnap: Destination = {
  id: "d1",
  name: "QNAP",
  provider: "s3",
  repo: "s3:http://nas:9000/bv/dxp",
  credsRef: "c1",
  storageClass: "",
  immutable: false,
  createdAt: 1,
  domains: [],
  adoptable: [{ id: "t1", domain: "containers", name: "Primary", repo: "s3:http://nas:9000/bv/dxp/container", primary: true }],
};

afterEach(() => {
  cleanup();
  listDestinations.mockReset();
  adoptIntoDestination.mockReset();
  confirm.mockReset();
});

describe("DestinationsCard takeover", () => {
  it("takes over a primary under the destination", async () => {
    listDestinations.mockResolvedValue({ ok: true, destinations: [qnap] });
    adoptIntoDestination.mockResolvedValue({ ok: true });
    confirm.mockResolvedValue(true);
    render(<DestinationsCard />);

    expect(await screen.findByText(en["dest.adopt.title"])).toBeTruthy();
    expect(screen.getByText("s3:http://nas:9000/bv/dxp/container")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: en["dest.adopt.take"] }));

    await waitFor(() => expect(adoptIntoDestination).toHaveBeenCalledWith("d1", "t1"));
    expect(String(confirm.mock.calls[0][0])).toContain("Let Primary (Containers) follow QNAP?");
  });

  it("leaves everything alone when the takeover is not confirmed", async () => {
    listDestinations.mockResolvedValue({ ok: true, destinations: [qnap] });
    confirm.mockResolvedValue(false);
    render(<DestinationsCard />);

    fireEvent.click(await screen.findByRole("button", { name: en["dest.adopt.take"] }));
    await waitFor(() => expect(confirm).toHaveBeenCalled());
    expect(adoptIntoDestination).not.toHaveBeenCalled();
  });
});
