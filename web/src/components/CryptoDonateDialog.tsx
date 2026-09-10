import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Badge } from "./Badge";
import { Button } from "./Button";
import { CoinMark } from "./donateMarks";
import { QRCode } from "./QRCode";
import { copyText } from "../lib/clipboard";
import { useT } from "../lib/i18n";
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
      className="glim-modal-backdrop fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
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
            <p className="text-center text-xs text-carbon-textMuted">
              {coin.symbol} · {network.name}
            </p>
            {/* Warn-coloured, and it is not a warning: it is the line a donor
                would otherwise go hunting for. Exchanges train people to look
                for a destination tag or a memo, so the chain that wants
                neither has to say so where the address is. */}
            {network.noteKey && (
              <p className="text-center text-xs text-statusWarn">{t(network.noteKey)}</p>
            )}
            <Button
              label={t("common.copy")}
              labelKey="common.copy"
              tone="accent"
              onClick={() => void copy()}
            />
          </div>

          {/* The picker, under the answer it changes. */}
          <div className="grid grid-cols-4 gap-2" role="listbox" aria-label={t("about.cryptoTitle")}>
            {CRYPTO_COINS.map((c) => (
              <button
                key={c.id}
                type="button"
                role="option"
                aria-selected={c.id === coin.id}
                aria-label={`${c.name} (${c.symbol})`}
                onClick={() => pickCoin(c)}
                className={`flex flex-col items-center gap-1 rounded-control px-2 py-3 transition-colors ${
                  c.id === coin.id
                    ? "bg-accent text-accentContrast"
                    : "bg-carbon-surface2 text-carbon-textSub hover:bg-carbon-surface3 hover:text-carbon-text"
                }`}
              >
                <CoinMark coin={c.id} size={22} />
                <span className="text-xs font-medium">{c.symbol}</span>
              </button>
            ))}
          </div>

          {/* The chain, always shown, because it decides where the money goes. */}
          <div className="flex flex-col gap-1.5">
            <span className="text-xs text-carbon-textMuted">{t("about.cryptoNetworks")}</span>
            <div
              className="flex flex-wrap gap-2"
              role="listbox"
              aria-label={t("about.cryptoNetworks")}
            >
              {coin.networks.map((n) => (
                <button
                  key={n.id}
                  type="button"
                  role="option"
                  aria-selected={n.id === network.id}
                  onClick={() => setNetwork(n)}
                  className={`rounded-pill px-3 py-1 text-xs font-medium transition-colors ${
                    n.id === network.id
                      ? "bg-accent text-accentContrast"
                      : "bg-carbon-surface2 text-carbon-textSub hover:bg-carbon-surface3 hover:text-carbon-text"
                  }`}
                >
                  {n.name}
                </button>
              ))}
            </div>
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
