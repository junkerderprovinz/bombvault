// excludesEverything mirrors backup.ExcludesEverything on the server: a line
// that is "/" in any spelling, or a glob of only "*" and "**" segments,
// matches every path, so the backup ignores it. The segments are cleaned the
// way Go's path.Clean does it.
export function excludesEverything(line: string): boolean {
  const trimmed = line.trim();
  if (trimmed === "") return false;
  const rooted = trimmed.startsWith("/");
  const segs: string[] = [];
  for (const seg of trimmed.split("/")) {
    if (seg === "" || seg === ".") continue;
    if (seg === "..") {
      if (segs.length > 0 && segs[segs.length - 1] !== "..") segs.pop();
      else if (!rooted) segs.push("..");
      continue;
    }
    segs.push(seg);
  }
  if (!rooted && segs.length === 0) return false;
  return segs.every((s) => s === "*" || s === "**");
}
