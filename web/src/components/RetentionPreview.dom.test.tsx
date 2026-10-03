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
import { render, screen, cleanup, waitFor, fireEvent, within } from "@testing-library/react";
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

function renderPanel(hasOffsite: (domain: string) => boolean = () => false) {
  return render(<RetentionPreview t={t} hasOffsite={hasOffsite} />);
}

const policy = (over: Record<string, unknown> = {}) => ({
  on: true,
  keepLast: 0,
  keepDaily: 0,
  keepWeekly: 0,
  keepMonthly: 0,
  keepYearly: 0,
  own: false,
  ...over,
});

/** Answers the local and the off-site question each with its own preview. */
function answer(local: unknown, offsite: unknown = { ok: false, error: "unused" }) {
  previewRetention.mockImplementation((_domain: string, source?: string) =>
    Promise.resolve(source === "offsite" ? offsite : local)
  );
}

function show() {
  fireEvent.click(screen.getByRole("button", { name: new RegExp(en["retentionPreview.show"], "i") }));
}

/** The part of the answer about the local or the off-site copy. */
function part(name: string): HTMLElement {
  return screen.getByRole("region", { name });
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
    answer({
      ok: true,
      preview: {
        policy: policy({ keepLast: 1 }),
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
    show();

    await waitFor(() => expect(screen.getByText(/2b2b2b2b/)).toBeTruthy());
    expect(screen.getByText(/3c3c3c3c/)).toBeTruthy();
    expect(screen.queryByText(/1a1a1a1a/)).toBeNull();
    expect(screen.getByText(/container:plex/)).toBeTruthy();
  });

  it("says retention is off instead of showing an empty list", async () => {
    answer({ ok: true, preview: { policy: policy({ on: false }), repos: [{ name: "Containers", appendOnly: false, items: [] }] } });

    renderPanel();
    show();

    await waitFor(() => expect(screen.getByText(en["retentionPreview.off"])).toBeTruthy());
  });

  // The dangerous false positive, inverted: an append-only repository shows no
  // removals BECAUSE retention never runs there. Without the note, the empty
  // list reads as "nothing to do today".
  it("explains an append-only repository rather than leaving it blank", async () => {
    answer({ ok: true, preview: { policy: policy({ keepLast: 5 }), repos: [{ name: "Cold archive", appendOnly: true, items: [] }] } });

    renderPanel();
    show();

    await waitFor(() => expect(screen.getByText(en["retentionPreview.appendOnly"])).toBeTruthy());
    expect(screen.getByText(/Cold archive/)).toBeTruthy();
  });

  it("surfaces a repository the server could not cover", async () => {
    answer({
      ok: true,
      preview: {
        policy: policy({ keepLast: 5 }),
        repos: [{ name: "Containers", appendOnly: false, items: [] }],
        skipped: ["Cold archive (it was there before and is not reachable now)"],
      },
    });

    renderPanel();
    show();

    await waitFor(() => expect(screen.getByText(/not reachable now/)).toBeTruthy());
  });

  it("names its picker a source, as the rest of the Retention card does", () => {
    renderPanel();
    expect(screen.getByText(en["retentionPreview.sourceLabel"])).toBeTruthy();
    expect(screen.getByRole("combobox", { name: en["retentionPreview.sourceLabel"] })).toBeTruthy();
  });

  it("offers every source that keeps snapshots, ZFS among them", () => {
    renderPanel();
    fireEvent.click(screen.getByRole("combobox"));
    expect(screen.getAllByRole("option").map((o) => o.textContent)).toEqual([
      en["settings.containersEnabled"],
      en["settings.vmsEnabled"],
      en["settings.flashEnabled"],
      en["settings.configEnabled"],
      en["settings.filesEnabled"],
      en["settings.zfsEnabled"],
    ]);
  });

  it("asks the server about the local and the off-site copy of the domain the picker names", async () => {
    answer({ ok: false, error: "no backups yet" }, { ok: false, error: "no backups yet" });

    renderPanel((d) => d === "zfs");
    fireEvent.click(screen.getByRole("combobox"));
    fireEvent.click(screen.getByRole("option", { name: en["settings.zfsEnabled"] }));
    show();

    await waitFor(() => expect(previewRetention).toHaveBeenCalledTimes(2));
    expect(previewRetention).toHaveBeenCalledWith("zfs", undefined);
    expect(previewRetention).toHaveBeenCalledWith("zfs", "offsite");
  });

  it("says a source without an off-site repository has no off-site copy, and asks nothing about it", async () => {
    answer({ ok: true, preview: { policy: policy({ keepLast: 5 }), repos: [] } });

    renderPanel(() => false);
    show();

    await waitFor(() => expect(within(part(en["source.offsite"])).getByText(en["dashboard.noOffsite"])).toBeTruthy());
    expect(previewRetention).toHaveBeenCalledTimes(1);
    expect(previewRetention).toHaveBeenCalledWith("containers", undefined);
  });

  // A held item removes nothing because an open anomaly keeps its backups,
  // and the empty list must not read as a policy that has nothing to do.
  it("shows paused items", async () => {
    answer({
      ok: true,
      preview: {
        policy: policy({ keepLast: 1 }),
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
    show();

    const paused = await screen.findByText(en["retentionPreview.paused"]);
    expect(paused.className).toContain("text-statusWarn");
    expect(paused.closest("div.mt-2")?.textContent).toContain("container:plex");
    expect(screen.getAllByText(en["retentionPreview.paused"])).toHaveLength(1);
  });

  it("shows the server's refusal instead of an empty panel", async () => {
    answer({ ok: false, error: "no backups yet" });

    renderPanel();
    show();

    await waitFor(() => expect(screen.getByText(/no backups yet/)).toBeTruthy());
  });

  it("keeps the local answer when the off-site question fails", async () => {
    answer(
      { ok: true, preview: { policy: policy({ keepLast: 5 }), repos: [{ name: "Containers", appendOnly: false, items: [] }] } },
      { ok: false, error: "the off-site repository is not reachable" }
    );

    renderPanel(() => true);
    show();

    await waitFor(() =>
      expect(within(part(en["source.offsite"])).getByText(/not reachable/)).toBeTruthy()
    );
    expect(within(part(en["source.local"])).getByText(en["retentionPreview.nothing"])).toBeTruthy();
  });
});

// jsdom lays nothing out, so this pins the classes the phone layout rests on.
describe("RetentionPreview at phone width", () => {
  it("keeps the button inside the card and wraps a repository's note under its name", async () => {
    answer({ ok: true, preview: { policy: policy({ keepLast: 5 }), repos: [{ name: "Hetzner storage box", appendOnly: true, items: [] }] } });
    renderPanel();
    const button = screen.getByRole("button", { name: new RegExp(en["retentionPreview.show"], "i") });
    expect(button.className).toContain("glim-btn-wrap");
    fireEvent.click(button);
    const note = await screen.findByText(en["retentionPreview.appendOnly"]);
    expect(note.parentElement!.className).toContain("flex-wrap");
  });
});

describe("RetentionPreview's policy lines", () => {
  it("names each copy's rules, its own or the shared ones", async () => {
    answer(
      { ok: true, preview: { policy: policy({ keepDaily: 7, keepYearly: 2, own: true }), repos: [] } },
      { ok: true, preview: { policy: policy({ keepLast: 7 }), repos: [] } }
    );
    renderPanel(() => true);
    show();

    await waitFor(() =>
      expect(
        within(part(en["source.local"])).getByText(
          `${en["retentionPreview.ownPolicy"]}: ${en["settings.retentionDaily"]} 7 · ${en["settings.retentionYearly"]} 2`
        )
      ).toBeTruthy()
    );
    expect(
      within(part(en["source.offsite"])).getByText(`${en["retentionPreview.sharedPolicy"]}: ${en["settings.retentionLast"]} 7`)
    ).toBeTruthy();
  });

  it("names a source's own off-site rules", async () => {
    answer(
      { ok: true, preview: { policy: policy({ keepLast: 5 }), repos: [] } },
      { ok: true, preview: { policy: policy({ keepMonthly: 12, own: true }), repos: [] } }
    );
    renderPanel(() => true);
    show();

    await waitFor(() =>
      expect(
        within(part(en["source.offsite"])).getByText(`${en["retentionPreview.ownPolicy"]}: ${en["settings.retentionMonthly"]} 12`)
      ).toBeTruthy()
    );
    expect(
      within(part(en["source.local"])).getByText(`${en["retentionPreview.sharedPolicy"]}: ${en["settings.retentionLast"]} 5`)
    ).toBeTruthy();
  });
});
