// A control dimmed by a decision taken elsewhere has to say who is in charge.
// Dimming alone suits a control reporting on itself, such as an arrow at the
// end of its range. A control whose value still applies while another setting
// overrides it (the accent swatches under rainbow mode, the domain schedules
// while "use the Containers schedule" is on) stays visible and dimmed, with an
// InfoBubble shown under the same condition. Without the bubble the control
// looks broken.
//
// The check runs per component, so one section's bubble cannot cover for a
// sibling that lost its own.
import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const SRC = join(dirname(fileURLToPath(import.meta.url)), "..");

function sources(dir: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir)) {
    if (entry === "node_modules" || entry === "dist" || entry === "locales") continue;
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) {
      out.push(...sources(full));
      continue;
    }
    if (/\.tsx$/.test(entry) && !/\.test\.tsx$/.test(entry)) out.push(full);
  }
  return out;
}

// Strips comments, which may quote props that no longer exist.
function code(text: string): string {
  return text
    .replace(/\/\*[\s\S]*?\*\//g, (m) => m.replace(/[^\n]/g, " "))
    .replace(/(^|[^:\w])\/\/[^\n]*/g, (m, p1: string) => p1 + m.slice(p1.length).replace(/./g, " "));
}

/**
 * Returns the flag a `disabled={...}` expression takes from elsewhere, or null.
 * A flag counts when it looks like a feature switch (a settings field,
 * `...Enabled`, `rainbowOn`, `syncSchedules`) and is not declared in this file;
 * a flag computed here, like NumberField's `enabled`, is the component's own
 * state. An expression mentioning `busy` is a row saving itself and never
 * counts, even as `fieldBusy.digestEnabled`.
 */
function ownerFlag(expr: string, file: string): string | null {
  if (/busy/i.test(expr)) return null;
  const candidates = expr.match(/!?\b(?:settings\.\w+|\w*(?:Enabled|Schedules)|rainbowOn|rainbow\.on)\b/g);
  for (const raw of candidates ?? []) {
    const flag = raw.replace(/^!/, "");
    if (new RegExp(`\\bconst\\s+${flag.replace(/\W/g, "\\$&")}\\b`).test(file)) continue;
    return flag;
  }
  return null;
}

/**
 * Spans of the top-level functions and consts with a capitalised name. The
 * bubble has to sit in the same component as the control it explains.
 */
function components(file: string): { name: string; start: number; end: number }[] {
  const heads = [...file.matchAll(/^(?:export\s+)?(?:function|const)\s+([A-Z]\w*)/gm)];
  return heads.map((h, i) => ({
    name: h[1],
    start: h.index!,
    end: i + 1 < heads.length ? heads[i + 1].index! : file.length,
  }));
}

/** A bubble that appears only while `flag` is on, in either spelling. */
function namesTheOwner(window: string, flag: string): boolean {
  const f = flag.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  return (
    // {flag && <InfoBubble tip={...} />}
    new RegExp(`${f}\\s*&&\\s*(?:\\(\\s*)?<InfoBubble`).test(window) ||
    // hint={flag ? t("...") : undefined}
    new RegExp(`hint=\\{[^}]*${f}\\s*\\?`).test(window) ||
    // {flag && <UnavailableNotice ... />}
    new RegExp(`${f}\\s*&&\\s*(?:\\(\\s*)?<UnavailableNotice`).test(window)
  );
}

describe("a control dimmed by a decision elsewhere names who is in charge", () => {
  const files = sources(SRC);

  it("scans the components that have such a control", () => {
    const offenders: string[] = [];
    let checked = 0;

    for (const path of files) {
      const text = code(readFileSync(path, "utf8"));
      const windows = components(text);

      for (const m of text.matchAll(/disabled=\{([^}]*)\}/g)) {
        const flag = ownerFlag(m[1], text);
        if (!flag) continue;
        checked++;
        const w = windows.find((c) => m.index! >= c.start && m.index! < c.end);
        const window = w ? text.slice(w.start, w.end) : text;
        if (namesTheOwner(window, flag)) continue;
        const line = text.slice(0, m.index!).split("\n").length;
        offenders.push(
          `${relative(SRC, path)}:${line} - disabled={${m[1].trim()}} is grey because ` +
            `${flag} was decided elsewhere, and ${w ? w.name : "the file"} never says so. ` +
            `Either drop the control while ${flag} is on, or render an InfoBubble under the same condition.`
        );
      }
    }

    // A scan that finds nothing, say after a prop rename, would pass silently.
    expect(checked).toBeGreaterThanOrEqual(5);
    expect(offenders).toEqual([]);
  });
});
