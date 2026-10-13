// Where the flow picture puts its parts: a row per section on the left, a
// band per local storage location in the middle and a box per off-site one on
// the right. All of it is arithmetic over three counts, so the picture can be
// drawn at any width.

export interface FlowMeasures {
  width: number;
  row: number;
  top: number;
  /** Where the section names end and the lines begin. */
  label: number;
  bandX: number;
  bandWidth: number;
  stub: number;
  stubLabel: number;
  nodeX: number;
  nodeWidth: number;
  nodePad: number;
  /** The distance between two lines arriving at the same box. */
  gap: number;
  nodeMin: number;
  narrow: boolean;
}

const WIDE = { row: 50, top: 46, label: 168, bandWidth: 56, stub: 64, stubLabel: 170, nodeWidth: 220, nodePad: 16, gap: 9, nodeMin: 56 };
const NARROW: FlowMeasures = {
  width: 350,
  row: 50,
  top: 58,
  label: 112,
  bandX: 122,
  bandWidth: 30,
  stub: 20,
  stubLabel: 98,
  nodeX: 276,
  nodeWidth: 74,
  nodePad: 14,
  gap: 8,
  nodeMin: 74,
  narrow: true,
};

/** Below this the picture is drawn small and scaled to its box. */
export const NARROW_BELOW = 620;
const BAND_GAP = 12;
const NODE_GAP = 14;
/** How far apart several bands stand, so each keeps room for its name. */
const BAND_PITCH = 124;
const HEAD_WIDTH = 140;

/** flowMeasures picks the picture's measures for the room it has. The wide
 *  picture is drawn as wide as its box at scale one, so its names stay the
 *  size of the text around it. */
export function flowMeasures(room: number): FlowMeasures {
  if (room < NARROW_BELOW) return NARROW;
  const width = Math.min(1400, Math.max(720, Math.round(room)));
  const nodeX = width - WIDE.nodeWidth;
  return { ...WIDE, width, nodeX, bandX: Math.round(WIDE.label + (nodeX - WIDE.label) * 0.32), narrow: false };
}

export interface FlowLayout {
  measures: FlowMeasures;
  height: number;
  /** Where the first row starts. The rows and the bands they cross stand
   *  centred beside a taller stack of boxes. */
  rowsTop: number;
  rowsHeight: number;
  nodeHeight: number;
  rowY: (row: number) => number;
  bandLeft: (band: number) => number;
  bandCentre: (band: number) => number;
  /** The room a band's name has above it. */
  headWidth: number;
  nodeTop: (node: number) => number;
  /** Where the line of a row meets a box. */
  port: (node: number, row: number) => [number, number];
  /** Where a line that passes no local band turns towards the boxes. */
  past: number;
}

export function flowLayout(measures: FlowMeasures, rows: number, bands: number, nodes: number): FlowLayout {
  const m = measures;
  const rowsHeight = rows * m.row;
  const nodeHeight = Math.max(m.nodeMin, m.nodePad * 2 + (rows - 1) * m.gap);
  const stack = nodes * nodeHeight + Math.max(0, nodes - 1) * NODE_GAP;
  const first = m.top + Math.max(0, (rowsHeight - stack) / 2);
  const rowsTop = m.top + Math.max(0, (stack - rowsHeight) / 2);
  const pitch = bands > 1 && !m.narrow ? BAND_PITCH : m.bandWidth + BAND_GAP;
  const bandLeft = (band: number) => m.bandX + band * pitch;
  const nodeTop = (node: number) => first + node * (nodeHeight + NODE_GAP);
  return {
    measures,
    height: Math.max(m.top + rowsHeight, first + stack) + 6,
    rowsTop,
    rowsHeight,
    nodeHeight,
    rowY: (row) => rowsTop + m.row * (row + 0.5),
    bandLeft,
    bandCentre: (band) => bandLeft(band) + m.bandWidth / 2,
    headWidth: bands > 1 ? pitch - 6 : HEAD_WIDTH,
    nodeTop,
    port: (node, row) => [m.nodeX, nodeTop(node) + (nodeHeight - (rows - 1) * m.gap) / 2 + row * m.gap],
    past: (bands > 0 ? bandLeft(bands - 1) + m.bandWidth : m.label) + 8,
  };
}

export interface FlowPath {
  d: string;
}

function bend(x1: number, y1: number, x2: number, y2: number): string {
  const mid = (x1 + x2) / 2;
  return `C${mid} ${y1} ${mid} ${y2} ${x2} ${y2}`;
}

/** The line from a section's name to a local band. */
export function nearPath(layout: FlowLayout, row: number, band: number): FlowPath {
  const y = layout.rowY(row);
  const from = layout.measures.label;
  const to = layout.bandCentre(band);
  return { d: `M${from} ${y}H${to}` };
}

/** The line to an off-site box. It leaves the band the section is stored on,
 *  or the section's name when it is stored on none. */
export function farPath(layout: FlowLayout, row: number, node: number, fromBand: number | null): FlowPath {
  const y = layout.rowY(row);
  const [px, py] = layout.port(node, row);
  if (fromBand !== null) {
    const from = layout.bandCentre(fromBand);
    return { d: `M${from} ${y}${bend(from, y, px, py)}` };
  }
  const { past } = layout;
  return { d: `M${layout.measures.label} ${y}H${past}${bend(past, y, px, py)}` };
}

export interface FlowStub {
  d: string;
  cross: string;
  labelX: number;
}

/** The short dashed end of a row that reaches no off-site box. A row stored
 *  on no band ends in its cross past the bands, so its lines to them stay in
 *  sight. */
export function stubPath(layout: FlowLayout, row: number, fromBand: number | null): FlowStub {
  const y = layout.rowY(row);
  const start = fromBand !== null ? layout.bandCentre(fromBand) + 7 : layout.past;
  const end = start + (fromBand !== null ? layout.measures.stub : 4);
  return { d: `M${start} ${y}H${end}`, cross: `M${end - 4} ${y - 4}l8 8m0 -8l-8 8`, labelX: end + 8 };
}
