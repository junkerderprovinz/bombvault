// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { en } from "../../lib/i18n";

const getMdns = vi.fn();
const setMdns = vi.fn();

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return { ...actual, getMdns: () => getMdns(), setMdns: (...a: unknown[]) => setMdns(...a) };
});

vi.mock("../../lib/toast", () => ({ useToast: () => ({ push: () => {} }) }));

const { MdnsRow } = await import("./MdnsRow");

afterEach(cleanup);

describe("MdnsRow", () => {
  it("shows the announced address and switches it off", async () => {
    const url = "https://bombvault.local:3443/";
    getMdns.mockResolvedValue({ ok: true, enabled: true, running: true, url, instance: "BombVault", error: "" });
    setMdns.mockResolvedValue({ ok: true, enabled: false, running: false, url: "", instance: "", error: "" });
    render(<MdnsRow />);

    expect(await screen.findByText(en["mdns.statusOn"].replace("{url}", url))).toBeTruthy();
    fireEvent.click(screen.getByRole("switch", { name: en["mdns.enable"] }));
    await waitFor(() => expect(setMdns).toHaveBeenCalledWith(false));
    expect(await screen.findByText(en["mdns.statusOff"])).toBeTruthy();
  });

  it("says the announcement stays inside Docker instead of showing an address nobody reaches", async () => {
    getMdns.mockResolvedValue({ ok: true, enabled: true, running: true, hostOnly: true, url: "", instance: "BombVault", error: "" });
    render(<MdnsRow />);

    expect(await screen.findByText(en["mdns.statusHostOnly"])).toBeTruthy();
    expect(screen.queryByText(/bombvault\.local:/)).toBeNull();
  });
});
