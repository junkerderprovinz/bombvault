// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import type { BackendOption } from "../../lib/api";
import { I18nProvider, en } from "../../lib/i18n";
import { MoreSettings, holdsOwnValue } from "./steps";

const option = (name: string, def = ""): BackendOption => ({ name, help: "", required: false, password: false, advanced: true, default: def }) as BackendOption;

function show(own: boolean) {
  render(
    <I18nProvider>
      <MoreSettings own={own}>
        <p>the port field</p>
      </MoreSettings>
    </I18nProvider>
  );
  return screen.getByRole("button", { name: en["dest.more"] });
}

afterEach(cleanup);

describe("the more settings of a provider", () => {
  it("stay folded until asked for", () => {
    const head = show(false);
    expect(head.getAttribute("aria-expanded")).toBe("false");
    expect(screen.queryByText("the port field")).toBeNull();
    fireEvent.click(head);
    expect(screen.getByText("the port field")).toBeTruthy();
    fireEvent.click(head);
    expect(screen.queryByText("the port field")).toBeNull();
  });

  it("open by themselves while one of them holds a value of its own", () => {
    const head = show(true);
    expect(head.getAttribute("aria-expanded")).toBe("true");
    expect(screen.getByText("the port field")).toBeTruthy();
  });
});

describe("a value of its own", () => {
  const options = [option("port", "22"), option("ciphers")];

  it("is one that differs from the field's default", () => {
    expect(holdsOwnValue(options, { port: "2222" })).toBe(true);
    expect(holdsOwnValue(options, { ciphers: "aes256-ctr" })).toBe(true);
  });

  it("is neither an empty field nor the default typed out", () => {
    expect(holdsOwnValue(options, {})).toBe(false);
    expect(holdsOwnValue(options, { port: "22", ciphers: "" })).toBe(false);
  });
});
