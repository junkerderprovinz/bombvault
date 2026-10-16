import { describe, expect, it } from "vitest";
import type { OffsiteTarget, Settings } from "./api";
import { tamperPlanned } from "./tamperSchedule";

const none = {
  containersOffsite: "",
  vmsOffsite: "",
  flashOffsite: "",
  configOffsite: "",
  filesOffsite: "",
  zfsOffsite: "",
  containersOffsiteImmutable: false,
  vmsOffsiteImmutable: false,
  flashOffsiteImmutable: false,
  configOffsiteImmutable: false,
  filesOffsiteImmutable: false,
  zfsOffsiteImmutable: false,
} as Settings;

const target = (over: Partial<OffsiteTarget>) => ({ enabled: true, immutable: true, sortOrder: 1, ...over }) as OffsiteTarget;

describe("tamperPlanned", () => {
  it("counts an append-only additional target when no off-site settings are", () => {
    expect(tamperPlanned(none, [target({})])).toBe(true);
  });

  it("leaves out a switched-off or unflagged target", () => {
    expect(tamperPlanned(none, [target({ enabled: false }), target({ immutable: false })])).toBe(false);
  });

  it("counts a section whose off-site settings are append-only", () => {
    expect(tamperPlanned({ ...none, vmsOffsite: "rest:http://box:8000/vms", vmsOffsiteImmutable: true }, [])).toBe(true);
  });
});
