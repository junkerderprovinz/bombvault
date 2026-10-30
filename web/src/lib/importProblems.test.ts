import { describe, expect, it } from "vitest";
import { en, type TranslationKey } from "./i18n";
import { importProblemText } from "./importProblems";

const t = (key: TranslationKey) => en[key];
const bare = (text: string | null) => text?.replace(/[⁦-⁩]/g, "") ?? null;

describe("importProblemText", () => {
  it("names the row of the file a problem is in", () => {
    expect(importProblemText(t, "en", { ok: false, code: "import-row-name", part: "repository", row: 2 })).toBe(
      "Repository 2 in the file has no name."
    );
  });

  it("says why a row's location is refused, with the path to enter", () => {
    const text = importProblemText(t, "en", {
      ok: false,
      code: "import-row-location",
      part: "repository",
      row: 1,
      cause: "path-absolute",
      suggest: "user/warm",
    });
    expect(bare(text)).toBe(
      "Repository 1 in the file has a location BombVault refuses. " +
        "This is a full path on the host. Paths here start at the Host Data mount, so enter user/warm instead, or use a restic remote such as rest:, s3: or rclone:."
    );
  });

  it("names the section whose place a repository of the file takes", () => {
    const text = importProblemText(t, "en", {
      ok: false,
      code: "import-place-taken",
      part: "repository",
      row: 1,
      place: "path:containers",
    });
    expect(text).toBe("Repository 1 in the file is at the same place as the Containers folder. A repository needs a place of its own.");
  });

  it("names a repository set up here by its name", () => {
    const text = importProblemText(t, "en", { ok: false, code: "import-place-twice", part: "stored-repository", name: "Cold" });
    expect(bare(text)).toBe("The repository Cold set up here is at the same place as a repository before it. Two repositories cannot share a place.");
  });

  it("names the setting and the file's format", () => {
    expect(bare(importProblemText(t, "en", { ok: false, code: "import-setting", name: "compression" }))).toBe(
      "The setting compression in the file holds a value this version does not accept."
    );
    expect(importProblemText(t, "en", { ok: false, code: "import-version", row: 3 })).toBe(
      "This file has format version 3, which this version of BombVault cannot read."
    );
  });

  it("leaves a refusal that is not one of its problems to the placement codes", () => {
    expect(importProblemText(t, "en", { ok: false, code: "copy-on-own-folder" })).toBeNull();
    expect(importProblemText(t, "en", { ok: false, error: "x" })).toBeNull();
  });
});
