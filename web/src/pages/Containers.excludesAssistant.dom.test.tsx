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
  error?: string;
  suggestions: ExcludeSuggestion[];
  truncated: boolean;
  source?: "snapshot" | "live";
  liveReason?: "no-snapshot" | "requested" | "not-in-snapshot";
  snapshotTime?: string;
  stoppedAt?: string;
  unexaminedRoots?: string[];
  unreadableRoots?: string[];
  pathsUnavailable?: boolean;
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

    // The partial row hedges, and carries the explanation in an InfoBubble —
    // a native title= is invisible on touch, and the house rule puts an
    // explanation behind an inline (i) rather than in permanent grey prose.
    const atLeast = screen.getByText(/at least/);
    expect(atLeast.textContent).toBe("at least 5.3 GB");
    expect(
      screen.getByLabelText("The scan stopped inside this folder, so this size is a minimum.")
    ).toBeTruthy();
    // …and nothing anywhere renders that number as a finished total.
    expect(screen.queryByText("5.3 GB")).toBeNull();

    // The finished row is a plain size, with no hedge bolted onto it.
    const exact = screen.getByText("1.0 MB");
    expect(exact.textContent).toBe("1.0 MB");
    expect(exact.getAttribute("title")).toBeNull();

    // The caveat may not be the least legible text in its own row: both figures
    // carry the same tone (the hedge used to be text-carbon-textMuted while the
    // exact one was text-carbon-textSub).
    expect(atLeast.className).toContain("text-carbon-textSub");
    expect(exact.className).toContain("text-carbon-textSub");

    // The list-level banner names where the scan ran out, which no per-row flag
    // can express: a folder never visited has no row at all. (The folder itself
    // is its own dir="ltr" span, so the sentence is read off the parent.)
    expect(screen.getByText("/config/Media").parentElement?.textContent).toBe(
      "The scan hit its time limit inside /config/Media. Folders after it were not examined."
    );
    // A live scan says so, so an exact-looking number is never mistaken for
    // one taken from the backup.
    expect(
      screen.getByText("This container has no backup yet, so the sizes come from a live scan of the folders.")
    ).toBeTruthy();
  });

  // The banner interpolates a runtime path into translated prose. `/` is a weak
  // bidi class, so in ar/he/fa an unisolated leading slash migrates to the far
  // end of the path and the user is shown a folder they cannot type back. The
  // row above it already does this right; the banner did not.
  it("pins the truncation banner's folder to LTR so RTL locales do not mangle it", async () => {
    replies = [
      {
        ok: true,
        truncated: true,
        source: "live",
        liveReason: "no-snapshot",
        stoppedAt: "/config/Media",
        suggestions: [],
      },
    ];
    await openAssistant();

    const path = screen.getByText("/config/Media");
    expect(path.tagName).toBe("SPAN");
    expect(path.getAttribute("dir")).toBe("ltr");
    // The surrounding sentence stays in the page's own direction: only the
    // technical value is pinned, never the translated prose around it.
    expect(path.parentElement?.getAttribute("dir")).toBeNull();
    expect(path.parentElement?.textContent).toBe(
      "The scan hit its time limit inside /config/Media. Folders after it were not examined."
    );
  });

  // Backup folders that produced no rows at all. No per-row flag can speak for
  // them, and with several roots their silence read as a finished scan of all
  // of them.
  it("names backup folders it never opened and ones it could not read", async () => {
    replies = [
      {
        ok: true,
        truncated: true,
        source: "live",
        liveReason: "no-snapshot",
        stoppedAt: "/config/Media",
        unexaminedRoots: ["/data"],
        unreadableRoots: ["/config/Locked"],
        suggestions: [],
      },
    ];
    await openAssistant();

    expect(screen.getByText(/were not examined at all/).textContent).toBe(
      "These backup folders were not examined at all: /data"
    );
    expect(screen.getByText(/could not be read/).textContent).toBe(
      "These backup folders could not be read, so anything inside them is missing from this list: /config/Locked"
    );
    // Both name their folders LTR, same reason as the banner above.
    expect(screen.getByText("/data").getAttribute("dir")).toBe("ltr");
    expect(screen.getByText("/config/Locked").getAttribute("dir")).toBe("ltr");
  });

  // An unmounted array with no backup to fall back on. "Nothing left to
  // exclude" is a positive finding and may only be said when the scan looked.
  it("does not claim there is nothing to exclude when it could not look", async () => {
    replies = [
      {
        ok: true,
        truncated: false,
        source: "live",
        liveReason: "no-snapshot",
        pathsUnavailable: true,
        suggestions: [],
      },
    ];
    await openAssistant();

    expect(screen.queryByText(/Nothing left to exclude/)).toBeNull();
    // Two separate facts, carried by two separate lines. The unreachable-folders
    // sentence no longer asserts anything about backups: it is also shown when
    // the user asks for a live scan on a container that HAS one, and claiming
    // otherwise there was its own lie. The "no backup yet" half is the reason
    // line's job, so this pins both rather than one merged sentence.
    expect(
      screen.getByText(
        "This container's backup folders cannot be reached right now. Check that the array or share holding them is mounted."
      )
    ).toBeTruthy();
    expect(
      screen.getByText(
        "This container has no backup yet, so the sizes come from a live scan of the folders."
      )
    ).toBeTruthy();
  });

  // A truncation banner describes ONE scan: the one whose list is on screen. A
  // rescan that fails leaves no list, so the old banner would sit above an
  // empty panel describing a scan that no longer exists.
  it("clears the truncation banner when a rescan fails", async () => {
    replies = [
      {
        ok: true,
        truncated: true,
        source: "live",
        liveReason: "no-snapshot",
        stoppedAt: "/config/Media",
        unexaminedRoots: ["/data"],
        suggestions: [
          { path: "Media", line: "/config/Media", sizeBytes: 1024, reason: "large", complete: false },
        ],
      },
      { ok: false, error: "Scan failed", truncated: false, suggestions: [] },
    ];
    await openAssistant();
    expect(screen.getByText(/The scan hit its time limit/)).toBeTruthy();

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Rescan" }));
    });

    expect(screen.queryByText(/The scan hit its time limit/)).toBeNull();
    expect(screen.queryByText(/were not examined at all/)).toBeNull();
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

    // …and it must not then claim the container has never been backed up. The
    // index read was only attempted BECAUSE there is a backup.
    expect(
      screen.queryByText("This container has no backup yet, so the sizes come from a live scan of the folders.")
    ).toBeNull();
    expect(
      screen.getByText("The sizes come from a scan of the folders as they are right now.")
    ).toBeTruthy();
  });

  // The last backup covers folders the user has since swapped out. Neither "no
  // backup yet" nor "you asked for this" is true; say the actual thing.
  it("says the last backup does not cover the current folder selection", async () => {
    replies = [
      {
        ok: true,
        truncated: false,
        source: "live",
        liveReason: "not-in-snapshot",
        suggestions: [
          { path: "Cache", line: "/config/Cache", sizeBytes: 1024, reason: "known-cache", complete: true },
        ],
      },
    ];
    await openAssistant();

    expect(
      screen.getByText(
        "The last backup does not cover the folders selected now, so the sizes come from a live scan of the folders."
      )
    ).toBeTruthy();
    expect(
      screen.queryByText("This container has no backup yet, so the sizes come from a live scan of the folders.")
    ).toBeNull();
  });

  // "What can I stop backing up" is a question about the disk as it is now. A
  // snapshot cannot answer it: a junk folder created since the last backup has
  // no row and no warning, because it is not in the index at all.
  it("offers a live folder scan on a snapshot result, without needing a failure first", async () => {
    replies = [
      {
        ok: true,
        truncated: false,
        source: "snapshot",
        snapshotTime: "2026-08-01T10:00:00Z",
        suggestions: [
          { path: "Cache", line: "/config/Cache", sizeBytes: 2048, reason: "known-cache", complete: true },
        ],
      },
      {
        ok: true,
        truncated: false,
        source: "live",
        liveReason: "requested",
        suggestions: [
          { path: "Junk", line: "/config/Junk", sizeBytes: 4096, reason: "large", complete: true },
        ],
      },
    ];
    await openAssistant();

    // The snapshot is old enough that its list cannot describe the disk today,
    // and the panel says so rather than leaving the user to read a date.
    expect(
      screen.getByText(
        "This backup is more than a day old, so anything created since then is missing from this list."
      )
    ).toBeTruthy();

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Check the folders as they are now" }));
    });

    expect(suggestCalls).toEqual([undefined, "live"]);
    // The folder that did not exist when the backup ran is now on the list.
    expect(screen.getByText("Junk")).toBeTruthy();
    // A live result has nothing to offer a live scan for, so the button is gone.
    expect(screen.queryByRole("button", { name: "Check the folders as they are now" })).toBeNull();
  });

  // The same control must NOT nag on a backup made minutes ago: the stale
  // warning is only honest when the list can actually be out of date.
  it("does not call a fresh backup stale", async () => {
    replies = [
      {
        ok: true,
        truncated: false,
        source: "snapshot",
        snapshotTime: new Date(Date.now() - 60_000).toISOString(),
        suggestions: [
          { path: "Cache", line: "/config/Cache", sizeBytes: 2048, reason: "known-cache", complete: true },
        ],
      },
    ];
    await openAssistant();

    expect(screen.queryByText(/more than a day old/)).toBeNull();
    // The live scan stays available regardless: it answers a different question.
    expect(screen.getByRole("button", { name: "Check the folders as they are now" })).toBeTruthy();
  });
});
