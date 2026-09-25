// @vitest-environment jsdom
// The host-integration table says in the page's language whether a check is
// required, and a check without detail leaves that cell empty.
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { I18nProvider, de, en, type TranslationKey, type useT } from "../lib/i18n";

vi.mock("../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/api")>()),
  runSpike: async () => ({
    ok: true,
    allOk: true,
    checks: [
      { Name: "docker", OK: true, BestEffort: false, Detail: "" },
      { Name: "rclone", OK: false, BestEffort: true, Detail: "not found" },
    ],
  }),
}));

const { SpikePanel } = await import("./SpikePanel");

afterEach(cleanup);

/** The check's row, cell by cell: name, status, detail, required or optional. */
function cells(name: string): string[] {
  const row = screen.getByText(name).parentElement!;
  return [...row.children].map((c) => c.textContent ?? "");
}

describe.each([
  ["en", en],
  ["de", de],
])("the host-integration table in %s", (_lang, dict) => {
  const t = ((key: TranslationKey) => dict[key]) as unknown as ReturnType<typeof useT>["t"];

  it("names each check required or optional and leaves a missing detail empty", async () => {
    render(
      <I18nProvider>
        <SpikePanel t={t} />
      </I18nProvider>,
    );
    await act(async () => screen.getByRole("button", { name: dict["spike.checkNow"] }).click());

    expect(cells("docker").slice(2)).toEqual(["", dict["spike.required"]]);
    expect(cells("rclone").slice(2)).toEqual(["not found", dict["spike.bestEffort"]]);
  });
});
