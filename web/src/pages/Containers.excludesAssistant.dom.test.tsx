// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// Exclusion assistant, size presentation (issue #175).
//
// The reporter was shown "5.7 GB" next to a folder that actually backs up
// 55 GB. The number was not wrong by accident: the scan had run out of time
// part-way through that subtree, and the panel rendered the fraction it had
// reached with the same plain humanBytes() it uses for a finished total. There
// was nothing in the row to tell the two apart.
//
// Backend-side the fix is a per-candidate `complete` flag. What this file
// guards is the half that the user actually sees: a row whose size is only a
// LOWER BOUND must never render as a bare size, and a finished row must not
// pick up a hedge it does not need. Both appear in the SAME list here, because
// a truncated scan produces exactly that mixture and rendering them
// identically is the whole defect.
//
// The second test covers the other new state: the backup index could not be
// read, so the panel offers a folder scan and that retry must actually ask for
// source=live (without it the retry would silently repeat the failing request).
//
// jsdom opted in explicitly (render + click + async fetch) — see
// Selector.dom.test.tsx's header for this repo's naming convention.
// ---------------------------------------------------------------------------
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, useT } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { ExcludeSuggestion } from "../lib/api";

type SuggestReply = {
  ok: boolean;
  suggestions: ExcludeSuggestion[];
  truncated: boolean;
  source?: "snapshot" | "live";
  snapshotTime?: string;
  stoppedAt?: string;
  indexFailed?: boolean;
};

const suggestCalls: (string | undefined)[] = [];
let replies: SuggestReply[] = [];

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    previewContainerExcludes: () => Promise.resolve({ ok: true, preview: [] }),
    setContainerExcludes: () => Promise.resolve({ ok: true }),
    suggestContainerExcludes: (_name: string, source?: "live") => {
      suggestCalls.push(source);
      return Promise.resolve(replies.shift() ?? { ok: true, suggestions: [], truncated: false });
    },
  };
});

// Imported AFTER vi.mock so the component picks up the mocked client.
const { ExcludesEditor } = await import("./Containers");

function Harness() {
  const { t } = useT();
  return <ExcludesEditor name="plex" initial={[]} open t={t} />;
}

/** Renders the editor and opens the assistant, which triggers its first scan. */
async function openAssistant() {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <Harness />
        </ToastProvider>
      </I18nProvider>
    );
  });
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: "Exclusion assistant" }));
  });
}

beforeEach(() => {
  localStorage.setItem("bv-lang", "en");
  suggestCalls.length = 0;
  replies = [];
});

afterEach(() => {
  cleanup();
  localStorage.removeItem("bv-lang");
});

describe("exclusion assistant size presentation", () => {
  it("renders a lower bound as a minimum and a finished size plainly, in the same list", async () => {
    replies = [
      {
        ok: true,
        truncated: true,
        source: "live",
        stoppedAt: "/config/Media",
        suggestions: [
          // The reporter's row: the walk stopped inside it, so 5.7 GB is a floor.
          { path: "Media", line: "/config/Media", sizeBytes: 5_700_000_000, reason: "large", complete: false },
          // A sibling the walk finished: exact, and presented as exact.
          { path: "Cache", line: "/config/Cache", sizeBytes: 1_048_576, reason: "known-cache", complete: true },
        ],
      },
    ];
    await openAssistant();

    // The partial row hedges, and carries the explanation on hover.
    const atLeast = screen.getByText(/at least/);
    expect(atLeast.textContent).toBe("at least 5.3 GB");
    expect(atLeast.getAttribute("title")).toBe(
      "The scan stopped inside this folder, so this size is a minimum."
    );
    // …and nothing anywhere renders that number as a finished total.
    expect(screen.queryByText("5.3 GB")).toBeNull();

    // The finished row is a plain size, with no hedge bolted onto it.
    const exact = screen.getByText("1.0 MB");
    expect(exact.textContent).toBe("1.0 MB");
    expect(exact.getAttribute("title")).toBeNull();

    // The list-level banner names where the scan ran out, which no per-row flag
    // can express: a folder never visited has no row at all.
    expect(
      screen.getByText("The scan hit its time limit inside /config/Media. Folders after it were not examined.")
    ).toBeTruthy();
    // A live scan says so, so an exact-looking number is never mistaken for
    // one taken from the backup.
    expect(
      screen.getByText("This container has no backup yet, so the sizes come from a live scan of the folders.")
    ).toBeTruthy();
  });

  it("states the snapshot's date next to sizes taken from a backup", async () => {
    replies = [
      {
        ok: true,
        truncated: false,
        source: "snapshot",
        snapshotTime: "2026-08-01T10:00:00Z",
        suggestions: [
          { path: "Cache", line: "/config/Cache", sizeBytes: 2_097_152, reason: "known-cache", complete: true },
        ],
      },
    ];
    await openAssistant();

    // As-of-last-backup sizes are only honest WITH their date, so the source
    // line must carry a rendered timestamp, not an empty placeholder.
    const when = new Date("2026-08-01T10:00:00Z").toLocaleString();
    expect(screen.getByText(`Sizes come from the backup of ${when}, so they are exact.`)).toBeTruthy();
    expect(screen.getByText("2.0 MB")).toBeTruthy();
    expect(screen.queryByText(/at least/)).toBeNull();
  });

  it("offers a folder scan when the backup index cannot be read, and asks for source=live", async () => {
    replies = [
      { ok: true, truncated: false, source: "snapshot", indexFailed: true, suggestions: [] },
      {
        ok: true,
        truncated: false,
        source: "live",
        suggestions: [
          { path: "Cache", line: "/config/Cache", sizeBytes: 1024, reason: "known-cache", complete: true },
        ],
      },
    ];
    await openAssistant();

    expect(screen.getByText("Could not finish reading the backup index.")).toBeTruthy();
    // The failure is stated, not swallowed into "nothing found".
    expect(screen.queryByText(/Nothing left to exclude/)).toBeNull();

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Scan the folders instead" }));
    });

    expect(suggestCalls).toEqual([undefined, "live"]);
    expect(screen.getByText("1.0 KB")).toBeTruthy();
    expect(screen.queryByText("Could not finish reading the backup index.")).toBeNull();
  });
});
