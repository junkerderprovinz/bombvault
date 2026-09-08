// Palette positions for the page-level bulk-action bars (jdp, 2026-09-08).
//
// WHY THIS FILE EXISTS. A list row carries its own palette position and every
// control inside it inherits that colour. The bar ABOVE the list does not: it
// sits outside the rows, in a plain wrapper, so under rainbow mode its buttons
// kept painting the one flat accent while every row beneath them was coloured.
// jdp asked for them to be coloured too ("ja auch einfärben").
//
// WHY THE POSITION IS PER ACTION, NOT PER PAGE. These buttons are the same four
// verbs on three pages: include everything, discover, back up the selection,
// restore the selection. Numbering them per page would give "Back up selected"
// a different colour on Containers than on VMs for no reason a reader could
// name. Keying the position to the ACTION means the colour says which action it
// is, and says the same thing wherever it appears — the same rule glyphFor
// already follows for symbols.
//
// WHY LITERALS AND NOT A COUNTER. A running counter shifts every colour after
// it whenever a button is conditionally hidden, and these bars hide buttons all
// the time (no selection, no repository, a domain switched off). A fixed
// position holds still.
//
// The row wash is gone as of the same round: `.glim-tint` painted the whole row
// in its hue, and with the bars coloured as well that was too much colour at
// once (jdp: "die zeilen sollen nicht eingefärbt werden, das ist dann zu
// farbig"). Rows keep `.glim-hue`, so the controls INSIDE a row still take that
// row's position — what went is the wash over the card itself.
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
