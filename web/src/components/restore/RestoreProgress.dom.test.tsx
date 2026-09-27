// @vitest-environment jsdom
// The banner under every restore control. A dump saved to a folder or imported
// has no percentage, and some of these runs cannot be stopped, so the banner
// has to show what moves and offer only what the backend can do.
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { en } from "../../lib/i18n";
import type { ProgressState } from "../../lib/progress";
import { RestoreProgress } from "./RestoreProgress";

const t = ((k: string) => en[k as keyof typeof en] ?? k) as never;

function importing(bytes: number): ProgressState {
  return { phase: "restore", percent: 0, active: true, lastSeen: Date.now(), stage: "dbimport", bytes };
}

function renderBanner(props: Partial<Parameters<typeof RestoreProgress>[0]> = {}) {
  return render(
    <RestoreProgress
      state={{ phase: "pending" }}
      isPending
      prog={undefined}
      cancelKey="container:immich_postgres"
      inPlace
      name="immich_postgres"
      successMessage="done"
      t={t}
      {...props}
    />
  );
}

afterEach(cleanup);

describe("RestoreProgress", () => {
  it("counts the bytes of a dump being imported", () => {
    renderBanner({ prog: importing(5_242_880) });
    expect(document.body.textContent).toContain(
      en["activityLog.lineImportingItem"].replace("{name}", "immich_postgres").replace("{bytes}", "5.0 MB")
    );
  });

  it("offers no cancel for a run the backend cannot stop", () => {
    renderBanner({ cancellable: false });
    expect(screen.queryByRole("button", { name: en["restore.cancel"] })).toBeNull();
    cleanup();
    renderBanner();
    expect(screen.getByRole("button", { name: en["restore.cancel"] })).toBeTruthy();
  });

  it("translates a failure BombVault worded and keeps the tool's message", () => {
    renderBanner({
      isPending: false,
      state: {
        phase: "error",
        message: "database import failed: the import tool reported an error: ERROR: relation exists",
      },
    });
    expect(document.body.textContent).toContain(en["runReason.dbimportFailed"]);
    expect(document.querySelector('bdi[dir="ltr"]')?.textContent).toBe("ERROR: relation exists");
  });

  it("shows a restic error as it was written", () => {
    renderBanner({ isPending: false, state: { phase: "error", message: "Fatal: repository is locked" } });
    expect(screen.getByText("Fatal: repository is locked")).toBeTruthy();
  });

  it("shows a success that needs attention in the warning tone", () => {
    renderBanner({ isPending: false, state: { phase: "success" }, successWarn: true, successMessage: "imported" });
    expect(screen.getByText("imported").className).toContain("statusWarn");
  });
});
