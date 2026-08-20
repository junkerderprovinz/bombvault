import type { ReactNode } from "react";

// ---------------------------------------------------------------------------
// withLtrFragments — isolates a literal technical substring (a path) that is
// baked directly INTO an already-translated hint sentence, for the handful
// of `t()` strings where the example isn't a separately-interpolated live
// value the call site could wrap on its own (the RTL sweep's usual
// `<span dir="ltr">{value}</span>` pattern — see e.g. Settings.tsx's
// hostMountRoot hint — only works when the technical bit is a JSX-level
// variable, not text that's already inside a translated string).
//
// form-engine Phase 2 Task 6 pinned every technical value it could reach as
// a JSX variable to dir="ltr", but missed three hint strings whose example
// path is embedded directly in the (translated, per-locale) prose: a
// mid-sentence fragment beginning with a letter resolves fine untouched
// (Latin letters are a "strong" LTR bidi class that anchors its own
// direction even inside RTL prose), but one beginning with `/` does NOT —
// `/` is a weak/neutral bidi character, so with nothing anchoring it the
// Unicode Bidi Algorithm sweeps it into the surrounding RTL run and the
// leading `/` visually migrates to the fragment's trailing edge instead of
// staying put (confirmed live in Arabic and Hebrew: "/mnt/remotes/nas/
// bombvault" was rendering as "mnt/remotes/nas/bombvault/"). This is the
// same class of bug `dir="ltr"` fixes everywhere else in the sweep; the only
// difference here is WHERE the isolation has to be applied, since there's no
// live variable at the call site to hang a `dir="ltr"` span off of directly.
//
// Splits `text` on each literal string in `fragments`, in that order, and
// wraps every match in its own `dir="ltr"` span; everything else renders
// untouched, so the surrounding TRANSLATED sentence keeps reading in the
// page's own direction and only the technical fragment is pinned — never
// the whole sentence (wrapping the whole `<p>` in dir="ltr" would flip the
// translated prose itself into reading left-to-right too).
//
// Safe with NO per-locale changes: every fragment this is used for is a
// filesystem path, which is never translated, so it appears byte-for-byte
// identical in every one of this app's 27 shipped locales' copy of the
// string (verified against all of them for every call site below) — this
// can match against whatever `t()` already returned, in any locale.
//
// List longer/more-specific fragments before a shorter one that could be a
// substring of them (e.g. a full path before a standalone directory name
// that also occurs, unprefixed, elsewhere in the same sentence) — matching
// runs left-to-right through the list, and text already consumed into a
// span on an earlier pass is no longer eligible for a later, shorter match.
// ---------------------------------------------------------------------------
export function withLtrFragments(text: string, fragments: readonly string[]): ReactNode {
  let parts: ReactNode[] = [text];
  fragments.forEach((frag, fi) => {
    if (!frag) return;
    parts = parts.flatMap((part, pi): ReactNode[] => {
      if (typeof part !== "string" || !part.includes(frag)) return [part];
      const pieces = part.split(frag);
      const out: ReactNode[] = [];
      pieces.forEach((piece, i) => {
        if (piece) out.push(piece);
        if (i < pieces.length - 1) {
          out.push(
            <span key={`${fi}-${pi}-${i}`} dir="ltr" className="font-mono text-start">
              {frag}
            </span>
          );
        }
      });
      return out;
    });
  });
  return parts;
}

/** offsite.repoLocalHint's two leading-`/` fragments: the standalone `/mnt`
 *  reference and the full example path. The relative counterpart it also
 *  names ("remotes/nas/bombvault") starts with a letter and needs no
 *  wrapping — it already renders correctly, per the class of bug above. */
export const REPO_LOCAL_HINT_LTR_FRAGMENTS = ["/mnt/remotes/nas/bombvault", "/mnt"] as const;

/** excludes.hint's leading-`/` example path. `.git` and `{a,b}` elsewhere in
 *  the same sentence start with a non-`/` character and already render
 *  correctly untouched. */
export const EXCLUDES_HINT_LTR_FRAGMENTS = ["/config/Library/.../Cache"] as const;

/** recovery.foreignAppdataDestHint's leading-`/` example pool path. */
export const FOREIGN_APPDATA_DEST_HINT_LTR_FRAGMENTS = ["/mnt/zfs"] as const;

// ---------------------------------------------------------------------------
// Which translation key each fragment list belongs to. The whole mechanism
// above matches by LITERAL SUBSTRING against whatever `t()` returned, so its
// one failure mode is silent: if a translator ever retypes the path in one
// locale (a full-width slash, a stray space, a "translated" folder name, a
// different example), nothing throws and nothing looks wrong in code review —
// that ONE locale just quietly stops being protected and goes back to
// rendering the leading `/` at the wrong end of the path, which for
// offsite.repoLocalHint means teaching that locale's readers the wrong path
// syntax. The parity guard in ltrFragments.test.ts iterates this map against
// the REAL locale registry so that drift fails the build instead.
//
// REGISTER EVERY NEW FRAGMENT LIST HERE — the test also asserts that every
// `*_LTR_FRAGMENTS` export from this module appears below, so a list added
// without a key can't slip past unguarded.
// ---------------------------------------------------------------------------
export const LTR_FRAGMENTS_BY_KEY = {
  "offsite.repoLocalHint": REPO_LOCAL_HINT_LTR_FRAGMENTS,
  "excludes.hint": EXCLUDES_HINT_LTR_FRAGMENTS,
  "recovery.foreignAppdataDestHint": FOREIGN_APPDATA_DEST_HINT_LTR_FRAGMENTS,
} as const satisfies Record<string, readonly string[]>;
