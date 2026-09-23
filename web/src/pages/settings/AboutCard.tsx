// GlimStone's About card, the last card on the General tab. It replaces the
// version footer instead of joining it, so the version appears once. Both
// versions come from the build and each links to its own tag's release page,
// since the next question after "which build" is "what changed".
import { useEffect, useState } from "react";
import { Button } from "../../components/Button";
import { CryptoDonateDialog } from "../../components/CryptoDonateDialog";
import { IconBitcoin, IconBuyMeACoffee, IconPayPal } from "../../components/donateMarks";
import { IconGithub, IconMail } from "../../components/glyphs";
import { getHealth } from "../../lib/api";
import { GLIMSTONE_VERSION } from "../../lib/glimstoneVersion";
import { useT } from "../../lib/i18n";
import { Card } from "./shared";

/**
 * Lives in lib/glimstoneVersion.ts, a copy of GlimStone's own version file, so
 * it is updated together with the other copied GlimStone files. It is rendered
 * as a link to its release page, so check `gh release list` before changing
 * it: a version that was never released links to a 404.
 */
export { GLIMSTONE_VERSION };
const REPO = "https://github.com/junkerderprovinz/bombvault";
const GLIMSTONE_REPO = "https://github.com/junkerderprovinz/glimstone";
/** Same handle as .github/FUNDING.yml. */
const COFFEE = "https://buymeacoffee.com/junkerderprovinz";
/**
 * A hosted donate button, because PayPal refuses the open
 * `?business=<id>&item_name=<project>` address for this account
 * (`ppccNotConfirmed`). The same button serves every project; the donor picks
 * the project on PayPal's page and the link cannot preselect it.
 *
 * Declared as `string` so the empty check at the button still compiles; with
 * the literal type TypeScript rejects it as a comparison with no overlap.
 */
const PAYPAL: string =
  "https://www.paypal.com/donate/?hosted_button_id=76FVV52TKXTUS";
/** Shared by every tool of the workshop; the mail subject names the product. */
const MAIL = "hello@halleluja.design";

/**
 * releaseTag returns the git tag for a running version such as
 * `v8.3.1+feature-control-engine.59b73a6`: the part before the semver build
 * metadata, with a leading "v".
 */
export function releaseTag(version: string): string {
  const bare = version.split("+")[0].trim();
  if (!bare) return "";
  return bare.startsWith("v") ? bare : `v${bare}`;
}

/** A `Label 1.2.3` pair where only the number links to the release page. */
function VersionLink({ label, version, repo }: { label: string; version: string; repo: string }) {
  const tag = releaseTag(version);
  // A dev build has no tag and so no release page.
  if (!tag) {
    return (
      <span className="text-carbon-textMuted">
        {label} <span className="font-mono tabular-nums">{version}</span>
      </span>
    );
  }
  return (
    // No underline, as on KnightLoader's About card; the hover colour is the
    // affordance.
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

/**
 * brand returns the classes that give a button a coloured brand mark and the
 * brand colour on hover. The colours live in index.css, which keeps hex out of
 * this file and spares Button a style prop it already computes itself.
 */
function brand(name: string): string {
  return `glim-brand-btn glim-brand-${name}`;
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
      {/* GlimStone fixes the order for every app: what this is, giving,
          reporting, then the versions as a footer. Each sentence sits directly
          above the buttons it asks for, which reads as one offer rather than
          a form. The prose has no reading-width cap because no other card on
          the page has one. */}
      <p className="text-sm text-carbon-textSub">{t("about.body")}</p>

      <p className="text-sm text-carbon-textSub">{t("about.coffee")}</p>
      <div className="flex flex-wrap items-center gap-2">
        {/* Brand marks are passed explicitly rather than resolved from the
            label key: a pattern on "coffee", "crypto" or "repo" would put a
            vendor's logo on unrelated settings. */}
        <Button
          label={t("about.coffeeButton")}
          labelKey="about.coffeeButton"
          glyph={<IconBuyMeACoffee />}
          tone="neutral"
          className={brand("coffee")}
          onClick={() => window.open(COFFEE, "_blank", "noopener,noreferrer")}
        />
        {/* Hosted pages first, the wallet last, as GlimStone orders them: the
            routes most people have an account for, then the one that needs
            none. */}
        {PAYPAL !== "" && (
          <Button
            label={t("about.paypal")}
            labelKey="about.paypal"
            glyph={<IconPayPal />}
            tone="neutral"
            className={brand("paypal")}
            onClick={() => window.open(PAYPAL, "_blank", "noopener,noreferrer")}
          />
        )}
        <Button
          label={t("about.crypto")}
          labelKey="about.crypto"
          glyph={<IconBitcoin />}
          tone="neutral"
          className={brand("bitcoin")}
          onClick={() => setCryptoOpen(true)}
        />
      </div>
      {cryptoOpen && <CryptoDonateDialog onClose={() => setCryptoOpen(false)} />}

      {/* The extra space keeps the buttons above visually attached to their
          own sentence rather than to this one. */}
      <p className="mt-2 text-sm text-carbon-textSub">{t("about.report")}</p>

      {/* The report sentence names exactly the routes that have a button here. */}
      <div className="flex flex-wrap items-center gap-2">
        <Button
          label={t("about.repo")}
          labelKey="about.repo"
          glyph={<IconGithub />}
          tone="neutral"
          className={brand("github")}
          onClick={() => window.open(REPO, "_blank", "noopener,noreferrer")}
        />
        {/* Subject only: a prefilled body reads as a form to fill in. The
            product name in the subject lets one inbox serve every tool. This is
            the one button without a vendor behind it, so it wears the app's
            accent instead of a brand colour. */}
        <Button
          label={t("about.mail")}
          labelKey="about.mail"
          glyph={<IconMail />}
          tone="neutral"
          className={brand("house")}
          onClick={() =>
            window.open(
              `mailto:${MAIL}?subject=${encodeURIComponent(`BombVault ${t("about.mailSubject")}`)}`,
              "_blank",
              "noopener,noreferrer"
            )
          }
        />
      </div>

      {/* One line with a middle dot: both numbers describe one build. */}
      <p className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs">
        {version && <VersionLink label={t("about.version")} version={version} repo={REPO} />}
        {version && <span aria-hidden="true" className="text-carbon-textMuted">·</span>}
        <VersionLink label="GlimStone" version={GLIMSTONE_VERSION} repo={GLIMSTONE_REPO} />
      </p>
    </Card>
  );
}
