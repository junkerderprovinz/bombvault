// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider } from "./i18n";
import { useConfirm } from "./useConfirm";

function Harness({ onResult }: { onResult: (ok: boolean) => void }) {
  const { confirm, confirmDialog } = useConfirm();
  const ask = () =>
    void confirm("Delete it?", {
      confirmLabel: "Delete",
      requireText: "vaultwarden",
      requirePrompt: "Type vaultwarden to confirm",
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

describe("useConfirm with a text to type", () => {
  afterEach(cleanup);

  it("keeps the confirm button locked until the text matches exactly", async () => {
    const onResult = vi.fn();
    render(
      <I18nProvider>
        <Harness onResult={onResult} />
      </I18nProvider>
    );
    fireEvent.click(screen.getByText("open"));
    const button = (await screen.findByRole("button", { name: "Delete" })) as HTMLButtonElement;
    const field = screen.getByLabelText("Type vaultwarden to confirm");
    expect(button.disabled).toBe(true);
    fireEvent.change(field, { target: { value: "Vaultwarden" } });
    expect(button.disabled).toBe(true);
    fireEvent.change(field, { target: { value: "vaultwarden" } });
    expect(button.disabled).toBe(false);
    fireEvent.click(button);
    await waitFor(() => expect(onResult).toHaveBeenCalledWith(true));
  });

  it("starts empty every time it asks", async () => {
    render(
      <I18nProvider>
        <Harness onResult={vi.fn()} />
      </I18nProvider>
    );
    fireEvent.click(screen.getByText("open"));
    fireEvent.change(await screen.findByLabelText("Type vaultwarden to confirm"), { target: { value: "vaultwarden" } });
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    fireEvent.click(screen.getByText("open"));
    expect(((await screen.findByLabelText("Type vaultwarden to confirm")) as HTMLInputElement).value).toBe("");
  });
});
