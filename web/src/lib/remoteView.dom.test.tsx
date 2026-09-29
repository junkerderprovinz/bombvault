// @vitest-environment jsdom
// useRemoteView is the one gate every page's controls check. Locally it lets
// everything through; remotely it lets through only the named triggers, and
// anything else, present or added later, is refused by default.
import { describe, expect, it } from "vitest";
import { renderHook } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import type { ReactNode } from "react";
import { InstanceProvider } from "./instanceScope";
import { useRemoteView, remoteViewCanAct } from "./remoteView";

function wrapper(initialEntries: string[]) {
  return ({ children }: { children: ReactNode }) => (
    <MemoryRouter initialEntries={initialEntries}>
      <InstanceProvider>{children}</InstanceProvider>
    </MemoryRouter>
  );
}

describe("remoteViewCanAct", () => {
  it("knows the fixed trigger list", () => {
    expect(remoteViewCanAct("containers.backup")).toBe(true);
    expect(remoteViewCanAct("check")).toBe(true);
  });

  it("refuses anything not on the list, including a made-up key", () => {
    expect(remoteViewCanAct("containers.restore")).toBe(false);
    expect(remoteViewCanAct("containers.delete")).toBe(false);
    expect(remoteViewCanAct("some.new.button.nobody.registered")).toBe(false);
  });
});

describe("useRemoteView", () => {
  it("lets every action through locally", () => {
    const { result } = renderHook(() => useRemoteView(), { wrapper: wrapper(["/dashboard"]) });
    expect(result.current.remote).toBe(false);
    expect(result.current.canAct("containers.restore")).toBe(true);
    expect(result.current.canAct("anything")).toBe(true);
  });

  it("remotely lets only a listed trigger through", () => {
    const { result } = renderHook(() => useRemoteView(), {
      wrapper: wrapper(["/dashboard?instance=m1&instanceName=attic"]),
    });
    expect(result.current.remote).toBe(true);
    expect(result.current.canAct("containers.backup")).toBe(true);
    expect(result.current.canAct("containers.restore")).toBe(false);
    expect(result.current.canAct("containers.delete")).toBe(false);
  });
});
