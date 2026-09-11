/**
 * The version of the GlimStone reference files this app carries.
 *
 * A COPY of `reference/react/version.ts` from the design language's own repo,
 * and that is the whole point of it existing (GlimStone 1.8.0). The number used
 * to be a constant beside the About card, which made it and the files it
 * describes two things somebody had to change together by hand, and did not:
 * this card claimed 1.7.5 while index.css and lib/appearance.ts had moved on,
 * and 1.7.5 was never cut as a release at all, so the one screen that exists to
 * say what you are looking at linked to a 404 (measured, not assumed).
 *
 * It lives beside the other copied files (appearance.ts, controls.ts,
 * useLabelMode.ts, useTipBubble.tsx) rather than in the About card, so a
 * re-copy carries the number with it instead of leaving it behind.
 *
 * THE NUMBER MUST NAME A PUBLISHED RELEASE, not a changelog heading: the card
 * turns it into a link to that tag's release page. `gh release list` in the
 * glimstone repo is the check, and it is the check that found the 404 above.
 */
export const GLIMSTONE_VERSION = "1.8.3";
