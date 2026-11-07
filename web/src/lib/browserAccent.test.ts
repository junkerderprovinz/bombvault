import { describe, expect, it } from "vitest";
import { cssColourToHex } from "./browserAccent";

describe("a computed colour", () => {
  it("reads rgb() and rgba() in either notation", () => {
    expect(cssColourToHex("rgb(0, 95, 184)")).toBe("#005FB8");
    expect(cssColourToHex("rgba(10, 132, 255, 1)")).toBe("#0A84FF");
    expect(cssColourToHex("rgb(10 132 255)")).toBe("#0A84FF");
  });

  it("gives nothing for a value that is no rgb colour", () => {
    expect(cssColourToHex("AccentColor")).toBeUndefined();
    expect(cssColourToHex("")).toBeUndefined();
  });
});
