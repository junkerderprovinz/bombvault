// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, useT } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";

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

function Harness({ initial }: { initial: string[] }) {
  const { t } = useT();
  return <ExcludesEditor name="plex" initial={initial} open t={t} />;
}

function renderEditor(initial: string[] = []) {
  render(
    <I18nProvider>
      <ToastProvider>
        <Harness initial={initial} />
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

  it("puts the field back when removing a chip is refused", async () => {
    answer = { ok: false, error: "refused" };
    renderEditor(["/config/cache", "*.log"]);
    await removeChip("/config/cache");
    expect(field().value).toBe("/config/cache\n*.log");
  });
});
