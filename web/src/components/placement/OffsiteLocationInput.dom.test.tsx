// @vitest-environment jsdom
import type { ReactElement } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import { renderWithProviders, targetPreview } from "../../lib/placement.testsupport";
import { I18nProvider } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";

const fake = await vi.hoisted(async () => (await import("../../lib/placement.testsupport")).createPlacementApi());

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  ...fake.api,
}));

const { OffsiteLocationInput } = await import("./OffsiteLocationInput");

// rerender() replaces whatever renderWithProviders mounted, so a prop-change
// test has to hand back the same provider tree or React sees a different root
// element and remounts the field from scratch, losing the draft under test.
function withProviders(ui: ReactElement): ReactElement {
  return (
    <I18nProvider>
      <ToastProvider>{ui}</ToastProvider>
    </I18nProvider>
  );
}

const PLACEHOLDER = "rest:http://host:8000/repo";

describe("OffsiteLocationInput", () => {
  beforeEach(() => fake.reset());
  afterEach(cleanup);

  it("keeps a typed location when the stored value changes underneath", () => {
    const onSave = vi.fn().mockResolvedValue(true);
    const { rerender } = renderWithProviders(
      <OffsiteLocationInput domain="flash" value="" placeholder={PLACEHOLDER} className="" onSave={onSave} />
    );
    fireEvent.change(screen.getByPlaceholderText(PLACEHOLDER), { target: { value: "b2:bucket:flash" } });
    rerender(
      withProviders(
        <OffsiteLocationInput domain="flash" value="b2:bucket:other" placeholder={PLACEHOLDER} className="" onSave={onSave} />
      )
    );
    expect(screen.getByPlaceholderText<HTMLInputElement>(PLACEHOLDER).value).toBe("b2:bucket:flash");
    expect(onSave).not.toHaveBeenCalled();
  });

  it("follows the stored value when nothing was typed", () => {
    const onSave = vi.fn().mockResolvedValue(true);
    const { rerender } = renderWithProviders(
      <OffsiteLocationInput domain="flash" value="" placeholder={PLACEHOLDER} className="" onSave={onSave} />
    );
    rerender(
      withProviders(
        <OffsiteLocationInput domain="flash" value="b2:bucket:other" placeholder={PLACEHOLDER} className="" onSave={onSave} />
      )
    );
    expect(screen.getByPlaceholderText<HTMLInputElement>(PLACEHOLDER).value).toBe("b2:bucket:other");
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("keeps the typed location when the question is answered with no", async () => {
    const onSave = vi.fn().mockResolvedValue(true);
    renderWithProviders(
      <OffsiteLocationInput
        domain="containers"
        value=""
        targetId="t-b2"
        targetName="B2"
        placeholder={PLACEHOLDER}
        className=""
        onSave={onSave}
      />
    );
    const field = screen.getByPlaceholderText(PLACEHOLDER);
    fireEvent.change(field, { target: { value: "b2:bucket:containers" } });
    fireEvent.keyDown(field, { key: "Enter" });
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(screen.getByPlaceholderText<HTMLInputElement>(PLACEHOLDER).value).toBe("b2:bucket:containers");
    expect(onSave).not.toHaveBeenCalled();
  });

  it("leaves nothing out and keeps the typed location when the server refuses the save", async () => {
    fake.reply("getNewTargetPreview", { ok: true, preview: targetPreview({ defaultExcludes: true }) });
    const onSave = vi.fn().mockResolvedValue(false);
    renderWithProviders(
      <OffsiteLocationInput
        domain="containers"
        value=""
        targetId="t-b2"
        targetName="B2"
        placeholder={PLACEHOLDER}
        className=""
        onSave={onSave}
      />
    );
    const field = screen.getByPlaceholderText(PLACEHOLDER);
    fireEvent.change(field, { target: { value: "b2:bucket:containers" } });
    fireEvent.keyDown(field, { key: "Enter" });
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("switch", { name: "Leave B2 out of the default too" }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Confirm" }));
    await waitFor(() => expect(onSave).toHaveBeenCalledWith("b2:bucket:containers"));
    expect(fake.callsTo("excludeFromTarget")).toEqual([]);
    expect(screen.getByPlaceholderText<HTMLInputElement>(PLACEHOLDER).value).toBe("b2:bucket:containers");
  });
});
