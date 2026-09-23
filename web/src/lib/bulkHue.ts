// Palette positions for the page-level bulk-action bars. A bar sits above the
// list, outside every row, so under rainbow mode it has no row position to
// inherit and would stay in the flat accent.
//
// Positions belong to the action, not the page: the same verbs appear on
// several pages, and "Back up selected" should be the same colour on
// Containers as on VMs, the same rule glyphFor follows for symbols. They are
// literals rather than a running counter because the bars hide buttons
// conditionally (no selection, no repository, a domain switched off), and a
// counter would shift every colour after a hidden one.
//
// Rows keep `.glim-hue`, so controls inside a row take that row's position,
// but the row itself is not washed in it: coloured bars on top of coloured
// rows is too much colour.
export const BULK_HUE = {
  /** "Include all in schedule" / "Exclude all". */
  include: 0,
  /** "Discover", the scan that finds new items. */
  discover: 1,
  /** "Back up selected". */
  backup: 2,
  /** "Restore selected". */
  restore: 3,
} as const;
