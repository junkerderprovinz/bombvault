import { useEffect, useRef, useState, type CSSProperties } from "react";
import { createPortal } from "react-dom";
import { Badge } from "./Badge";
import { Button } from "./Button";
import { CoinMark } from "./donateMarks";
import { QRCode } from "./QRCode";
import { hueVars } from "../lib/appearance";
import { copyText } from "../lib/clipboard";
import { useT } from "../lib/i18n";
import { useLabelMode } from "../lib/useLabelMode";
import { useToast } from "../lib/toast";
import { CRYPTO_COINS, type CryptoCoin, type CryptoNetwork } from "../lib/donate";

// The crypto donation window: pick a coin, then its chain, and get the address
// as text and as a QR code. It works offline, sends nothing and shows no name.
//
// The code comes first and the picker sits under it, so what the donor came to
// scan stays at the top while the picker changes it in place. Coins are the
// tiles because a donor knows "I have USDT" before they know the chain. The
// chain stays a separate choice, and only chains with their own address are
// offered (see lib/donate.ts).

export function CryptoDonateDialog({ onClose }: { onClose: () => void }) {
  const { t } = useT();
  const { push } = useToast();
  const [coin, setCoin] = useState<CryptoCoin>(CRYPTO_COINS[0]!);
  const [network, setNetwork] = useState<CryptoNetwork>(CRYPTO_COINS[0]!.networks[0]!);
  const cardRef = useRef<HTMLDivElement>(null);

  // The label mode decides what a tile shows. In reactive mode the ticker is
  // collapsed at rest like any other label and the selected tile keeps it; the
  // coin marks stay, and a donor recognises those faster than a ticker anyway.
  const mode = useLabelMode("buttons");
  const reactive = mode === "reactive";
  const showMark = mode !== "text";
  const showTicker = mode !== "glyph";

  // Escape closes, and focus starts inside the window.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKey);
    cardRef.current?.focus();
    return () => document.removeEventListener("keydown", onKey);
  }, [onClose]);

  // A new coin always starts on its own first network, so a chain chosen for
  // another coin never stays selected unchecked.
  function pickCoin(next: CryptoCoin) {
    setCoin(next);
    setNetwork(next.networks[0]!);
  }

  async function copy() {
    const ok = await copyText(network.address);
    push(
      ok ? `${coin.symbol}: ${t("common.copied")}` : t("vm.ssh.copyFailed"),
      ok ? "success" : "fail"
    );
  }

  return createPortal(
    <div
      className="glim-modal-backdrop fixed inset-0 z-50 flex items-center justify-center p-4"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        ref={cardRef}
        tabIndex={-1}
        role="dialog"
        aria-modal="true"
        aria-labelledby="cryptodonate-title"
        className="glim-modal-card relative flex max-h-[85vh] w-full max-w-lg flex-col rounded-card bg-carbon-surface shadow-2xl"
      >
        <div className="flex items-start justify-between gap-4 px-5 py-4">
          <h2 id="cryptodonate-title" className="flex items-center">
            <Badge tone="heading" size="heading" wrap>{t("about.cryptoTitle")}</Badge>
          </h2>
        </div>

        <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-5 pb-5">
          <p className="text-sm text-carbon-text">{t("about.donateAppeal")}</p>
          <p className="text-sm text-carbon-textSub">{t("about.cryptoIntro")}</p>

          <div className="flex flex-col items-center gap-3 rounded-card bg-carbon-surface2 p-4">
            {/* Black on white in both themes, because wallet scanners refuse an
                inverted code (see QRCode.tsx). The rounded corners only clip
                the quiet zone, which is white. */}
            <QRCode value={network.address} size={224} className="rounded-card" />
            {/* Never shortened: a donor checks the address by eye before
                sending to it. */}
            <p dir="ltr" className="w-full break-all text-center font-mono text-xs text-carbon-text">
              {network.address}
            </p>
            {/* The chain switches right under the address it changes. The row
                shows even for a single chain, because it says which network
                the address belongs to. */}
            <div
              className="flex flex-wrap justify-center gap-2"
              role="listbox"
              aria-label={t("about.cryptoNetworks")}
            >
              {/* Chain names are data without a glyph, so they stay words in
                  every mode. Each chain owns a rainbow position. */}
              {coin.networks.map((n, i) => (
                <button
                  key={n.id}
                  type="button"
                  role="option"
                  aria-selected={n.id === network.id}
                  onClick={() => setNetwork(n)}
                  style={hueVars(i) as CSSProperties}
                  className={`glim-hue rounded-pill px-3 py-1 text-xs font-medium transition-colors ${
                    n.id === network.id
                      ? "glim-active bg-accent text-accentContrast"
                      : "bg-carbon-surface3 text-carbon-textSub hover:bg-carbon-hoverRaised hover:text-carbon-text"
                  }`}
                >
                  {n.name}
                </button>
              ))}
            </div>
            {/* Exchanges teach people to look for a destination tag or memo, so
                a chain that needs neither says so next to the address. */}
            {network.noteKey && (
              <p className="text-center text-xs text-statusWarn">{t(network.noteKey)}</p>
            )}
            {/* Takes the coin's rainbow position, so it matches the selected
                tile. */}
            <Button
              label={t("common.copy")}
              labelKey="common.copy"
              tone="accent"
              hueIndex={CRYPTO_COINS.findIndex((c) => c.id === coin.id)}
              onClick={() => void copy()}
            />
          </div>

          {/* Each tile owns a rainbow position. `.glim-hue` only, not
              `.glim-hue-icon`, because the coin marks keep their brand
              colours. */}
          <div className="grid grid-cols-4 gap-2" role="listbox" aria-label={t("about.cryptoTitle")}>
            {CRYPTO_COINS.map((c, i) => (
              <button
                key={c.id}
                type="button"
                role="option"
                aria-selected={c.id === coin.id}
                aria-label={`${c.name} (${c.symbol})`}
                title={c.name}
                onClick={() => pickCoin(c)}
                style={{ ...hueVars(i), "--tile": c.tile.color, "--tile-ink": c.tile.ink } as CSSProperties}
                // `aspect-square` keeps the tile square in every label mode, so
                // the grid does not reflow when the mode changes. Under the
                // pointer a tile lights up in its coin's colour, without a
                // transition, since the mark would trail the tile.
                className={`glim-coin-tile glim-hue flex aspect-square flex-col items-center justify-center gap-2 rounded-control px-2 ${
                  reactive ? "glim-reactive " : ""
                }${
                  c.id === coin.id
                    ? "glim-active bg-accent text-accentContrast transition-colors"
                    : "glim-brand-tile bg-carbon-surface2 text-carbon-textSub"
                }`}
              >
                {/* About half the tile. */}
                {showMark && <CoinMark coin={c.id} size={44} />}
                {showTicker && (
                  // The shared reactive label: collapsed at rest, shown on
                  // hover, focus and `.glim-active`.
                  <span
                    className={`text-xs font-medium${reactive ? " glim-label-reactive" : ""}`}
                    style={reactive ? ({ "--reactive-chars": c.symbol.length } as CSSProperties) : undefined}
                  >
                    {c.symbol}
                  </span>
                )}
              </button>
            ))}
          </div>

        </div>

        <div className="flex justify-end gap-2 px-5 pb-5">
          <Button label={t("common.close")} labelKey="common.close" tone="neutral" onClick={onClose} />
        </div>
      </div>
    </div>,
    document.body
  );
}
