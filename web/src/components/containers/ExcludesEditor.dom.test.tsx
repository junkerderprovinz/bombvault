// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { en, I18nProvider, useT } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import type { ExcludePreview } from "./ExcludesEditor";

const saved: string[][] = [];
let answer: { ok: boolean; error?: string } = { ok: true };

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    previewContainerExcludes: () => Promise.resolve({ ok: true, preview: [] }),
    suggestContainerExcludes: () => Promise.resolve({ ok: true, suggestions: [], truncated: false }),
    setContainerExcludes: (_name: string, lines: string[]) => {
      saved.push(lines);
      return Promise.resolve(answer);
    },
  };
});

const { ExcludesEditor } = await import("./ExcludesEditor");

function Harness({ initial, preview }: { initial: string[]; preview?: ExcludePreview }) {
  const { t } = useT();
  return <ExcludesEditor name="plex" initial={initial} open t={t} preview={preview} />;
}

function renderEditor(initial: string[] = [], preview?: ExcludePreview) {
  render(
    <I18nProvider>
      <ToastProvider>
        <Harness initial={initial} preview={preview} />
      </ToastProvider>
    </I18nProvider>,
  );
}

function field() {
  return screen.getByRole("textbox") as HTMLTextAreaElement;
}

async function settle() {
  await act(async () => {
    vi.advanceTimersByTime(1000);
    await Promise.resolve();
  });
}

async function removeChip(line: string) {
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: "Exclusion assistant" }));
    await Promise.resolve();
  });
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: `Remove exclusion ${line}` }));
    await Promise.resolve();
  });
}

beforeEach(() => {
  localStorage.setItem("bv-lang", "en");
  saved.length = 0;
  answer = { ok: true };
  vi.useFakeTimers();
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  localStorage.removeItem("bv-lang");
});

describe("exclusions editor", () => {
  it("keeps the line just started when the save of the one before it comes back", async () => {
    renderEditor();
    fireEvent.change(field(), { target: { value: "/config/cache\n" } });
    await settle();
    expect(saved).toEqual([["/config/cache"]]);
    expect(field().value).toBe("/config/cache\n");

    fireEvent.change(field(), { target: { value: "/config/cache\n*.log" } });
    await settle();
    expect(saved.at(-1)).toEqual(["/config/cache", "*.log"]);
  });

  it("saves what was typed just before the editor goes away", () => {
    renderEditor();
    fireEvent.change(field(), { target: { value: "/config/cache" } });
    cleanup();
    expect(saved).toEqual([["/config/cache"]]);
  });

  it("shows a removed chip's list in the field and saves it", async () => {
    renderEditor(["/config/cache", "*.log"]);
    await removeChip("/config/cache");
    expect(saved).toEqual([["*.log"]]);
    expect(field().value).toBe("*.log");
  });

  it("says a line for a folder outside Host Data turns off its warning", async () => {
    const preview: ExcludePreview = (_name, lines) =>
      Promise.resolve(
        lines.map((raw) =>
          raw === "/cache"
            ? { raw, resolved: raw, status: "outside", matches: false }
            : { raw, resolved: raw, status: "passthrough", matches: false },
        ),
      );
    renderEditor(["/cache", "/elsewhere"], preview);
    await settle();
    expect(screen.getByText(en["excludes.outsideHostData"], { exact: false }).textContent).toContain("✓");
    expect(screen.getByText(en["excludes.noMatch"], { exact: false }).textContent).toContain("⚠");
  });

  it("says a host path outside Host Data turns off its warning instead of going to restic", async () => {
    const host = "/var/lib/docker/volumes/jellyfin_cache/_data";
    const preview: ExcludePreview = (_name, lines) =>
      Promise.resolve(lines.map((raw) => ({ raw, resolved: raw, status: "outside", matches: false })));
    renderEditor([host], preview);
    await settle();
    expect(screen.getByText(en["excludes.outsideHostData"], { exact: false }).textContent).toContain("✓");
    expect(screen.queryByText(en["excludes.noMatch"], { exact: false })).toBeNull();
  });

  it("warns that a line which would leave out a whole backup folder is ignored", async () => {
    const preview: ExcludePreview = (_name, lines) =>
      Promise.resolve(lines.map((raw) => ({ raw, resolved: raw, status: "root", matches: false })));
    renderEditor(["/host/user"], preview);
    await settle();
    expect(screen.getByText(en["excludes.excludesRoot"], { exact: false }).textContent).toContain("⚠");
    expect(screen.queryByText(en["excludes.noMatch"], { exact: false })).toBeNull();
  });

  it("puts the warning under a long line on a phone instead of beside it", async () => {
    const long = "/config/Library/Application Support/Plex Media Server/Cache/PhotoTranscoder";
    const preview: ExcludePreview = (_name, lines) =>
      Promise.resolve(lines.map((raw) => ({ raw, resolved: raw, status: "root", matches: false })));
    renderEditor([long], preview);
    await settle();
    const row = screen.getByTestId("exclude-preview-row");
    const classes = row.className.split(" ");
    expect(classes).toContain("flex-col");
    expect(classes).toContain("md:flex-row");
    expect(classes).not.toContain("items-baseline");
    expect((row.firstElementChild as HTMLElement).className).toContain("wrap-anywhere");
  });

  it("warns that a line which would exclude everything is ignored", async () => {
    const preview: ExcludePreview = (_name, lines) =>
      Promise.resolve(lines.map((raw) => ({ raw, resolved: raw, status: "everything", matches: false })));
    renderEditor(["/"], preview);
    await settle();
    expect(screen.getByText(en["excludes.excludesEverything"], { exact: false }).textContent).toContain("⚠");
  });

  it("puts the field back when removing a chip is refused", async () => {
    answer = { ok: false, error: "refused" };
    renderEditor(["/config/cache", "*.log"]);
    await removeChip("/config/cache");
    expect(field().value).toBe("/config/cache\n*.log");
  });
});
