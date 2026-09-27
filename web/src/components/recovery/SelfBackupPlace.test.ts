import { describe, expect, it } from "vitest";
import type { OffsiteTarget } from "../../lib/api";
import type { Place } from "../../lib/places";
import { selfBackupCopies } from "./SelfBackupPlace";

function target(id: string, sortOrder: number, placeId: string, enabled = true): OffsiteTarget {
  return { id, domain: "config", name: `target ${id}`, repo: `repo-${id}`, sortOrder, placeId, enabled } as OffsiteTarget;
}

function place(id: string, offPremises: boolean): Place {
  return { id, name: `place ${id}`, provider: "minio", offPremises } as Place;
}

describe("selfBackupCopies", () => {
  it("offers every switched-on target, whatever its sort order", () => {
    const copies = selfBackupCopies([target("a", 1, "p"), target("b", 2, "p"), target("c", 3, "p", false)], [place("p", true)]);
    expect(copies.map((c) => c.targetId)).toEqual(["a", "b"]);
  });

  it("puts a copy at another site before one here", () => {
    const copies = selfBackupCopies(
      [target("near", 0, "home"), target("away", 1, "cloud")],
      [place("home", false), place("cloud", true)],
    );
    expect(copies.map((c) => c.targetId)).toEqual(["away", "near"]);
    expect(copies[0]).toMatchObject({ name: "target away", repo: "repo-away", place: { id: "cloud" } });
  });

  it("keeps a target on no place", () => {
    const copies = selfBackupCopies([target("x", 0, "")], []);
    expect(copies).toEqual([{ targetId: "x", name: "target x", repo: "repo-x", place: undefined }]);
  });
});
