// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import type { DeploySnippetData } from "../../lib/api";

const SNIPPET: DeploySnippetData = {
  user: "tower",
  password: "Xy12",
  htpasswd: "tower:$2a$12$hash",
  dockerRun: "docker run --append-only",
  compose: "services: rest-server",
  unraid: "<Container/>",
};
let answer: { ok: boolean; error?: string; snippet?: DeploySnippetData } = { ok: true, snippet: SNIPPET };

vi.mock("../../lib/places", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/places")>()),
  restServerRecipe: () => Promise.resolve(answer),
}));

const { RestServerRecipe } = await import("./RestServerRecipe");

beforeEach(() => {
  answer = { ok: true, snippet: SNIPPET };
});
afterEach(cleanup);

async function show(onLogin = vi.fn()) {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <RestServerRecipe onLogin={onLogin} />
        </ToastProvider>
      </I18nProvider>
    );
  });
  return onLogin;
}

async function ask(key: keyof typeof en) {
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: en[key] }));
  });
}

describe("RestServerRecipe", () => {
  it("shows the recipe once asked, and hands its login to the form", async () => {
    const onLogin = await show();
    expect(screen.queryByText(SNIPPET.dockerRun)).toBeNull();
    await ask("places.recipe.show");
    for (const text of [SNIPPET.password, SNIPPET.dockerRun, SNIPPET.compose, SNIPPET.unraid]) {
      expect(screen.getByText(text)).toBeTruthy();
    }
    expect(onLogin).toHaveBeenCalledWith("tower", "Xy12");
    expect(screen.getByRole("button", { name: en["places.recipe.newPassword"] })).toBeTruthy();
  });

  it("says why no recipe came, and shakes", async () => {
    answer = { ok: false, error: "no randomness" };
    const onLogin = await show();
    await ask("places.recipe.show");
    expect(await screen.findByText("no randomness")).toBeTruthy();
    expect(screen.getByRole("button", { name: en["places.recipe.show"] }).className).toContain("glim-shake");
    expect(onLogin).not.toHaveBeenCalled();
  });
});
