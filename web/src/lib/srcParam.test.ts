// ---------------------------------------------------------------------------
// srcParam — the single seam that decides which repo an API call reads from.
//
// It used to emit the ?source= param ONLY for the exact literal "offsite", so a
// per-target "offsite:<id>" source silently produced no param at all and the
// request fell back to the LOCAL repo. That is what kept the multi-off-site
// restore picker from being wired up (issue #138); these cases pin both forms.
// ---------------------------------------------------------------------------
import { describe, expect, it } from "vitest";
import { srcParam } from "./api";
import { isOffsiteSource } from "../components/SourceToggle";
import { offsiteTargetSource } from "./useOffsiteTargets";
import type { OffsiteTarget } from "./api";

describe("srcParam", () => {
  it("omits the param for the local repo", () => {
    expect(srcParam("local")).toBe("");
    expect(srcParam(undefined)).toBe("");
    expect(srcParam("")).toBe("");
  });

  it("keeps the bare off-site form byte-identical", () => {
    expect(srcParam("offsite")).toBe("?source=offsite");
    expect(srcParam("offsite", "&")).toBe("&source=offsite");
  });

  it("carries a per-target off-site source instead of dropping it", () => {
    const id = "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6";
    expect(srcParam(`offsite:${id}`)).toBe(`?source=offsite%3A${id}`);
    expect(srcParam(`offsite:${id}`, "&")).toBe(`&source=offsite%3A${id}`);
  });

  it("ignores a source that merely starts with 'offsite' as a word", () => {
    expect(srcParam("offsitex")).toBe("");
    expect(srcParam("nonsense")).toBe("");
  });
});

describe("isOffsiteSource", () => {
  it("accepts both off-site forms and nothing else", () => {
    expect(isOffsiteSource("offsite")).toBe(true);
    expect(isOffsiteSource("offsite:abc123")).toBe(true);
    expect(isOffsiteSource("local")).toBe(false);
    expect(isOffsiteSource("offsitex")).toBe(false);
  });
});

describe("offsiteTargetSource", () => {
  const target = (id: string): OffsiteTarget =>
    ({ id, domain: "containers", name: "", repo: "s3:x" }) as OffsiteTarget;

  it("addresses the primary with the bare form so the default is unchanged", () => {
    expect(offsiteTargetSource(target("first"), 0)).toBe("offsite");
  });

  it("addresses every other target by id", () => {
    expect(offsiteTargetSource(target("second"), 1)).toBe("offsite:second");
  });
});
