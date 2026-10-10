import { describe, expect, it } from "vitest";
import {
  CARD_SPANS,
  cardSpan,
  defaultLayout,
  mergeOrder,
  migrateLayout,
  nextSpan,
  packSpans,
  snapHeight,
  snapSpan,
  type CardDefault,
  type CardSpan,
  type LayoutMigration,
} from "./dashboardLayout";

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

describe("snapSpan", () => {
  it("leaves an allowed span alone", () => {
    expect(CARD_SPANS.map(snapSpan)).toEqual([2, 3, 4, 6]);
  });

  it("picks the nearest span", () => {
    expect([2.4, 2.6, 3.6, 4.9, 5.2].map(snapSpan)).toEqual([2, 3, 4, 4, 6]);
  });

  it("picks the narrower span on a tie", () => {
    expect(snapSpan(2.5)).toBe(2);
    expect(snapSpan(5)).toBe(4);
  });

  it("stays inside the grid", () => {
    expect(snapSpan(0.3)).toBe(2);
    expect(snapSpan(9)).toBe(6);
  });
});

describe("snapHeight", () => {
  it("rounds to steps of 20px", () => {
    expect([149, 150, 151, 400].map(snapHeight)).toEqual([140, 160, 160, 400]);
  });

  it("does not go below 140px", () => {
    expect(snapHeight(60)).toBe(140);
    expect(snapHeight(-5)).toBe(140);
  });
});

describe("nextSpan", () => {
  it("steps through the widths and starts over after the full row", () => {
    expect(CARD_SPANS.map(nextSpan)).toEqual([3, 4, 6, 2]);
  });
});

describe("packSpans", () => {
  it("leaves full rows alone", () => {
    expect(packSpans([6, 3, 3, 2, 2, 2, 4, 2])).toEqual([6, 3, 3, 2, 2, 2, 4, 2]);
  });

  it("widens the last card of a row the next card does not fit", () => {
    expect(packSpans([3, 2, 3, 3])).toEqual([3, 3, 3, 3]);
    expect(packSpans([2, 2, 3, 3])).toEqual([2, 4, 3, 3]);
    expect(packSpans([2, 6])).toEqual([6, 6]);
  });

  it("widens the last card of the last row", () => {
    expect(packSpans([3])).toEqual([6]);
    expect(packSpans([6, 2, 2])).toEqual([6, 2, 4]);
  });

  it("returns nothing for no cards", () => {
    expect(packSpans([])).toEqual([]);
  });

  it("does not touch the spans it was given", () => {
    const spans: CardSpan[] = [3, 2, 3];
    packSpans(spans);
    expect(spans).toEqual([3, 2, 3]);
  });

  it("fills every row with allowed spans, whatever the cards", () => {
    const sequences: CardSpan[][] = [[]];
    for (let n = 0; n < 5; n++) {
      for (const seq of sequences.filter((s) => s.length === n)) {
        for (const span of CARD_SPANS) sequences.push([...seq, span]);
      }
    }
    for (const seq of sequences) {
      const packed = packSpans(seq);
      expect(packed.every((s) => CARD_SPANS.includes(s)), seq.join(",")).toBe(true);
      let used = 0;
      for (const span of packed) {
        used += span;
        expect(used, seq.join(",")).toBeLessThanOrEqual(6);
        if (used === 6) used = 0;
      }
      expect(used, seq.join(",")).toBe(0);
    }
  });
});

const CARDS: CardDefault[] = [
  { id: "needs", span: 6 },
  { id: "protection", span: 3 },
  { id: "targets", span: 3 },
  { id: "history", span: 6 },
  { id: "anomalies", span: 3, hidden: true },
  { id: "coverage", span: 3, hidden: true },
  { id: "heatmap", span: 3, hidden: true },
  { id: "storage", span: 3, hidden: true },
  { id: "summary", span: 6, hidden: true },
  { id: "stats", span: 6, hidden: true },
  { id: "spike", span: 3, hidden: true },
];

const MIGRATION: LayoutMigration = {
  renames: { runHistory: "history", activityLog: "history", ransomware: "protection" },
  drops: ["lastBackups"],
};

const OLD_ORDER = [
  "summary",
  "activityLog",
  "stats",
  "protection",
  "coverage",
  "anomalies",
  "ransomware",
  "lastBackups",
  "runHistory",
  "heatmap",
  "storage",
  "spike",
];

const TWO: CardDefault[] = [
  { id: "protection", span: 3 },
  { id: "history", span: 6 },
];

describe("defaultLayout", () => {
  it("takes order and hidden cards from the card set", () => {
    expect(defaultLayout(CARDS)).toEqual({
      v: 2,
      order: CARDS.map((c) => c.id),
      hidden: ["anomalies", "coverage", "heatmap", "storage", "summary", "stats", "spike"],
      widths: {},
      heights: {},
    });
  });
});

describe("cardSpan", () => {
  it("prefers the stored width over the default", () => {
    const layout = { ...defaultLayout(CARDS), widths: { protection: 4 as CardSpan } };
    expect(cardSpan(layout, CARDS[1])).toBe(4);
    expect(cardSpan(layout, CARDS[2])).toBe(3);
  });
});

describe("migrateLayout", () => {
  it("gives the default when nothing is stored", () => {
    expect(migrateLayout(null, CARDS, MIGRATION)).toEqual(defaultLayout(CARDS));
    expect(migrateLayout(undefined, CARDS, MIGRATION)).toEqual(defaultLayout(CARDS));
  });

  it("gives the default for a value that is no layout", () => {
    expect(migrateLayout("junk", CARDS, MIGRATION)).toEqual(defaultLayout(CARDS));
    expect(migrateLayout([], CARDS, MIGRATION)).toEqual(defaultLayout(CARDS));
  });

  it("keeps every card of a stored old default visible", () => {
    expect(migrateLayout({ order: OLD_ORDER, hidden: [], widths: {} }, CARDS, MIGRATION)).toEqual({
      v: 2,
      order: [
        "needs",
        "summary",
        "history",
        "stats",
        "protection",
        "targets",
        "coverage",
        "anomalies",
        "heatmap",
        "storage",
        "spike",
      ],
      hidden: [],
      widths: {},
      heights: {},
    });
  });

  it("keeps a customised order, its hidden cards and its widths", () => {
    const stored = {
      order: [
        "storage",
        "runHistory",
        "summary",
        "protection",
        "activityLog",
        "stats",
        "coverage",
        "anomalies",
        "ransomware",
        "lastBackups",
        "heatmap",
        "spike",
      ],
      hidden: ["stats", "heatmap", "lastBackups"],
      widths: { storage: "half", summary: "full", runHistory: "half", lastBackups: "half" },
    };
    expect(migrateLayout(stored, CARDS, MIGRATION)).toEqual({
      v: 2,
      order: [
        "needs",
        "storage",
        "history",
        "summary",
        "protection",
        "targets",
        "stats",
        "coverage",
        "anomalies",
        "heatmap",
        "spike",
      ],
      hidden: ["stats", "heatmap"],
      widths: { storage: 3, history: 3, summary: 6 },
      heights: {},
    });
  });

  it("drops ids it does not know and fills in the cards that are missing", () => {
    const stored = {
      order: ["protection", "mystery", "summary"],
      hidden: ["mystery", "ghost"],
      widths: { mystery: "half", protection: "wide" },
    };
    expect(migrateLayout(stored, CARDS, MIGRATION)).toEqual({
      v: 2,
      order: CARDS.map((c) => c.id),
      hidden: ["anomalies", "coverage", "heatmap", "storage", "stats", "spike"],
      widths: {},
      heights: {},
    });
  });

  it("puts the merged card where the first of its old cards stood", () => {
    const logFirst = { order: ["protection", "activityLog", "runHistory"] };
    const runsFirst = { order: ["runHistory", "protection", "activityLog"] };
    expect(migrateLayout(logFirst, TWO, MIGRATION).order).toEqual(["protection", "history"]);
    expect(migrateLayout(runsFirst, TWO, MIGRATION).order).toEqual(["history", "protection"]);
  });

  it("hides the merged card only if all of its old cards were hidden", () => {
    const order = ["runHistory", "activityLog", "protection"];
    expect(migrateLayout({ order, hidden: ["runHistory"] }, TWO, MIGRATION).hidden).toEqual([]);
    expect(migrateLayout({ order, hidden: ["activityLog"] }, TWO, MIGRATION).hidden).toEqual([]);
    expect(
      migrateLayout({ order, hidden: ["runHistory", "activityLog"] }, TWO, MIGRATION).hidden
    ).toEqual(["history"]);
  });

  it("gives the merged card the first width one of its old cards had", () => {
    const order = ["runHistory", "activityLog", "protection"];
    expect(
      migrateLayout({ order, widths: { activityLog: "half" } }, TWO, MIGRATION).widths
    ).toEqual({ history: 3 });
    expect(
      migrateLayout({ order, widths: { runHistory: "full", activityLog: "half" } }, TWO, MIGRATION)
        .widths
    ).toEqual({ history: 6 });
  });

  it("lets a card that is already there keep its state when another folds into it", () => {
    const stored = {
      order: ["ransomware", "history", "protection"],
      hidden: ["protection"],
      widths: { ransomware: "half" },
    };
    expect(migrateLayout(stored, TWO, MIGRATION)).toEqual({
      v: 2,
      order: ["history", "protection"],
      hidden: ["protection"],
      widths: {},
      heights: {},
    });
  });

  it("renames a card whose successor the layout does not have yet", () => {
    const stored = { order: ["ransomware", "history"], hidden: ["ransomware"] };
    expect(migrateLayout(stored, TWO, MIGRATION)).toMatchObject({
      order: ["protection", "history"],
      hidden: ["protection"],
    });
  });

  it("forgets the place of a dropped card", () => {
    const migration = { renames: {}, drops: ["history"] };
    const stored = { order: ["history", "protection"], hidden: ["history"] };
    expect(migrateLayout(stored, TWO, migration)).toMatchObject({
      order: ["protection", "history"],
      hidden: [],
    });
  });

  it("keeps a layout that already carries the version", () => {
    const cards: CardDefault[] = [
      { id: "a", span: 6 },
      { id: "b", span: 3 },
      { id: "c", span: 3, hidden: true },
    ];
    const stored = {
      v: 2,
      order: ["b", "a"],
      hidden: ["b"],
      widths: { b: 4, a: 5, gone: 2, c: "half" },
      heights: { b: 333, a: 90, gone: 200 },
    };
    expect(migrateLayout(stored, cards, MIGRATION)).toEqual({
      v: 2,
      order: ["b", "c", "a"],
      hidden: ["b", "c"],
      widths: { b: 4, a: 4 },
      heights: { b: 340, a: 140 },
    });
  });

  it("does not rename the cards of a layout that carries the version", () => {
    const stored = { v: 2, order: ["runHistory", "protection"], hidden: [] };
    expect(migrateLayout(stored, TWO, MIGRATION).order).toEqual(["protection", "history"]);
  });

  it("gives the default for a version it does not know", () => {
    expect(migrateLayout({ v: 3, order: ["history", "protection"] }, TWO, MIGRATION)).toEqual(
      defaultLayout(TWO)
    );
  });

  it("does not change the stored value", () => {
    const stored = { order: OLD_ORDER.slice(), hidden: ["stats"], widths: { stats: "half" } };
    const before = JSON.stringify(stored);
    migrateLayout(stored, CARDS, MIGRATION);
    expect(JSON.stringify(stored)).toBe(before);
  });
});
