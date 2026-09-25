import type { ReactNode } from "react";
import type { TranslationKey } from "./i18n";

/**
 * withLtrFragments wraps each literal fragment of a translated sentence in its
 * own dir="ltr" span, for paths baked into the translation rather than passed
 * in as a value. A fragment starting with a letter keeps its direction inside
 * RTL prose, but "/" is a neutral bidi character, so an unisolated leading
 * slash moves to the far end ("/mnt/x" shows as "mnt/x/" in Arabic and
 * Hebrew). Only the fragment is pinned; the sentence keeps the page's
 * direction.
 *
 * Paths are not translated, so the fragments match in every locale. They are
 * matched in list order and wrapped text is not matched again, so list a full
 * path before a shorter one it contains.
 */
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
 * withLtrPlaceholder replaces `token` with `value` inside a dir="ltr" span, for
 * a path only known at render time, such as the folder a scan stopped in. A
 * plain `.replace("{path}", value)` would leave the value with the same
 * leading-slash bug. A missing token leaves the sentence unchanged.
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
 * withLtrIsolates is the plain-string form of withLtrFragments, for text that
 * cannot hold markup, such as an InfoBubble tip that is also the trigger's
 * aria-label. Each fragment is wrapped in U+2066 LEFT-TO-RIGHT ISOLATE and
 * U+2069 POP DIRECTIONAL ISOLATE, the characters HTML's dir attribute is
 * defined by. Both are zero-width and ignored by screen readers, so the
 * accessible name does not change. It shares the fragment lists, and their
 * guard in ltrFragments.test.ts, with withLtrFragments.
 */
export function withLtrIsolates(text: string, fragments: readonly string[]): string {
  // Same precedence as withLtrFragments: wrapped text is not matched again.
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
  return parts.map((p) => (p.wrapped ? isolateLtr(p.text) : p.text)).join("");
}

/**
 * isolateLtr keeps a value put into a sentence, such as "4.0 GB" or a date, in
 * one left-to-right piece. Digits and the spaces and commas between them are
 * weak or neutral in the bidi algorithm, so in Arabic or Hebrew prose "4.0 GB"
 * shows as "GB 4.0".
 */
export function isolateLtr(value: string): string {
  // Escapes, because a literal zero-width character is invisible in an editor
  // and easy to delete by accident.
  return `\u2066${value}\u2069`;
}

/** offsite.repoLocalHint's standalone `/mnt` and its full example path. The
 *  relative "remotes/nas/bombvault" starts with a letter and needs no
 *  wrapping. */
export const REPO_LOCAL_HINT_LTR_FRAGMENTS = ["/mnt/remotes/nas/bombvault", "/mnt"] as const;

/** excludes.hint's leading-`/` example path. `.git` and `{a,b}` elsewhere in
 *  the same sentence start with a non-`/` character and already render
 *  correctly untouched. */
export const EXCLUDES_HINT_LTR_FRAGMENTS = ["/config/Library/.../Cache"] as const;

/** recovery.foreignAppdataDestHint's leading-`/` example pool path. */
export const FOREIGN_APPDATA_DEST_HINT_LTR_FRAGMENTS = ["/mnt/zfs"] as const;

/** BombVault's own config volume, named in prose by four different cards. */
export const CONFIG_VOLUME_LTR_FRAGMENTS = ["/config"] as const;

/** The Unraid USB mount, named by the Flash tab and its schedule/settings rows. */
export const BOOT_VOLUME_LTR_FRAGMENTS = ["/boot"] as const;

/** notify.unraidPlatformMismatch names both the host path and the in-container
 *  one, so the longer `/host/boot` comes first. */
export const UNRAID_PLATFORM_MISMATCH_LTR_FRAGMENTS = ["/host/boot", "/boot"] as const;

/** excludes.placeholder's example path (the input's own placeholder text). */
export const EXCLUDES_PLACEHOLDER_LTR_FRAGMENTS = ["/config/Library/Application"] as const;

/** The Prometheus scrape endpoint, named by the metrics toggle and its hint. */
export const METRICS_ENDPOINT_LTR_FRAGMENTS = ["/metrics"] as const;

/** Apprise's endpoint. Only the `/notify/` prefix is pinned, since the `<key>`
 *  placeholder after it is translated in some locales (sl "ključ", sr "кључ")
 *  and the prefix holds the leading slash. */
export const APPRISE_ENDPOINT_LTR_FRAGMENTS = ["/notify/"] as const;

/** The authorized-keys path the VM SSH card tells the user to append to. */
export const VM_SSH_KEY_PATH_LTR_FRAGMENTS = ["/root/.ssh/authorized_keys"] as const;

/** The Unraid user share, which the ZFS pages name as the mapping that hides
 *  snapshots. */
export const USER_SHARE_LTR_FRAGMENTS = ["/mnt/user"] as const;

/** The mount root a dataset needs a mountpoint below. */
export const MOUNT_ROOT_LTR_FRAGMENTS = ["/mnt"] as const;

/** Both spellings, for the fix that contrasts them. The longer one comes
 *  first so the bare mount root is only wrapped where it stands alone. */
export const USER_SHARE_VS_MOUNT_ROOT_LTR_FRAGMENTS = ["/mnt/user", "/mnt"] as const;

/** zfs.excludesHint's example pattern, measured from the item's dataset. */
export const ZFS_EXCLUDE_EXAMPLE_LTR_FRAGMENTS = ["/plex/Library/Cache"] as const;

/** cadence.cronInvalid's worked example. Not a path, but the same bug: a run of
 *  digits, spaces, `*` and `/` is entirely weak/neutral bidi classes, so an RTL
 *  paragraph reorders the whole expression and the user is shown a cron line
 *  they cannot type back. */
export const CRON_EXAMPLE_LTR_FRAGMENTS = ["0 */6 * * *"] as const;

/**
 * LTR_FRAGMENTS_BY_KEY maps each translation key to its fragment list. The
 * match is a literal substring, so a translator retyping a path silently
 * unprotects that locale. ltrFragments.test.ts checks this map against the
 * real locale tables, requires every `*_LTR_FRAGMENTS` export to appear here,
 * and derives from en which keys need an entry.
 */
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
  "zfs.code.shfs-only": USER_SHARE_LTR_FRAGMENTS,
  "zfs.fix.shfs-only": USER_SHARE_VS_MOUNT_ROOT_LTR_FRAGMENTS,
  "zfs.fix.legacy-mount": MOUNT_ROOT_LTR_FRAGMENTS,
  "zfs.excludesHint": ZFS_EXCLUDE_EXAMPLE_LTR_FRAGMENTS,
} as const satisfies Record<string, readonly string[]>;

/**
 * tLtr translates `key` and isolates its registered fragments in one step, so
 * a call site cannot forget the isolation: `tip={tLtr(t, "flash.backupHint")}`.
 * A key without fragments passes through unchanged. It returns withLtrIsolates'
 * plain string, which tip, hint, label and placeholder props need.
 */
export function tLtr(t: (key: TranslationKey) => string, key: TranslationKey): string {
  const fragments = (LTR_FRAGMENTS_BY_KEY as Record<string, readonly string[]>)[key];
  const text = t(key);
  return fragments ? withLtrIsolates(text, fragments) : text;
}
