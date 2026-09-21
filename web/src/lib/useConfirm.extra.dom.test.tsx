// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { I18nProvider } from "./i18n";
import { useConfirm } from "./useConfirm";

function Harness({ onResult }: { onResult: (ok: boolean) => void }) {
  const { confirm, confirmDialog } = useConfirm();
  const ask = () =>
    void confirm("Back up nginx to NAS Keller from now on?", {
      confirmLabel: "Set",
      confirmLabelKey: "settings.save",
      extra: <p>B2: about 40</p>,
    }).then(onResult);
  return (
    <>
      <button type="button" onClick={ask}>
        open
      </button>
      {confirmDialog}
    </>
  );
}

describe("useConfirm with extra content", () => {
  afterEach(cleanup);

  it("shows the extra lines under the question and answers with the confirm button", async () => {
    const onResult = vi.fn();
    render(
      <I18nProvider>
        <Harness onResult={onResult} />
      </I18nProvider>
    );
    fireEvent.click(screen.getByText("open"));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("B2: about 40")).toBeTruthy();
    fireEvent.click(within(dialog).getByRole("button", { name: "Set" }));
    await waitFor(() => expect(onResult).toHaveBeenCalledWith(true));
  });

  it("still cancels on Escape", async () => {
    const onResult = vi.fn();
    render(
      <I18nProvider>
        <Harness onResult={onResult} />
      </I18nProvider>
    );
    fireEvent.click(screen.getByText("open"));
    await screen.findByRole("dialog");
    fireEvent.keyDown(document, { key: "Escape" });
    await waitFor(() => expect(onResult).toHaveBeenCalledWith(false));
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});
