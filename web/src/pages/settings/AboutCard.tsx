// The About card ([363]) — GlimStone's "The About card (replaces the version
// footer)", adopted here.
//
// It REPLACES the version footer rather than joining it. That is the failure
// mode the spec names explicitly, and it is easy to walk into because both are
// individually defensible: the result is one number in two type sizes twelve
// pixels apart. The footer is gone in the same commit that adds this.
//
// jdp asked for it in the System tab specifically, which is a departure from
// the spec's "end of Settings" — and the right one for a tabbed Settings page.
// "The end of Settings" assumes a single scrolling page; here it would mean
// either repeating the card on all seven tabs or picking one, and System is
// where a version number belongs among the host integration and the export.
//
// What the spec asks for, and what each part is doing:
//
//   - Both versions, app first, read from the build. Never typed into the
//     card: a number written down twice disagrees with itself the day one of
//     them is bumped.
//   - Every version links to ITS OWN tag's release page, not to the releases
//     index. A version string answers "which build is this"; the question
//     straight after is "and what changed", and a number nobody can follow
//     makes somebody search a repository for a tag they then retype.
//   - One sentence inviting a report, naming the route it actually has.
//   - One button: the repository. There was a second, a mailto, and it is gone
//     with the mail clause of that sentence — see [501] at the call site.
//
// This is also the one card in the language whose body is prose rather than an
// info bubble, and that is deliberate: a bubble hangs an explanation off a
// control, and this card has no control to explain.
import { useEffect, useState } from "react";
import { Button } from "../../components/Button";
import { IconGithub } from "../../components/glyphs";
import { getHealth } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { Card } from "./shared";

/**
 * The GlimStone release this interface is built against.
 *
 * Bumped by hand when index.css / lib/appearance.ts / lib/controls.ts are
 * re-copied from a newer release; check that repo's CHANGELOG before moving
 * it. Raised 1.2.0 -> 1.6.0 in [362] after an audit found it four releases
 * behind while the engines themselves had been kept current - a number that
 * had come to say the opposite of what it is for. 1.7.0 in [411], the release
 * this app's own number field was built from.
 *
 * BACK to 1.6.0 in [552], and the reason is the rule this card is built on.
 * -------------------------------------------------------------------------
 * Every version here links to ITS OWN tag's release page. 1.7.0 through 1.7.3
 * were four same-day versions in GlimStone's changelog that were never cut as
 * releases; GlimStone has since folded all four into the 1.6.0 that actually
 * ships (its own commit puts it as "The version on screen must be a published
 * release, not a tag"). So this card was linking to
 * /releases/tag/v1.7.2 - measured, HTTP 404, while v1.6.0 answers 200.
 *
 * The failure is worth naming because nothing local could catch it: the string
 * was accurate about a CHANGELOG heading and wrong about the world, and a
 * version number is only as good as the page it opens. Read the RELEASE list
 * when moving this, not the changelog - `gh release list` is the check.
 *
 * 1.7.5 now, and the check above was run before writing it: all six tags from
 * v1.7.0 to v1.7.5 answer 200 on their own release page, so what was a
 * changelog heading in [552] is a published release today. The engines moved
 * with the number rather than after it, which is the whole point of this
 * string: the second button height, the transition duration the motion switch
 * reaches, the wheel on the number field, the stepper wrapper that stopped
 * stretching, and the confirmation dialog's four changes are all in this
 * build. The class prefix moved too, in a sweep of its own right after: the
 * forty classes and fourteen keyframes this app had under `bv-` are `glim-`
 * now, the same names the language uses. What deliberately did NOT move are the
 * browser storage keys, which also begin with `bv-` and are not classes at all
 * (`bv-theme`, `bv-lang`, `bv-accent`, the filters); renaming those would have
 * silently reset every user's language, colour and filters, and broken the
 * display-prefs round trip with the server, which names the same keys in Go.
 */
export const GLIMSTONE_VERSION = "1.7.5";
const REPO = "https://github.com/junkerderprovinz/bombvault";
const GLIMSTONE_REPO = "https://github.com/junkerderprovinz/glimstone";
/** The handle from .github/FUNDING.yml, so one place in the product knows it. */
const COFFEE = "https://buymeacoffee.com/junkerderprovinz";
/** The workshop's own mailbox, shared by every tool in it: the subject carries
 *  the product name, so one inbox can tell them apart. */
const MAIL = "hello@halleluja.design";

/**
 * The tag behind a running version string.
 *
 * The build stamps `v8.3.1+feature-control-engine.59b73a6`; the tag is the part
 * before the build metadata, which is exactly what semver says that "+" means.
 * Built from the version rather than kept in a list, because a hand-maintained
 * list of links is wrong the first time somebody forgets it.
 */
export function releaseTag(version: string): string {
  const bare = version.split("+")[0].trim();
  if (!bare) return "";
  return bare.startsWith("v") ? bare : `v${bare}`;
}

/**
 * One `Label 1.2.3` pair, where only the NUMBER is the link.
 *
 * The label is plain text on purpose, the same way KnightLoader writes it: the
 * word is not the thing anybody wants to open, and underlining it as part of
 * the link makes the eye read "Version" as a destination.
 */
function VersionLink({ label, version, repo }: { label: string; version: string; repo: string }) {
  const tag = releaseTag(version);
  // No tag means a dev build ("dev", ""), and a link to a release page that
  // does not exist is worse than plain text.
  if (!tag) {
    return (
      <span className="text-carbon-textMuted">
        {label} <span className="font-mono tabular-nums">{version}</span>
      </span>
    );
  }
  return (
    // No underline ([500]). KnightLoader's card gives its version links no
    // decoration at all — `.versionLink` and `.aboutVersions` carry no CSS
    // rules there, only `.glim-num` for tabular figures on the line — and jdp
    // asked for this card to match that one. The affordance is carried by the
    // ink lifting on hover, which is enough for a number nobody is hunting for:
    // a dotted rule under a version string reads as an annotation, and there is
    // nothing here to annotate.
    <span className="text-carbon-textMuted">
      {label}{" "}
      <a
        href={`${repo}/releases/tag/${encodeURIComponent(tag)}`}
        target="_blank"
        rel="noopener noreferrer"
        className="font-mono tabular-nums text-carbon-textMuted no-underline hover:text-carbon-text"
      >
        {version}
      </a>
    </span>
  );
}

export function AboutCard({ hueIndex }: { hueIndex?: number }) {
  const { t } = useT();
  const [version, setVersion] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    getHealth()
      .then((h) => {
        if (active) setVersion(h.version ?? null);
      })
      .catch(() => {
        /* best-effort: the card is still worth showing without the number */
      });
    return () => {
      active = false;
    };
  }, []);


  return (
    <Card title={t("about.title")} hueIndex={hueIndex}>
      {/* The house About card, as jdp asked for it (2026-09-06: "kannst du in
          BV die Übercard so machen wie in KL? Die soll standard werden für alle
          meine Apps"). KnightLoader is the reference and GlimStone now carries
          the rule, so the ORDER below is the standard rather than this card's
          own arrangement: what this is, then the coffee with its button, then
          the way to report something with its buttons, then the versions as a
          footer.

          Each sentence sits directly above the thing it asks for. Three
          sentences stacked over one row of buttons reads as a form; a sentence
          with its own button under it reads as one offer. */}
      <p className="max-w-2xl text-sm text-carbon-textSub">{t("about.body")}</p>

      <p className="max-w-2xl text-sm text-carbon-textSub">{t("about.coffee")}</p>
      <div className="flex flex-wrap items-center gap-2">
        <Button
          label={t("about.coffeeButton")}
          labelKey="about.coffeeButton"
          tone="neutral"
          onClick={() => window.open(COFFEE, "_blank", "noopener,noreferrer")}
        />
      </div>

      {/* One extra step of space above this line, and only above this one
          (jdp, 2026-09-06). The card holds two offers, and without the break
          the coffee button sits as close to the next sentence as to the one it
          belongs to, so the eye pairs it with the wrong text. */}
      <p className="mt-2 max-w-2xl text-sm text-carbon-textSub">{t("about.report")}</p>

      {/* Two buttons again, and the sentence names both routes again.
          The mail button had been removed ([501]) for one reason: it pointed at
          KnightLoader's address, a different product's inbox, and a contact
          route that reaches the wrong place is worse than none, because
          somebody writes and then waits. That reason is gone. hello@ on the
          workshop's own domain exists as of 2026-09-06 and is a real mailbox,
          not a forward, so the card may offer it. The rule it was obeying all
          along still stands unchanged: never name a route no control here can
          reach. */}
      <div className="flex flex-wrap items-center gap-2">
        {/* The one place in this app that names GitHub, so the one place that
            wears its mark (jdp, 2026-09-08: "der github button soll das github
            logo als glyph haben"). Passed explicitly rather than through
            glyphFor on purpose: a pattern on "repo" would put this logo on
            repository settings that have nothing to do with GitHub. The rule
            is written out at gen_glyphs.py's IconGithub entry. */}
        <Button
          label={t("about.repo")}
          labelKey="about.repo"
          glyph={<IconGithub />}
          tone="neutral"
          onClick={() => window.open(REPO, "_blank", "noopener,noreferrer")}
        />
        {/* Subject only, never a body: a prefilled body reads as a form to fill
            in, and this is meant to be a message somebody writes. The product
            name rides in the subject so a mail arrives already saying which of
            the workshop's tools it is about — one inbox serves them all. */}
        <Button
          label={t("about.mail")}
          labelKey="about.mail"
          tone="neutral"
          onClick={() =>
            window.open(
              `mailto:${MAIL}?subject=${encodeURIComponent(`BombVault ${t("about.mailSubject")}`)}`,
              "_blank",
              "noopener,noreferrer"
            )
          }
        />
      </div>

      {/* Last line in the card, under the buttons. It reads as a footer, which
          is what it is: the sentences above are what the card wants to say and
          each has its own button, while a build number is what somebody looks
          up afterwards. One line with a middle dot, not two rows: two rows read
          as two facts of equal weight that happen to sit together, and this is
          one fact about one build. */}
      <p className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs">
        {version && <VersionLink label={t("about.version")} version={version} repo={REPO} />}
        {version && <span aria-hidden="true" className="text-carbon-textMuted">·</span>}
        <VersionLink label="GlimStone" version={GLIMSTONE_VERSION} repo={GLIMSTONE_REPO} />
      </p>
    </Card>
  );
}
