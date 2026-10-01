// @vitest-environment jsdom
// The card is the only place a key can be minted, and a key it hands out is
// gone from the server the moment the panel closes. So the cases that matter
// are the ones where it must refuse (a public host name without a password, a
// certificate that does not cover this address) and the ones where a wrong
// sentence would send the operator to the wrong fix.
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, useLocation } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { countText, en } from "../../lib/i18n";
import type { McpKeyView, McpKeysResponse } from "../../lib/api";
import { CLIENT_MARKS } from "../../lib/mcpClientMarks";
import { DESKTOP_QUERY } from "../../lib/useMediaQuery";
import { LOGIN_PASSWORD_FIELD } from "./shared";

const listMcpKeys = vi.fn();
const createMcpKey = vi.fn();
const updateMcpKey = vi.fn();
const rotateMcpKey = vi.fn();
const revokeMcpKey = vi.fn();
const purgeMcpKey = vi.fn();
const addMcpCertificateName = vi.fn();
const getMcpKeyActivity = vi.fn();
const setMcpOAuth = vi.fn();

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    listMcpKeys: () => listMcpKeys(),
    createMcpKey: (...a: unknown[]) => createMcpKey(...a),
    updateMcpKey: (...a: unknown[]) => updateMcpKey(...a),
    rotateMcpKey: (...a: unknown[]) => rotateMcpKey(...a),
    revokeMcpKey: (...a: unknown[]) => revokeMcpKey(...a),
    purgeMcpKey: (...a: unknown[]) => purgeMcpKey(...a),
    addMcpCertificateName: (...a: unknown[]) => addMcpCertificateName(...a),
    getMcpKeyActivity: (...a: unknown[]) => getMcpKeyActivity(...a),
    setMcpOAuth: (...a: unknown[]) => setMcpOAuth(...a),
  };
});

const pushed: { message: string; severity?: string }[] = [];
vi.mock("../../lib/toast", () => ({
  useToast: () => ({ push: (message: string, severity?: string) => pushed.push({ message, severity }) }),
}));

// The width query reads this flag, so a test can put the card on a phone. The
// page keeps the first MediaQueryList it gets, so the stub is in place before
// the first render.
let desktop = true;
window.matchMedia = ((query: string) => ({
  get matches() {
    return query === DESKTOP_QUERY ? desktop : /\bmin-width\s*:/.test(query);
  },
  media: query,
  onchange: null,
  addEventListener: () => {},
  removeEventListener: () => {},
  addListener: () => {},
  removeListener: () => {},
  dispatchEvent: () => false,
})) as unknown as typeof window.matchMedia;

const { McpServerCard } = await import("./McpServerCard");

function key(over: Partial<McpKeyView> = {}): McpKeyView {
  return {
    id: "k1",
    viaOAuth: false,
    label: "office laptop",
    hint: "f3a9",
    canStartBackups: true,
    createdAt: 1_700_000_000,
    rotatedAt: 0,
    lastUsedAt: 0,
    lastUsedFrom: "",
    revokedAt: 0,
    revokedReason: "",
    client: "",
    inUse: false,
    unusable: "",
    callsToday: 0,
    ...over,
  };
}

function payload(over: Partial<McpKeysResponse> = {}): McpKeysResponse {
  return {
    ok: true,
    endpointPath: "/mcp",
    limit: 10,
    authEnabled: true,
    hostAllowsKeys: true,
    startsPerHour: 12,
    cooldownMinutes: 15,
    itemStartsPerDay: 5,
    certificate: null,
    keys: [],
    revoked: [],
    oauth: { enabled: false, issuer: "", active: false, grantLimit: 10, connectorPath: "/mcp" },
    ...over,
  };
}

/** Renders the card and waits for the first load to land. */
async function renderCard(...answers: McpKeysResponse[]) {
  listMcpKeys.mockReset();
  for (const a of answers) listMcpKeys.mockResolvedValueOnce(a);
  listMcpKeys.mockResolvedValue(answers[answers.length - 1]);
  const view = render(<McpServerCard hueIndex={0} />, { wrapper: MemoryRouter });
  await waitFor(() => expect(listMcpKeys).toHaveBeenCalled());
  return view;
}

function cardText(): string {
  return document.body.textContent ?? "";
}

beforeEach(() => {
  pushed.length = 0;
  for (const m of [createMcpKey, updateMcpKey, rotateMcpKey, revokeMcpKey, purgeMcpKey, addMcpCertificateName, getMcpKeyActivity, setMcpOAuth]) {
    m.mockReset();
  }
});

afterEach(() => {
  cleanup();
  desktop = true;
  vi.unstubAllGlobals();
  Reflect.deleteProperty(navigator, "clipboard");
});

/** The client button of the picker, by the name and kind it announces, once
 *  the first load has drawn the picker. */
function clientButton(name: string, kind?: string): Promise<HTMLElement> {
  return screen.findByRole("button", { name: kind ? `${name} ${kind}` : name });
}

async function openClient(name: string, kind?: string): Promise<HTMLElement> {
  fireEvent.click(await clientButton(name, kind));
  return screen.getByRole("dialog");
}

function dialogText(): string {
  return screen.getByRole("dialog").textContent ?? "";
}

describe("the MCP card without a key", () => {
  it("shows off state and the open-interface warning", async () => {
    await renderCard(payload({ authEnabled: false }));

    await waitFor(() => expect(screen.getByText(en["mcp.statusOff"])).toBeTruthy());
    expect(screen.getByText(en["mcp.noPasswordWarning"])).toBeTruthy();
    expect(screen.getByRole("button", { name: en["mcp.setPassword"] })).toBeTruthy();
    expect(await clientButton("Claude Code", en["mcp.kindTerminal"])).toBeTruthy();
    expect(screen.queryAllByRole("listitem")).toHaveLength(0);
  });

  it("leads Set password to the field on the Security page", async () => {
    listMcpKeys.mockReset();
    listMcpKeys.mockResolvedValue(payload({ authEnabled: false }));
    function Where() {
      const at = useLocation();
      return <output data-testid="where">{at.pathname + at.hash}</output>;
    }
    render(
      <MemoryRouter initialEntries={["/settings/integrations"]}>
        <McpServerCard hueIndex={0} />
        <Where />
      </MemoryRouter>,
    );
    fireEvent.click(await screen.findByRole("button", { name: en["mcp.setPassword"] }));
    expect(screen.getByTestId("where").textContent).toBe(`/settings/security#${LOGIN_PASSWORD_FIELD}`);
  });

  it("hides key creation for a public host without a password", async () => {
    vi.stubGlobal("location", new URL("https://backup.example.com/settings"));
    await renderCard(payload({ authEnabled: false, hostAllowsKeys: false, keys: [key()] }));

    await waitFor(() =>
      expect(
        screen.getByText(en["mcp.needsPasswordForHost"].replace("{host}", "backup.example.com"))
      ).toBeTruthy()
    );
    expect(screen.queryByRole("button", { name: en["mcp.rotate"] })).toBeNull();
    const dialog = await openClient("Cursor", en["mcp.kindEditor"]);
    expect(within(dialog).queryByRole("button", { name: en["mcp.createKey"] })).toBeNull();
    expect(within(dialog).getByRole("button", { name: /office laptop/ })).toBeTruthy();
  });

  it("offers key creation once a login password is set on this page", async () => {
    vi.stubGlobal("location", new URL("https://backup.example.com/settings"));
    listMcpKeys.mockReset();
    listMcpKeys.mockResolvedValue(payload({ authEnabled: false, hostAllowsKeys: false }));
    const view = render(<McpServerCard hueIndex={0} passwordSet={false} />, { wrapper: MemoryRouter });
    await waitFor(() => expect(screen.getByText(en["mcp.noPasswordWarning"])).toBeTruthy());

    listMcpKeys.mockResolvedValue(payload());
    view.rerender(<McpServerCard hueIndex={0} passwordSet />);

    await waitFor(() => expect(screen.queryByText(en["mcp.noPasswordWarning"])).toBeNull());
    const dialog = await openClient("Cursor", en["mcp.kindEditor"]);
    expect(within(dialog).getByRole("button", { name: en["mcp.createKey"] })).toBeTruthy();
  });
});

describe("the client picker", () => {
  it("lists the clients on this computer, then the cloud ones, then Other client", async () => {
    await renderCard(payload());
    await screen.findByText(en["mcp.groupLocal"]);

    const names = screen
      .getAllByRole("button")
      .filter((b) => b.classList.contains("glim-readme-btn"))
      .map((b) => b.getAttribute("aria-label"));
    expect(names).toHaveLength(28 + 4 + 1);
    expect(names.slice(0, 3)).toEqual([
      `AnythingLLM ${en["mcp.kindLocalModels"]}`,
      `Antigravity ${en["mcp.kindEditor"]}`,
      `Claude Code ${en["mcp.kindTerminal"]}`,
    ]);
    expect(names.slice(28, 32)).toEqual(
      ["ChatGPT", "Claude", "Grok", "Le Chat"].map((n) => `${n} ${en["mcp.kindWebChat"]}`)
    );
    expect(names[32]).toBe(en["mcp.otherClient"]);
  });

  it("explains the picker through one (i) on its label and one on the cloud group", async () => {
    await renderCard(payload({ keys: [key()] }));

    const label = await screen.findByText(en["mcp.snippetsLabel"]);
    const bubbles = label.querySelectorAll("[aria-label]");
    expect(bubbles).toHaveLength(1);
    expect(bubbles[0].getAttribute("aria-label")).toBe(en["mcp.connectHint"]);

    const cloud = screen.getByText(en["mcp.groupCloud"]);
    expect([...cloud.querySelectorAll("[aria-label]")].map((b) => b.getAttribute("aria-label"))).toEqual([
      en["mcp.cloudWarning"],
    ]);
  });

  it("shows the endpoint of this address", async () => {
    vi.stubGlobal("location", new URL("https://tower.local:3443/settings"));
    await renderCard(payload({ keys: [key()] }));

    const field = (await screen.findByLabelText(en["mcp.endpointLabel"])) as HTMLInputElement;
    expect(field.tagName).toBe("INPUT");
    expect(field.readOnly).toBe(true);
    expect(field.value).toBe("https://tower.local:3443/mcp");
    expect(field.parentElement?.contains(screen.getByRole("button", { name: en["common.copy"] }))).toBe(true);
  });
});

describe("a client's setup dialog", () => {
  it("creates a key named after the client and shows it exactly once", async () => {
    await renderCard(payload(), payload({ keys: [key({ label: "Claude Code", client: "claude-code" })] }));
    createMcpKey.mockResolvedValue({
      ok: true,
      key: "bvmcp_abcdef123456",
      item: key({ label: "Claude Code", client: "claude-code" }),
    });

    const dialog = await openClient("Claude Code", en["mcp.kindTerminal"]);
    expect((within(dialog).getByLabelText(en["mcp.labelLabel"]) as HTMLInputElement).value).toBe("Claude Code");
    fireEvent.click(within(dialog).getByRole("button", { name: en["mcp.createKey"] }));

    await waitFor(() => expect(createMcpKey).toHaveBeenCalledWith("Claude Code", false, "claude-code"));
    await waitFor(() => expect(within(dialog).getByDisplayValue("bvmcp_abcdef123456")).toBeTruthy());
    expect(within(dialog).getByText(en["mcp.showOnce"])).toBeTruthy();
    // The Claude Code command reads the key from a file, so only the field holds it.
    expect(within(dialog).getByText(/claude mcp add/).textContent).not.toContain("bvmcp_abcdef123456");
  });

  it("writes the new key into the configuration of a client that has to hold it", async () => {
    await renderCard(payload(), payload({ keys: [key({ label: "Zed", client: "zed" })] }));
    createMcpKey.mockResolvedValue({ ok: true, key: "bvmcp_abcdef123456", item: key({ label: "Zed", client: "zed" }) });

    const dialog = await openClient("Zed", en["mcp.kindEditor"]);
    expect(dialogText()).toContain("<your key>");
    fireEvent.click(within(dialog).getByRole("button", { name: en["mcp.createKey"] }));

    await waitFor(() =>
      expect(within(dialog).getByText(/context_servers/).textContent).toContain("bvmcp_abcdef123456")
    );
    expect(dialogText()).toContain(en["mcp.keyInFile"].replace("{app}", "Zed"));
  });

  it("points Claude Desktop at a key file and says how to write its path in JSON", async () => {
    await renderCard(payload(), payload({ keys: [key({ label: "Claude Desktop", client: "claude-desktop" })] }));
    createMcpKey.mockResolvedValue({
      ok: true,
      key: "bvmcp_abcdef123456",
      item: key({ label: "Claude Desktop", client: "claude-desktop" }),
    });

    const dialog = await openClient("Claude Desktop", en["mcp.kindDesktop"]);
    fireEvent.click(within(dialog).getByRole("button", { name: en["mcp.createKey"] }));
    await waitFor(() => expect(within(dialog).getByDisplayValue("bvmcp_abcdef123456")).toBeTruthy());

    expect(within(dialog).getByText(/--header-file/).textContent).not.toContain("bvmcp_abcdef123456");
    expect(dialogText()).toContain(en["mcp.keyFileLine"].replace("{app}", "Claude Desktop"));
    expect(within(dialog).getByLabelText(en["mcp.keyFileJsonTip"].replace("{app}", "Claude Desktop"))).toBeTruthy();
  });

  it("offers Warp's + Add field next to its file, without claiming the button opens the file", async () => {
    await renderCard(payload());
    await openClient("Warp", en["mcp.kindTerminal"]);

    expect(dialogText()).toContain("~/.warp/.mcp.json");
    expect(dialogText()).toContain(
      en["mcp.setupPaste"].replace("{app}", "Warp").replace("{ui}", "Settings, Agents, Warp Agent, Manage MCP servers, + Add")
    );
    expect(dialogText()).not.toContain(en["mcp.setupFileUi"].replace("{app}", "Warp").split("{ui}")[0]);
  });

  it("names Junie for JetBrains' file and AI Assistant's own Add dialog", async () => {
    await renderCard(payload());
    await openClient("JetBrains", en["mcp.kindEditor"]);

    expect(dialogText()).toContain("~/.junie/mcp/mcp.json");
    expect(dialogText()).toContain(
      en["mcp.setupFileUi"].replace("{app}", "JetBrains").replace("{ui}", "Settings, Tools, Junie, MCP Settings")
    );
    expect(dialogText()).toContain(
      en["mcp.setupPaste"]
        .replace("{app}", "JetBrains")
        .replace("{ui}", "Settings, Tools, AI Assistant, Model Context Protocol (MCP), Add")
    );
  });

  it("masks the key in the configuration while the key field is hidden, and still copies the real one", async () => {
    await renderCard(payload(), payload({ keys: [key({ label: "Zed", client: "zed" })] }));
    createMcpKey.mockResolvedValue({ ok: true, key: "bvmcp_abcdef123456", item: key({ label: "Zed", client: "zed" }) });
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });

    const dialog = await openClient("Zed", en["mcp.kindEditor"]);
    fireEvent.click(within(dialog).getByRole("button", { name: en["mcp.createKey"] }));
    await waitFor(() => expect(within(dialog).getByDisplayValue("bvmcp_abcdef123456")).toBeTruthy());

    fireEvent.click(within(dialog).getByRole("button", { name: en["common.hideValue"] }));
    expect(within(dialog).getByText(/context_servers/).textContent).not.toContain("bvmcp_abcdef123456");
    fireEvent.click(within(dialog).getByRole("button", { name: en["mcp.copyConfig"] }));
    await waitFor(() => expect(writeText).toHaveBeenCalled());
    expect(writeText.mock.calls[0][0]).toContain("Bearer bvmcp_abcdef123456");
  });

  it("keeps a created key on the card when the dialog closes before the server answers", async () => {
    await renderCard(payload());
    let answer: (v: unknown) => void = () => {};
    createMcpKey.mockReturnValue(new Promise((resolve) => (answer = resolve)));

    const dialog = await openClient(en["mcp.otherClient"]);
    fireEvent.change(within(dialog).getByLabelText(en["mcp.labelLabel"]), { target: { value: "laptop" } });
    fireEvent.click(within(dialog).getByRole("button", { name: en["mcp.createKey"] }));
    await waitFor(() => expect(createMcpKey).toHaveBeenCalled());
    fireEvent.click(within(dialog).getByRole("button", { name: en["common.close"] }));
    expect(screen.queryByRole("dialog")).toBeNull();

    answer({ ok: true, key: "bvmcp_abcdef123456", item: key({ label: "laptop" }) });
    await waitFor(() => expect(screen.getByDisplayValue("bvmcp_abcdef123456")).toBeTruthy());
  });

  it("drops a created key from the card once a call has used it", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      const made = key({ label: "laptop" });
      await renderCard(payload(), payload({ keys: [made] }));
      createMcpKey.mockResolvedValue({ ok: true, key: "bvmcp_abcdef123456", item: made });

      const dialog = await openClient(en["mcp.otherClient"]);
      fireEvent.change(within(dialog).getByLabelText(en["mcp.labelLabel"]), { target: { value: "laptop" } });
      fireEvent.click(within(dialog).getByRole("button", { name: en["mcp.createKey"] }));
      await waitFor(() => expect(within(dialog).getByDisplayValue("bvmcp_abcdef123456")).toBeTruthy());

      listMcpKeys.mockResolvedValue(payload({ keys: [{ ...made, lastUsedAt: 1_700_000_900 }] }));
      await vi.advanceTimersByTimeAsync(3100);
      await waitFor(() => expect(within(dialog).getByText(en["mcp.connectedTitle"])).toBeTruthy());
      fireEvent.click(within(dialog).getByRole("button", { name: en["common.done"] }));
      expect(screen.queryByDisplayValue("bvmcp_abcdef123456")).toBeNull();
    } finally {
      vi.useRealTimers();
    }
  });

  it("says in the key step why no key can be made here", async () => {
    vi.stubGlobal("location", new URL("https://backup.example.com/settings"));
    await renderCard(payload({ authEnabled: false, hostAllowsKeys: false }));

    await openClient("Cursor", en["mcp.kindEditor"]);
    expect(dialogText()).toContain(en["mcp.needsPasswordForHost"].replace("{host}", "backup.example.com"));
  });

  it("hands an unused new key back to the card when it closes", async () => {
    await renderCard(payload());
    listMcpKeys.mockRejectedValueOnce(new Error("503"));
    createMcpKey.mockResolvedValue({ ok: true, key: "bvmcp_abcdef123456", item: key({ label: "laptop" }) });

    const dialog = await openClient(en["mcp.otherClient"]);
    fireEvent.change(within(dialog).getByLabelText(en["mcp.labelLabel"]), { target: { value: "laptop" } });
    fireEvent.click(within(dialog).getByRole("button", { name: en["mcp.createKey"] }));
    await waitFor(() => expect(createMcpKey).toHaveBeenCalledWith("laptop", false, ""));
    await waitFor(() => expect(within(dialog).getByDisplayValue("bvmcp_abcdef123456")).toBeTruthy());

    fireEvent.click(within(dialog).getByRole("button", { name: en["common.close"] }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.getByDisplayValue("bvmcp_abcdef123456")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: en["mcp.dismissKey"] }));
    await waitFor(() => expect(screen.queryByText(en["mcp.newKeyTitle"])).toBeNull());
    expect(cardText()).not.toContain("bvmcp_abcdef123456");
  });

  it("turns green once the chosen key is used, and stops asking once it closes", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      const used = key({ label: "Cursor", client: "cursor", lastUsedAt: 1_700_000_500 });
      await renderCard(payload({ keys: [used] }));
      await screen.findByRole("listitem");

      const dialog = await openClient("Cursor", en["mcp.kindEditor"]);
      fireEvent.click(within(dialog).getByRole("tab", { name: en["mcp.existingKey"] }));
      fireEvent.click(within(dialog).getByRole("button", { name: /^Cursor/ }));
      expect(dialogText()).toContain(en["mcp.waitLine"].replace("{app}", "Cursor"));

      const calls = listMcpKeys.mock.calls.length;
      listMcpKeys.mockResolvedValue(
        payload({ keys: [{ ...used, lastUsedAt: 1_700_000_900, lastUsedFrom: "192.168.10.16" }] })
      );
      await vi.advanceTimersByTimeAsync(3100);
      expect(listMcpKeys.mock.calls.length).toBeGreaterThan(calls);
      await waitFor(() => expect(within(dialog).getByText(en["mcp.connectedTitle"])).toBeTruthy());
      expect(dialogText()).toContain("192.168.10.16");

      fireEvent.click(within(dialog).getByRole("button", { name: en["common.done"] }));
      const after = listMcpKeys.mock.calls.length;
      await vi.advanceTimersByTimeAsync(10_000);
      expect(listMcpKeys.mock.calls.length).toBe(after);
    } finally {
      vi.useRealTimers();
    }
  });

  it("does not turn green on a call an existing key made before the dialog opened", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      const used = key({ label: "Cursor", client: "cursor", lastUsedAt: 1_700_000_500 });
      await renderCard(payload({ keys: [used] }));
      await screen.findByRole("listitem");

      // Another client used the key after the card last loaded the list.
      listMcpKeys.mockResolvedValue(payload({ keys: [{ ...used, lastUsedAt: 1_700_000_900 }] }));
      const dialog = await openClient("Cursor", en["mcp.kindEditor"]);
      fireEvent.click(within(dialog).getByRole("tab", { name: en["mcp.existingKey"] }));
      fireEvent.click(within(dialog).getByRole("button", { name: /^Cursor/ }));
      expect(dialogText()).toContain(en["mcp.pickShared"]);

      await vi.advanceTimersByTimeAsync(3100);
      expect(within(dialog).queryByText(en["mcp.connectedTitle"])).toBeNull();

      listMcpKeys.mockResolvedValue(payload({ keys: [{ ...used, lastUsedAt: 1_700_001_300 }] }));
      await vi.advanceTimersByTimeAsync(3100);
      await waitFor(() => expect(within(dialog).getByText(en["mcp.connectedTitle"])).toBeTruthy());
    } finally {
      vi.useRealTimers();
    }
  });

  it("puts the internet warning first in a cloud client's dialog", async () => {
    await renderCard(payload());
    const dialog = await openClient("Le Chat", en["mcp.kindWebChat"]);
    expect(within(dialog).getByText(en["mcp.cloudWarningTitle"])).toBeTruthy();
    expect(dialogText().indexOf(en["mcp.cloudWarningTitle"])).toBeLessThan(dialogText().indexOf(en["mcp.kindWebChat"]));
    expect(within(dialog).getByText(/Custom Connector/)).toBeTruthy();
  });

  it("sets up ChatGPT and Claude on the web through sign-in instead of a key", async () => {
    await renderCard(payload());
    let dialog = await openClient("ChatGPT", en["mcp.kindWebChat"]);
    expect(within(dialog).getByText(en["mcp.stepOAuth"])).toBeTruthy();
    expect(dialogText()).toContain(en["mcp.setupChatGPT"]);
    // The paths sit in bidi isolates, so an RTL paragraph keeps their slashes in place.
    expect(dialogText().replace(/[\u2066\u2069]/g, "")).toContain(en["mcp.oauthProxyNote"]);
    expect(within(dialog).getByRole("switch", { name: en["mcp.oauthToggle"] })).toBeTruthy();
    expect(within(dialog).queryByRole("button", { name: en["mcp.createKey"] })).toBeNull();
    fireEvent.click(within(dialog).getByRole("button", { name: en["common.close"] }));

    dialog = await openClient("Claude", en["mcp.kindWebChat"]);
    expect(dialogText()).toContain(en["mcp.setupClaudeAi"]);
    expect(within(dialog).queryByRole("button", { name: en["mcp.createKey"] })).toBeNull();
  });

  it("hands a cloud client the connector URL and turns green when it has signed in", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      const oauth = { enabled: true, issuer: "https://vault.example", active: true, grantLimit: 10, connectorPath: "/mcp" };
      await renderCard(payload({ oauth }));
      const dialog = await openClient("ChatGPT", en["mcp.kindWebChat"]);
      expect(within(dialog).getByText("https://vault.example/mcp", { selector: "code" })).toBeTruthy();
      expect(within(dialog).getByRole("button", { name: en["mcp.copyConnector"] })).toBeTruthy();
      expect(within(dialog).getByText(en["mcp.oauthWaitTitle"])).toBeTruthy();

      await vi.advanceTimersByTimeAsync(3100);
      const grant = key({ id: "g1", viaOAuth: true, client: "chatgpt", label: "ChatGPT", hint: "" });
      listMcpKeys.mockResolvedValue(payload({ oauth, keys: [grant] }));
      await vi.advanceTimersByTimeAsync(3100);
      await waitFor(() => expect(within(dialog).getByText(en["mcp.connectedTitle"])).toBeTruthy());
      expect(dialogText()).toContain("ChatGPT signed in at");
    } finally {
      vi.useRealTimers();
    }
  });

  it("does not offer a grant as a key another client could reuse", async () => {
    const grant = key({ id: "g1", viaOAuth: true, client: "chatgpt", label: "ChatGPT", hint: "" });
    await renderCard(payload({ keys: [grant] }));
    await screen.findByRole("listitem");
    const dialog = await openClient("Cursor", en["mcp.kindEditor"]);
    expect(within(dialog).queryByRole("tab", { name: en["mcp.existingKey"] })).toBeNull();
  });

  it("closes on Escape", async () => {
    await renderCard(payload());
    await openClient("Cursor", en["mcp.kindEditor"]);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});

describe("the certificate of this address", () => {
  const selfIssued = { selfIssued: true, names: ["localhost", "127.0.0.1"], fingerprint: "ab:cd" };

  it("warns when the certificate does not cover this address", async () => {
    vi.stubGlobal("location", new URL("https://192.168.1.10:3443/settings"));
    await renderCard(payload({ keys: [key()], certificate: selfIssued }));
    addMcpCertificateName.mockResolvedValue({ ok: true });

    await waitFor(() =>
      expect(
        screen.getByText(en["mcp.certNotForThisAddress"].replace("{host}", "192.168.1.10"))
      ).toBeTruthy()
    );
    fireEvent.click(screen.getByRole("button", { name: en["mcp.certAddAddress"] }));
    const dialog = screen.getByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: en["mcp.certAddAddress"] }));

    await waitFor(() => expect(addMcpCertificateName).toHaveBeenCalledWith("192.168.1.10"));
    await waitFor(() => expect(pushed.map((p) => p.message)).toContain(en["mcp.certAdded"]));
    expect(listMcpKeys.mock.calls.length).toBeGreaterThan(1);
  });

  it("offers the download once the certificate names this address", async () => {
    vi.stubGlobal("location", new URL("https://192.168.1.10:3443/settings"));
    await renderCard(
      payload({
        keys: [key()],
        certificate: { ...selfIssued, names: ["localhost", "192.168.1.10"] },
      })
    );

    const download = await screen.findByRole("button", { name: en["mcp.certDownload"] });
    expect(download.querySelector(".glim-btn-glyph svg")).not.toBeNull();
    const saved: string[] = [];
    const click = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(function (this: HTMLAnchorElement) {
      saved.push(`${this.getAttribute("href")} ${this.hasAttribute("download")}`);
    });
    fireEvent.click(download);
    click.mockRestore();
    expect(saved).toEqual(["/api/mcp/certificate true"]);
    expect(cardText()).not.toContain(en["mcp.certNotForThisAddress"].replace("{host}", "192.168.1.10"));
    await openClient("Claude Code", en["mcp.kindTerminal"]);
    expect(dialogText()).toContain("NODE_EXTRA_CA_CERTS");
    expect(dialogText()).toContain(en["mcp.certPlaceholder"]);
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: en["common.close"] }));

    await openClient(en["mcp.otherClient"]);
    expect(screen.getByLabelText(en["mcp.certOtherTip"])).toBeTruthy();
  });

  it("words a certificate step the client's documentation leaves open as a condition", async () => {
    vi.stubGlobal("location", new URL("https://192.168.1.10:3443/settings"));
    await renderCard(payload({ certificate: { ...selfIssued, names: ["localhost", "192.168.1.10"] } }));

    await openClient("Zed", en["mcp.kindEditor"]);
    expect(dialogText()).toContain(en["mcp.certSystem"].replaceAll("{app}", "Zed"));
  });

  it("puts the certificate into Perplexity's command, since the Mac app reads no shell profile", async () => {
    vi.stubGlobal("location", new URL("https://192.168.1.10:3443/settings"));
    await renderCard(payload({ certificate: { ...selfIssued, names: ["localhost", "192.168.1.10"] } }));

    await openClient("Perplexity", en["mcp.kindDesktop"]);
    expect(within(screen.getByRole("dialog")).getByText(/^Server Name/).textContent).toContain(
      'Command: env "NODE_EXTRA_CA_CERTS='
    );
    expect(dialogText()).toContain(en["mcp.certPlaceholder"]);
  });

  it("tells a Dock app on macOS to take a variable from launchctl", async () => {
    vi.stubGlobal("location", new URL("https://192.168.1.10:3443/settings"));
    await renderCard(payload({ certificate: { ...selfIssued, names: ["localhost", "192.168.1.10"] } }));

    await openClient("AnythingLLM", en["mcp.kindLocalModels"]);
    const certTip = en["mcp.certEnvTip"].replaceAll("{var}", "NODE_EXTRA_CA_CERTS");
    expect(certTip).toContain("launchctl setenv NODE_EXTRA_CA_CERTS");
    expect(screen.getByLabelText(certTip)).toBeTruthy();
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: en["common.close"] }));

    await openClient("Goose", en["mcp.kindDesktop"]);
    const keyTip = en["mcp.keyEnvTip"].replaceAll("{app}", "Goose").replaceAll("{var}", "BOMBVAULT_MCP_KEY");
    expect(keyTip).toContain("launchctl setenv BOMBVAULT_MCP_KEY");
    expect(screen.getByLabelText(keyTip)).toBeTruthy();
  });

  it("matches an IPv6 address without its brackets", async () => {
    vi.stubGlobal("location", new URL("https://[fd00::10]:3443/settings"));
    await renderCard(payload({ keys: [key()], certificate: selfIssued }));
    addMcpCertificateName.mockResolvedValue({ ok: true });

    await waitFor(() =>
      expect(screen.getByText(en["mcp.certNotForThisAddress"].replace("{host}", "fd00::10"))).toBeTruthy()
    );
    fireEvent.click(screen.getByRole("button", { name: en["mcp.certAddAddress"] }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: en["mcp.certAddAddress"] }));
    await waitFor(() => expect(addMcpCertificateName).toHaveBeenCalledWith("fd00::10"));
    cleanup();

    await renderCard(payload({ keys: [key()], certificate: { ...selfIssued, names: ["localhost", "fd00::10"] } }));
    await waitFor(() => expect(screen.getByText(en["mcp.certDownload"])).toBeTruthy());
    await openClient("Claude Code", en["mcp.kindTerminal"]);
    expect(dialogText()).toContain("NODE_EXTRA_CA_CERTS");
  });

  it("does not offer a certificate name the server would refuse", async () => {
    vi.stubGlobal("location", new URL("https://backup.example.com/settings"));
    await renderCard(
      payload({ authEnabled: false, hostAllowsKeys: false, keys: [key()], certificate: selfIssued })
    );

    await waitFor(() =>
      expect(
        screen.getByText(en["mcp.certNotForThisAddress"].replace("{host}", "backup.example.com"))
      ).toBeTruthy()
    );
    expect(screen.queryByRole("button", { name: en["mcp.certAddAddress"] })).toBeNull();
  });

  it("leaves an operator's own certificate alone", async () => {
    vi.stubGlobal("location", new URL("https://192.168.1.10:3443/settings"));
    await renderCard(
      payload({ keys: [key()], certificate: { ...selfIssued, selfIssued: false } })
    );

    await waitFor(() =>
      expect(
        screen.getByText(en["mcp.certOwnNotForThisAddress"].replace("{host}", "192.168.1.10"))
      ).toBeTruthy()
    );
    expect(screen.queryByRole("button", { name: en["mcp.certAddAddress"] })).toBeNull();
  });

  it("puts the certificate warning right under the password warning", async () => {
    vi.stubGlobal("location", new URL("https://192.168.1.10:3443/settings"));
    await renderCard(
      payload({
        authEnabled: false,
        keys: [key({ unusable: "app-key-changed" })],
        revoked: [key({ id: "k0", revokedAt: 1_800_000_000, revokedReason: "config-restore" })],
        certificate: selfIssued,
      })
    );

    await waitFor(() => expect(screen.getByText(en["mcp.appKeyChanged"])).toBeTruthy());
    const order = [
      en["mcp.noPasswordWarning"],
      en["mcp.certNotForThisAddress"].replace("{host}", "192.168.1.10"),
      en["mcp.restoreRevokedNotice"],
      en["mcp.appKeyChanged"],
    ].map((text) => screen.getByText(text));
    for (let i = 1; i < order.length; i++) {
      expect(order[i - 1].compareDocumentPosition(order[i]) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    }
  });

  it("says nothing about certificates behind a proxy that ends TLS", async () => {
    vi.stubGlobal("location", new URL("https://bombvault.home.example.com/settings"));
    await renderCard(payload({ keys: [key()], certificate: null }));

    await waitFor(() => expect(screen.getByText(en["mcp.snippetsLabel"])).toBeTruthy());
    expect(screen.queryByRole("button", { name: en["mcp.certAddAddress"] })).toBeNull();
    await openClient("Claude Code", en["mcp.kindTerminal"]);
    expect(dialogText()).not.toContain("NODE_EXTRA_CA_CERTS");
  });

  it("says nothing about certificates over plain HTTP", async () => {
    vi.stubGlobal("location", new URL("http://tower:3443/settings"));
    await renderCard(payload({ keys: [key()], certificate: selfIssued }));

    await waitFor(() => expect(screen.getByText(en["mcp.snippetsLabel"])).toBeTruthy());
    expect(cardText()).not.toContain(en["mcp.certNotForThisAddress"].replace("{host}", "tower"));
    expect(screen.queryByText(en["mcp.certDownload"])).toBeNull();
  });
});

describe("a key list", () => {
  it("shows keys broken by an APP_KEY change", async () => {
    await renderCard(payload({ keys: [key({ unusable: "app-key-changed" })] }));

    await waitFor(() => expect(screen.getByText(en["mcp.appKeyChanged"])).toBeTruthy());
    expect(screen.getByText(en["mcp.keyUnusable"])).toBeTruthy();
    expect(screen.getByRole("button", { name: en["mcp.rotate"] })).toBeTruthy();
  });

  it("counts only the keys that still work as active", async () => {
    await renderCard(
      payload({ keys: [key({ id: "k1", unusable: "app-key-changed" }), key({ id: "k2", unusable: "app-key-changed" })] })
    );
    await waitFor(() => expect(screen.getByText(en["mcp.statusNoneWorks"])).toBeTruthy());
    expect(screen.queryByText(/active key/)).toBeNull();
  });

  it("labels the permission on a key row as it was labelled when the key was created", async () => {
    await renderCard(payload({ keys: [key()] }));
    await waitFor(() => expect(screen.getByRole("switch", { name: en["mcp.allowStart"] })).toBeTruthy());
  });

  it("auto-saves the permission toggle and reverts on failure", async () => {
    await renderCard(payload({ keys: [key()] }));
    updateMcpKey.mockResolvedValue({ ok: true, item: key({ canStartBackups: false }) });

    const toggle = () => screen.getByRole("switch", { name: en["mcp.allowStart"] });
    await waitFor(() => expect(toggle().getAttribute("aria-checked")).toBe("true"));
    fireEvent.click(toggle());

    await waitFor(() => expect(updateMcpKey).toHaveBeenCalledWith("k1", { canStartBackups: false }));
    await waitFor(() => expect(toggle().getAttribute("aria-checked")).toBe("false"));
    expect(pushed.map((p) => p.message)).toContain(en["mcp.permissionSaved"]);

    updateMcpKey.mockResolvedValue({ ok: false });
    fireEvent.click(toggle());
    await waitFor(() => expect(pushed.map((p) => p.message)).toContain(en["common.saveFailed"]));
    expect(toggle().getAttribute("aria-checked")).toBe("false");
    expect(toggle().parentElement?.className).toContain("glim-shake");
  });

  it("rotates and revokes through confirm dialogs", async () => {
    await renderCard(
      payload({ keys: [key()] }),
      payload({ keys: [key()] }),
      payload({ keys: [], revoked: [key({ revokedAt: 1_700_100_000, revokedReason: "user" })] })
    );
    rotateMcpKey.mockResolvedValue({ ok: true, key: "bvmcp_rotated9999", item: key() });
    revokeMcpKey.mockResolvedValue({ ok: true });

    fireEvent.click(screen.getByRole("button", { name: en["mcp.rotate"] }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: en["mcp.rotate"] }));
    await waitFor(() => expect(rotateMcpKey).toHaveBeenCalledWith("k1"));
    await waitFor(() => expect(screen.getByDisplayValue("bvmcp_rotated9999")).toBeTruthy());

    fireEvent.click(screen.getByRole("button", { name: en["mcp.revoke"] }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: en["mcp.revoke"] }));
    await waitFor(() => expect(revokeMcpKey).toHaveBeenCalledWith("k1"));
    await waitFor(() =>
      expect(screen.getByText(countText(en["mcp.revokedList"], "en", 1).replace("{n}", "1"))).toBeTruthy()
    );
  });

  it("purge is disabled for keys still named in history, and says why", async () => {
    await renderCard(
      payload({
        revoked: [
          key({ id: "r1", label: "old laptop", revokedAt: 1_700_100_000, revokedReason: "user", inUse: true }),
          key({ id: "r2", label: "old desktop", revokedAt: 1_700_100_000, revokedReason: "user" }),
        ],
      })
    );
    purgeMcpKey.mockResolvedValue({ ok: true });

    fireEvent.click(await screen.findByText(countText(en["mcp.revokedList"], "en", 2).replace("{n}", "2")));
    const rows = screen.getAllByRole("listitem");
    const inUse = rows.find((r) => r.textContent?.includes("old laptop"))!;
    const free = rows.find((r) => r.textContent?.includes("old desktop"))!;
    const blocked = within(inUse).getByRole("button", { name: en["common.delete"] });
    expect(blocked.hasAttribute("disabled")).toBe(true);
    // The reason rides on the button, yet outside the <button>, which takes no
    // hover or focus while disabled.
    const reason = within(inUse).getByLabelText(en["mcp.inUseTip"]);
    expect(blocked.contains(reason)).toBe(false);
    expect(blocked.parentElement?.contains(reason)).toBe(true);
    expect(within(free).queryByLabelText(en["mcp.inUseTip"])).toBeNull();

    fireEvent.click(within(free).getByRole("button", { name: en["common.delete"] }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: en["common.delete"] }));
    await waitFor(() => expect(purgeMcpKey).toHaveBeenCalledWith("r2"));
  });

  it("shows the restore notice and says in the dialog why no key can be added", async () => {
    const many = Array.from({ length: 10 }, (_, i) => key({ id: `k${i}`, label: `key ${i}` }));
    await renderCard(
      payload({
        keys: many,
        revoked: [key({ id: "r1", revokedAt: 1_800_000_000, revokedReason: "config-restore" })],
      })
    );

    await waitFor(() => expect(screen.getByText(en["mcp.restoreRevokedNotice"])).toBeTruthy());
    const dialog = await openClient("Cursor", en["mcp.kindEditor"]);
    fireEvent.click(within(dialog).getByRole("tab", { name: en["mcp.newKey"] }));
    const button = within(dialog).getByRole("button", { name: en["mcp.createKey"] });
    expect(button.hasAttribute("disabled")).toBe(true);
    const reason = within(dialog).getByLabelText(countText(en["mcp.limitReached"], "en", 10).replace("{n}", "10"));
    expect(button.parentElement?.contains(reason)).toBe(true);
  });

  it("takes the start budget and the last use from the server", async () => {
    await renderCard(
      payload({
        startsPerHour: 7,
        cooldownMinutes: 20,
        itemStartsPerDay: 3,
        keys: [key({ lastUsedAt: 1_700_050_000, lastUsedFrom: "" })],
      })
    );

    await waitFor(() => expect(cardText()).toContain(en["mcp.keyLastUsedNoAddr"].split("{when}")[0]));
    expect(cardText()).not.toContain(en["mcp.keyNeverUsed"]);

    await openClient("Cursor", en["mcp.kindEditor"]);
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("tab", { name: en["mcp.newKey"] }));
    const hint = en["mcp.allowStartHint"]
      .replace("{n}", "7")
      .replace("{minutes}", "20")
      .replace("{perDay}", "3");
    expect(screen.getByLabelText(hint)).toBeTruthy();
  });
});

describe("a key's tile", () => {
  function tileOf(label: string): HTMLElement {
    const tile = screen.getAllByRole("listitem").find((li) => li.textContent?.includes(label));
    if (!tile) throw new Error(`no tile for ${label}`);
    return tile;
  }

  it("sets each tile straight on the card, with its chips a step above it", async () => {
    await renderCard(payload({ keys: [key({ label: "laptop" })] }));
    await screen.findByText("laptop");

    const tile = tileOf("laptop");
    expect(tile.className).toContain("glim-tile");
    expect(tile.parentElement?.className).not.toMatch(/bg-/);
    expect(within(tile).getByText(en["mcp.canStart"]).closest(".glim-tile-raise")).not.toBeNull();
  });

  it("shows the mark of the client a key was made for, and the key glyph for none", async () => {
    await renderCard(
      payload({
        keys: [
          key({ id: "k1", label: "laptop", client: "cursor" }),
          key({ id: "k2", label: "nightly report" }),
          key({ id: "k3", label: "gone", client: "a-client-since-dropped" }),
        ],
      })
    );
    await screen.findByText("laptop");

    const mark = (label: string) => tileOf(label).querySelector(".glim-client-mark")!;
    const cursorPath = / d="([^"]+)"/.exec(CLIENT_MARKS.cursor.rest)![1];
    expect(mark("laptop").querySelector("path")?.getAttribute("d")).toBe(cursorPath);
    expect(mark("nightly report").innerHTML).not.toContain(cursorPath);
    expect(mark("nightly report").innerHTML).toBe(mark("gone").innerHTML);
    expect(mark("laptop").getAttribute("aria-hidden")).toBe("true");
  });

  it("shows the permission, the hint and today's calls", async () => {
    await renderCard(
      payload({
        keys: [
          key({ id: "k1", label: "laptop", callsToday: 3 }),
          key({ id: "k2", label: "desktop", canStartBackups: false, callsToday: 1, hint: "77aa" }),
        ],
      })
    );

    await waitFor(() => expect(tileOf("laptop")).toBeTruthy());
    const laptop = tileOf("laptop");
    expect(within(laptop).getByText(en["mcp.canStart"])).toBeTruthy();
    expect(within(laptop).getByText(countText(en["mcp.callsToday"], "en", 3))).toBeTruthy();
    const desktop = tileOf("desktop");
    expect(within(desktop).getByText(en["mcp.readOnly"])).toBeTruthy();
    expect(within(desktop).getByText(countText(en["mcp.callsToday"], "en", 1))).toBeTruthy();
    expect(desktop.textContent).toContain(en["mcp.keyHint"].replace("{hint}", "77aa"));
    expect(within(desktop).getByRole("button", { name: en["mcp.revoke"] })).toBeTruthy();
    expect(within(desktop).getByRole("button", { name: en["mcp.rotate"] })).toBeTruthy();
    expect(getMcpKeyActivity).not.toHaveBeenCalled();
  });

  it("opens the key's log with its runs, calls and refusals", async () => {
    await renderCard(payload({ keys: [key({ id: "k1", label: "laptop" })] }));
    getMcpKeyActivity.mockResolvedValue({
      ok: true,
      runs: [
        {
          id: "run1",
          targetId: "t1",
          kind: "backup",
          status: "cancelled",
          startedAt: 1_700_000_500,
          finishedAt: 1_700_000_600,
          snapshotId: "",
          bytes: 0,
          error: "",
          acknowledged: false,
          target: "plex",
          domain: "container",
        },
      ],
      events: [
        { at: 1_700_000_700, tool: "cancel_backup", outcome: "ok", runId: "run1" },
        { at: 1_700_000_650, tool: "start_backup", outcome: "cooldown", runId: "" },
        { at: 1_700_000_640, tool: "start_backup", outcome: "rate_limited", runId: "" },
        { at: 1_700_000_630, tool: "", outcome: "rate_limited", runId: "" },
        { at: 1_700_000_620, tool: "list_runs", outcome: "invalid_argument", runId: "" },
        { at: 1_700_000_610, tool: "cancel_backup", outcome: "too_late", runId: "" },
        { at: 1_700_000_600, tool: "start_backup", outcome: "domain_off", runId: "" },
      ],
    });

    fireEvent.click(within(await waitFor(() => tileOf("laptop"))).getByRole("button", { name: en["mcp.log"] }));

    await waitFor(() => expect(getMcpKeyActivity).toHaveBeenCalledWith("k1"));
    const tile = tileOf("laptop");
    await waitFor(() => expect(within(tile).getByText("plex")).toBeTruthy());
    expect(within(tile).getByText(en["run.statusCancelled"])).toBeTruthy();
    const links = within(tile).getAllByRole("link", { name: en["mcp.logShowRun"] });
    expect(links.map((a) => a.getAttribute("href"))).toEqual(["/dashboard?run=run1", "/dashboard?run=run1"]);
    for (const text of [
      en["mcp.outcomeOk"],
      en["mcp.outcomeCooldown"],
      en["mcp.outcomeStartLimit"],
      en["mcp.outcomeRateLimited"],
      en["mcp.outcomeInvalidArgument"],
      en["mcp.outcomeTooLate"],
      en["mcp.outcomeDomainOff"],
      en["mcp.logRequest"],
    ]) {
      expect(within(tile).getByText(text)).toBeTruthy();
    }
    expect(within(tile).getAllByText("cancel_backup")).toHaveLength(2);
    expect(tile.textContent).not.toContain(en["mcp.outcomeOther"].replace("{code}", "").trim());

    fireEvent.click(within(tile).getByRole("button", { name: en["mcp.log"] }));
    await waitFor(() => expect(within(tileOf("laptop")).queryByText("plex")).toBeNull());
  });

  it("says when a key has no log yet or it cannot be read", async () => {
    await renderCard(payload({ keys: [key({ id: "k1", label: "laptop" }), key({ id: "k2", label: "desktop" })] }));
    getMcpKeyActivity.mockImplementation((id: string) =>
      id === "k1" ? Promise.resolve({ ok: true, runs: [], events: [] }) : Promise.reject(new Error("offline"))
    );

    await waitFor(() => expect(tileOf("laptop")).toBeTruthy());
    fireEvent.click(within(tileOf("laptop")).getByRole("button", { name: en["mcp.log"] }));
    fireEvent.click(within(tileOf("desktop")).getByRole("button", { name: en["mcp.log"] }));

    await waitFor(() => expect(within(tileOf("laptop")).getByText(en["mcp.logEmpty"])).toBeTruthy());
    await waitFor(() => expect(within(tileOf("desktop")).getByText(en["mcp.logFailed"])).toBeTruthy());
  });

  it("does not claim a used key never called when its log is empty", async () => {
    await renderCard(payload({ keys: [key({ id: "k1", label: "laptop", lastUsedAt: Math.floor(Date.now() / 1000) - 7200 })] }));
    getMcpKeyActivity.mockResolvedValue({ ok: true, runs: [], events: [] });

    fireEvent.click(within(await waitFor(() => tileOf("laptop"))).getByRole("button", { name: en["mcp.log"] }));

    await waitFor(() => expect(within(tileOf("laptop")).getByText(en["mcp.logEmptyUsed"])).toBeTruthy());
    expect(within(tileOf("laptop")).queryByText(en["mcp.logEmpty"])).toBeNull();
  });

  it("announces whether the log is open and which panel it opened", async () => {
    await renderCard(payload({ keys: [key({ id: "k1", label: "laptop" })] }));
    getMcpKeyActivity.mockResolvedValue({ ok: true, runs: [], events: [] });

    const button = within(await waitFor(() => tileOf("laptop"))).getByRole("button", { name: en["mcp.log"] });
    expect(button.getAttribute("aria-expanded")).toBe("false");
    fireEvent.click(button);

    await waitFor(() => expect(button.getAttribute("aria-expanded")).toBe("true"));
    const panel = document.getElementById(button.getAttribute("aria-controls") ?? "");
    expect(panel).not.toBeNull();
    await waitFor(() => expect(within(panel as HTMLElement).getByText(en["mcp.logCalls"])).toBeTruthy());

    fireEvent.click(button);
    await waitFor(() => expect(button.getAttribute("aria-expanded")).toBe("false"));
  });

  it("gives a revoked key a log of its own", async () => {
    await renderCard(
      payload({ revoked: [key({ id: "r1", label: "old laptop", revokedAt: 1_700_100_000, revokedReason: "user" })] })
    );
    getMcpKeyActivity.mockResolvedValue({ ok: true, runs: [], events: [] });

    fireEvent.click(await screen.findByText(countText(en["mcp.revokedList"], "en", 1)));
    fireEvent.click(within(tileOf("old laptop")).getByRole("button", { name: en["mcp.log"] }));
    await waitFor(() => expect(getMcpKeyActivity).toHaveBeenCalledWith("r1"));
  });
});

describe("a refusal from the server", () => {
  async function createWith(answer: Record<string, unknown>) {
    await renderCard(payload());
    createMcpKey.mockResolvedValue(answer);
    const dialog = await openClient(en["mcp.otherClient"]);
    fireEvent.change(within(dialog).getByLabelText(en["mcp.labelLabel"]), { target: { value: "laptop" } });
    fireEvent.click(within(dialog).getByRole("button", { name: en["mcp.createKey"] }));
    await waitFor(() => expect(createMcpKey).toHaveBeenCalled());
  }

  it("maps server codes to translated toasts", async () => {
    await createWith({ ok: false, code: "mcp-key-label-taken", error: "label taken" });
    await waitFor(() => expect(pushed.map((p) => p.message)).toContain(en["mcp.labelTaken"]));
    cleanup();

    await createWith({ ok: false, code: "mcp-key-limit", error: "limit" });
    await waitFor(() =>
      expect(pushed.map((p) => p.message)).toContain(
        countText(en["mcp.limitReached"], "en", 10).replace("{n}", "10")
      )
    );
    cleanup();

    vi.stubGlobal("location", new URL("https://backup.example.com/settings"));
    await createWith({ ok: false, code: "mcp-key-needs-password", error: "needs password" });
    await waitFor(() =>
      expect(pushed.map((p) => p.message)).toContain(
        en["mcp.needsPasswordForHost"].replace("{host}", "backup.example.com")
      )
    );
  });

  it("refreshes the list when the server wants a login password first", async () => {
    vi.stubGlobal("location", new URL("https://backup.example.com/settings"));
    await createWith({ ok: false, code: "mcp-key-needs-password", error: "needs password" });

    await waitFor(() => expect(listMcpKeys.mock.calls.length).toBeGreaterThan(1));
  });

  it("keeps a refused new name open and shakes it", async () => {
    await renderCard(payload({ keys: [key()] }));
    updateMcpKey.mockResolvedValue({ ok: false, code: "mcp-key-label-taken", error: "taken" });

    fireEvent.click(screen.getByRole("button", { name: en["common.edit"] }));
    const input = screen.getByLabelText(en["mcp.labelLabel"]);
    fireEvent.change(input, { target: { value: "desk" } });
    fireEvent.keyDown(input, { key: "Enter" });

    await waitFor(() => expect(pushed.map((p) => p.message)).toContain(en["mcp.labelTaken"]));
    expect(screen.getByDisplayValue("desk").className).toContain("glim-shake");
    expect(screen.getByRole("button", { name: en["mcp.revoke"] }).className).not.toContain("glim-shake");
  });

  it("refreshes the list when a key is already gone", async () => {
    await renderCard(payload({ keys: [key()] }), payload());
    revokeMcpKey.mockResolvedValue({ ok: false, code: "mcp-key-not-found", error: "gone" });

    fireEvent.click(screen.getByRole("button", { name: en["mcp.revoke"] }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: en["mcp.revoke"] }));

    await waitFor(() => expect(pushed.map((p) => p.message)).toContain(en["mcp.notFound"]));
    expect(listMcpKeys.mock.calls.length).toBeGreaterThan(1);
  });

  it("names the certificate refusals", async () => {
    vi.stubGlobal("location", new URL("https://192.168.1.10:3443/settings"));
    await renderCard(
      payload({ keys: [key()], certificate: { selfIssued: true, names: ["localhost"], fingerprint: "ab" } })
    );
    addMcpCertificateName.mockResolvedValue({ ok: false, code: "cert-write-failed", error: "disk" });

    fireEvent.click(screen.getByRole("button", { name: en["mcp.certAddAddress"] }));
    fireEvent.click(
      within(screen.getByRole("dialog")).getByRole("button", { name: en["mcp.certAddAddress"] })
    );
    await waitFor(() => expect(pushed.map((p) => p.message)).toContain(en["mcp.certWriteFailed"]));
  });

  it("says when the list cannot be loaded", async () => {
    listMcpKeys.mockReset();
    listMcpKeys.mockRejectedValue(new Error("offline"));
    render(<McpServerCard hueIndex={0} />, { wrapper: MemoryRouter });

    await waitFor(() => expect(screen.getByText(en["mcp.loadFailed"])).toBeTruthy());
    expect(screen.queryByRole("button", { name: en["mcp.otherClient"] })).toBeNull();
  });
});

describe("sign-in through OAuth on the card", () => {
  const on = { enabled: true, issuer: "https://vault.example", active: true, grantLimit: 10, connectorPath: "/mcp" };

  it("asks for the public address before it switches on", async () => {
    await renderCard(payload());
    const toggle = await screen.findByRole("switch", { name: en["mcp.oauthToggle"] });
    expect(toggle.getAttribute("aria-checked")).toBe("false");
    fireEvent.click(toggle);
    expect(setMcpOAuth).not.toHaveBeenCalled();

    const field = screen.getByLabelText(en["mcp.oauthAddressLabel"]);
    fireEvent.change(field, { target: { value: "https://vault.example" } });
    setMcpOAuth.mockResolvedValue({ ok: true, oauth: on });
    fireEvent.click(screen.getByRole("button", { name: en["mcp.oauthSave"] }));

    await waitFor(() => expect(setMcpOAuth).toHaveBeenCalledWith(true, "https://vault.example"));
    const connector = (await screen.findByLabelText(en["mcp.oauthConnectorLabel"])) as HTMLInputElement;
    expect(connector.value).toBe("https://vault.example/mcp");
    expect(pushed.at(-1)).toEqual({ message: en["mcp.oauthSaved"], severity: "success" });
  });

  it("names the rule a refused address broke", async () => {
    await renderCard(payload());
    fireEvent.click(await screen.findByRole("switch", { name: en["mcp.oauthToggle"] }));
    fireEvent.change(screen.getByLabelText(en["mcp.oauthAddressLabel"]), { target: { value: "http://vault.example" } });
    setMcpOAuth.mockResolvedValue({ ok: false, code: "mcp-oauth-issuer-invalid", error: "x" });
    fireEvent.click(screen.getByRole("button", { name: en["mcp.oauthSave"] }));
    await waitFor(() => expect(pushed.at(-1)).toEqual({ message: en["mcp.oauthAddressInvalid"], severity: "fail" }));
  });

  it("cannot be switched on without a login password", async () => {
    await renderCard(payload({ authEnabled: false }));
    const toggle = await screen.findByRole("switch", { name: en["mcp.oauthToggle"] });
    expect((toggle as HTMLButtonElement).disabled).toBe(true);
    expect(screen.getByText(en["mcp.oauthNeedsPassword"])).toBeTruthy();
  });

  it("switches off with the address kept", async () => {
    await renderCard(payload({ oauth: on }));
    setMcpOAuth.mockResolvedValue({ ok: true, oauth: { ...on, enabled: false, active: false } });
    fireEvent.click(await screen.findByRole("switch", { name: en["mcp.oauthToggle"] }));
    await waitFor(() => expect(setMcpOAuth).toHaveBeenCalledWith(false, "https://vault.example"));
  });

  it("reads the tiles again after switching off, which ends every grant", async () => {
    const grant = key({ id: "g1", viaOAuth: true, client: "chatgpt", label: "ChatGPT", hint: "" });
    const ended = { ...grant, revokedAt: 1_700_000_100, revokedReason: "oauth-off" as const };
    await renderCard(payload({ oauth: on, keys: [grant] }));
    await screen.findByRole("listitem");
    listMcpKeys.mockResolvedValue(payload({ oauth: { ...on, enabled: false, active: false }, keys: [], revoked: [ended] }));
    setMcpOAuth.mockResolvedValue({ ok: true, oauth: { ...on, enabled: false, active: false } });
    fireEvent.click(screen.getByRole("switch", { name: en["mcp.oauthToggle"] }));
    fireEvent.click(await screen.findByRole("button", { name: countText(en["mcp.revokedList"], "en", 1) }));
    expect(cardText()).toContain(en["mcp.revokedOAuthOff"].split("{date}")[0]);
    expect(screen.queryByRole("button", { name: en["mcp.revoke"] })).toBeNull();
  });

  it("says the endpoint is on while only sign-in can reach it", async () => {
    await renderCard(payload({ oauth: on }));
    await waitFor(() => expect(screen.getByText(en["mcp.statusOAuthOnly"])).toBeTruthy());
  });

  it("shows a grant as a tile with its log, Revoke and the start switch but nothing to replace", async () => {
    const grant = key({ id: "g1", viaOAuth: true, client: "chatgpt", label: "ChatGPT", hint: "", canStartBackups: false });
    await renderCard(payload({ oauth: on, keys: [grant] }));
    const tile = await screen.findByRole("listitem");
    expect(within(tile).getByText("ChatGPT")).toBeTruthy();
    expect(tile.textContent).toContain(en["mcp.grantCreated"].split("{date}")[0]);
    expect(within(tile).getByRole("button", { name: en["mcp.revoke"] })).toBeTruthy();
    expect(within(tile).getByRole("button", { name: en["mcp.log"] })).toBeTruthy();
    expect(within(tile).queryByRole("button", { name: en["mcp.rotate"] })).toBeNull();
    const toggle = within(tile).getByRole("switch", { name: en["mcp.allowStart"] });
    expect(toggle.getAttribute("aria-checked")).toBe("false");
    expect(tile.querySelector(".glim-client-mark svg")).not.toBeNull();
  });

  it("says why a grant was revoked", async () => {
    const reused = key({ id: "g1", viaOAuth: true, label: "ChatGPT", revokedAt: 1_700_000_100, revokedReason: "refresh-reuse" });
    await renderCard(payload({ revoked: [reused] }));
    fireEvent.click(await screen.findByRole("button", { name: countText(en["mcp.revokedList"], "en", 1) }));
    expect(cardText()).toContain(en["mcp.revokedReuse"].split("{date}")[0]);
  });

  it("counts only keys against the key limit", async () => {
    const grants = Array.from({ length: 10 }, (_, i) => key({ id: `g${i}`, viaOAuth: true, label: `Client ${i}`, hint: "" }));
    await renderCard(payload({ oauth: on, keys: grants }));
    const dialog = await openClient("Cursor", en["mcp.kindEditor"]);
    const create = within(dialog).getByRole("button", { name: en["mcp.createKey"] }) as HTMLButtonElement;
    expect(create.disabled).toBe(false);
  });
});

// jsdom lays nothing out, so these pin the classes the phone layout rests on.
describe("the MCP card at phone width", () => {
  it("lets a key's name wrap instead of cutting it beside Edit", async () => {
    await renderCard(payload({ keys: [key({ label: "Claude Code on the workstation" })] }));
    const name = await screen.findByText("Claude Code on the workstation");
    expect(name.className).toContain("max-md:whitespace-normal");
    expect(name.parentElement!.className).toContain("max-md:flex-wrap");
  });

  it("keeps the certificate button inside its warning", async () => {
    vi.stubGlobal("location", new URL("https://192.168.1.10:3443/settings"));
    await renderCard(
      payload({ keys: [key()], certificate: { selfIssued: true, names: ["localhost"], fingerprint: "ab:cd" } })
    );
    expect((await screen.findByRole("button", { name: en["mcp.certAddAddress"] })).className).toContain(
      "glim-btn-wrap"
    );
  });
});

describe("a confirmation on a phone", () => {
  // The sheet's confirm button stays live while the call runs, so a double tap
  // reaches the card twice.
  it("runs a double-tapped rotate, revoke or purge once", async () => {
    desktop = false;
    const answer = payload({
      keys: [key()],
      revoked: [key({ id: "r2", label: "old desktop", revokedAt: 1_700_100_000, revokedReason: "user" })],
    });
    await renderCard(answer);
    rotateMcpKey.mockResolvedValue({ ok: true, key: "bvmcp_rotated9999", item: key() });
    revokeMcpKey.mockResolvedValue({ ok: true });
    purgeMcpKey.mockResolvedValue({ ok: true });

    const doubleTap = async (action: string) => {
      const confirm = within(screen.getByRole("dialog")).getByRole("button", { name: action });
      fireEvent.click(confirm);
      fireEvent.click(confirm);
      await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    };

    fireEvent.click(await screen.findByRole("button", { name: en["mcp.rotate"] }));
    await doubleTap(en["mcp.rotate"]);
    expect(rotateMcpKey).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole("button", { name: en["mcp.revoke"] }));
    await doubleTap(en["mcp.revoke"]);
    expect(revokeMcpKey).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByText(countText(en["mcp.revokedList"], "en", 1).replace("{n}", "1")));
    fireEvent.click(screen.getByRole("button", { name: en["common.delete"] }));
    await doubleTap(en["common.delete"]);
    expect(purgeMcpKey).toHaveBeenCalledTimes(1);
  });
});
