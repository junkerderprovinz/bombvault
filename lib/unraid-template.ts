import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

/** Returns the canonical filename for a container template, e.g. "my-Plex.xml". */
export function templateFileName(name: string): string {
  return `my-${name}.xml`;
}

/**
 * Reads the Unraid template XML for the given container name from `dir`.
 * Returns the file contents, or `null` if the file does not exist.
 */
export function readTemplate(dir: string, name: string): string | null {
  const filePath = join(dir, templateFileName(name));
  if (!existsSync(filePath)) return null;
  return readFileSync(filePath, "utf8");
}

/**
 * Writes the Unraid template XML for the given container name into `dir`.
 * Creates the directory (and any parents) if it does not exist.
 */
export function writeTemplate(dir: string, name: string, xml: string): void {
  mkdirSync(dir, { recursive: true });
  writeFileSync(join(dir, templateFileName(name)), xml, "utf8");
}
