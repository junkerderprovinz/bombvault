import type { ReactNode } from "react";
import {
  IconBook,
  IconCalendar,
  IconCube,
  IconDisc,
  IconFile,
  IconFileAudio,
  IconFileDesign,
  IconFileDoc,
  IconFileImage,
  IconFilePdf,
  IconFilePlaylist,
  IconFileProgram,
  IconFileRaw,
  IconFileSheet,
  IconFileSlides,
  IconFileSubtitles,
  IconFileText,
  IconFileVideo,
  IconFolder,
  IconFont,
  IconHash,
  IconHourglass,
  IconMagnet,
} from "./navGlyphs";
import { IconArchive } from "./glyphs";

const KINDS: [() => ReactNode, string[]][] = [
  [IconFileVideo, ["mkv", "mp4", "m4v", "avi", "mov", "webm", "wmv", "mpg", "mpeg", "ts", "m2ts", "flv", "3gp"]],
  [IconFileAudio, ["mp3", "flac", "wav", "ogg", "oga", "opus", "m4a", "m4b", "aac", "wma", "alac", "aiff", "ape"]],
  [IconFileRaw, ["cr2", "cr3", "nef", "arw", "dng", "raf", "orf", "rw2", "pef", "srw"]],
  [IconFileDesign, ["svg", "ai", "eps", "psd", "xcf", "sketch", "fig", "kra"]],
  [IconFileImage, ["jpg", "jpeg", "png", "gif", "webp", "bmp", "tif", "tiff", "heic", "heif", "avif", "ico"]],
  [IconFileSubtitles, ["srt", "ass", "ssa", "vtt", "sub", "idx", "sup"]],
  [IconFilePlaylist, ["m3u", "m3u8", "pls", "xspf", "cue"]],
  [IconFilePdf, ["pdf"]],
  [IconFileDoc, ["doc", "docx", "odt", "rtf", "pages"]],
  [IconFileSheet, ["xls", "xlsx", "ods", "csv", "tsv", "numbers"]],
  [IconFileSlides, ["ppt", "pptx", "odp", "key"]],
  [IconBook, ["epub", "mobi", "azw", "azw3", "cbz", "cbr", "fb2"]],
  [IconArchive, ["zip", "tar", "gz", "tgz", "bz2", "xz", "zst", "7z", "rar", "lz4"]],
  [IconDisc, ["iso", "img", "qcow2", "vmdk", "vdi", "vhd", "vhdx", "raw"]],
  [IconFileProgram, ["exe", "msi", "deb", "rpm", "apk", "appimage", "dmg", "pkg", "jar", "sh", "bin"]],
  [IconFont, ["ttf", "otf", "woff", "woff2"]],
  [IconHash, ["sha1", "sha256", "sha512", "md5", "sfv", "sig", "asc", "gpg"]],
  [IconMagnet, ["torrent"]],
  [IconCube, ["stl", "obj", "3mf", "step", "stp", "blend", "fbx", "gltf", "glb"]],
  [IconCalendar, ["ics", "vcf"]],
  [IconHourglass, ["bak", "tmp", "temp", "part", "crdownload", "swp", "old"]],
  [IconFileText, ["txt", "md", "log", "nfo", "ini", "conf", "cfg", "yml", "yaml", "json", "xml", "toml", "env"]],
];

const BY_EXTENSION = new Map(KINDS.flatMap(([glyph, exts]) => exts.map((e) => [e, glyph] as const)));

/** The glyph that says what a file holds, read from the extension of its name or path. */
export function fileKindGlyph(path: string, isDir: boolean): ReactNode {
  if (isDir) return <IconFolder />;
  const name = path.slice(path.lastIndexOf("/") + 1);
  const dot = name.lastIndexOf(".");
  const Glyph = (dot > 0 && BY_EXTENSION.get(name.slice(dot + 1).toLowerCase())) || IconFile;
  return <Glyph />;
}
