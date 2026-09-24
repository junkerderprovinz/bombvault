// @vitest-environment jsdom
/**
 * The retention preview panel.
 *
 * Retention tells you what it keeps, never what it is about to delete, and the
 * answer only helps if the panel is honest about three things a plain removal
 * list would get wrong: a policy that is switched off removes nothing, an
 * append-only repository is never pruned here at all, and a repository the
 * server could not cover must be named rather than silently missing.
 */
import { render, screen, cleanup, waitFor, fireEvent } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

const previewRetention = vi.fn();

vi.mock("../lib/api", () => ({
  previewRetention: (...a: unknown[]) => previewRetention(...a),
}));

import { RetentionPreview } from "./RetentionPreview";
import { en } from "../lib/i18n";

const t = ((key: string) => (en as Record<string, string>)[key] ?? key) as unknown as Parameters<
  typeof RetentionPreview
>[0]["t"];

function renderPanel(source: "local" | "offsite" = "local") {
  return render(<RetentionPreview t={t} source={source} />);
}

afterEach(() => {
  cleanup();
  previewRetention.mockReset();
});

describe("RetentionPreview", () => {
  // The Settings page's own DOM tests mock lib/api by spreading the real
  // module, so a fetch fired during render would escape to real fetch under
  // jsdom and make unrelated suites flaky. Asking only on click also matches
  // what the call costs: one restic invocation per item per repository.
  it("asks nothing until the button is pressed", async () => {
    renderPanel();
    expect(previewRetention).not.toHaveBeenCalled();
  });

  it("names the snapshots that would be removed, and not the kept one", async () => {
    previewRetention.mockResolvedValue({
      ok: true,
      preview: {
        policy: { on: true, keepLast: 1, keepDaily: 0, keepWeekly: 0, keepMonthly: 0 },
        repos: [
          {
            name: "Containers",
            appendOnly: false,
            items: [
              {
                tag: "container:plex",
                keep: [{ id: "1a1a1a1a", time: "2026-09-15T02:00:00Z" }],
                remove: [
                  { id: "2b2b2b2b", time: "2026-09-14T02:00:00Z" },
                  { id: "3c3c3c3c", time: "2026-09-13T02:00:00Z" },
                ],
              },
            ],
          },
        ],
      },
    });

    renderPanel();
    fireEvent.click(screen.getByRole("button", { name: new RegExp(en["retentionPreview.show"], "i") }));

    await waitFor(() => expect(screen.getByText(/2b2b2b2b/)).toBeTruthy());
    expect(screen.getByText(/3c3c3c3c/)).toBeTruthy();
    expect(screen.queryByText(/1a1a1a1a/)).toBeNull();
    expect(screen.getByText(/container:plex/)).toBeTruthy();
  });

  it("says retention is off instead of showing an empty list", async () => {
    previewRetention.mockResolvedValue({
      ok: true,
      preview: {
        policy: { on: false, keepLast: 0, keepDaily: 0, keepWeekly: 0, keepMonthly: 0 },
        repos: [{ name: "Containers", appendOnly: false, items: [] }],
      },
    });

    renderPanel();
    fireEvent.click(screen.getByRole("button", { name: new RegExp(en["retentionPreview.show"], "i") }));

    await waitFor(() => expect(screen.getByText(en["retentionPreview.off"])).toBeTruthy());
  });

  // The dangerous false positive, inverted: an append-only repository shows no
  // removals BECAUSE retention never runs there. Without the note, the empty
  // list reads as "nothing to do today".
  it("explains an append-only repository rather than leaving it blank", async () => {
    previewRetention.mockResolvedValue({
      ok: true,
      preview: {
        policy: { on: true, keepLast: 5, keepDaily: 0, keepWeekly: 0, keepMonthly: 0 },
        repos: [{ name: "Cold archive", appendOnly: true, items: [] }],
      },
    });

    renderPanel();
    fireEvent.click(screen.getByRole("button", { name: new RegExp(en["retentionPreview.show"], "i") }));

    await waitFor(() => expect(screen.getByText(en["retentionPreview.appendOnly"])).toBeTruthy());
    expect(screen.getByText(/Cold archive/)).toBeTruthy();
  });

  it("surfaces a repository the server could not cover", async () => {
    previewRetention.mockResolvedValue({
      ok: true,
      preview: {
        policy: { on: true, keepLast: 5, keepDaily: 0, keepWeekly: 0, keepMonthly: 0 },
        repos: [{ name: "Containers", appendOnly: false, items: [] }],
        skipped: ["Cold archive (it was there before and is not reachable now)"],
      },
    });

    renderPanel();
    fireEvent.click(screen.getByRole("button", { name: new RegExp(en["retentionPreview.show"], "i") }));

    await waitFor(() => expect(screen.getByText(/not reachable now/)).toBeTruthy());
  });

  // A held item removes nothing because an open anomaly keeps its backups,
  // and the empty list must not read as a policy that has nothing to do.
  it("shows paused items", async () => {
    previewRetention.mockResolvedValue({
      ok: true,
      preview: {
        policy: { on: true, keepLast: 1, keepDaily: 0, keepWeekly: 0, keepMonthly: 0 },
        repos: [
          {
            name: "Containers",
            appendOnly: false,
            items: [
              {
                tag: "container:plex",
                keep: [{ id: "1a1a1a1a", time: "2026-09-15T02:00:00Z" }],
                remove: [],
                paused: true,
              },
              {
                tag: "container:sonarr",
                keep: [{ id: "4d4d4d4d", time: "2026-09-15T02:00:00Z" }],
                remove: [{ id: "5e5e5e5e", time: "2026-09-14T02:00:00Z" }],
              },
            ],
          },
        ],
      },
    });

    renderPanel();
    fireEvent.click(screen.getByRole("button", { name: new RegExp(en["retentionPreview.show"], "i") }));

    const paused = await screen.findByText(en["retentionPreview.paused"]);
    expect(paused.className).toContain("text-statusWarn");
    expect(paused.closest("div.mt-2")?.textContent).toContain("container:plex");
    expect(screen.getAllByText(en["retentionPreview.paused"])).toHaveLength(1);
  });

  it("shows the server's refusal instead of an empty panel", async () => {
    previewRetention.mockResolvedValue({ ok: false, error: "no backups yet" });

    renderPanel();
    fireEvent.click(screen.getByRole("button", { name: new RegExp(en["retentionPreview.show"], "i") }));

    await waitFor(() => expect(screen.getByText(/no backups yet/)).toBeTruthy());
  });
});
