import { describe, expect, it } from "vitest";
import { en, type TranslationKey } from "./i18n";
import { importProblemText } from "./importProblems";
import { allLocales } from "./localesForTests";

const t = (key: TranslationKey) => en[key];
const bare = (text: string | null) => text?.replace(/[⁦-⁩]/g, "") ?? null;

describe("importProblemText", () => {
  it("names the row of the file a problem is in", () => {
    expect(importProblemText(t, "en", { ok: false, code: "import-row-name", part: "repository", row: 2 })).toBe(
      "In the file, repository 2 has no name."
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
      "In the file, repository 1 has a location BombVault refuses. " +
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
    expect(text).toBe("In the file, repository 1 is at the same place as the own folder of Containers. A repository needs a place of its own.");
  });

  it("names a repository set up here by its name", () => {
    const text = importProblemText(t, "en", { ok: false, code: "import-place-twice", part: "stored-repository", name: "Cold" });
    expect(bare(text)).toBe("The repository Cold set up here is at the same place as a repository in the file. Two repositories cannot share a place.");
  });

  it("words a bad compression on an off-site storage location as one", () => {
    expect(importProblemText(t, "en", { ok: false, code: "import-row-compression", part: "target", row: 3 })).toBe(
      "In the file, off-site storage location 3 has a compression setting this version does not know."
    );
  });

  it("has a whole sentence for every row a problem can name, in every language", () => {
    const rows: [string, string, string?][] = [
      ["import-row-invalid", "target"], ["import-row-invalid", "zfs-server"], ["import-row-invalid", "zfs-replica"],
      ["import-row-name", "repository"], ["import-row-name", "destination"],
      ["import-row-location", "repository"], ["import-row-location", "destination"],
      ["import-row-compression", "target"], ["import-row-compression", "repository"], ["import-row-compression", "destination"],
      ["import-place-taken", "repository", "path:vms"], ["import-place-taken", "repository", "offsite:vms"], ["import-place-taken", "repository", "target"],
      ["import-place-taken", "stored-repository", "path:vms"], ["import-place-taken", "stored-repository", "offsite:vms"],
      ["import-place-taken", "stored-repository", "target"],
      ["import-place-twice", "repository"], ["import-place-twice", "stored-repository"],
    ];
    for (const [lang, table] of Object.entries(allLocales)) {
      const tr = (key: TranslationKey) => table[key] ?? en[key];
      for (const [code, part, place] of rows) {
        const text = bare(importProblemText(tr, lang, { ok: false, code, part, row: 7, name: "Cold", place }));
        expect(text, `${lang} ${code} ${part} ${place ?? ""}`).not.toMatch(/\{[a-z]+\}/);
        expect(text?.includes(part === "stored-repository" ? "Cold" : "7"), `${lang} ${code} ${part}`).toBe(true);
      }
    }
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
