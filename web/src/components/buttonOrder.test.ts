// In a pair of buttons, the one that moves the thing forward comes last: Save
// after Cancel, Confirm after Cancel, Run after Pause. The accent already marks
// the primary control; its position repeats that in glyph-only mode and for
// colour-blind readers. Under RTL the row mirrors, so the guard checks source
// order rather than pixels.
//
// Only two <Button> tags adjacent in the source are checked, one with a known
// hold key and one with a known forward key. Pairs split by a conditional or
// with unknown keys are skipped, since inferring roles from arbitrary keys
// would be guesswork.
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { buttonTags } from "./buttonTags.testsupport";

const SRC = join(__dirname, "..");

// Keys that hold or take back, matched anywhere in the labelKey.
const HOLD = /cancel|close$|skip|decline|reset$|back$|dismiss/i;

// Keys that move the thing forward.
const FORWARD =
  /save$|confirm|apply|create$|accept|connect|restore|import|enable|disable|regenerate|proceed|continue/i;

// Keys from real pairs that the patterns must classify. A pattern can look
// right and still miss: `\bcreate$` does not match `auth.passkeyCreate`, since
// there is no word boundary between "y" and "C".
const MUST_BE_FORWARD = [
  "offsite.targets.save",
  "settings.save",
  "auth.passkeyCreate",
  "auth.twoFactorConfirm",
  "auth.twoFactorDisable",
  "settingsIO.confirmButton",
  "backupOrder.save",
  "fleet.mesh.accept",
  "recovery.foreignConnect",
  "recovery.configRestore",
];
const MUST_BE_HOLD = [
  "offsite.targets.cancel",
  "common.close",
  "common.cancel",
  "settingsIO.cancel",
  "backupOrder.reset",
  "fleet.mesh.decline",
  "recovery.foreignClose",
  "recovery.configSkip",
];

function keyOf(props: string): string | null {
  const m = /\blabelKey\s*=\s*"([^"]+)"/.exec(props);
  return m ? m[1] : null;
}

const tags = buttonTags(SRC);

type Wrong = { where: string; forward: string; hold: string };

const wrong: Wrong[] = [];
for (let i = 0; i < tags.length - 1; i++) {
  const a = tags[i];
  const b = tags[i + 1];
  if (a.file !== b.file) continue;
  // Only siblings form a pair: nothing but whitespace and a closing tag may lie
  // between them. A `?`, `:`, `&&` or `||` in the gap means two branches that
  // never render together.
  const gap = a.source.slice(a.end, b.start);
  if (/[?:]|&&|\|\|/.test(gap)) continue;
  if (!/^[\s]*(\/>|<\/[A-Za-z.]+>)?[\s]*$/.test(gap)) continue;

  const ka = keyOf(a.props);
  const kb = keyOf(b.props);
  if (!ka || !kb) continue;

  if (FORWARD.test(ka) && !HOLD.test(ka) && HOLD.test(kb)) {
    wrong.push({ where: `${a.file}:${a.line}`, forward: ka, hold: kb });
  }
}

describe("button order in a pair", () => {
  it("the scan reached the buttons", () => {
    expect(
      tags.length,
      "the <Button> scan found almost nothing, so this guard is measuring its\n" +
        "own regex rather than the app.",
    ).toBeGreaterThan(150);
    const keys = tags.map((t) => keyOf(t.props)).filter(Boolean) as string[];
    expect(
      keys.filter((k) => HOLD.test(k)).length,
      "no hold-role keys were recognised at all, so nothing below can fail.",
    ).toBeGreaterThan(5);
    expect(
      keys.filter((k) => FORWARD.test(k)).length,
      "no forward-role keys were recognised at all.",
    ).toBeGreaterThan(5);

    // The counts above would also pass with a pattern that matches the wrong set.
    for (const k of MUST_BE_FORWARD) {
      expect(FORWARD.test(k), `${k} is not recognised as a forward role`).toBe(true);
      expect(HOLD.test(k), `${k} is wrongly recognised as a hold role`).toBe(false);
    }
    for (const k of MUST_BE_HOLD) {
      expect(HOLD.test(k), `${k} is not recognised as a hold role`).toBe(true);
    }
  });

  it("puts the control that goes ahead last", () => {
    const report = wrong
      .map((w) => `  ${w.where}  ${w.forward} stands before ${w.hold}`)
      .join("\n");
    expect(
      wrong,
      `These pairs have the forward control first (GlimStone 1.14.0):\n\n${report}\n\n` +
        `Swap the two <Button> blocks. Nothing about the props changes - the\n` +
        `accent stays on the forward one, it just moves to the end of the row,\n` +
        `where it mirrors correctly under RTL for free.`,
    ).toEqual([]);
  });
});
