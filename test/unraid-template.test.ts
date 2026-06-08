import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import {
  templateFileName,
  readTemplate,
  writeTemplate,
} from "../lib/unraid-template.js";

const FIXTURE_XML = readFileSync(
  new URL("./fixtures/my-Plex.xml", import.meta.url),
  "utf8",
);

describe("templateFileName", () => {
  it("returns my-<Name>.xml for a given container name", () => {
    assert.strictEqual(templateFileName("Plex"), "my-Plex.xml");
  });

  it("preserves casing of the name", () => {
    assert.strictEqual(templateFileName("HomeAssistant"), "my-HomeAssistant.xml");
  });
});

describe("readTemplate", () => {
  it("returns null when the template file does not exist", () => {
    const result = readTemplate("/nonexistent/flash/templates", "Plex");
    assert.strictEqual(result, null);
  });

  it("returns file contents when the template exists", () => {
    const dir = mkdtempSync(join(tmpdir(), "bv-tmpl-read-"));
    try {
      writeTemplate(dir, "Plex", FIXTURE_XML);
      const result = readTemplate(dir, "Plex");
      assert.strictEqual(result, FIXTURE_XML);
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  });
});

describe("writeTemplate / readTemplate roundtrip", () => {
  it("writes xml to <dir>/my-<Name>.xml and reads it back identically", () => {
    const dir = mkdtempSync(join(tmpdir(), "bv-tmpl-rt-"));
    try {
      const xml = FIXTURE_XML;
      writeTemplate(dir, "Plex", xml);
      const result = readTemplate(dir, "Plex");
      assert.strictEqual(result, xml);
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  });

  it("creates the directory recursively if it does not exist", () => {
    const base = mkdtempSync(join(tmpdir(), "bv-tmpl-mkdir-"));
    const nested = join(base, "deep", "nested", "dir");
    try {
      writeTemplate(nested, "Plex", FIXTURE_XML);
      const result = readTemplate(nested, "Plex");
      assert.strictEqual(result, FIXTURE_XML);
    } finally {
      rmSync(base, { recursive: true, force: true });
    }
  });
});
