import type { CardDefault, LayoutMigration } from "../../lib/dashboardLayout";

/** The overview's cards in their default order. The first four show from the
 *  start, the others wait among the hidden cards. */
export const OVERVIEW_CARDS: CardDefault[] = [
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

/** What became of the cards a layout from before the six-column grid names. */
export const OVERVIEW_MIGRATION: LayoutMigration = {
  renames: { runHistory: "history", activityLog: "history", ransomware: "protection" },
  drops: ["lastBackups"],
};

/** A phone shows the visible cards in this order, whatever the stored one. */
export const PHONE_ORDER = [
  "needs",
  "protection",
  "targets",
  "history",
  "anomalies",
  "coverage",
  "heatmap",
  "summary",
  "storage",
  "stats",
  "spike",
];
