// The About card ([363]) — GlimStone's "The About card (replaces the version
// footer)", adopted here.
//
// It REPLACES the version footer rather than joining it. That is the failure
// mode the spec names explicitly, and it is easy to walk into because both are
// individually defensible: the result is one number in two type sizes twelve
// pixels apart. The footer is gone in the same commit that adds this.
//
// It stands at the END OF THE GENERAL TAB. The language says "the end of
// Settings", which assumes a single scrolling page; on a tabbed one that means
// either repeating the card on all seven tabs or picking one. It was System
// until [3559], on the reading that a version number belongs among the host
// integration and the export. That was defensible and wrong: System holds
// things somebody comes here to OPERATE, while General is the first tab in the
// strip, so this is the last card of the first thing anybody opens. The sibling
// apps already had it there, so the move also ends a three-way disagreement
// about one standard card.
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
import { CryptoDonateDialog } from "../../components/CryptoDonateDialog";
import { IconBitcoin, IconBuyMeACoffee, IconPayPal } from "../../components/donateMarks";
import { IconGithub } from "../../components/glyphs";
import { getHealth } from "../../lib/api";
import { GLIMSTONE_VERSION } from "../../lib/glimstoneVersion";
import { useT } from "../../lib/i18n";
import { Card } from "./shared";

/**
 * The GlimStone release this interface is built against, re-exported from the
 * copied file that owns it.
 *
 * The number USED TO LIVE HERE, and GlimStone 1.8.0 moved it out for a reason
 * this card proved twice over. A constant beside the card is a second place:
 * the engines were re-copied and the number was not, so the card said 1.7.5
 * while the files were newer, and 1.7.5 had never been cut as a release at all,
 * because the language folded 1.7.1 through 1.7.10 into 1.8.0. Every version on
 * this card is a link to its own tag's release page, so the string was not just
 * stale, it opened a 404 (measured: v1.7.5 answers 404, v1.8.0 answers 200).
 *
 * So it comes from lib/glimstoneVersion.ts now, which is a copy of the design
 * language's own version.ts and travels with the other copied files. Read the
 * RELEASE list before moving it, never the changelog: `gh release list` is the
 * check, and it is what caught this.
 */
export { GLIMSTONE_VERSION };
const REPO = "https://github.com/junkerderprovinz/bombvault";
const GLIMSTONE_REPO = "https://github.com/junkerderprovinz/glimstone";
/** The handle from .github/FUNDING.yml, so one place in the product knows it. */
const COFFEE = "https://buymeacoffee.com/junkerderprovinz";
/**
 * The PayPal.Me page, and it is EMPTY until that page exists.
 *
 * The card's own rule, applied to a route rather than to a sentence: never
 * offer a control that reaches nowhere. A PayPal.Me link is created once and
 * cannot be renamed afterwards without asking their support, so the name has
 * to be chosen deliberately rather than guessed at here. Fill this in and the
 * button appears; leave it empty and the card offers coffee and crypto alone.
 *
 * Typed as `string` rather than inferred, so the emptiness is a value this
 * file expects to change and not a constant the compiler folds away.
 */
const PAYPAL: string = "";
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
  const [cryptoOpen, setCryptoOpen] = useState(false);

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
      {/* No reading-width cap on the card's own prose. It carried max-w-2xl
          (42rem) until 1.8.1, which is a defensible typographic width in the
          abstract and looked wrong here for a concrete reason: it is the ONLY
          capped text on its page. Every other card lets its sentences run the
          card, so three paragraphs stopping two thirds of the way across read
          as hand-set line breaks rather than as a measure. Reported exactly
          that way ("in der übercard sind künstliche Zeilenumbrüche") and
          measured before believing it: 672px of text in a 1244px card.

          The rule that follows, and it is the reusable half: a reading width
          is a property of a PAGE, never of one card on it. Cap all the prose
          or none of it. */}
      <p className="text-sm text-carbon-textSub">{t("about.body")}</p>

      <p className="text-sm text-carbon-textSub">{t("about.coffee")}</p>
      {/* Two ways to give, and they are two because they reach different
          people. The coffee takes a card, Apple Pay or Google Pay; the crypto
          window takes what somebody already holds in a wallet and shows no
          name at either end. Both sit under the one sentence that asks, which
          is the card's own rule: a sentence directly above the thing it asks
          for. */}
      <div className="flex flex-wrap items-center gap-2">
        {/* Both marks are passed here rather than resolved from the label key,
            which is the house rule for a BRAND (see the GitHub button below).
            A pattern on "coffee" would put another company's cup on anything
            that mentions coffee, and one on "crypto" would put the Bitcoin
            symbol on settings that have nothing to do with it. jdp asked for
            both by name (2026-09-10). */}
        <Button
          label={t("about.coffeeButton")}
          labelKey="about.coffeeButton"
          glyph={<IconBuyMeACoffee />}
          tone="neutral"
          onClick={() => window.open(COFFEE, "_blank", "noopener,noreferrer")}
        />
        <Button
          label={t("about.crypto")}
          labelKey="about.crypto"
          glyph={<IconBitcoin />}
          tone="neutral"
          onClick={() => setCryptoOpen(true)}
        />
        {PAYPAL !== "" && (
          <Button
            label={t("about.paypal")}
            labelKey="about.paypal"
            glyph={<IconPayPal />}
            tone="neutral"
            onClick={() => window.open(PAYPAL, "_blank", "noopener,noreferrer")}
          />
        )}
      </div>
      {cryptoOpen && <CryptoDonateDialog onClose={() => setCryptoOpen(false)} />}

      {/* One extra step of space above this line, and only above this one
          (jdp, 2026-09-06). The card holds two offers, and without the break
          the coffee button sits as close to the next sentence as to the one it
          belongs to, so the eye pairs it with the wrong text. */}
      <p className="mt-2 text-sm text-carbon-textSub">{t("about.report")}</p>

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
