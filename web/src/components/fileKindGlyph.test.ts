import type { ReactElement } from "react";
import { describe, expect, it } from "vitest";
import { fileKindGlyph } from "./fileKindGlyph";
import { IconArchive } from "./glyphs";
import { IconFile, IconFileText, IconFileVideo, IconFolder } from "./navGlyphs";

const glyphOf = (path: string, isDir = false) => (fileKindGlyph(path, isDir) as ReactElement).type;

describe("fileKindGlyph", () => {
  it("reads the kind from the extension, in any case", () => {
    expect(glyphOf("/mnt/user/media/Holiday.MKV")).toBe(IconFileVideo);
    expect(glyphOf("backup.tar.gz")).toBe(IconArchive);
    expect(glyphOf("notes.md")).toBe(IconFileText);
  });

  it("looks only at the name, never at a folder on the way to it", () => {
    expect(glyphOf("/appdata/plex.v2/Preferences")).toBe(IconFile);
  });

  it("takes a leading dot for part of the name", () => {
    expect(glyphOf("/root/.bashrc")).toBe(IconFile);
  });

  it("shows a folder as a folder whatever its name says", () => {
    expect(glyphOf("/mnt/user/movies.mkv", true)).toBe(IconFolder);
  });
});
