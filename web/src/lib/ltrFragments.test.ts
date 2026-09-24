// withLtrFragments returns a plain React element tree, so it is inspected as
// objects without jsdom. The cases use copies of the real en, ar and he
// strings. The pages that render excludes.hint and
// recovery.foreignAppdataDestHint need live containers and restic snapshots,
// so this is their coverage.
import { describe, expect, it } from "vitest";
import type { ReactNode } from "react";
import * as ltrFragmentsModule from "./ltrFragments";
import {
  withLtrFragments,
  withLtrIsolates,
  LTR_FRAGMENTS_BY_KEY,
  REPO_LOCAL_HINT_LTR_FRAGMENTS,
  EXCLUDES_HINT_LTR_FRAGMENTS,
  FOREIGN_APPDATA_DEST_HINT_LTR_FRAGMENTS,
} from "./ltrFragments";
import { type TranslationKey } from "./i18n";
import { allLocales as locales } from "./localesForTests";

interface ElementNode {
  type?: unknown;
  props?: { dir?: string; children?: unknown };
}

function isElementNode(node: unknown): node is ElementNode {
  return typeof node === "object" && node !== null && "props" in node;
}

/** flattenAndCheckSpans joins the nodes back into a string and checks that
 *  every non-string piece is a `dir="ltr"` span, so a round trip proves the
 *  split dropped or duplicated no text. */
function flattenAndCheckSpans(nodes: ReactNode): { text: string; ltrPieces: string[] } {
  const arr = Array.isArray(nodes) ? nodes : [nodes];
  let text = "";
  const ltrPieces: string[] = [];
  for (const n of arr) {
    if (typeof n === "string") {
      text += n;
      continue;
    }
    expect(isElementNode(n)).toBe(true);
    const el = n as ElementNode;
    expect(el.props?.dir).toBe("ltr");
    const child = el.props?.children;
    expect(typeof child).toBe("string");
    text += child as string;
    ltrPieces.push(child as string);
  }
  return { text, ltrPieces };
}

describe("withLtrFragments", () => {
  it("returns the original string untouched, as a single string node, when no fragment matches", () => {
    const out = withLtrFragments("plain text with no path in it", ["/mnt"]);
    expect(out).toEqual(["plain text with no path in it"]);
  });

  it("isolates a single leading-/ fragment without disturbing the rest of the sentence", () => {
    const out = withLtrFragments("see /etc/hosts for details", ["/etc/hosts"]);
    const arr = out as ReactNode[];
    expect(arr[0]).toBe("see ");
    const span = arr[1] as ElementNode;
    expect(span.props?.dir).toBe("ltr");
    expect(span.props?.children).toBe("/etc/hosts");
    expect(arr[2]).toBe(" for details");
  });

  it("matches a longer fragment before a shorter one it contains", () => {
    // Listed first, "/mnt/x" takes the full path in one piece and leaves only
    // the standalone "/mnt" for the second pass.
    const out = withLtrFragments("no /mnt here, but /mnt/x there", ["/mnt/x", "/mnt"]);
    const { text, ltrPieces } = flattenAndCheckSpans(out);
    expect(text).toBe("no /mnt here, but /mnt/x there");
    expect(ltrPieces).toEqual(["/mnt", "/mnt/x"]);
  });

  describe("offsite.repoLocalHint", () => {
    const EN =
      'Also accepts a plain folder under the "Host Data" mount — enter it relative to that mount, without the leading /mnt: a share at /mnt/remotes/nas/bombvault is entered as remotes/nas/bombvault.';
    const AR =
      'يقبل أيضًا مجلدًا عاديًا داخل نقطة الوصل "Host Data": أدخله بالنسبة إلى نقطة الوصل هذه، بدون /mnt في البداية. المشاركة الموجودة في /mnt/remotes/nas/bombvault تُدخل هكذا: remotes/nas/bombvault.';
    const HE =
      'אפשר גם תיקייה רגילה מתחת לעיגון "Host Data": יש להזין אותה יחסית לעיגון הזה, בלי /mnt בהתחלה. שיתוף שנמצא ב-/mnt/remotes/nas/bombvault מוזן כ-remotes/nas/bombvault.';

    it.each([
      ["en", EN],
      ["ar", AR],
      ["he", HE],
    ])("isolates both leading-/ fragments and round-trips to the exact %s source string", (_locale, source) => {
      const out = withLtrFragments(source, REPO_LOCAL_HINT_LTR_FRAGMENTS);
      const { text, ltrPieces } = flattenAndCheckSpans(out);
      expect(text).toBe(source);
      // The relative "remotes/nas/bombvault" starts with a letter and stays
      // unwrapped.
      expect(ltrPieces).toEqual(["/mnt", "/mnt/remotes/nas/bombvault"]);
    });
  });

  describe("excludes.hint", () => {
    const EN =
      "One pattern per line. A container path (e.g. /config/Library/.../Cache) is matched against the backed-up volume; a bare name like .git matches at any depth. Brace lists like {a,b} are not supported; use one line each.";
    const AR =
      "نمط واحد لكل سطر. يُطابَق مسار الحاوية (مثل /config/Library/.../Cache) مع وحدة التخزين المنسوخة احتياطياً؛ أما اسم مجرد مثل .git فيطابق على أي عمق. لا تُدعم قوائم الأقواس المعقوفة مثل {a,b}؛ استخدم سطراً لكل واحدة.";
    const HE =
      "תבנית אחת בכל שורה. נתיב מכל (למשל /config/Library/.../Cache) מושווה אל אמצעי האחסון המגובה; שם פשוט כמו .git תואם בכל עומק. רשימות בסוגריים מסולסלים כמו {a,b} אינן נתמכות; השתמש בשורה אחת לכל אחת.";

    it.each([
      ["en", EN],
      ["ar", AR],
      ["he", HE],
    ])("isolates the leading-/ path example and round-trips to the exact %s source string", (_locale, source) => {
      const out = withLtrFragments(source, EXCLUDES_HINT_LTR_FRAGMENTS);
      const { text, ltrPieces } = flattenAndCheckSpans(out);
      expect(text).toBe(source);
      expect(ltrPieces).toEqual(["/config/Library/.../Cache"]);
    });
  });

  describe("recovery.foreignAppdataDestHint", () => {
    const EN =
      "Where the container's appdata is restored. Leave blank for the default. A container backed up from a pool this server does not have (for example /mnt/zfs) is remapped here so it lands correctly.";
    const AR =
      "المكان الذي تُستعاد إليه بيانات appdata للحاوية. اتركه فارغًا للوضع الافتراضي. الحاوية التي جرى نسخها احتياطيًا من مجمّع لا يملكه هذا الخادم (مثل /mnt/zfs) يُعاد تعيينها هنا لتصل إلى المكان الصحيح.";
    const HE =
      "לאן משוחזרים נתוני ה-appdata של הקונטיינר. השאירו ריק לברירת המחדל. קונטיינר שגובה מ-pool שאין לשרת הזה (למשל /mnt/zfs) ממופה מחדש לכאן כדי שינחת נכון.";

    it.each([
      ["en", EN],
      ["ar", AR],
      ["he", HE],
    ])("isolates the leading-/ pool path example and round-trips to the exact %s source string", (_locale, source) => {
      const out = withLtrFragments(source, FOREIGN_APPDATA_DEST_HINT_LTR_FRAGMENTS);
      const { text, ltrPieces } = flattenAndCheckSpans(out);
      expect(text).toBe(source);
      expect(ltrPieces).toEqual(["/mnt/zfs"]);
    });
  });
});

// A translator retyping a path makes the substring match fail silently in that
// locale, so every registered key is checked against the live locale tables.
describe("declared fragments vs. the real locale tables", () => {
  it("registers every fragment list this module exports", () => {
    const exported = Object.keys(ltrFragmentsModule).filter((n) => n.endsWith("_LTR_FRAGMENTS"));
    const registered = Object.values(LTR_FRAGMENTS_BY_KEY);
    for (const name of exported) {
      const list = (ltrFragmentsModule as unknown as Record<string, readonly string[]>)[name];
      expect(registered, `${name} is not registered in LTR_FRAGMENTS_BY_KEY`).toContain(list);
    }
    // No inline array may escape the checks below. Several keys share one
    // list, hence a set rather than a length comparison.
    expect(new Set(registered).size).toBe(exported.length);
  });

  it("lists longer fragments before any shorter one they contain (match order matters)", () => {
    for (const [key, frags] of Object.entries(LTR_FRAGMENTS_BY_KEY)) {
      frags.forEach((frag, i) => {
        for (const later of frags.slice(i + 1)) {
          expect(
            frag.includes(later) || !later.includes(frag),
            `${key}: "${later}" contains "${frag}" but is listed after it, so the shorter one eats it first`
          ).toBe(true);
        }
      });
    }
  });

  for (const [key, frags] of Object.entries(LTR_FRAGMENTS_BY_KEY)) {
    describe(key, () => {
      const defining = Object.entries(locales).filter(
        ([, table]) => typeof table[key as TranslationKey] === "string"
      );

      it("is translated in more than just English", () => {
        expect(defining.length).toBeGreaterThan(20);
      });

      it.each(defining.map(([code]) => code))(
        "still contains every declared technical fragment, verbatim, in %s",
        (code) => {
          const value = locales[code][key as TranslationKey] as string;
          for (const frag of frags) {
            expect(
              value.includes(frag),
              `locales.${code}["${key}"] does not contain "${frag}", so withLtrFragments() would stop pinning it LTR for this locale`
            ).toBe(true);
          }
          // Every fragment becomes its own span, with no text lost or
          // duplicated.
          const { text, ltrPieces } = flattenAndCheckSpans(withLtrFragments(value, frags));
          expect(text).toBe(value);
          for (const frag of frags) expect(ltrPieces).toContain(frag);
        }
      );
    });
  }
});

describe("withLtrIsolates", () => {
  const LRI = "\u2066";
  const PDI = "\u2069";

  it("wraps the fragment in a bidi isolate pair and leaves the rest untouched", () => {
    const out = withLtrIsolates("pool such as /mnt/zfs is remapped", ["/mnt/zfs"]);
    expect(out).toBe(`pool such as ${LRI}/mnt/zfs${PDI} is remapped`);
  });

  it("keeps the visible text identical once the two zero-width controls are stripped", () => {
    const src = "a /mnt/zfs b /mnt/zfs c";
    const out = withLtrIsolates(src, ["/mnt/zfs"]);
    expect(out.replaceAll(LRI, "").replaceAll(PDI, "")).toBe(src);
  });

  it("wraps every occurrence, not just the first", () => {
    const out = withLtrIsolates("x /mnt/zfs y /mnt/zfs", ["/mnt/zfs"]);
    expect(out.split(LRI)).toHaveLength(3);
    expect(out.split(PDI)).toHaveLength(3);
  });

  it("honours list order: a longer fragment consumes the shorter one inside it", () => {
    // The inner "/mnt" is not wrapped again inside the wrapped path.
    const out = withLtrIsolates("see /mnt/remotes/nas/bombvault or /mnt", [
      "/mnt/remotes/nas/bombvault",
      "/mnt",
    ]);
    expect(out).toBe(`see ${LRI}/mnt/remotes/nas/bombvault${PDI} or ${LRI}/mnt${PDI}`);
    expect(out).not.toContain(`${LRI}/mnt${PDI}/remotes`);
  });

  it("is a no-op when the fragment is absent (a locale that retyped the path)", () => {
    expect(withLtrIsolates("no path here", ["/mnt/zfs"])).toBe("no path here");
  });

  it("is a no-op for an empty fragment list", () => {
    expect(withLtrIsolates("unchanged", [])).toBe("unchanged");
  });

  // The live string the Recovery tab's appdata-destination bubble renders.
  it.each(["ar", "he"])(
    "protects the real recovery.foreignAppdataDestHint the bubble renders, in %s",
    (code) => {
      const source = locales[code as keyof typeof locales][
        "recovery.foreignAppdataDestHint" as TranslationKey
      ] as string;
      expect(typeof source).toBe("string");
      const out = withLtrIsolates(source, FOREIGN_APPDATA_DEST_HINT_LTR_FRAGMENTS);
      expect(out).toContain(`${LRI}/mnt/zfs${PDI}`);
      expect(out.replaceAll(LRI, "").replaceAll(PDI, "")).toBe(source);
    }
  );
});

// The keys that need registering are derived from en, because a hand-kept list
// of strings that mention a path lags the strings. A new hint with a leading
// "/" path fails until it is registered or exempted with a reason. Whether a
// render site applies the isolation is not checked; tLtr makes that the easy
// path, but a bare t() call would pass.

/** An absolute path embedded in prose: a `/` that is not preceded by a letter,
 *  digit, `:` or another `/` (so a URL scheme and a mid-path slash are not
 *  false starts), followed by at least one path-ish character. */
const PATH_IN_PROSE = /(?<![A-Za-z0-9:/])\/[A-Za-z0-9._*{}<>-]+(?:\/[A-Za-z0-9._*{}<>-]*)*/;

/** Keys whose `/` match is not a leading path, each with the reason. */
const NOT_A_PATH: Record<string, string> = {
  "dashboard.forecastGrowth": "a unit, not a path: {bytes}/week, and 'week' is translated per locale",
  "dashboard.forecastShrink": "same unit as forecastGrowth",
  "rclone.pathHint": "the example is rclone:<remote>:<bucket>/path. It begins with letters, a strong LTR class that anchors the whole run; only a leading `/` misrenders",
  "recovery.foreignVMDestHint": "the run is <destination>/<vm-name>/ and BOTH placeholder words are translated (sl 'ime-vm', sr 'naziv-vm'), so no literal fragment can match in every locale; the leading character is `<`, not `/`",
  "folders.customPlaceholder": "orphaned key, rendered nowhere (see i18n.orphans.test.ts's ratchet)",
  "anomaly.learning": "a fraction, not a path: {n}/{needed} counts the backups the detector has learned from",
};

describe("coverage: every en string that embeds a path is accounted for", () => {
  const withPaths = Object.entries(locales.en)
    .filter(([, value]) => PATH_IN_PROSE.test(value as string))
    .map(([key]) => key);

  it("finds the path-bearing strings", () => {
    expect(withPaths.length).toBeGreaterThan(15);
    expect(withPaths).toContain("flash.backupHint");
    expect(withPaths).toContain("offsite.repoLocalHint");
  });

  it("leaves no path-bearing string unregistered and unexplained", () => {
    const unaccounted = withPaths.filter(
      (key) => !(key in LTR_FRAGMENTS_BY_KEY) && !(key in NOT_A_PATH)
    );
    expect(
      unaccounted,
      `these en strings embed a leading-"/" path and are neither registered in ` +
        `LTR_FRAGMENTS_BY_KEY nor listed in NOT_A_PATH with a reason: ${unaccounted.join(", ")}. ` +
        `In ar/he/fa the leading slash migrates to the far end of the path.`
    ).toEqual([]);
  });

  it("keeps every NOT_A_PATH exemption matching a real key", () => {
    for (const key of Object.keys(NOT_A_PATH)) {
      expect(withPaths, `NOT_A_PATH lists "${key}", which no longer embeds a path`).toContain(key);
      expect(NOT_A_PATH[key].length, `NOT_A_PATH["${key}"] needs a real reason`).toBeGreaterThan(20);
    }
  });
});
