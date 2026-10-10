// What GlimStone's About card says: what this is, the ways to give, where to
// report, and the versions. The Info page spreads the parts over three cards;
// the Android app shows them in one.
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

export { GLIMSTONE_VERSION };
export const REPO = "https://github.com/junkerderprovinz/bombvault";
export const GLIMSTONE_REPO = "https://github.com/junkerderprovinz/glimstone";
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

/** The address of a version's release page, "" for a build without a tag. */
export function releaseUrl(repo: string, version: string): string {
  const tag = releaseTag(version);
  return tag ? `${repo}/releases/tag/${encodeURIComponent(tag)}` : "";
}

/** A `Label 1.2.3` pair where only the number links to the release page. */
function VersionLink({ label, version, repo }: { label: string; version: string; repo: string }) {
  const href = releaseUrl(repo, version);
  if (!href) {
    return (
      <span className="text-carbon-textMuted">
        {label} <span className="font-mono tabular-nums">{version}</span>
      </span>
    );
  }
  return (
    // The hover colour is the affordance. Under a coarse pointer the padding
    // widens the target and the negative margin takes it back, so the line
    // keeps its height.
    <span className="text-carbon-textMuted">
      {label}{" "}
      <a
        href={href}
        target="_blank"
        rel="noopener noreferrer"
        className="font-mono tabular-nums text-carbon-textMuted no-underline hover:text-carbon-text pointer-coarse:inline-block pointer-coarse:-m-2.5 pointer-coarse:p-2.5"
      >
        {version}
      </a>
    </span>
  );
}

/**
 * AboutGive says what BombVault is and offers the ways to give, each sentence
 * directly above the buttons it asks for. `paypal` replaces the PayPal window,
 * for a WebView that opens no popups.
 */
export function AboutGive({ paypal }: { paypal?: () => void }) {
  const { t } = useT();
  const [coffeeOpen, setCoffeeOpen] = useState(false);
  const [paypalOpen, setPaypalOpen] = useState(false);
  const [cryptoOpen, setCryptoOpen] = useState(false);

  return (
    <>
      <p className="text-sm text-carbon-textSub">{t("about.body")}</p>
      <p className="text-sm text-carbon-textSub">{t("about.coffee")}</p>
      {/* The hosted routes most people have an account for come first, the
          wallet, which needs none, last. Brand marks are passed here and not
          resolved from the label, or a rule on "coffee" would put a vendor's
          logo on unrelated settings. */}
      <div className="glim-readme-btn-rows glim-about-give">
        <ReadmeButton
          tile="glim-tile-coffee"
          parts={[{ name: t("about.coffeeButton"), onClick: () => setCoffeeOpen(true) }]}
          art={<BrandMark svg={COFFEE_BUTTON_SVG} />}
        />
        <ReadmeButton
          tile="glim-tile-paypal"
          parts={[{ name: t("about.paypal"), onClick: paypal ?? (() => setPaypalOpen(true)) }]}
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
      {/* Mounted only while open, so nothing from a payment service loads
          before somebody asks for it. */}
      {coffeeOpen && <CoffeeDialog onClose={() => setCoffeeOpen(false)} />}
      {paypalOpen && <PaypalDialog onClose={() => setPaypalOpen(false)} />}
      {cryptoOpen && <CryptoDonateDialog onClose={() => setCryptoOpen(false)} />}
    </>
  );
}

/** ReportButtons are the two routes the report sentence names. */
export function ReportButtons() {
  const { t } = useT();
  return (
    <>
      <ReadmeButton
        tile="glim-tile-github"
        parts={[{ name: t("about.repo"), onClick: () => window.open(REPO, "_blank", "noopener,noreferrer") }]}
        mark={<IconGithub />}
        markClass="glim-github-mark"
      />
      {/* Subject only: a prefilled body reads as a form to fill in. The
          product name in the subject lets one inbox serve every tool. */}
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
    </>
  );
}

/**
 * AboutContent is the whole About card for a page that draws its own card
 * around it. The Android app passes its own version and its privacy policy,
 * and hands PayPal to the browser.
 */
export function AboutContent({ app }: { app?: { version: string; paypal: () => void; privacy: { label: string; href: string } } }) {
  const { t } = useT();
  const [version, setVersion] = useState<string | null>(app?.version ?? null);

  useEffect(() => {
    if (app) return;
    let active = true;
    getHealth()
      .then((h) => {
        if (active) setVersion(h.version ?? null);
      })
      .catch(() => undefined);
    return () => {
      active = false;
    };
  }, [app]);

  return (
    <>
      <AboutGive paypal={app?.paypal} />
      <p className="text-sm text-carbon-textSub">{t("about.report")}</p>
      <div className="glim-readme-btn-rows">
        <ReportButtons />
      </div>
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
