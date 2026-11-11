// @vitest-environment jsdom
// The tile for the Unraid dashboard is TollRoll, which reads BombVault through
// the fortress. Its card on the Apps page says where to get it.
import { act, cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, expect, it, vi } from "vitest";
import { I18nProvider, useT } from "../../lib/i18n";

vi.mock("../../lib/toast", () => ({ useToast: () => ({ push: () => {} }) }));

const { WidgetAppCard } = await import("./AppsCards");

function Harness() {
  const { t } = useT();
  return <WidgetAppCard t={t} />;
}

async function renderCard() {
  await act(async () => {
    render(
      <MemoryRouter>
        <I18nProvider>
          <Harness />
        </I18nProvider>
      </MemoryRouter>
    );
  });
}

afterEach(cleanup);

it("names TollRoll and finds it in Community Applications", async () => {
  await renderCard();
  expect(screen.getByText("TollRoll")).toBeTruthy();
  const unraid = screen.getAllByRole("link").find((a) => a.getAttribute("href")?.includes("tollroll"));
  expect(unraid).toBeTruthy();
  expect(screen.queryByText(/bombvault-widget/i)).toBeNull();
});
