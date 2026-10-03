// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { adopt, following, keepAsOwn, setFollowing } from "./look";

afterEach(() => localStorage.clear());

const followed = (accent: string) => JSON.stringify({ ok: true, prefs: { "bv-accent": accent }, stored: true });

describe("following the first server", () => {
  it("gives the phone's own look back after a round trip through the switch", () => {
    setFollowing(false);
    localStorage.setItem("bv-accent", "#42be65");
    setFollowing(true);
    adopt(followed("#ff7eb6"));
    expect(localStorage.getItem("bv-accent")).toBe("#ff7eb6");

    setFollowing(false);
    expect(following()).toBe(false);
    expect(localStorage.getItem("bv-accent")).toBe("#42be65");
  });

  it("keeps the followed look the first time there is nothing to give back", () => {
    adopt(followed("#ff7eb6"));
    setFollowing(false);
    expect(localStorage.getItem("bv-accent")).toBe("#ff7eb6");
  });
});

describe("a change made while following", () => {
  it("keeps the look on screen instead of the one put away", () => {
    setFollowing(false);
    localStorage.setItem("bv-accent", "#42be65");
    setFollowing(true);
    localStorage.setItem("bv-accent", "#be95ff");
    keepAsOwn();
    setFollowing(false);
    expect(localStorage.getItem("bv-accent")).toBe("#be95ff");
  });
});
