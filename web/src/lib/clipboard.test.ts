// #112 — the copy helper must succeed via the async API in secure contexts,
// fall back to execCommand elsewhere, and report false instead of throwing
// when neither path exists (so buttons never silently "do nothing" again —
// callers simply skip the "copied" feedback).
import { afterEach, describe, expect, it, vi } from "vitest";
import { copyText } from "./clipboard";

describe("copyText", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("uses the async clipboard API when present", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("navigator", { clipboard: { writeText } });
    expect(await copyText("hello")).toBe(true);
    expect(writeText).toHaveBeenCalledWith("hello");
  });

  it("falls back to execCommand when the API is missing", async () => {
    vi.stubGlobal("navigator", {});
    const ta = {
      value: "",
      style: {} as Record<string, string>,
      setAttribute: vi.fn(),
      select: vi.fn(),
      remove: vi.fn(),
    };
    const doc = {
      createElement: vi.fn().mockReturnValue(ta),
      body: { appendChild: vi.fn() },
      execCommand: vi.fn().mockReturnValue(true),
    };
    vi.stubGlobal("document", doc);
    expect(await copyText("fallback")).toBe(true);
    expect(ta.value).toBe("fallback");
    expect(ta.select).toHaveBeenCalled();
    expect(doc.execCommand).toHaveBeenCalledWith("copy");
    expect(ta.remove).toHaveBeenCalled();
  });

  it("returns false without throwing when nothing is available", async () => {
    vi.stubGlobal("navigator", {});
    vi.stubGlobal("document", undefined);
    expect(await copyText("nope")).toBe(false);
  });
});
