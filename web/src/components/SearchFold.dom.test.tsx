// @vitest-environment jsdom
import { useState } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { SearchFold } from "./SearchFold";

function Host({ initial = "" }: { initial?: string }) {
  const [value, setValue] = useState(initial);
  return (
    <>
      <SearchFold value={value} onChange={setValue} label="Search" placeholder="Search by name" />
      <output>{value}</output>
    </>
  );
}

const button = () => screen.queryByRole("button", { name: "Search" });
const field = () => screen.queryByRole("searchbox", { name: "Search" });

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("SearchFold", () => {
  it("waits as a button", () => {
    render(<Host />);
    expect(button()).not.toBeNull();
    expect(field()).toBeNull();
  });

  it("opens into a focused field on a press", () => {
    render(<Host />);
    fireEvent.click(button()!);
    expect(button()).toBeNull();
    expect(document.activeElement).toBe(field());
  });

  it("hands every keystroke to its owner", () => {
    render(<Host />);
    fireEvent.click(button()!);
    fireEvent.change(field()!, { target: { value: "plex" } });
    expect(screen.getByRole("status").textContent).toBe("plex");
  });

  it("folds back a moment after it is left empty, so a press beside it still lands", () => {
    render(<Host />);
    fireEvent.click(button()!);
    fireEvent.blur(field()!);
    expect(field()).not.toBeNull();
    act(() => {
      vi.advanceTimersByTime(300);
    });
    expect(field()).toBeNull();
    expect(button()).not.toBeNull();
  });

  it("stays open when focus comes back before it folds", () => {
    render(<Host />);
    fireEvent.click(button()!);
    fireEvent.blur(field()!);
    fireEvent.focus(field()!);
    act(() => {
      vi.advanceTimersByTime(300);
    });
    expect(field()).not.toBeNull();
  });

  it("stays open while it holds a query, focused or not", () => {
    render(<Host initial="plex" />);
    expect(field()).not.toBeNull();
    fireEvent.blur(field()!);
    act(() => {
      vi.advanceTimersByTime(300);
    });
    expect(field()).not.toBeNull();
  });

  it("clears on Escape and leaves the Escape of an empty field to whatever is around it", () => {
    const outer = vi.fn();
    render(
      <div onKeyDown={outer}>
        <Host initial="plex" />
      </div>
    );
    fireEvent.focus(field()!);
    fireEvent.keyDown(field()!, { key: "Escape" });
    expect(screen.getByRole("status").textContent).toBe("");
    expect(outer).not.toHaveBeenCalled();
    fireEvent.keyDown(field()!, { key: "Escape" });
    expect(outer).toHaveBeenCalledTimes(1);
  });
});
