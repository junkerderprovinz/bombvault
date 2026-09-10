import { useEffect, useRef, useState, type CSSProperties } from "react";
import { createPortal } from "react-dom";
import { Badge } from "./Badge";
import { Button } from "./Button";
import { CoinMark } from "./donateMarks";
import { QRCode } from "./QRCode";
import { hueVars, rainbowAt } from "../lib/appearance";
import { copyText } from "../lib/clipboard";
import { useT } from "../lib/i18n";
import { useLabelMode } from "../lib/useLabelMode";
import { useRainbow } from "../lib/useRainbow";
import { useToast } from "../lib/toast";
import { CRYPTO_COINS, type CryptoCoin, type CryptoNetwork } from "../lib/donate";

// ---------------------------------------------------------------------------
// The crypto donation window ([3524], rebuilt as coin-first in [3554]).
//
// A house window (rule 15: a window is a window) rather than a link to somebody
// else's page. Everything a donor needs is here: pick a coin, pick a chain, get
// the address as text and as a QR code, copy it. It works with no internet,
// sends nothing anywhere, and shows no name — which is the whole reason this
// route exists beside the coffee button.
//
// THE ORDER IS UPSIDE DOWN ON PURPOSE (jdp, 2026-09-10): the code comes first
// and the picker sits under it. A dialog usually asks before it answers, and
// this one answers first, because the answer is what the window was opened
// for. The picker below changes that answer in place, so the thing somebody
// came here to scan never moves off the top of the window.
//
// THE COINS ARE THE TILES, THE CHAINS ARE UNDERNEATH. A donor thinks "I have
// USDT", not "I have Ethereum", so the first choice is the one they can
// actually answer. The second choice is the dangerous one, and it stays a
// real, separate choice: every chain offered here carries its own address, so
// a chain we cannot receive on is unofferable rather than merely discouraged.
// lib/donate.ts carries the near miss that made this the rule.
//
// The chain row is shown even for a coin that has only one, and that is not
// filler. It is the line that says WHICH network the address on screen belongs
// to, and hiding it for the single-chain coins would make the one fact that
// decides whether the money arrives appear and disappear depending on which
// tile is lit.
// ---------------------------------------------------------------------------

export function CryptoDonateDialog({ onClose }: { onClose: () => void }) {
  const { t } = useT();
  const { push } = useToast();
  const [coin, setCoin] = useState<CryptoCoin>(CRYPTO_COINS[0]!);
  const [network, setNetwork] = useState<CryptoNetwork>(CRYPTO_COINS[0]!.networks[0]!);
  const cardRef = useRef<HTMLDivElement>(null);

  // Both engines, joined the way every other control on the page is.
  //
  // The label mode decides what a tile SHOWS. `reactive` deliberately resolves
  // to the same thing as text-and-glyph HERE and nowhere else: reactive means
  // the words appear under the pointer, which is right for a strip of verbs
  // somebody already knows and wrong for a grid of eight coins somebody is
  // SEARCHING - it would turn "find USDT" into hovering every tile in turn. A
  // picker is the one surface where hiding the labels until asked defeats the
  // surface; the sibling app's provider picker made the same call for the same
  // reason.
  const mode = useLabelMode("buttons");
  const showMark = mode !== "text";
  const showTicker = mode !== "glyph";
  // Subscribed once for the whole window rather than once per tile: the
  // palette changes for every tile at once anyway.
  useRainbow();

  // Escape closes, and focus starts inside the window rather than wherever it
  // happened to be — the same contract every other window in this app keeps.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKey);
    cardRef.current?.focus();
    return () => document.removeEventListener("keydown", onKey);
  }, [onClose]);

  // Picking a coin always lands on a network of THAT coin. Keeping the previous
  // chain when it happens to also carry the new coin would be a convenience
  // with one bad case: the chain somebody last looked at staying selected under
  // a coin they never checked it against.
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
          <p className="text-sm text-carbon-textSub">{t("about.cryptoIntro")}</p>

          {/* The answer, first. */}
          <div className="flex flex-col items-center gap-3 rounded-card bg-carbon-surface2 p-4">
            <QRCode value={network.address} size={168} />
            {/* Whole, in one piece, in a mono face, and never shortened: an
                address is read back by eye before somebody sends to it, so an
                ellipsis in the middle turns the one string that has to be
                exact into a string nobody can check. */}
            <p dir="ltr" className="w-full break-all text-center font-mono text-xs text-carbon-text">
              {network.address}
            </p>
            {/* The chain, switched HERE, directly under the address it changes
                (jdp, 2026-09-10: "die netzwerke soll man unter der Adresse
                umschalten können"). A picker one box away from its own effect
                makes somebody look twice to see whether the address moved; a
                row of chips under it changes the string in front of their
                eyes. Shown even when a coin has only one chain, because this
                is also the line that SAYS which network the address belongs
                to, and that fact may not come and go with the tile. */}
            <div
              className="flex flex-wrap justify-center gap-2"
              role="listbox"
              aria-label={t("about.cryptoNetworks")}
            >
              {/* A chain name is DATA and has no symbol, so the label engine
                  has nothing to hide here and these stay words in every mode.
                  The colour engine still applies: each chain owns a position,
                  so the chosen one fills in its own hue. */}
              {coin.networks.map((n, i) => (
                <button
                  key={n.id}
                  type="button"
                  role="option"
                  aria-selected={n.id === network.id}
                  onClick={() => setNetwork(n)}
                  style={hueVars(rainbowAt(i)) as CSSProperties}
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
            {/* Warn-coloured, and it is not a warning: it is the line a donor
                would otherwise go hunting for. Exchanges train people to look
                for a destination tag or a memo, so the chain that wants
                neither has to say so where the address is. */}
            {network.noteKey && (
              <p className="text-center text-xs text-statusWarn">{t(network.noteKey)}</p>
            )}
            {/* The one accent surface in this box, so it takes the position of
                the coin it belongs to: in rainbow mode the copy button is the
                same colour as the tile the address came from. The close button
                below has no position, and that is not an omission - it is
                neutral-toned, and a palette colour on a control that paints no
                accent resolves to nothing. */}
            <Button
              label={t("common.copy")}
              labelKey="common.copy"
              tone="accent"
              hueIndex={CRYPTO_COINS.findIndex((c) => c.id === coin.id)}
              onClick={() => void copy()}
            />
          </div>

          {/* The picker, under the answer it changes. Each tile owns a palette
              position, so rainbow mode makes eight coins scannable by colour
              the way it makes any other list scannable.

              `.glim-hue` only, never `.glim-hue-icon`. That class paints a
              tile's glyph in its own position hue, and this is the one grid in
              the app whose glyphs may not be painted: they are brand marks
              carrying brand colours. It sat here while the marks were
              `currentColor` and did real work; since they took their own
              colours it has been inert, and an inert class on a control reads
              as a decision that is still in force. The rule it would have
              broken is already written out beside .glim-hue-icon in index.css
              ("die icons sollen nicht eingefärbt werden, nur die badges also
              der hintergrund"). */}
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
                style={hueVars(rainbowAt(i)) as CSSProperties}
                // Square (jdp, 2026-09-10: "Die kacheln der Kryptowährungen
                // sollen quadratisch sein"). `aspect-square` rather than a
                // fixed height, so the tile stays square in all three label
                // modes: mark alone, ticker alone, or both. Without it the row
                // changed height whenever the labelling engine changed what is
                // inside it, and a picker that reflows when you switch label
                // mode reads as a different grid.
                //
                // The hover follows KnightLoader's browser tiles, which
                // settled this already (BrowserTools.tsx: "The hover goes
                // light in the dark theme"): a surface one step up is not a
                // hover anybody notices on a dark ground, so the dark theme
                // goes to white and flips the ink. The coin marks keep their
                // own colours straight through it - only the surface and the
                // ticker change - which is the same rule that file states.
                // XRP is the one mark with no colour of its own, so index.css
                // moves --coin-xrp for this hover and for the selected tile.
                //
                // The flipped ink is `text-carbon-background`, not the literal
                // KnightLoader writes: on this theme that token IS #161616, so
                // the value is the same one and it now comes from the engine
                // rather than from a hex somebody has to keep in step. The
                // sibling's own lint rule would have caught the literal here,
                // and did.
                className={`glim-coin-tile glim-hue flex aspect-square flex-col items-center justify-center gap-2 rounded-control px-2 transition-colors ${
                  c.id === coin.id
                    ? "glim-active bg-accent text-accentContrast"
                    : "bg-carbon-surface2 text-carbon-textSub hover:bg-carbon-surface3 hover:text-carbon-text dark:hover:bg-white dark:hover:text-carbon-background"
                }`}
              >
                {/* Half the tile, which is the sibling's proportion rather than
                    a number chosen here: its browser tiles are 112px with a
                    56px mark, and these came out 110px wide. The mark was 22px
                    while the tile sized itself to its contents; once the tile
                    became a square it was a fifth of it, and a logo floating in
                    an empty square is not the grid jdp pointed at. */}
                {showMark && <CoinMark coin={c.id} size={44} />}
                {showTicker && <span className="text-xs font-medium">{c.symbol}</span>}
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
