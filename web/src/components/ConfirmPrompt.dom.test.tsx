// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider } from "../lib/i18n";
import { ConfirmPrompt } from "./ConfirmPrompt";

let desktop = true;
vi.mock("../lib/useMediaQuery", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/useMediaQuery")>()),
  useIsDesktop: () => desktop,
}));

afterEach(cleanup);

function renderPrompt(onConfirm = vi.fn(), onCancel = vi.fn()) {
  render(
    <I18nProvider>
      <ConfirmPrompt
        title="Replace this key?"
        message="The old key stops working at once."
        confirmLabel="Replace key"
        cancelLabel="Cancel"
        onConfirm={onConfirm}
        onCancel={onCancel}
      />
    </I18nProvider>,
  );
  return { onConfirm, onCancel };
}

describe("ConfirmPrompt", () => {
  it("shows the dialog card on the desktop, answers side by side", () => {
    desktop = true;
    renderPrompt();
    const cancel = screen.getByRole("button", { name: "Cancel" });
    expect(cancel.className).not.toContain("w-full");
    expect(screen.getByRole("dialog").className).toContain("glim-modal-card");
  });

  it("shows the bottom sheet on a phone, answers stacked full width", () => {
    desktop = false;
    const { onConfirm, onCancel } = renderPrompt();
    expect(screen.getByRole("dialog").className).not.toContain("glim-modal-card");
    const cancel = screen.getByRole("button", { name: "Cancel" });
    const confirm = screen.getByRole("button", { name: "Replace key" });
    expect(cancel.className).toContain("w-full");
    expect(confirm.className).toContain("w-full");
    fireEvent.click(confirm);
    expect(onConfirm).toHaveBeenCalledOnce();
    fireEvent.click(cancel);
    expect(onCancel).toHaveBeenCalledOnce();
  });
});
