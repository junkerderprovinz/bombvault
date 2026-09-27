import { describe, expect, it } from "vitest";
import { en } from "./i18n";
import { runKindLabel } from "./runKind";

const t = ((key: keyof typeof en) => en[key]) as Parameters<typeof runKindLabel>[0];

describe("runKindLabel", () => {
  it("names an Appdata.Backup import instead of printing its raw kind", () => {
    expect(runKindLabel(t, "import")).toBe(en["run.kindImport"]);
  });

  it("keeps an unknown kind as it came", () => {
    expect(runKindLabel(t, "someday")).toBe("someday");
  });
});
