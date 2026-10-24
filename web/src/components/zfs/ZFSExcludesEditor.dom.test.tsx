// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { en, I18nProvider, useT } from "../../lib/i18n";
import type { ZFSDatasetView } from "../../lib/api";
import { ToastProvider } from "../../lib/toast";

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    previewZFSExcludes: (_id: string, lines: string[]) =>
      Promise.resolve({
        ok: true,
        rows: lines.map((pattern) =>
          pattern === "*.tmp"
            ? { pattern, matches: 2, sample: ["/a.tmp", "/b.tmp"] }
            : pattern === "/plex/*"
              ? { pattern, matches: 0, sample: [], root: true }
              : { pattern, matches: 0, sample: [], everything: true },
        ),
      }),
    patchZFSDataset: () => Promise.resolve({ ok: true }),
  };
});

const { ZFSExcludesEditor } = await import("./ZFSExcludesEditor");

function Harness() {
  const { t } = useT();
  const item = { id: "zfs-1", excludes: ["/", "*.tmp", "/plex/*"] } as unknown as ZFSDatasetView;
  return <ZFSExcludesEditor item={item} t={t} onSaved={() => undefined} />;
}

beforeEach(() => {
  localStorage.setItem("bv-lang", "en");
  vi.useFakeTimers();
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  localStorage.removeItem("bv-lang");
});

describe("ZFS exclude editor", () => {
  it("warns that a line which would exclude everything is ignored", async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <Harness />
        </ToastProvider>
      </I18nProvider>,
    );
    await act(async () => {
      vi.advanceTimersByTime(1000);
      await Promise.resolve();
    });
    const warning = screen.getByText(en["excludes.excludesEverything"], { exact: false });
    expect(warning.textContent).toContain("/");
    expect(screen.getAllByText(en["excludes.excludesEverything"], { exact: false })).toHaveLength(1);
  });

  it("warns that a line which would leave a whole dataset out is ignored", async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <Harness />
        </ToastProvider>
      </I18nProvider>,
    );
    await act(async () => {
      vi.advanceTimersByTime(1000);
      await Promise.resolve();
    });
    const warning = screen.getByText(en["excludes.excludesDataset"], { exact: false });
    expect(warning.textContent).toContain("/plex/*");
    expect(warning.textContent).toContain("⚠");
  });
});
