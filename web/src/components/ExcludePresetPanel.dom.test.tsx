// @vitest-environment jsdom
import { afterEach, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { ExcludePresetPanel } from "./ExcludePresetPanel";
import { en, type TranslationKey } from "../lib/i18n";

const t = (key: TranslationKey) => en[key] as string;

const preset = {
  app: "jellyfin",
  entries: [
    { line: "/config/cache", kind: "cache", optional: false },
    { line: "/config/data/metadata", kind: "metadata", optional: true },
    { line: "/config/log", kind: "logs", optional: false },
  ],
};

afterEach(cleanup);

function open() {
  fireEvent.click(screen.getByRole("button", { name: /Load recommended excludes for Jellyfin/ }));
}

it("applies nothing until the user presses apply", () => {
  const onApply = vi.fn();
  render(<ExcludePresetPanel preset={preset} currentLines={[]} saving={false} onApply={onApply} t={t} />);
  open();
  expect(onApply).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: /Exclude selected/ }));
  expect(onApply).toHaveBeenCalledWith(["/config/cache", "/config/log"]);
});

it("starts an optional entry switched off and lets the user untick others", () => {
  const onApply = vi.fn();
  render(<ExcludePresetPanel preset={preset} currentLines={[]} saving={false} onApply={onApply} t={t} />);
  open();
  expect(screen.getByRole("switch", { name: "Downloaded artwork" }).getAttribute("aria-checked")).toBe("false");
  fireEvent.click(screen.getByRole("switch", { name: "Logs" }));
  fireEvent.click(screen.getByRole("button", { name: /Exclude selected/ }));
  expect(onApply).toHaveBeenCalledWith(["/config/cache"]);
});

it("hides lines that are already excluded and disappears when none are left", () => {
  const { container } = render(
    <ExcludePresetPanel
      preset={preset}
      currentLines={["/config/cache", "/config/data/metadata", "/config/log"]}
      saving={false}
      onApply={() => {}}
      t={t}
    />
  );
  expect(container.textContent).toBe("");
});
