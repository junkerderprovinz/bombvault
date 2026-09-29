// @vitest-environment jsdom
// The remote-view bar is the one thing on screen that says "this is not your
// own instance", so it must show whenever the scope is remote and never
// otherwise, and its back control must actually leave the scope.
import { afterEach, describe, expect, it } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { RemoteViewBar } from "./RemoteViewBar";
import { InstanceProvider } from "../lib/instanceScope";
import { I18nProvider, en } from "../lib/i18n";

afterEach(cleanup);

function renderBar(initialEntries: string[]) {
  return render(
    <MemoryRouter initialEntries={initialEntries}>
      <InstanceProvider>
        <I18nProvider>
          <RemoteViewBar />
        </I18nProvider>
      </InstanceProvider>
    </MemoryRouter>,
  );
}

describe("RemoteViewBar", () => {
  it("renders nothing while this instance's own pages are shown", () => {
    renderBar(["/dashboard"]);
    expect(screen.queryByText(en["remoteView.back"])).toBeNull();
  });

  it("names the open member while remote", () => {
    renderBar(["/dashboard?instance=member-1&instanceName=attic"]);
    expect(screen.getByText(en["remoteView.viewing"].replace("{name}", "attic"))).toBeTruthy();
    expect(screen.getByText(en["remoteView.back"])).toBeTruthy();
  });

  it("falls back to the unnamed label when no name reached the URL", () => {
    renderBar(["/dashboard?instance=member-1"]);
    expect(screen.getByText(en["remoteView.viewing"].replace("{name}", en["remoteView.unnamed"]))).toBeTruthy();
  });

  it("the back control drops the scope back to this instance", () => {
    renderBar(["/dashboard?instance=member-1&instanceName=attic"]);
    act(() => fireEvent.click(screen.getByText(en["remoteView.back"])));
    expect(screen.queryByText(en["remoteView.back"])).toBeNull();
  });
});
