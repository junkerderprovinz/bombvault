// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { en, I18nProvider } from "../lib/i18n";
import { BackButton } from "./BackButton";

afterEach(cleanup);

describe("BackButton", () => {
  it("stands at the height of the other buttons in a page's head", () => {
    render(
      <I18nProvider>
        <MemoryRouter>
          <BackButton to="/storage" label="Storage locations" />
        </MemoryRouter>
      </I18nProvider>
    );
    const back = screen.getByRole("button", { name: en["common.backTo"].replace("{page}", "Storage locations") });
    expect(back.className).toContain("glim-btn-icon");
    expect(back.className).not.toContain("glim-btn-key");
  });
});
