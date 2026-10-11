// @vitest-environment jsdom
// The widget card on the Integrations page: the token, the plugin for the
// Unraid dashboard and how to set it up, each as a row of its own.
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { I18nProvider, en, useT } from "../../lib/i18n";

const generateWidgetToken = vi.fn();
const getDashboardPlugin = vi.fn();
const installDashboardPlugin = vi.fn();

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    generateWidgetToken: () => generateWidgetToken(),
    getDashboardPlugin: () => getDashboardPlugin(),
    installDashboardPlugin: () => installDashboardPlugin(),
  };
});

vi.mock("../../lib/toast", () => ({ useToast: () => ({ push: () => {} }) }));

const { DashboardWidgetCard } = await import("./DashboardWidgetCard");

function Harness({ tokenSet = false }: { tokenSet?: boolean }) {
  const { t } = useT();
  return <DashboardWidgetCard t={t} tokenSet={tokenSet} onTokenSet={() => {}} />;
}

async function renderCard(tokenSet = false) {
  await act(async () => {
    render(
      <MemoryRouter>
        <I18nProvider>
          <Harness tokenSet={tokenSet} />
        </I18nProvider>
      </MemoryRouter>
    );
  });
}

const row = (label: string) => document.querySelector<HTMLElement>(`[data-search-row="${label}"]`)!;

beforeEach(() => {
  getDashboardPlugin.mockResolvedValue({ ok: true, sshConfigured: true, installed: false });
  installDashboardPlugin.mockResolvedValue({ ok: true });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("the widget token", () => {
  it("says there is none and shows the URL once one is generated", async () => {
    generateWidgetToken.mockResolvedValue({ ok: true, token: "bvw_1234" });
    await renderCard();
    const token = row(en["settings.widgetToken"]);
    expect(within(token).getByText(en["settings.widgetNoToken"])).toBeTruthy();
    fireEvent.click(within(token).getByRole("button", { name: en["settings.widgetGenerate"] }));
    expect(await screen.findByText(/\/widget\?token=bvw_1234$/)).toBeTruthy();
  });

  it("offers to regenerate or disable a stored token", async () => {
    await renderCard(true);
    const token = row(en["settings.widgetToken"]);
    expect(within(token).getByRole("button", { name: en["settings.widgetRegenerate"] })).toBeTruthy();
    expect(within(token).getByRole("button", { name: en["settings.widgetDisable"] })).toBeTruthy();
  });
});

describe("the Unraid dashboard plugin", () => {
  it("asks before it installs the plugin", async () => {
    await renderCard();
    const plugin = row(en["settings.dashTile"]);
    await waitFor(() => within(plugin).getByText(en["settings.dashTileNotInstalled"]));
    fireEvent.click(within(plugin).getByRole("button", { name: en["settings.dashTileInstall"] }));
    const ask = await screen.findByRole("dialog");
    expect(ask.textContent).toContain(en["settings.dashTileConfirm"]);
    fireEvent.click(within(ask).getByRole("button", { name: en["settings.dashTileInstall"] }));
    await waitFor(() => expect(installDashboardPlugin).toHaveBeenCalledTimes(1));
  });

  it("gives the address to install it by hand without the host SSH connection", async () => {
    getDashboardPlugin.mockResolvedValue({ ok: true, sshConfigured: false });
    await renderCard();
    expect(await screen.findByText(en["settings.dashTileNoSshShort"])).toBeTruthy();
    expect(screen.getByText(/bombvaultwidget\.plg$/)).toBeTruthy();
  });

  it("names how to set the widget up", async () => {
    await renderCard();
    expect(screen.getByText(en["settings.widgetStep1"])).toBeTruthy();
    expect(screen.getByRole("link", { name: en["settings.tab.apps"] }).getAttribute("href")).toBe("/settings/apps");
  });
});
