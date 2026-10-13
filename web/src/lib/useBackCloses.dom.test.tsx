// @vitest-environment jsdom
import { useState } from "react";
import { afterEach, describe, expect, it } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { BrowserRouter, useSearchParams } from "react-router-dom";
import { leaveLayers, useBackCloses } from "./useBackCloses";

function Layer({ name, onClose }: { name: string; onClose: () => void }) {
  useBackCloses(true, onClose);
  return (
    <div>
      <span>{name}</span>
      <button type="button" onClick={onClose}>
        close {name}
      </button>
    </div>
  );
}

function Page() {
  const [outer, setOuter] = useState(false);
  const [inner, setInner] = useState(false);
  return (
    <>
      <button type="button" onClick={() => setOuter(true)}>
        open outer
      </button>
      {outer && <Layer name="outer" onClose={() => setOuter(false)} />}
      {outer && (
        <button type="button" onClick={() => setInner(true)}>
          open inner
        </button>
      )}
      {inner && <Layer name="inner" onClose={() => setInner(false)} />}
    </>
  );
}

// A window its URL opens, as the emergency on the Restore page.
function UrlWindow() {
  const [params, setParams] = useSearchParams();
  return (
    <>
      <button type="button" onClick={() => setParams({ window: "open" }, { replace: true })}>
        open by url
      </button>
      {params.get("window") === "open" && (
        <Layer name="by url" onClose={() => leaveLayers(() => setParams({}, { replace: true }))} />
      )}
    </>
  );
}

function renderOnPage() {
  window.history.pushState(null, "", "/before");
  window.history.pushState(null, "", "/page");
  render(
    <BrowserRouter>
      <UrlWindow />
    </BrowserRouter>
  );
}

const back = () => act(() => window.history.back());
const settled = () => new Promise((resolve) => setTimeout(resolve, 20));

afterEach(async () => {
  cleanup();
  await settled();
});

describe("useBackCloses", () => {
  it("closes the open layer on Back and leaves the page where it was", async () => {
    render(<Page />);
    const before = window.history.length;
    fireEvent.click(screen.getByText("open outer"));
    expect(window.history.length).toBe(before + 1);
    back();
    await waitFor(() => expect(screen.queryByText("outer")).toBeNull());
    expect(screen.getByText("open outer")).toBeTruthy();
  });

  it("closes the top layer first when two are open", async () => {
    render(<Page />);
    fireEvent.click(screen.getByText("open outer"));
    fireEvent.click(screen.getByText("open inner"));
    back();
    await waitFor(() => expect(screen.queryByText("inner")).toBeNull());
    expect(screen.getByText("outer")).toBeTruthy();
    back();
    await waitFor(() => expect(screen.queryByText("outer")).toBeNull());
  });

  it("takes its entry back when the layer is closed by hand", async () => {
    render(<Page />);
    const state = window.history.state;
    fireEvent.click(screen.getByText("open outer"));
    expect(window.history.state).not.toEqual(state);
    fireEvent.click(screen.getByText("close outer"));
    await waitFor(() => expect(window.history.state).toEqual(state));
  });

  it("gives a layer opened right after another closed an entry of its own", async () => {
    render(<Page />);
    fireEvent.click(screen.getByText("open outer"));
    fireEvent.click(screen.getByText("close outer"));
    fireEvent.click(screen.getByText("open outer"));
    await settled();
    expect(screen.getByText("outer")).toBeTruthy();
    back();
    await waitFor(() => expect(screen.queryByText("outer")).toBeNull());
  });

  it("does not open a window its URL opened again on Back after it was closed by hand", async () => {
    renderOnPage();
    fireEvent.click(screen.getByText("open by url"));
    await settled();
    fireEvent.click(screen.getByText("close by url"));
    await waitFor(() => expect(screen.queryByText("by url")).toBeNull());
    expect(window.location.search).toBe("");
    back();
    await waitFor(() => expect(window.location.pathname).toBe("/before"));
    expect(screen.queryByText("by url")).toBeNull();
  });

  it("closes a window its URL opened on Back and leaves the page's entry as it was", async () => {
    renderOnPage();
    fireEvent.click(screen.getByText("open by url"));
    await settled();
    back();
    await waitFor(() => expect(screen.queryByText("by url")).toBeNull());
    expect(window.location.pathname).toBe("/page");
    expect(window.location.search).toBe("");
    back();
    await waitFor(() => expect(window.location.pathname).toBe("/before"));
  });
});
