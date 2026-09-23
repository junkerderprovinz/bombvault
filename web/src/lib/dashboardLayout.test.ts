import { describe, expect, it } from "vitest";
import { mergeOrder } from "./dashboardLayout";

describe("mergeOrder", () => {
  it("keeps the order a user arranged", () => {
    expect(mergeOrder(["b", "a"], ["a", "b"])).toEqual(["b", "a"]);
  });

  it("drops an id the dashboard does not know", () => {
    expect(mergeOrder(["a", "gone", "b"], ["a", "b"])).toEqual(["a", "b"]);
  });

  // A card belongs beside the one it answers together with. Appending it would
  // put it below the fold for everyone who ever reordered or hid a card, which
  // is exactly the reader who has the most cards.
  it("inserts a new card after its predecessor", () => {
    expect(mergeOrder(["a", "coverage", "b"], ["a", "coverage", "anomalies", "b"])).toEqual([
      "a",
      "coverage",
      "anomalies",
      "b",
    ]);
  });

  it("appends a new card that nothing precedes", () => {
    expect(mergeOrder(["a", "b"], ["first", "a", "b"])).toEqual(["a", "b", "first"]);
  });

  it("inserts a run of new cards in their default order", () => {
    expect(mergeOrder(["a", "b"], ["a", "x", "y", "b"])).toEqual(["a", "x", "y", "b"]);
  });
});
