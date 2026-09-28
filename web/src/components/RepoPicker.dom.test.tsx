// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { en, I18nProvider } from "../lib/i18n";

vi.mock("../lib/api", () => ({ listRepos: () => Promise.resolve({ ok: true, repos: [] }) }));

import { RepoPicker } from "./RepoPicker";

describe("RepoPicker", () => {
  afterEach(cleanup);

  it("carries one (i) beside its label, locked or not", () => {
    render(
      <I18nProvider>
        <RepoPicker value="" onChange={() => {}} locked />
      </I18nProvider>,
    );
    const label = screen.getByText(en["repos.itemLabel"]).closest("label")!;
    const bubbles = [...label.querySelectorAll("[aria-label]")];
    expect(bubbles).toHaveLength(1);
    const tip = bubbles[0].getAttribute("aria-label") ?? "";
    expect(tip).toContain(en["repos.itemHint"]);
    expect(tip).toContain(en["repos.itemLocked"]);
  });
});
