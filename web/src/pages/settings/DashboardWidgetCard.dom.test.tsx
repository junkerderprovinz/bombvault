// @vitest-environment jsdom
// The widget card on the Integrations page: the token, the tile for the
// Unraid dashboard and how to set the widget up.
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { I18nProvider, en, useT } from "../../lib/i18n";

const generateWidgetToken = vi.fn();

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    generateWidgetToken: () => generateWidgetToken(),
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

describe("the Unraid dashboard", () => {
  it("leads to TollRoll and installs nothing itself", async () => {
    await renderCard();
    const tile = row(en["settings.dashTile"]);
    expect(within(tile).getByText(en["settings.dashTileHint"])).toBeTruthy();
    expect(within(tile).queryByRole("button")).toBeNull();
  });

  it("names how to set the widget up", async () => {
    await renderCard();
    expect(screen.getByText(en["settings.widgetStep1"])).toBeTruthy();
  });
});
