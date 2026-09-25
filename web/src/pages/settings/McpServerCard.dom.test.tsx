// @vitest-environment jsdom
// The card is the only place a key can be minted, and a key it hands out is
// gone from the server the moment the panel closes. So the cases that matter
// are the ones where it must refuse (a public host name without a password, a
// certificate that does not cover this address) and the ones where a wrong
// sentence would send the operator to the wrong fix.
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { countText, en } from "../../lib/i18n";
import type { McpKeyView, McpKeysResponse } from "../../lib/api";

const listMcpKeys = vi.fn();
const createMcpKey = vi.fn();
const updateMcpKey = vi.fn();
const rotateMcpKey = vi.fn();
const revokeMcpKey = vi.fn();
const purgeMcpKey = vi.fn();
const addMcpCertificateName = vi.fn();

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
  };
});

const pushed: { message: string; severity?: string }[] = [];
vi.mock("../../lib/toast", () => ({
  useToast: () => ({ push: (message: string, severity?: string) => pushed.push({ message, severity }) }),
}));

const { McpServerCard } = await import("./McpServerCard");

function key(over: Partial<McpKeyView> = {}): McpKeyView {
  return {
    id: "k1",
    label: "office laptop",
    hint: "f3a9",
    canStartBackups: true,
    createdAt: 1_700_000_000,
    rotatedAt: 0,
    lastUsedAt: 0,
    lastUsedFrom: "",
    revokedAt: 0,
    revokedReason: "",
    inUse: false,
    unusable: "",
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
    ...over,
  };
}

/** Renders the card and waits for the first load to land. */
async function renderCard(...answers: McpKeysResponse[]) {
  listMcpKeys.mockReset();
  for (const a of answers) listMcpKeys.mockResolvedValueOnce(a);
  listMcpKeys.mockResolvedValue(answers[answers.length - 1]);
  const view = render(<McpServerCard hueIndex={0} />);
  await waitFor(() => expect(listMcpKeys).toHaveBeenCalled());
  return view;
}

function cardText(): string {
  return document.body.textContent ?? "";
}

beforeEach(() => {
  pushed.length = 0;
  for (const m of [createMcpKey, updateMcpKey, rotateMcpKey, revokeMcpKey, purgeMcpKey, addMcpCertificateName]) {
    m.mockReset();
  }
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("the MCP card without a key", () => {
  it("shows off state and the open-interface warning", async () => {
    await renderCard(payload({ authEnabled: false }));

    await waitFor(() => expect(screen.getByText(en["mcp.statusOff"])).toBeTruthy());
    expect(screen.getByText(en["mcp.noPasswordWarning"])).toBeTruthy();
    expect(screen.getByRole("button", { name: en["mcp.setPassword"] })).toBeTruthy();
    expect(screen.getByRole("button", { name: en["mcp.newKey"] })).toBeTruthy();
    expect(screen.queryByText(en["mcp.snippetsLabel"])).toBeNull();
  });

  it("hides key creation for a public host without a password", async () => {
    vi.stubGlobal("location", new URL("https://backup.example.com/settings"));
    await renderCard(payload({ authEnabled: false, hostAllowsKeys: false, keys: [key()] }));

    await waitFor(() =>
      expect(
        screen.getByText(en["mcp.needsPasswordForHost"].replace("{host}", "backup.example.com"))
      ).toBeTruthy()
    );
    expect(screen.queryByRole("button", { name: en["mcp.newKey"] })).toBeNull();
    expect(screen.queryByRole("button", { name: en["mcp.rotate"] })).toBeNull();
  });

  it("offers key creation once a login password is set on this page", async () => {
    vi.stubGlobal("location", new URL("https://backup.example.com/settings"));
    listMcpKeys.mockReset();
    listMcpKeys.mockResolvedValue(payload({ authEnabled: false, hostAllowsKeys: false }));
    const view = render(<McpServerCard hueIndex={0} passwordSet={false} />);
    await waitFor(() => expect(screen.getByText(en["mcp.noPasswordWarning"])).toBeTruthy());

    listMcpKeys.mockResolvedValue(payload());
    view.rerender(<McpServerCard hueIndex={0} passwordSet />);

    await waitFor(() => expect(screen.getByRole("button", { name: en["mcp.newKey"] })).toBeTruthy());
    expect(screen.queryByText(en["mcp.noPasswordWarning"])).toBeNull();
  });
});

describe("minting a key", () => {
  it("creates a key and shows it exactly once", async () => {
    await renderCard(payload(), payload({ keys: [key({ label: "laptop" })] }));
    createMcpKey.mockResolvedValue({ ok: true, key: "bvmcp_abcdef123456", item: key({ label: "laptop" }) });

    fireEvent.click(screen.getByRole("button", { name: en["mcp.newKey"] }));
    fireEvent.change(screen.getByLabelText(en["mcp.labelLabel"]), { target: { value: "laptop" } });
    fireEvent.click(screen.getByRole("button", { name: en["mcp.createKey"] }));

    await waitFor(() => expect(createMcpKey).toHaveBeenCalledWith("laptop", true));
    await waitFor(() => expect(screen.getByText(en["mcp.newKeyTitle"])).toBeTruthy());
    expect(screen.getByDisplayValue("bvmcp_abcdef123456")).toBeTruthy();
    expect(screen.getByRole("button", { name: en["mcp.copyKey"] })).toBeTruthy();
    expect(cardText()).toContain("bvmcp_abcdef123456");

    fireEvent.click(screen.getByRole("button", { name: en["mcp.dismissKey"] }));
    await waitFor(() => expect(screen.queryByText(en["mcp.newKeyTitle"])).toBeNull());
    expect(cardText()).not.toContain("bvmcp_abcdef123456");
    expect(cardText()).toContain("<your key>");
  });

  it("keeps a new key on screen when the list cannot be reloaded", async () => {
    await renderCard(payload());
    listMcpKeys.mockRejectedValueOnce(new Error("503"));
    createMcpKey.mockResolvedValue({ ok: true, key: "bvmcp_abcdef123456", item: key({ label: "laptop" }) });

    fireEvent.click(screen.getByRole("button", { name: en["mcp.newKey"] }));
    fireEvent.change(screen.getByLabelText(en["mcp.labelLabel"]), { target: { value: "laptop" } });
    fireEvent.click(screen.getByRole("button", { name: en["mcp.createKey"] }));

    await waitFor(() => expect(screen.getByText(en["mcp.loadFailed"])).toBeTruthy());
    expect(screen.getByDisplayValue("bvmcp_abcdef123456")).toBeTruthy();
    expect(screen.getByRole("button", { name: en["mcp.copyKey"] })).toBeTruthy();
  });

  it("takes a new key off screen once the list shows it revoked", async () => {
    await renderCard(
      payload({ keys: [key()] }),
      payload({ keys: [key({ hint: "9999", rotatedAt: 1_700_000_100 })] }),
      payload({ revoked: [key({ hint: "9999", revokedAt: 1_700_000_200, revokedReason: "user" })] })
    );
    rotateMcpKey.mockResolvedValue({ ok: true, key: "bvmcp_rotated9999", item: key({ hint: "9999" }) });
    revokeMcpKey.mockResolvedValue({ ok: true });

    fireEvent.click(screen.getByRole("button", { name: en["mcp.rotate"] }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: en["mcp.rotate"] }));
    await waitFor(() => expect(screen.getByDisplayValue("bvmcp_rotated9999")).toBeTruthy());

    fireEvent.click(screen.getByRole("button", { name: en["mcp.revoke"] }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: en["mcp.revoke"] }));

    await waitFor(() => expect(revokeMcpKey).toHaveBeenCalledWith("k1"));
    await waitFor(() => expect(screen.queryByText(en["mcp.newKeyTitle"])).toBeNull());
    expect(cardText()).not.toContain("bvmcp_rotated9999");
  });

  it("keeps the connect section while a key is active", async () => {
    await renderCard(payload({ keys: [key()] }));

    await waitFor(() => expect(screen.getByText(en["mcp.snippetsLabel"])).toBeTruthy());
    const clients = screen.getAllByRole("tab").map((el) => el.textContent);
    expect(clients).toEqual(["Claude Code", "Claude Desktop", en["mcp.snippetOther"]]);
    expect(cardText()).toContain("<your key>");
    expect(screen.getByText(en["mcp.snippetShellHistory"])).toBeTruthy();

    fireEvent.click(screen.getByRole("tab", { name: "Claude Desktop" }));
    await waitFor(() => expect(cardText()).toContain("mcp-remote"));
    expect(screen.queryByText(en["mcp.snippetShellHistory"])).toBeNull();
  });

  it("shows the endpoint of this address", async () => {
    vi.stubGlobal("location", new URL("https://tower.local:3443/settings"));
    await renderCard(payload({ keys: [key()] }));

    await waitFor(() => expect(cardText()).toContain("https://tower.local:3443/mcp"));
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

    await waitFor(() => expect(screen.getByText(en["mcp.certDownload"])).toBeTruthy());
    expect(screen.getByText(en["mcp.certDownload"]).getAttribute("href")).toBe("/api/mcp/certificate");
    expect(cardText()).not.toContain(en["mcp.certNotForThisAddress"].replace("{host}", "192.168.1.10"));
    expect(cardText()).toContain("NODE_EXTRA_CA_CERTS");

    fireEvent.click(screen.getByRole("tab", { name: en["mcp.snippetOther"] }));
    await waitFor(() => expect(screen.getByText(en["mcp.snippetOtherCert"])).toBeTruthy());
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
    expect(cardText()).toContain("NODE_EXTRA_CA_CERTS");
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
    expect(cardText()).not.toContain("NODE_EXTRA_CA_CERTS");
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

  it("purge is disabled for keys still named in history", async () => {
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
    expect(within(inUse).getByRole("button", { name: en["common.delete"] }).hasAttribute("disabled")).toBe(true);

    fireEvent.click(within(free).getByRole("button", { name: en["common.delete"] }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: en["common.delete"] }));
    await waitFor(() => expect(purgeMcpKey).toHaveBeenCalledWith("r2"));
  });

  it("shows the restore notice and the limit line", async () => {
    const many = Array.from({ length: 10 }, (_, i) => key({ id: `k${i}`, label: `key ${i}` }));
    await renderCard(
      payload({
        keys: many,
        revoked: [key({ id: "r1", revokedAt: 1_800_000_000, revokedReason: "config-restore" })],
      })
    );

    await waitFor(() => expect(screen.getByText(en["mcp.restoreRevokedNotice"])).toBeTruthy());
    expect(screen.getByText(countText(en["mcp.limitReached"], "en", 10).replace("{n}", "10"))).toBeTruthy();
    expect(screen.queryByRole("button", { name: en["mcp.newKey"] })).toBeNull();
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

    fireEvent.click(screen.getByRole("button", { name: en["mcp.newKey"] }));
    const hint = en["mcp.allowStartHint"]
      .replace("{n}", "7")
      .replace("{minutes}", "20")
      .replace("{perDay}", "3");
    expect(screen.getByLabelText(hint)).toBeTruthy();
  });
});

describe("a refusal from the server", () => {
  async function createWith(answer: Record<string, unknown>) {
    await renderCard(payload());
    createMcpKey.mockResolvedValue(answer);
    fireEvent.click(screen.getByRole("button", { name: en["mcp.newKey"] }));
    fireEvent.change(screen.getByLabelText(en["mcp.labelLabel"]), { target: { value: "laptop" } });
    fireEvent.click(screen.getByRole("button", { name: en["mcp.createKey"] }));
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
    render(<McpServerCard hueIndex={0} />);

    await waitFor(() => expect(screen.getByText(en["mcp.loadFailed"])).toBeTruthy());
    expect(screen.queryByRole("button", { name: en["mcp.newKey"] })).toBeNull();
  });
});
