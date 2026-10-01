// @vitest-environment jsdom
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { useRevealOnScrollUp } from "./revealOnScrollUp";

function Bar({ page, pinned = false }: { page: string; pinned?: boolean }) {
  const { revealed, barRef } = useRevealOnScrollUp(page, pinned);
  return revealed ? <div ref={barRef}>search</div> : null;
}

let main: HTMLElement;

beforeEach(() => {
  main = document.createElement("main");
  main.id = "bv-main";
  document.body.appendChild(main);
  vi.spyOn(performance, "now").mockReturnValue(10_000);
});

afterEach(() => {
  cleanup();
  main.remove();
  vi.restoreAllMocks();
});

const wheel = (deltaY: number) => act(() => void main.dispatchEvent(new WheelEvent("wheel", { deltaY })));

it("stays hidden until the reader scrolls up", () => {
  render(<Bar page="/settings/general" />);
  expect(screen.queryByText("search")).toBeNull();
  wheel(40);
  expect(screen.queryByText("search")).toBeNull();
  wheel(-40);
  expect(screen.queryByText("search")).not.toBeNull();
});

it("goes away on a scroll down and on a page change", () => {
  const { rerender } = render(<Bar page="/settings/general" />);
  wheel(-40);
  wheel(40);
  expect(screen.queryByText("search")).toBeNull();
  wheel(-40);
  rerender(<Bar page="/settings/look" />);
  expect(screen.queryByText("search")).toBeNull();
});

it("ignores a wheel tick too small to be a direction", () => {
  render(<Bar page="/settings/general" />);
  wheel(-3);
  expect(screen.queryByText("search")).toBeNull();
});

it("stays while it is in use", () => {
  const { rerender } = render(<Bar page="/settings/general" />);
  wheel(-40);
  rerender(<Bar page="/settings/general" pinned />);
  wheel(40);
  expect(screen.queryByText("search")).not.toBeNull();
});

it("does not answer the scroll a page change makes itself", () => {
  vi.spyOn(performance, "now").mockReturnValue(0);
  render(<Bar page="/settings/general" />);
  main.scrollTop = 300;
  act(() => void main.dispatchEvent(new Event("scroll")));
  main.scrollTop = 0;
  act(() => void main.dispatchEvent(new Event("scroll")));
  expect(screen.queryByText("search")).toBeNull();
});
