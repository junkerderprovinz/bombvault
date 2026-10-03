// @vitest-environment jsdom
// Docker refuses a container whose GPU or runtime the host lacks. The banner
// says so in a sentence and offers the restore without them.
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { en } from "../../lib/i18n";
import { RestoreProgress } from "./RestoreProgress";

const t = ((k: string) => en[k as keyof typeof en] ?? k) as never;

const refused =
  "restore failed: the container used a GPU or runtime this host does not have: Error response from daemon: unknown or invalid runtime name: nvidia";

function renderBanner(props: Partial<Parameters<typeof RestoreProgress>[0]> = {}) {
  return render(
    <RestoreProgress
      state={{ phase: "error", message: refused }}
      isPending={false}
      prog={undefined}
      cancelKey="container:plex"
      inPlace
      name="plex"
      successMessage="done"
      t={t}
      {...props}
    />
  );
}

afterEach(cleanup);

describe("a restore the host has no GPU or runtime for", () => {
  it("says so and offers the restore without them", () => {
    const retry = vi.fn();
    renderBanner({ onRestoreWithoutRuntime: retry });
    expect(document.body.textContent).toContain(en["restore.noRuntime"].replace("{name}", "plex"));
    fireEvent.click(screen.getByRole("button", { name: en["restore.withoutRuntime"] }));
    expect(retry).toHaveBeenCalledTimes(1);
  });

  it("offers no retry for any other failure", () => {
    renderBanner({
      onRestoreWithoutRuntime: vi.fn(),
      state: { phase: "error", message: "restore: pull image: manifest unknown" },
    });
    expect(screen.queryByRole("button", { name: en["restore.withoutRuntime"] })).toBeNull();
    expect(document.body.textContent).toContain("manifest unknown");
  });

  it("says what the restore left out once it succeeded", () => {
    renderBanner({ state: { phase: "success", note: "restored without the GPU or runtime the container used" } });
    expect(document.body.textContent).toContain("done");
    expect(document.body.textContent).toContain(en["restore.withoutRuntimeDone"].replace("{name}", "plex"));
  });
});
