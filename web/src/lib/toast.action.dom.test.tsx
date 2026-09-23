// @vitest-environment jsdom
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { ToastProvider, useToast } from "./toast";
import { ACTION_TOAST_DURATION_MS, TOAST_DURATION_MS, type ToastAction } from "./toastEngine";

function Pusher({ message, action }: { message: string; action?: ToastAction }) {
  const { push } = useToast();
  return <button onClick={() => push(message, "success", action)}>push</button>;
}

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

it("keeps a toast with an action on screen past the plain duration, then lets it go", () => {
  render(
    <ToastProvider>
      <Pusher message="radarr-movies now belongs to radarr." action={{ label: "Undo", onClick: () => {} }} />
    </ToastProvider>,
  );
  fireEvent.click(screen.getByText("push"));

  act(() => vi.advanceTimersByTime(TOAST_DURATION_MS + 1));
  expect(screen.getByRole("button", { name: "Undo" })).toBeTruthy();

  act(() => vi.advanceTimersByTime(ACTION_TOAST_DURATION_MS - TOAST_DURATION_MS));
  expect(screen.queryByRole("button", { name: "Undo" })).toBeNull();
});

it("lets a plain toast go after the plain duration", () => {
  render(
    <ToastProvider>
      <Pusher message="Saved." />
    </ToastProvider>,
  );
  fireEvent.click(screen.getByText("push"));
  expect(screen.getByText("Saved.")).toBeTruthy();

  act(() => vi.advanceTimersByTime(TOAST_DURATION_MS + 1));
  expect(screen.queryByText("Saved.")).toBeNull();
});
