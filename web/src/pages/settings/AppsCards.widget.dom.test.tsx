// @vitest-environment jsdom
// The widget's card on the Apps page says where to get it. Installing the
// dashboard plugin belongs to the widget card under Integrations, so this card
// only leads there.
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { afterEach, expect, it, vi } from "vitest";
import { I18nProvider, en, useT } from "../../lib/i18n";

const getDashboardPlugin = vi.fn(() => Promise.resolve({ ok: true, sshConfigured: true, installed: false }));

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return { ...actual, getDashboardPlugin: () => getDashboardPlugin() };
});

vi.mock("../../lib/toast", () => ({ useToast: () => ({ push: () => {} }) }));

const { WidgetAppCard } = await import("./AppsCards");

function Where() {
  return <p data-testid="where">{useLocation().pathname}</p>;
}

function Harness() {
  const { t } = useT();
  return (
    <Routes>
      <Route path="/settings/apps" element={<WidgetAppCard t={t} />} />
      <Route path="*" element={<Where />} />
    </Routes>
  );
}

async function renderCard() {
  await act(async () => {
    render(
      <MemoryRouter initialEntries={["/settings/apps"]}>
        <I18nProvider>
          <Harness />
        </I18nProvider>
      </MemoryRouter>
    );
  });
}

afterEach(cleanup);

it("does not install the dashboard plugin itself", async () => {
  await renderCard();
  expect(screen.queryByRole("button", { name: en["settings.dashTileInstall"] })).toBeNull();
  expect(screen.queryByText(en["settings.dashTile"])).toBeNull();
  expect(getDashboardPlugin).not.toHaveBeenCalled();
});

it("leads to the widget settings under Integrations", async () => {
  await renderCard();
  fireEvent.click(screen.getByRole("button", { name: en["apps.widget.toSettings"] }));
  expect(screen.getByTestId("where").textContent).toBe("/settings/integrations");
});
