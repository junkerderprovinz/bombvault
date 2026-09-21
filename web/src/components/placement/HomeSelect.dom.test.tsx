// @vitest-environment jsdom
import type { ReactElement } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen } from "@testing-library/react";
import { renderWithProviders, wheel } from "../../lib/placement.testsupport";
import { I18nProvider } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import { HomeSelect } from "./HomeSelect";

// rerender() replaces whatever renderWithProviders mounted, so a prop-change
// test has to hand back the same provider tree or React sees a different root
// element and remounts HomeSelect from scratch, losing the draft under test.
function withProviders(ui: ReactElement): ReactElement {
  return (
    <I18nProvider>
      <ToastProvider>{ui}</ToastProvider>
    </I18nProvider>
  );
}

const OPTIONS = [
  { value: "", label: "Unraid" },
  { value: "repo-nas", label: "NAS Keller · mounted" },
  { value: "repo-cold", label: "Cold · mounted" },
];

describe("HomeSelect", () => {
  afterEach(cleanup);

  it("lets the wheel change only the draft until Set", () => {
    const onCommit = vi.fn();
    renderWithProviders(<HomeSelect label="Stored on" value="" options={OPTIONS} locked={false} onCommit={onCommit} />);
    wheel(screen.getByRole("combobox", { name: "Stored on" }), 5);
    expect(onCommit).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Set" }));
    expect(onCommit).toHaveBeenCalledWith("repo-cold");
  });

  it("offers no Set while the draft is the stored value", () => {
    renderWithProviders(<HomeSelect label="Stored on" value="" options={OPTIONS} locked={false} onCommit={vi.fn()} />);
    expect(screen.queryByRole("button", { name: "Set" })).toBeNull();
  });

  it("shows a single possibility as text", () => {
    renderWithProviders(<HomeSelect label="Stored on" value="" options={OPTIONS.slice(0, 1)} locked={false} onCommit={vi.fn()} />);
    expect(screen.queryByRole("combobox")).toBeNull();
    expect(screen.getByText("Unraid")).toBeTruthy();
  });

  it("shows a locked home as text, fixed since the first backup", () => {
    renderWithProviders(<HomeSelect label="Stored on" value="repo-nas" options={OPTIONS} locked onCommit={vi.fn()} />);
    expect(screen.queryByRole("combobox")).toBeNull();
    expect(screen.getByText("NAS Keller · mounted")).toBeTruthy();
    expect(screen.getByText("fixed since the first backup")).toBeTruthy();
  });

  it("keeps a stored value that is off in the list without letting it be picked again", () => {
    const withOff = [...OPTIONS.slice(0, 2), { value: "repo-old", label: "Old (off)", disabled: true }];
    renderWithProviders(<HomeSelect label="Stored on" value="repo-old" options={withOff} locked={false} onCommit={vi.fn()} />);
    fireEvent.click(screen.getByRole("combobox", { name: "Stored on" }));
    expect((screen.getByRole("option", { name: "Old (off)" }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("keeps a pending draft when the stored value changes underneath", () => {
    const onCommit = vi.fn();
    const { rerender } = renderWithProviders(
      <HomeSelect label="Stored on" value="" options={OPTIONS} locked={false} onCommit={onCommit} />
    );
    wheel(screen.getByRole("combobox", { name: "Stored on" }), 1);
    rerender(withProviders(<HomeSelect label="Stored on" value="repo-cold" options={OPTIONS} locked={false} onCommit={onCommit} />));
    fireEvent.click(screen.getByRole("button", { name: "Set" }));
    expect(onCommit).toHaveBeenCalledWith("repo-nas");
  });

  it("follows the stored value when the draft had not diverged", () => {
    const { rerender } = renderWithProviders(
      <HomeSelect label="Stored on" value="" options={OPTIONS} locked={false} onCommit={vi.fn()} />
    );
    rerender(withProviders(<HomeSelect label="Stored on" value="repo-cold" options={OPTIONS} locked={false} onCommit={vi.fn()} />));
    expect(screen.queryByRole("button", { name: "Set" })).toBeNull();
    expect(screen.getByText("Cold · mounted").getAttribute("aria-hidden")).toBe("false");
  });
});
