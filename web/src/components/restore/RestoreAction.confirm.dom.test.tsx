// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// A row-action restore asks before it overwrites anything.
//
// Recovery's card-5 rows passed `requireConfirm={false}` on the strength of a
// prop doc claiming "its own stepper gates the whole flow". The stepper gates
// nothing, so one click on an unlabelled glyph badge started an in-place
// restore over live appdata or VM disks — while the "Restore all" button in the
// same card asked first. It was the only requireConfirm={false} in the tree.
//
// The checkbox does not fit a one-line row action, so the guard is the modal
// every other destructive action already uses. These tests pin that the restore
// call itself is what waits for the answer, not just that a dialog appears.
// ---------------------------------------------------------------------------
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider, en } from "../../lib/i18n";

const restore = vi.fn(() => Promise.resolve({ ok: true, started: true }));
const restoreVM = vi.fn(() => Promise.resolve({ ok: true, started: true }));

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    restore: (...a: unknown[]) => restore(...(a as [])),
    restoreVM: (...a: unknown[]) => restoreVM(...(a as [])),
    listRuns: () => Promise.resolve({ ok: true, runs: [] }),
  };
});

// The progress store opens an EventSource on mount and jsdom has none. It is
// irrelevant to what these tests assert (whether restore() is called at all),
// so it gets the minimum surface that lets the effect run.
class StubEventSource {
  onmessage: ((e: MessageEvent) => void) | null = null;
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
(globalThis as { EventSource?: unknown }).EventSource = StubEventSource;

const { RestoreAction } = await import("./RestoreAction");

function renderAction(props: Record<string, unknown> = {}) {
  const Harness = () => {
    const t = ((k: string) => en[k as keyof typeof en] ?? k) as never;
    return (
      <RestoreAction
        domain="container"
        name="plex"
        snapshotId="latest"
        otherActive={{ active: false }}
        successMessage="done"
        requireConfirm={false}
        showLeaveStopped={false}
        forceLeaveStopped
        showBusyHint={false}
        showStartedHint={false}
        iconBadge
        t={t}
        {...props}
      />
    );
  };
  return render(
    <I18nProvider>
      <Harness />
    </I18nProvider>
  );
}

/** The row action's trigger, by the accessible name its `tip`/label gives it. */
function trigger() {
  return screen.getByRole("button", { name: en["snapshots.restore"] });
}

afterEach(() => {
  cleanup();
  restore.mockClear();
  restoreVM.mockClear();
});

describe("row-action restore confirmation", () => {
  it("does not restore while the question is still open", async () => {
    renderAction({ confirmMessage: "Really restore plex?" });

    fireEvent.click(trigger());
    await waitFor(() => expect(screen.getByText("Really restore plex?")).toBeTruthy());
    // The dialog is up and nothing has been overwritten yet.
    expect(restore).not.toHaveBeenCalled();
  });

  it("does not restore when the answer is no", async () => {
    renderAction({ confirmMessage: "Really restore plex?" });

    fireEvent.click(trigger());
    await waitFor(() => expect(screen.getByText("Really restore plex?")).toBeTruthy());
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["common.cancel"] }));
    });
    expect(restore).not.toHaveBeenCalled();
  });

  it("restores once the answer is yes", async () => {
    renderAction({ confirmMessage: "Really restore plex?" });

    fireEvent.click(trigger());
    await waitFor(() => expect(screen.getByText("Really restore plex?")).toBeTruthy());
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["common.confirm"] }));
    });
    await waitFor(() => expect(restore).toHaveBeenCalledTimes(1));
  });

  it("leaves a caller without confirmMessage exactly as it was", async () => {
    // The form call sites (RestorePanel, VMs) gate on the checkbox instead and
    // must not grow a second question.
    renderAction();
    await act(async () => {
      fireEvent.click(trigger());
    });
    await waitFor(() => expect(restore).toHaveBeenCalledTimes(1));
  });
});
