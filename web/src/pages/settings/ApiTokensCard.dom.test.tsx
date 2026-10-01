// @vitest-environment jsdom
// A token the card hands out is gone from the server once the panel closes, so
// the cases worth pinning are the ones where it must refuse to mint, the one
// place the token shows, and the log naming routes rather than MCP tools.
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, useLocation } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { en } from "../../lib/i18n";
import type { ApiTokensResponse, McpKeyView } from "../../lib/api";
import { LOGIN_PASSWORD_FIELD } from "./shared";

const listApiTokens = vi.fn();
const createApiToken = vi.fn();
const getApiTokenActivity = vi.fn();

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    listApiTokens: () => listApiTokens(),
    createApiToken: (...a: unknown[]) => createApiToken(...a),
    getApiTokenActivity: (...a: unknown[]) => getApiTokenActivity(...a),
  };
});

vi.mock("../../lib/toast", () => ({
  useToast: () => ({ push: () => {} }),
}));

const { ApiTokensCard } = await import("./ApiTokensCard");

function token(over: Partial<McpKeyView> = {}): McpKeyView {
  return {
    id: "t1",
    viaOAuth: false,
    label: "Uptime Kuma",
    hint: "x9Qa",
    canStartBackups: false,
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

function payload(over: Partial<ApiTokensResponse> = {}): ApiTokensResponse {
  return {
    ok: true,
    basePath: "/api/v1/",
    openapiPath: "/api/v1/openapi.json",
    limit: 10,
    authEnabled: true,
    hostAllowsKeys: true,
    startsPerHour: 12,
    cooldownMinutes: 15,
    itemStartsPerDay: 4,
    tokens: [],
    revoked: [],
    ...over,
  };
}

async function renderCard(answer: ApiTokensResponse) {
  listApiTokens.mockReset();
  listApiTokens.mockResolvedValue(answer);
  render(<ApiTokensCard hueIndex={0} />, { wrapper: MemoryRouter });
  await waitFor(() => expect(listApiTokens).toHaveBeenCalled());
}

beforeEach(() => {
  createApiToken.mockReset();
  getApiTokenActivity.mockReset();
});

afterEach(cleanup);

describe("ApiTokensCard", () => {
  it("creates a read-only token unless asked for more and shows it once", async () => {
    await renderCard(payload());
    createApiToken.mockResolvedValue({ ok: true, key: "bvapi_secret", item: token({ id: "t2", hint: "cret" }) });
    listApiTokens.mockResolvedValue(payload({ tokens: [token({ id: "t2", hint: "cret" })] }));

    fireEvent.change(await screen.findByLabelText(en["api.labelLabel"]), { target: { value: "Uptime Kuma" } });
    fireEvent.click(screen.getByRole("button", { name: new RegExp(en["api.createToken"]) }));

    await waitFor(() => expect(createApiToken).toHaveBeenCalledWith("Uptime Kuma", false));
    expect(await screen.findByDisplayValue("bvapi_secret")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: new RegExp(en["mcp.dismissKey"]) }));
    await waitFor(() => expect(screen.queryByDisplayValue("bvapi_secret")).toBeNull());
  });

  it("refuses to mint from a public-looking name without a password", async () => {
    await renderCard(payload({ authEnabled: false, hostAllowsKeys: false }));
    await screen.findByText(en["api.noPasswordWarning"]);
    expect(screen.queryByRole("button", { name: new RegExp(en["api.createToken"]) })).toBeNull();
  });

  it("leads Set password to the field on the Security page", async () => {
    listApiTokens.mockReset();
    listApiTokens.mockResolvedValue(payload({ authEnabled: false }));
    function Where() {
      const at = useLocation();
      return <output data-testid="where">{at.pathname + at.hash}</output>;
    }
    render(
      <MemoryRouter initialEntries={["/settings/integrations"]}>
        <ApiTokensCard hueIndex={0} />
        <Where />
      </MemoryRouter>
    );
    fireEvent.click(await screen.findByRole("button", { name: en["mcp.setPassword"] }));
    expect(screen.getByTestId("where").textContent).toBe(`/settings/security#${LOGIN_PASSWORD_FIELD}`);
  });

  it("names the route a logged call reached", async () => {
    await renderCard(payload({ tokens: [token({ lastUsedAt: 1_700_000_100 })] }));
    getApiTokenActivity.mockResolvedValue({
      ok: true,
      runs: [],
      events: [
        { at: 1_700_000_100, tool: "start_domain_backup", outcome: "not_permitted", runId: "" },
      ],
    });
    fireEvent.click(await screen.findByRole("button", { name: new RegExp(en["mcp.log"]) }));
    expect(await screen.findByText("POST /api/v1/backups")).toBeTruthy();
    expect(screen.getByText(en["api.outcomeNotPermitted"])).toBeTruthy();
  });
});
