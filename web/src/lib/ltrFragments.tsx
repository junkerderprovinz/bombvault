import type { ReactNode } from "react";
import type { TranslationKey } from "./i18n";

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

/**
 * withLtrPlaceholder — the RUNTIME-VALUE counterpart of withLtrFragments.
 *
 * withLtrFragments protects a technical string baked INTO the translation. This
 * protects one substituted into it: a `{path}` token whose replacement is only
 * known at render time (the folder a scan stopped inside, the backup folders it
 * could not read). The app's usual `t("…").replace("{path}", value)` produces
 * one flat string, and a value beginning with `/` then hits exactly the bug
 * this module exists for — `/` is a weak bidi class, so in ar/he/fa the leading
 * slash migrates to the far end of the path and the user is shown a folder name
 * they cannot type back.
 *
 * The surrounding TRANSLATED prose is left alone and keeps reading in the
 * page's own direction; only the substituted value is pinned. A token that does
 * not occur (a locale that dropped it — the parity test catches that on its
 * own) renders the sentence unchanged rather than throwing.
 */
export function withLtrPlaceholder(text: string, token: string, value: string): ReactNode {
  const pieces = text.split(token);
  const out: ReactNode[] = [];
  pieces.forEach((piece, i) => {
    if (piece) out.push(piece);
    if (i < pieces.length - 1) {
      out.push(
        <span key={`v-${i}`} dir="ltr" className="font-mono text-start">
          {value}
        </span>
      );
    }
  });
  return out;
}

/**
 * withLtrIsolates — the PLAIN-STRING counterpart of withLtrFragments above,
 * for the same sentences when they land somewhere that can only hold text.
 * An InfoBubble tip is exactly that place: it renders as a text node and is
 * ALSO the trigger's `aria-label`, so a `<span dir="ltr">` has nowhere to go.
 *
 * The isolation is expressed instead with the two Unicode characters the
 * bidi algorithm defines for this job: U+2066 LEFT-TO-RIGHT ISOLATE opens a
 * run whose direction is forced LTR and whose resolved direction cannot leak
 * into the surrounding text, U+2069 POP DIRECTIONAL ISOLATE closes it. This
 * is the same mechanism `dir="ltr"` uses — HTML's dir attribute is specified
 * in terms of these isolates — so a leading `/` stays at the fragment's
 * leading edge in Arabic and Hebrew instead of migrating to the other end,
 * which is the entire bug this module exists for.
 *
 * Both characters are zero-width, invisible to sighted readers and ignored
 * by screen readers, so the accessible name a bubble exposes is unchanged.
 *
 * Same matching rules, same fragment lists and the same single silent
 * failure mode as withLtrFragments (a translator retyping the path in one
 * locale quietly un-protects that locale) — guarded by the same
 * LTR_FRAGMENTS_BY_KEY parity test, which is why this deliberately shares
 * those lists instead of growing a parallel set of its own.
 */
export function withLtrIsolates(text: string, fragments: readonly string[]): string {
  // Written as escapes on purpose: both characters are zero-width, so a
  // literal one in source is invisible in every editor and diff — the exact
  // kind of thing a later edit deletes by accident without anything looking
  // wrong.
  const LRI = "\u2066";
  const PDI = "\u2069";
  // Mirrors withLtrFragments' own left-to-right precedence: a piece already
  // consumed into an isolate by an earlier (longer) fragment is no longer
  // eligible for a later, shorter match, so listing a full path before a
  // directory name that also occurs inside it behaves identically in both
  // functions.
  let parts: { text: string; wrapped: boolean }[] = [{ text, wrapped: false }];
  for (const frag of fragments) {
    if (!frag) continue;
    parts = parts.flatMap((part) => {
      if (part.wrapped || !part.text.includes(frag)) return [part];
      const pieces = part.text.split(frag);
      const out: { text: string; wrapped: boolean }[] = [];
      pieces.forEach((piece, i) => {
        if (piece) out.push({ text: piece, wrapped: false });
        if (i < pieces.length - 1) out.push({ text: frag, wrapped: true });
      });
      return out;
    });
  }
  return parts.map((p) => (p.wrapped ? `${LRI}${p.text}${PDI}` : p.text)).join("");
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
// The rest of the app's path-in-prose strings.
//
// The three lists above were the ones the RTL sweep happened to reach. They
// were never the only ones: fourteen more translated strings bake a
// leading-`/` path into their prose and hit exactly the same bug, so in ar/he/
// fa the Flash tab's "Back up" bubble read "boot/" instead of "/boot", the
// Storage tab's cache card said "config/", and so on. The coverage guard in
// ltrFragments.test.ts now derives the full set from `en` rather than trusting
// this list to be complete, so the next hint that mentions a path fails the
// build until it is either registered here or explicitly exempted.
//
// Every fragment below was checked to appear byte-for-byte in all 42 tables
// (the parity test in ltrFragments.test.ts re-checks it on every run, which is
// what keeps a translator's retyped path from silently un-protecting one
// locale).
// ---------------------------------------------------------------------------

/** BombVault's own config volume, named in prose by four different cards. */
export const CONFIG_VOLUME_LTR_FRAGMENTS = ["/config"] as const;

/** The Unraid USB mount, named by the Flash tab and its schedule/settings rows. */
export const BOOT_VOLUME_LTR_FRAGMENTS = ["/boot"] as const;

/** notify.unraidPlatformMismatch names BOTH the host path and the in-container
 *  one, so the longer `/host/boot` is listed first — see the ordering rule in
 *  withLtrFragments' own doc. */
export const UNRAID_PLATFORM_MISMATCH_LTR_FRAGMENTS = ["/host/boot", "/boot"] as const;

/** excludes.placeholder's example path (the input's own placeholder text). */
export const EXCLUDES_PLACEHOLDER_LTR_FRAGMENTS = ["/config/Library/Application"] as const;

/** The Prometheus scrape endpoint, named by the metrics toggle and its hint. */
export const METRICS_ENDPOINT_LTR_FRAGMENTS = ["/metrics"] as const;

/** Apprise's endpoint. Only the `/notify/` prefix is pinned: the `<key>`
 *  placeholder after it IS translated in some locales (sl "ključ", sr "кључ"),
 *  so a longer fragment would silently stop matching there — and the prefix is
 *  where the leading `/` that misrenders actually lives. */
export const APPRISE_ENDPOINT_LTR_FRAGMENTS = ["/notify/"] as const;

/** The authorized-keys path the VM SSH card tells the user to append to. */
export const VM_SSH_KEY_PATH_LTR_FRAGMENTS = ["/root/.ssh/authorized_keys"] as const;

/** cadence.cronInvalid's worked example. Not a path, but the same bug: a run of
 *  digits, spaces, `*` and `/` is entirely weak/neutral bidi classes, so an RTL
 *  paragraph reorders the whole expression and the user is shown a cron line
 *  they cannot type back. */
export const CRON_EXAMPLE_LTR_FRAGMENTS = ["0 */6 * * *"] as const;

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
  "config.backupHint": CONFIG_VOLUME_LTR_FRAGMENTS,
  "config.enabledHint": CONFIG_VOLUME_LTR_FRAGMENTS,
  "containers.discoverHint": CONFIG_VOLUME_LTR_FRAGMENTS,
  "settings.cacheHint": CONFIG_VOLUME_LTR_FRAGMENTS,
  "excludes.placeholder": EXCLUDES_PLACEHOLDER_LTR_FRAGMENTS,
  "flash.backupHint": BOOT_VOLUME_LTR_FRAGMENTS,
  "flash.restoreNote": BOOT_VOLUME_LTR_FRAGMENTS,
  "flash.subtitle": BOOT_VOLUME_LTR_FRAGMENTS,
  "jobs.flashScheduleHint": BOOT_VOLUME_LTR_FRAGMENTS,
  "settings.flashEnabledHint": BOOT_VOLUME_LTR_FRAGMENTS,
  "notify.unraidPlatformMismatch": UNRAID_PLATFORM_MISMATCH_LTR_FRAGMENTS,
  "notify.appriseHint": APPRISE_ENDPOINT_LTR_FRAGMENTS,
  "settings.metricsEnable": METRICS_ENDPOINT_LTR_FRAGMENTS,
  "settings.metricsHint": METRICS_ENDPOINT_LTR_FRAGMENTS,
  "vm.ssh.publicKey": VM_SSH_KEY_PATH_LTR_FRAGMENTS,
  "cadence.cronInvalid": CRON_EXAMPLE_LTR_FRAGMENTS,
} as const satisfies Record<string, readonly string[]>;

/**
 * tLtr — read a translation and isolate its registered technical fragments in
 * one step: `tip={tLtr(t, "flash.backupHint")}`.
 *
 * It exists so a call site cannot get this half right. The isolation only
 * works if the render site remembers to apply it, and "remember to wrap this
 * one" is precisely what left fourteen strings unprotected while the mechanism
 * to protect them already existed. A key with no registered fragments passes
 * straight through, so this is safe to use for any hint and there is no
 * decision for a caller to get wrong.
 *
 * It returns a plain string (withLtrIsolates' Unicode-isolate form), which is
 * what the tip/hint/label/placeholder props it feeds require — those render as
 * text nodes and double as accessible names, so a `<span dir="ltr">` has
 * nowhere to live. withLtrFragments' span form stays for the three call sites
 * that render into real JSX and want the mono styling with it.
 */
export function tLtr(t: (key: TranslationKey) => string, key: TranslationKey): string {
  const fragments = (LTR_FRAGMENTS_BY_KEY as Record<string, readonly string[]>)[key];
  const text = t(key);
  return fragments ? withLtrIsolates(text, fragments) : text;
}
