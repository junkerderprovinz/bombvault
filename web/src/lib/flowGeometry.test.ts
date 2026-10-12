// Where the flow picture puts its lines.
import { describe, expect, it } from "vitest";
import { farPath, flowLayout, flowMeasures, nearPath, stubPath } from "./flowGeometry";

describe("flowMeasures", () => {
  it("draws the wide picture as wide as its box, within limits", () => {
    expect(flowMeasures(1000).width).toBe(1000);
    expect(flowMeasures(640).width).toBe(720);
    expect(flowMeasures(2000).width).toBe(1400);
    expect(flowMeasures(1000).narrow).toBe(false);
  });

  it("switches to the small picture where the wide one has no room", () => {
    expect(flowMeasures(360)).toMatchObject({ narrow: true, width: 350 });
  });
});

describe("flowLayout", () => {
  const layout = flowLayout(flowMeasures(1000), 3, 1, 2);

  it("gives every row its own port on a box", () => {
    const ports = [0, 1, 2].map((row) => layout.port(0, row));
    expect(new Set(ports.map(([, y]) => y)).size).toBe(3);
    expect(ports.every(([x]) => x === layout.measures.nodeX)).toBe(true);
  });

  it("is tall enough for the rows and for the boxes", () => {
    expect(layout.height).toBeGreaterThanOrEqual(layout.measures.top + 3 * layout.measures.row);
    expect(layout.height).toBeGreaterThanOrEqual(layout.nodeTop(1) + layout.nodeHeight);
    expect(flowLayout(flowMeasures(1000), 1, 1, 4).height).toBeGreaterThan(layout.height);
  });

  it("centres the rows beside a taller stack of boxes", () => {
    const tall = flowLayout(flowMeasures(1000), 6, 1, 7);
    const stackTop = tall.nodeTop(0);
    const stackBottom = tall.nodeTop(6) + tall.nodeHeight;
    expect(tall.rowsTop).toBeGreaterThan(tall.measures.top);
    expect(tall.rowsTop + tall.rowsHeight / 2).toBeCloseTo((stackTop + stackBottom) / 2);
    expect(tall.rowY(0)).toBe(tall.rowsTop + tall.measures.row / 2);
    expect(layout.rowsTop).toBe(layout.measures.top);
  });

  it("sets several bands apart so each keeps room for its name", () => {
    const two = flowLayout(flowMeasures(1000), 3, 2, 1);
    expect(two.bandCentre(1) - two.bandCentre(0)).toBeGreaterThanOrEqual(two.headWidth);
  });
});

describe("the lines", () => {
  const layout = flowLayout(flowMeasures(1000), 3, 1, 2);

  it("run from the section's name to the band it is stored on", () => {
    const y = layout.rowY(1);
    expect(nearPath(layout, 1, 0).d).toBe(`M${layout.measures.label} ${y}H${layout.bandCentre(0)}`);
  });

  it("leave for a box from the band, or from the name when the section is stored on none", () => {
    const [px, py] = layout.port(1, 0);
    const fromBand = farPath(layout, 0, 1, 0);
    expect(fromBand.d.startsWith(`M${layout.bandCentre(0)} ${layout.rowY(0)}C`)).toBe(true);
    expect(fromBand.d.endsWith(`${px} ${py}`)).toBe(true);
    expect(farPath(layout, 0, 1, null).d.startsWith(`M${layout.measures.label} ${layout.rowY(0)}H${layout.past}C`)).toBe(true);
  });

  it("put a mark at the middle of a line", () => {
    const [px, py] = layout.port(0, 2);
    expect(farPath(layout, 2, 0, 0).middle).toEqual([(layout.bandCentre(0) + px) / 2, (layout.rowY(2) + py) / 2]);
  });

  it("end a row that reaches no box in a short stub after its band", () => {
    const stub = stubPath(layout, 0, 0);
    expect(stub.d).toBe(`M${layout.bandCentre(0) + 7} ${layout.rowY(0)}H${layout.bandCentre(0) + 7 + layout.measures.stub}`);
    expect(stub.labelX).toBeGreaterThan(layout.bandCentre(0));
  });
});
