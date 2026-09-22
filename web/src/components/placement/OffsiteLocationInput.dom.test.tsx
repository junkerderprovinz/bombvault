// @vitest-environment jsdom
import type { ReactElement } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen } from "@testing-library/react";
import { renderWithProviders } from "../../lib/placement.testsupport";
import { I18nProvider } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import { OffsiteLocationInput } from "./OffsiteLocationInput";

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
});
