// ---------------------------------------------------------------------------
// withLtrFragments — the leading-`/` hint-text fix (RTL sweep, form-engine
// Phase 2 Task 6 follow-up). Same test approach as RevealInput.test.ts/
// Toggle.test.ts: a pure function returning a plain React element tree, so
// it's called directly and the tree inspected as plain objects — no jsdom.
//
// Exercised against the REAL production translation strings (English,
// Arabic, Hebrew — copy-pasted from lib/i18n.ts / lib/locales/{ar,he}.ts,
// not paraphrased), since that's exactly what's live-verified elsewhere for
// offsite.repoLocalHint (pixel-measured in a real browser, both locales,
// both themes). excludes.hint and recovery.foreignAppdataDestHint use the
// identical shared function, but the Containers/Recovery pages that render
// them need real Docker containers / restic snapshots to reach live in a
// sandbox with neither — so their coverage is here instead of Playwright.
// ---------------------------------------------------------------------------
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
import { locales, type TranslationKey } from "./i18n";

interface ElementNode {
  type?: unknown;
  props?: { dir?: string; children?: unknown };
}

function isElementNode(node: unknown): node is ElementNode {
  return typeof node === "object" && node !== null && "props" in node;
}

/** Flattens the returned ReactNode[] back into a plain string, verifying
 *  along the way that every non-string piece is a `dir="ltr"` span (nothing
 *  else this function can produce) — i.e. round-trips to prove no text was
 *  dropped or duplicated by the split/wrap. */
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

  it("matches the longer fragment first so a shorter one that's its own substring doesn't cannibalize it", () => {
    // "/mnt" is a substring of "/mnt/x" — listing the longer one first must
    // consume the full path in one piece, leaving only the OTHER standalone
    // "/mnt" occurrence for the second pass.
    const out = withLtrFragments("no /mnt here, but /mnt/x there", ["/mnt/x", "/mnt"]);
    const { text, ltrPieces } = flattenAndCheckSpans(out);
    expect(text).toBe("no /mnt here, but /mnt/x there");
    expect(ltrPieces).toEqual(["/mnt", "/mnt/x"]);
  });

  describe("offsite.repoLocalHint — the reported worst case (reads as the wrong path syntax)", () => {
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
      // The standalone "/mnt" reference AND the full example path both get
      // isolated — the relative counterpart ("remotes/nas/bombvault", no
      // leading /) does NOT, since it starts with a letter and already
      // renders correctly untouched (per the class of bug this fixes).
      expect(ltrPieces).toEqual(["/mnt", "/mnt/remotes/nas/bombvault"]);
    });
  });

  describe("excludes.hint — the leading-/ path example", () => {
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

  describe("recovery.foreignAppdataDestHint — the leading-/ pool path example", () => {
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

// ---------------------------------------------------------------------------
// Locale drift guard — the ONLY way this mechanism can fail in production.
//
// The cases above prove the function does the right thing to strings that are
// copies of today's translations. They cannot notice a translator editing the
// REAL table tomorrow: a literal-substring matcher that stops matching just
// returns the sentence untouched, silently, in that one locale. Since the
// whole reason offsite.repoLocalHint is wrapped at all is that it TEACHES
// exact path syntax, an unprotected locale is a correctness bug, not a
// cosmetic one — so it has to fail the build, loudly, the moment it happens.
//
// This runs against the live registry (lib/i18n's `locales`), not a copy.
// ---------------------------------------------------------------------------
describe("declared fragments vs. the real locale tables", () => {
  it("registers every fragment list this module exports", () => {
    const exported = Object.keys(ltrFragmentsModule).filter((n) => n.endsWith("_LTR_FRAGMENTS"));
    const registered = Object.values(LTR_FRAGMENTS_BY_KEY);
    for (const name of exported) {
      const list = (ltrFragmentsModule as unknown as Record<string, readonly string[]>)[name];
      expect(registered, `${name} is not registered in LTR_FRAGMENTS_BY_KEY`).toContain(list);
    }
    // Every DISTINCT registered value is one of the exported lists, so the map
    // can hold no ad-hoc inline array that escapes the checks below. A set
    // comparison rather than a length equality, because several keys share one
    // list (four cards name /config, five name /boot) — which is why the lists
    // are named after the PATH rather than after a single key.
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

      it("is translated in more than just English (sanity floor for the sweep below)", () => {
        expect(defining.length).toBeGreaterThan(20);
      });

      it.each(defining.map(([code]) => code))(
        "still contains every declared technical fragment, verbatim, in %s",
        (code) => {
          const value = locales[code][key as TranslationKey] as string;
          for (const frag of frags) {
            expect(
              value.includes(frag),
              `locales.${code}["${key}"] no longer contains "${frag}" — withLtrFragments() would silently stop pinning it LTR for this locale`
            ).toBe(true);
          }
          // And the wrap must round-trip: every declared fragment actually
          // becomes its own dir="ltr" span, with no text lost or duplicated.
          const { text, ltrPieces } = flattenAndCheckSpans(withLtrFragments(value, frags));
          expect(text).toBe(value);
          for (const frag of frags) expect(ltrPieces).toContain(frag);
        }
      );
    });
  }
});

// ---------------------------------------------------------------------------
// withLtrIsolates — the plain-string counterpart, for the same sentences once
// they move into an InfoBubble tip (a text node that is ALSO the trigger's
// aria-label, so the `<span dir="ltr">` form above has nowhere to live).
// Same fragment lists, same guard below, U+2066/U+2069 instead of markup.
// ---------------------------------------------------------------------------
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
    // Same left-to-right precedence withLtrFragments documents — the inner
    // "/mnt" must NOT be wrapped a second time inside the already-wrapped path.
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

  // Read from the LIVE registry, not a pasted copy: this is the string the
  // Recovery tab's appdata-destination bubble actually renders, in the two
  // RTL locales the leading-`/` bug was originally confirmed in.
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

// ---------------------------------------------------------------------------
// COVERAGE — derived from `en`, not from the registry.
//
// The registry above proves that what IS registered stays correct in every
// locale. It says nothing about what is MISSING, and missing is how this got
// here: three strings were protected while fourteen others baked a leading-`/`
// path into the same kind of translated prose and rendered it unisolated, so in
// ar/he/fa the Flash tab's "Back up" bubble read "boot/" and the cache card
// read "config/". A hand-maintained list of "the strings that mention a path"
// will always lag the strings.
//
// So the required set is computed from the en table itself, and every key it
// finds must be either registered or explicitly exempted with a reason. Adding
// a hint that mentions a path now fails the build until someone decides which.
//
// What this guard does NOT check, stated plainly rather than implied: that each
// registered key's RENDER SITE actually applies the isolation. `tLtr` exists to
// make that hard to get wrong (pass the key, get the isolated string), but a
// call site that reaches for a bare `t()` is still possible and this test would
// not see it.
// ---------------------------------------------------------------------------

/** An absolute path embedded in prose: a `/` that is not preceded by a letter,
 *  digit, `:` or another `/` (so a URL scheme and a mid-path slash are not
 *  false starts), followed by at least one path-ish character. */
const PATH_IN_PROSE = /(?<![A-Za-z0-9:/])\/[A-Za-z0-9._*{}<>-]+(?:\/[A-Za-z0-9._*{}<>-]*)*/;

/** Keys whose `/` match is NOT the bug this module fixes. Each needs a reason:
 *  an unexplained entry here is how a real one gets waved through. */
const NOT_A_PATH: Record<string, string> = {
  "dashboard.forecastGrowth": "a unit, not a path: {bytes}/week, and 'week' is translated per locale",
  "dashboard.forecastShrink": "same unit as forecastGrowth",
  "rclone.pathHint": "the example is rclone:<remote>:<bucket>/path — it BEGINS with letters, which are a strong LTR class that anchors the whole run; only a leading `/` misrenders",
  "recovery.foreignVMDestHint": "the run is <destination>/<vm-name>/ and BOTH placeholder words are translated (sl 'ime-vm', sr 'naziv-vm'), so no literal fragment can match in every locale; the leading character is `<`, not `/`",
  "folders.customPlaceholder": "orphaned key — rendered nowhere (see i18n.orphans.test.ts's ratchet)",
};

describe("coverage: every en string that embeds a path is accounted for", () => {
  const withPaths = Object.entries(locales.en)
    .filter(([, value]) => PATH_IN_PROSE.test(value as string))
    .map(([key]) => key);

  it("finds the strings it is supposed to find (the scan is not silently empty)", () => {
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

  it("keeps NOT_A_PATH honest — every exemption still matches a real key", () => {
    for (const key of Object.keys(NOT_A_PATH)) {
      expect(withPaths, `NOT_A_PATH lists "${key}", which no longer embeds a path`).toContain(key);
      expect(NOT_A_PATH[key].length, `NOT_A_PATH["${key}"] needs a real reason`).toBeGreaterThan(20);
    }
  });
});
