// @vitest-environment jsdom
// Every save reconnects, so the card sends the whole form at once, keeps the
// stored password unless a new one is typed, and says when the broker did not
// let it clean up.
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { en } from "../../lib/i18n";
import type { HomeAssistantSettings } from "../../lib/api";

const getHomeAssistant = vi.fn();
const setHomeAssistant = vi.fn();

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    getHomeAssistant: () => getHomeAssistant(),
    setHomeAssistant: (...a: unknown[]) => setHomeAssistant(...a),
  };
});

const pushed: { message: string; severity?: string }[] = [];
vi.mock("../../lib/toast", () => ({
  useToast: () => ({ push: (message: string, severity?: string) => pushed.push({ message, severity }) }),
}));

const { HomeAssistantRows } = await import("./HomeAssistantRows");

function settings(over: Partial<HomeAssistantSettings> = {}): HomeAssistantSettings {
  return {
    enabled: false,
    host: "",
    port: 1883,
    username: "",
    passwordSet: false,
    tls: false,
    prefix: "bombvault",
    buttons: true,
    nodeId: "",
    status: { connected: false, error: "" },
    ...over,
  };
}

beforeEach(() => {
  pushed.length = 0;
  getHomeAssistant.mockReset();
  setHomeAssistant.mockReset();
});

afterEach(cleanup);

describe("HomeAssistantRows", () => {
  it("opens the form with the switch, saves it whole and shows the connection", async () => {
    getHomeAssistant.mockResolvedValue({ ok: true, settings: settings(), cooldownMinutes: 15, itemStartsPerDay: 4 });
    setHomeAssistant.mockResolvedValue({
      ok: true,
      settings: settings({ enabled: true, host: "10.0.0.2", nodeId: "a1b2c3d4", status: { connected: true, error: "" } }),
    });
    render(<HomeAssistantRows />);

    fireEvent.click(await screen.findByRole("switch", { name: en["ha.enable"] }));
    fireEvent.change(screen.getByPlaceholderText(en["ha.hostPlaceholder"]), { target: { value: "10.0.0.2" } });
    fireEvent.click(screen.getByRole("button", { name: new RegExp(en["settings.save"]) }));

    await waitFor(() =>
      expect(setHomeAssistant).toHaveBeenCalledWith({
        enabled: true,
        host: "10.0.0.2",
        port: 1883,
        username: "",
        password: "",
        tls: false,
        prefix: "bombvault",
        buttons: true,
      })
    );
    expect(await screen.findByText(en["ha.statusConnected"])).toBeTruthy();
  });

  it("names the broker's refusal and a failed clean-up", async () => {
    getHomeAssistant.mockResolvedValue({
      ok: true,
      settings: settings({ enabled: true, host: "broker", status: { connected: false, error: "connection refused" } }),
    });
    setHomeAssistant.mockResolvedValue({ ok: true, settings: settings(), warning: "mqtt-remove-failed" });
    render(<HomeAssistantRows />);

    expect(await screen.findByText(en["ha.statusError"].replace("{error}", "connection refused"))).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: new RegExp(en["settings.save"]) }));
    await waitFor(() => expect(pushed).toContainEqual({ message: en["ha.removeFailed"], severity: "fail" }));
  });
});

describe("HomeAssistantRows switch", () => {
  it("saves at once when a connection that is on is switched off", async () => {
    getHomeAssistant.mockResolvedValue({ ok: true, settings: settings({ enabled: true, host: "broker" }) });
    setHomeAssistant.mockResolvedValue({ ok: true, settings: settings({ host: "broker" }) });
    render(<HomeAssistantRows />);

    fireEvent.click(await screen.findByRole("switch", { name: en["ha.enable"] }));
    await waitFor(() => expect(setHomeAssistant).toHaveBeenCalledWith(expect.objectContaining({ enabled: false, host: "broker" })));
    await waitFor(() => expect(screen.queryByPlaceholderText(en["ha.hostPlaceholder"])).toBeNull());
  });
});

describe("HomeAssistantRows password", () => {
  it("stops showing the stored password once the broker changes", async () => {
    getHomeAssistant.mockResolvedValue({
      ok: true,
      settings: settings({ enabled: true, host: "10.0.0.2", username: "bv", passwordSet: true }),
      cooldownMinutes: 15,
      itemStartsPerDay: 4,
    });
    render(<HomeAssistantRows />);
    expect(await screen.findByPlaceholderText(en["cloud.secretSet"])).toBeTruthy();
    fireEvent.change(screen.getByPlaceholderText(en["ha.hostPlaceholder"]), { target: { value: "10.0.0.9" } });
    expect(screen.queryByPlaceholderText(en["cloud.secretSet"])).toBeNull();
  });
});
