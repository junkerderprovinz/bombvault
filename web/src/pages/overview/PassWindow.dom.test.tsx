// @vitest-environment jsdom
// The window of a pass over everything: the entries it has finished, the one
// it is on, and how many wait.
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";

import { en } from "../../lib/i18n";
import { run } from "./overview.testsupport";
import { inApp } from "./overviewApp.testsupport";
import { PassWindow } from "./PassWindow";
import type { LiveRun } from "./headState";

vi.mock("../../lib/api", async (importOriginal) => {
  const { stagedApi } = await import("./overview.testsupport");
  return stagedApi(await importOriginal<typeof import("../../lib/api")>());
});

const t = ((key: string) => (en as Record<string, string>)[key] ?? key) as Parameters<typeof PassWindow>[0]["t"];

afterEach(cleanup);

const windows11: LiveRun = { kind: "vm", name: "Windows 11", percent: 50, station: "read", stops: true };

function open(live: LiveRun | null = windows11) {
  const onClose = vi.fn();
  render(
    inApp(
      <PassWindow
        t={t}
        pass={{ done: 2, total: 14 }}
        run={live}
        done={[
          run({ id: "a", target: "plex", status: "success" }),
          run({ id: "b", target: "paperless-ngx", status: "failed" }),
        ]}
        onClose={onClose}
      />
    )
  );
  return onClose;
}

describe("PassWindow", () => {
  it("counts the pass and lists what it finished, the entry it is on and the rest", () => {
    open();

    expect(screen.getByText(en["overview.pass.count"].replace("{done}", "2").replace("{total}", "14"))).toBeTruthy();
    const lines = within(screen.getByTestId("pass-entries")).getAllByRole("listitem");
    expect(lines.map((l) => l.lastElementChild!.textContent)).toEqual([
      "plex",
      "paperless-ngx",
      "Windows 1150 %",
      en["overview.pass.waiting"].replace("{n}", "11"),
    ]);
    expect(lines[1].querySelector(".text-statusFail")).toBeTruthy();
  });

  it("counts every entry left as waiting between two entries", () => {
    open(null);

    const lines = within(screen.getByTestId("pass-entries")).getAllByRole("listitem");
    expect(lines.at(-1)!.lastElementChild!.textContent).toBe(en["overview.pass.waiting"].replace("{n}", "12"));
  });

  it("closes from its button", () => {
    const onClose = open();

    fireEvent.click(screen.getByRole("button", { name: en["common.close"] }));

    expect(onClose).toHaveBeenCalledOnce();
  });
});
