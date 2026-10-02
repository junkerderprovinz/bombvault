// GlimStone's About card, the last card on the General tab. It replaces the
// version footer instead of joining it, so the version appears once. Both
// versions come from the build and each links to its own tag's release page,
// since the next question after "which build" is "what changed".
import { useEffect, useState } from "react";
import { CoffeeDialog } from "../../components/CoffeeDialog";
import { CryptoDonateDialog } from "../../components/CryptoDonateDialog";
import { IconBitcoin, IconPayPal } from "../../components/donateMarks";
import { IconGithub } from "../../components/glyphs";
import { PaypalDialog } from "../../components/PaypalDialog";
import { BrandMark, ReadmeButton } from "../../components/ReadmeButton";
import { getHealth } from "../../lib/api";
import { COFFEE_BUTTON_SVG, MAIL_SVG } from "../../lib/appMarks";
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
    // affordance. Under a coarse pointer the padding widens the target and the
    // negative margin takes it back, so the caption line keeps its height.
    <span className="text-carbon-textMuted">
      {label}{" "}
      <a
        href={`${repo}/releases/tag/${encodeURIComponent(tag)}`}
        target="_blank"
        rel="noopener noreferrer"
        className="font-mono tabular-nums text-carbon-textMuted no-underline hover:text-carbon-text pointer-coarse:inline-block pointer-coarse:-m-2.5 pointer-coarse:p-2.5"
      >
        {version}
      </a>
    </span>
  );
}

export function AboutCard({ hueIndex }: { hueIndex?: number }) {
  const { t } = useT();
  return (
    <Card title={t("about.title")} hueIndex={hueIndex}>
      <AboutContent />
    </Card>
  );
}

/**
 * AboutContent is what the About card holds, for a page that draws its own
 * card around it. The Android app passes its own version and its privacy
 * policy, and hands PayPal to the browser, since PayPal's buttons need popups
 * a WebView does not open.
 */
export function AboutContent({ app }: { app?: { version: string; paypal: () => void; privacy: { label: string; href: string } } }) {
  const { t } = useT();
  const [version, setVersion] = useState<string | null>(app?.version ?? null);
  const [coffeeOpen, setCoffeeOpen] = useState(false);
  const [paypalOpen, setPaypalOpen] = useState(false);
  const [cryptoOpen, setCryptoOpen] = useState(false);

  useEffect(() => {
    if (app) return;
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
  }, [app]);

  return (
    <>
      {/* GlimStone fixes the order for every app: what this is, giving,
          reporting, then the versions as a footer. Each sentence sits directly
          above the buttons it asks for, which reads as one offer rather than
          a form. The prose has no reading-width cap because no other card on
          the page has one. */}
      <p className="text-sm text-carbon-textSub">{t("about.body")}</p>

      <p className="text-sm text-carbon-textSub">{t("about.coffee")}</p>
      {/* Every way to give in one row under its sentence, a blank line above
          and below: the hosted routes most people have an account for first,
          the wallet, which needs none, last. Brand marks are passed rather
          than resolved from the label key, since a rule on "coffee", "crypto"
          or "repo" would put a vendor's logo on unrelated settings. One line
          per button, as on the README, so nothing moves under the pointer. */}
      <div className="glim-readme-btn-rows glim-about-give">
        <ReadmeButton
          tile="glim-tile-coffee"
          parts={[{ name: t("about.coffeeButton"), onClick: () => setCoffeeOpen(true) }]}
          art={<BrandMark svg={COFFEE_BUTTON_SVG} />}
        />
        <ReadmeButton
          tile="glim-tile-paypal"
          parts={[{ name: t("about.paypal"), onClick: app ? app.paypal : () => setPaypalOpen(true) }]}
          mark={<IconPayPal />}
          markClass="glim-paypal-mark"
        />
        <ReadmeButton
          tile="glim-tile-bitcoin"
          parts={[{ name: t("about.crypto"), onClick: () => setCryptoOpen(true) }]}
          mark={<IconBitcoin />}
          markClass="glim-bitcoin-mark"
        />
      </div>
      {/* Mounted only while open, so nothing from BMAC or PayPal loads before
          somebody asks for it. */}
      {coffeeOpen && <CoffeeDialog onClose={() => setCoffeeOpen(false)} />}
      {paypalOpen && <PaypalDialog onClose={() => setPaypalOpen(false)} />}
      {cryptoOpen && <CryptoDonateDialog onClose={() => setCryptoOpen(false)} />}

      {/* The report sentence names exactly the routes that have a button here. */}
      <p className="text-sm text-carbon-textSub">{t("about.report")}</p>
      <div className="glim-readme-btn-rows">
        <ReadmeButton
          tile="glim-tile-github"
          parts={[{ name: t("about.repo"), onClick: () => window.open(REPO, "_blank", "noopener,noreferrer") }]}
          mark={<IconGithub />}
          markClass="glim-github-mark"
        />
        {/* Subject only: a prefilled body reads as a form to fill in. The
            product name in the subject lets one inbox serve every tool. This
            button reaches the app's own authors rather than a vendor, so it
            follows the user's accent and rainbow. */}
        <ReadmeButton
          tile="glim-tile-house"
          parts={[
            {
              name: t("about.mail"),
              onClick: () =>
                window.open(
                  `mailto:${MAIL}?subject=${encodeURIComponent(`BombVault ${t("about.mailSubject")}`)}`,
                  "_blank",
                  "noopener,noreferrer"
                ),
            },
          ]}
          mark={<BrandMark svg={MAIL_SVG} />}
          markClass="glim-house-mark"
        />
      </div>

      {/* One line with a middle dot: both numbers describe one build. */}
      <p className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs">
        {version && <VersionLink label={t("about.version")} version={version} repo={REPO} />}
        {version && <span aria-hidden="true" className="text-carbon-textMuted">·</span>}
        <VersionLink label="GlimStone" version={GLIMSTONE_VERSION} repo={GLIMSTONE_REPO} />
      </p>
      {app && (
        <a href={app.privacy.href} target="_blank" rel="noopener noreferrer" className="self-start text-xs text-accentText no-underline">
          {app.privacy.label}
        </a>
      )}
    </>
  );
}
