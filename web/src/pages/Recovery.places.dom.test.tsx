// @vitest-environment jsdom
// Recovery connects the storage a backup lies on through the add window, the
// same one the Storage tab uses, and shows the places already connected.
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, en, useT } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { Place } from "../lib/places";

vi.mock("../lib/places", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/places")>()),
  listPlaces: () =>
    Promise.resolve({ ok: true, places: [{ id: "p-b2", name: "B2 Backups", provider: "b2" } as Place], unplaced: [] }),
  getPlacesCatalog: () => Promise.resolve({ ok: true, providers: [] }),
}));

vi.mock("../components/placeMarks", () => ({
  PlaceMark: ({ provider }: { provider: string }) => <span data-testid="mark" data-provider={provider} />,
}));

const { PlacesDisclosure } = await import("./Recovery");

function Harness() {
  const { t } = useT();
  return <PlacesDisclosure t={t} hostMountRoot="/mnt" />;
}

afterEach(cleanup);

describe("Recovery's storage places", () => {
  it("shows the connected places once opened and adds one through the window", async () => {
    await act(async () => {
      render(
        <I18nProvider>
          <ToastProvider>
            <Harness />
          </ToastProvider>
        </I18nProvider>
      );
    });
    expect(screen.queryByText("B2 Backups")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: en["recovery.places"] }));
    expect(screen.getByText("B2 Backups")).toBeTruthy();
    expect(screen.getByTestId("mark").getAttribute("data-provider")).toBe("b2");
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["places.add"] }));
    });
    expect(screen.getByRole("dialog", { name: en["places.addTitle"] })).toBeTruthy();
  });
});
