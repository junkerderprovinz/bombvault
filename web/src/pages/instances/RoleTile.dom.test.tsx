// @vitest-environment jsdom
// A role tile says in one glance whether an instance plays a role: on,
// asked for and waiting, or off. The bubble beside the heading tells the
// three roles apart.
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import { I18nProvider, en } from "../../lib/i18n";
import { RoleTile, RoleTiles } from "./RoleTile";

afterEach(cleanup);

function tile(role: string): HTMLElement {
  const found = document.querySelector<HTMLElement>(`[data-role="${role}"]`);
  if (!found) throw new Error(`no ${role} tile`);
  return found;
}

describe("a role tile", () => {
  it("names the role and says it is off", () => {
    render(
      <I18nProvider>
        <RoleTile role="fetcher" state="off" hueIndex={1} />
      </I18nProvider>,
    );
    expect(tile("fetcher").textContent).toBe(en["instances.role.fetcher"] + en["zfs.replica.servers.off"]);
    expect(tile("fetcher").className).not.toContain("bg-accent");
  });

  it("is filled while the role is on", () => {
    render(
      <I18nProvider>
        <RoleTile role="receiver" state="on" hueIndex={0} />
      </I18nProvider>,
    );
    expect(tile("receiver").textContent).toBe(en["receiver.title"] + en["instances.role.on"]);
    expect(tile("receiver").className).toContain("bg-accent");
    expect(tile("receiver").className).not.toContain("outline-dashed");
  });

  it("is filled and wears a dashed line while the request waits", () => {
    render(
      <I18nProvider>
        <RoleTile role="zfs" state="waiting" hueIndex={2} />
      </I18nProvider>,
    );
    expect(tile("zfs").textContent).toBe(en["zfs.replica.servers.title"] + en["instances.role.waiting"]);
    expect(tile("zfs").className).toContain("bg-accent");
    expect(tile("zfs").className).toContain("outline-dashed");
  });

  it("shows its own line in place of the state's word", () => {
    render(
      <I18nProvider>
        <RoleTile role="receiver" state="on" detail="for attic" hueIndex={0} />
      </I18nProvider>,
    );
    expect(tile("receiver").textContent).toBe(en["receiver.title"] + "for attic");
  });

  it("is no control without a way to change the role", () => {
    render(
      <I18nProvider>
        <RoleTile role="receiver" state="on" hueIndex={0} />
      </I18nProvider>,
    );
    expect(screen.queryByRole("button")).toBeNull();
    expect(screen.queryByRole("switch")).toBeNull();
  });

  it("is a pressed button that reports the click once the role can be changed", () => {
    let clicks = 0;
    render(
      <I18nProvider>
        <RoleTile role="receiver" state="waiting" hueIndex={0} onToggle={() => clicks++} />
      </I18nProvider>,
    );
    const button = screen.getByRole("button", { name: new RegExp(en["receiver.title"]) });
    expect(button.getAttribute("aria-pressed")).toBe("true");
    fireEvent.click(button);
    expect(clicks).toBe(1);
  });

  it("is an unpressed button while the role is off", () => {
    render(
      <I18nProvider>
        <RoleTile role="fetcher" state="off" hueIndex={1} onToggle={() => undefined} />
      </I18nProvider>,
    );
    expect(screen.getByRole("button").getAttribute("aria-pressed")).toBe("false");
  });
});

describe("the tiles of a card", () => {
  it("stand under their question, with the bubble that tells the roles apart", () => {
    render(
      <I18nProvider>
        <RoleTiles heading="What attic does for this server" states={{ receiver: "on", fetcher: "waiting", zfs: "off" }} />
      </I18nProvider>,
    );
    expect(screen.getByText("What attic does for this server")).toBeTruthy();
    expect(screen.getByLabelText(en["instances.role.diff"])).toBeTruthy();
    expect([...document.querySelectorAll("[data-role]")].map((el) => [el.getAttribute("data-role"), el.getAttribute("data-state")])).toEqual([
      ["receiver", "on"],
      ["fetcher", "waiting"],
      ["zfs", "off"],
    ]);
  });

  it("leave out a role nothing is known about", () => {
    render(
      <I18nProvider>
        <RoleTiles heading="What attic does for this server" states={{ receiver: "off", zfs: "on" }} />
      </I18nProvider>,
    );
    expect([...document.querySelectorAll("[data-role]")].map((el) => el.getAttribute("data-role"))).toEqual(["receiver", "zfs"]);
  });

  it("are not drawn at all when no role is known", () => {
    const { container } = render(
      <I18nProvider>
        <RoleTiles heading="What attic does for this server" states={{}} />
      </I18nProvider>,
    );
    expect(container.textContent).toBe("");
  });

  it("report which role was clicked", () => {
    const clicked: string[] = [];
    render(
      <I18nProvider>
        <RoleTiles heading="x" states={{ receiver: "off", fetcher: "off", zfs: "off" }} onToggle={(k) => clicked.push(k)} />
      </I18nProvider>,
    );
    fireEvent.click(screen.getByRole("button", { name: new RegExp(en["instances.role.fetcher"]) }));
    expect(clicked).toEqual(["fetcher"]);
  });
});
