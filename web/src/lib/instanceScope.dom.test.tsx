// @vitest-environment jsdom
// InstanceProvider carries the ?instance= scope through the URL, and keeps
// api.ts's own module-level scope (the one fetchJSON actually reads) in step
// with it, so a call made anywhere in the tree lands on the right instance
// without every one of api.ts's ~150 functions taking a base parameter.
import { afterEach, describe, expect, it } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { InstanceProvider, useInstanceScope, withInstanceScope } from "./instanceScope";
import { currentInstanceScope } from "./api";

afterEach(cleanup);

function Probe() {
  const scope = useInstanceScope();
  return (
    <div>
      <span data-testid="remote">{String(scope.remote)}</span>
      <span data-testid="id">{scope.instanceId}</span>
      <span data-testid="name">{scope.instanceName}</span>
      <button onClick={() => scope.open("member-1", "attic")}>open</button>
      <button onClick={scope.leave}>leave</button>
    </div>
  );
}

function renderProbe(initialEntries: string[] = ["/dashboard"]) {
  return render(
    <MemoryRouter initialEntries={initialEntries}>
      <InstanceProvider>
        <Probe />
      </InstanceProvider>
    </MemoryRouter>,
  );
}

describe("useInstanceScope", () => {
  it("throws when used outside InstanceProvider", () => {
    // Errors thrown during render still log to the console; suppress that
    // expected noise for this one assertion.
    const spy = () => undefined;
    const orig = console.error;
    console.error = spy;
    try {
      expect(() => render(<Probe />)).toThrow(/useInstanceScope\(\) used outside/);
    } finally {
      console.error = orig;
    }
  });

  it("starts unscoped: this instance, not remote", () => {
    renderProbe();
    expect(screen.getByTestId("remote").textContent).toBe("false");
    expect(screen.getByTestId("id").textContent).toBe("");
    expect(currentInstanceScope()).toBe("");
  });

  it("reads an existing ?instance= from the URL on first render", () => {
    renderProbe(["/dashboard?instance=member-2&instanceName=cellar"]);
    expect(screen.getByTestId("remote").textContent).toBe("true");
    expect(screen.getByTestId("id").textContent).toBe("member-2");
    expect(screen.getByTestId("name").textContent).toBe("cellar");
    expect(currentInstanceScope()).toBe("member-2");
  });

  it("open() scopes to the member and syncs api.ts's active scope", () => {
    renderProbe();
    act(() => fireEvent.click(screen.getByText("open")));
    expect(screen.getByTestId("remote").textContent).toBe("true");
    expect(screen.getByTestId("id").textContent).toBe("member-1");
    expect(screen.getByTestId("name").textContent).toBe("attic");
    expect(currentInstanceScope()).toBe("member-1");
  });

  it("leave() drops back to this instance", () => {
    renderProbe(["/dashboard?instance=member-2&instanceName=cellar"]);
    act(() => fireEvent.click(screen.getByText("leave")));
    expect(screen.getByTestId("remote").textContent).toBe("false");
    expect(screen.getByTestId("id").textContent).toBe("");
    expect(currentInstanceScope()).toBe("");
  });
});

describe("withInstanceScope", () => {
  // The bug this guards against: a nav link built from a bare path drops the
  // scope the moment it is clicked, silently bouncing the reader back to
  // their own instance mid-browse.
  it("carries an active scope onto another page's link", () => {
    expect(withInstanceScope("/containers", "?instance=m1&instanceName=attic")).toBe(
      "/containers?instance=m1&instanceName=attic",
    );
  });

  it("leaves a link alone when nothing is scoped", () => {
    expect(withInstanceScope("/containers", "")).toBe("/containers");
  });

  it("carries a blank instanceName rather than dropping the scope over a missing name", () => {
    expect(withInstanceScope("/containers", "?instance=m1")).toBe("/containers?instance=m1&instanceName=");
  });
});
