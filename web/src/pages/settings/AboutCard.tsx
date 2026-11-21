// What GlimStone's About card says: what this is, the ways to give and where
// to report. The Info page spreads the parts over its Help and About cards;
// the Android app shows them in one.
import { useState } from "react";
import { CoffeeDialog } from "../../components/CoffeeDialog";
import { CryptoDonateDialog } from "../../components/CryptoDonateDialog";
import { IconBitcoin, IconPayPal } from "../../components/donateMarks";
import { IconDiscord, IconGithub } from "../../components/glyphs";
import { PaypalDialog } from "../../components/PaypalDialog";
import { BrandMark, ReadmeButton } from "../../components/ReadmeButton";
import { COFFEE_BUTTON_SVG } from "../../lib/appMarks";
import { useT } from "../../lib/i18n";

export const REPO = "https://github.com/junkerderprovinz/bombvault";
/** The BombVault forum on the workshop's Discord server. */
const DISCORD = "https://discord.gg/RSdmzWtN25";

/** A version as the card shows it: `v4.2.8` reads "4.2.8". The "v" names the tag, not the version. */
export function shownVersion(version: string): string {
  const trimmed = version.trim();
  return /^v\d/.test(trimmed) ? trimmed.slice(1) : trimmed;
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

/** ReportButtons are the routes the report sentence names. */
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
      <ReadmeButton
        tile="glim-tile-discord"
        parts={[{ name: t("about.discord"), onClick: () => window.open(DISCORD, "_blank", "noopener,noreferrer") }]}
        mark={<IconDiscord />}
        markClass="glim-discord-mark"
      />
    </>
  );
}

/**
 * AboutContent is the whole About card for a page that draws its own card
 * around it. The Android app passes its privacy policy and hands PayPal to
 * the browser.
 */
export function AboutContent({ app }: { app?: { paypal: () => void; privacy: { label: string; href: string } } }) {
  const { t } = useT();
  return (
    <>
      <AboutGive paypal={app?.paypal} />
      <p className="text-sm text-carbon-textSub">{t("about.report")}</p>
      <div className="glim-readme-btn-rows">
        <ReportButtons />
      </div>
      {app && (
        <a href={app.privacy.href} target="_blank" rel="noopener noreferrer" className="self-start text-xs text-accentText no-underline">
          {app.privacy.label}
        </a>
      )}
    </>
  );
}
