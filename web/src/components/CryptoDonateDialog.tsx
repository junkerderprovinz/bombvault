import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Badge } from "./Badge";
import { Button } from "./Button";
import { QRCode } from "./QRCode";
import { copyText } from "../lib/clipboard";
import { useT } from "../lib/i18n";
import { useToast } from "../lib/toast";
import { CRYPTO_CHAINS, type CryptoChain } from "../lib/donate";

// ---------------------------------------------------------------------------
// The crypto donation window (#3479).
//
// A house window (rule 15: a window is a window) rather than a link to somebody
// else's page. Everything a donor needs is here: pick a chain, get the address
// as text and as a QR code, copy it. It works with no internet, sends nothing
// anywhere, and shows no name — which is the whole reason this route exists
// beside the coffee button.
//
// The list is grouped BY CHAIN and never by coin. See lib/donate.ts for why
// that is a safety property and not a layout preference.
//
// The QR matters more than it looks: the address is the one thing here that
// must not be mistyped, and a phone wallet scans it in a second. Copy is for a
// desktop wallet, the QR is for a phone, and a donor uses whichever they have.
// ---------------------------------------------------------------------------

export function CryptoDonateDialog({ onClose }: { onClose: () => void }) {
  const { t } = useT();
  const { push } = useToast();
  const [picked, setPicked] = useState<CryptoChain>(CRYPTO_CHAINS[0]!);
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

  async function copy(chain: CryptoChain) {
    const ok = await copyText(chain.address);
    push(ok ? `${chain.name}: ${t("common.copied")}` : t("vm.ssh.copyFailed"), ok ? "success" : "fail");
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

          {/* The chains. A row is a button because it does something, and the
              picked one is FILLED with the accent, the way every other chosen
              thing in this house is marked. */}
          <div className="flex flex-col gap-1" role="listbox" aria-label={t("about.cryptoTitle")}>
            {CRYPTO_CHAINS.map((chain) => (
              <button
                key={chain.id}
                type="button"
                role="option"
                aria-selected={chain.id === picked.id}
                onClick={() => setPicked(chain)}
                className={`flex items-baseline gap-2 rounded-control px-3 py-2 text-start transition-colors ${
                  chain.id === picked.id
                    ? "bg-accent text-accentContrast"
                    : "text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
                }`}
              >
                <span className="font-medium">{chain.name}</span>
                <span className="text-xs opacity-80">{chain.coins}</span>
              </button>
            ))}
          </div>

          {/* The picked chain, in both forms a donor can use. */}
          <div className="flex flex-col items-center gap-3 rounded-card bg-carbon-surface2 p-4">
            <QRCode value={picked.address} size={168} />
            <p dir="ltr" className="w-full break-all text-center font-mono text-xs text-carbon-text">
              {picked.address}
            </p>
            {picked.networks && (
              <p className="text-center text-xs text-carbon-textMuted">
                {t("about.cryptoNetworks")}: {picked.networks}
              </p>
            )}
            {picked.noteKey && (
              <p className="text-center text-xs text-statusWarn">{t(picked.noteKey)}</p>
            )}
            <Button
              label={t("common.copy")}
              labelKey="common.copy"
              tone="accent"
              onClick={() => void copy(picked)}
            />
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
