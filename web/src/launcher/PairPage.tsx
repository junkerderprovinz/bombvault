import { useCallback, useEffect, useId, useState, type CSSProperties } from "react";
import { Badge } from "../components/Badge";
import { Button } from "../components/Button";
import { IconBack, IconLink, IconSearch } from "../components/glyphs";
import { InfoBubble } from "../components/InfoBubble";
import { IconAdd, IconCopy } from "../components/navGlyphs";
import { hueVars } from "../lib/appearance";
import type { useT } from "../lib/i18n";
import { PHRASE_WORDS, checkPhrase } from "../lib/phraseWords";
import { useConfirm } from "../lib/useConfirm";
import { WordSlots } from "../pages/settings/pairing/WordSlots";
import { displayAddress, type FoundServer, type LauncherState, type PairError } from "./bridge";
import { QRScanner } from "./QRScanner";

type T = ReturnType<typeof useT>["t"];

/** How long Pair stays busy after the words went out. Members arrive one by
 *  one and nothing says the last has come, so this is a pause for the list to
 *  fill, not a timeout. */
const SETTLE_MS = 3000;

/** The words of a refused code, in the sentences the pairing card uses. */
function pairErrorText(t: T, e: PairError): string {
  switch (e.reason) {
    case "count":
      return t("pairing.errWordCount").replace("{count}", String(e.count));
    case "word":
      return t("pairing.errUnknownWord").replace("{position}", String(e.position)).replace("{word}", e.word);
    case "checksum":
      return t("pairing.errChecksum");
  }
}

/**
 * PairPage joins the app to a BombVault group: twelve words, pasted, typed or
 * scanned, and every instance of the group turns up to be taken over at once.
 * Below it, a server outside a group is added from the network or by address.
 */
export function PairPage({
  t,
  group,
  pairError,
  found,
  camera,
  onJoin,
  onAdopt,
  onLeave,
  onPaste,
  onAskCamera,
  onPickFound,
  onByAddress,
  onBack,
}: {
  t: T;
  group: LauncherState["group"];
  pairError?: PairError;
  /** Servers announced on the network and not in the list yet. */
  found: FoundServer[];
  camera: boolean;
  onJoin: (words: string) => void;
  onAdopt: () => void;
  onLeave: () => void;
  onPaste: () => Promise<string>;
  onAskCamera: () => void;
  onPickFound: (f: FoundServer) => void;
  onByAddress: () => void;
  onBack: () => void;
}) {
  const fieldId = useId();
  const [text, setText] = useState("");
  const [searching, setSearching] = useState(false);
  const [scanning, setScanning] = useState(false);
  const [shake, setShake] = useState(0);
  const { confirm, confirmDialog } = useConfirm();
  const { words, unknown, complete } = checkPhrase(text);

  // A refusal from the app shakes Pair, the house's sign for "not like this".
  useEffect(() => {
    if (pairError) setShake((n) => n + 1);
  }, [pairError]);

  let problem = pairError ? pairErrorText(t, pairError) : null;
  if (!problem && unknown.length > 0) {
    problem = t("pairing.errUnknownWord").replace("{position}", String(unknown[0] + 1)).replace("{word}", words[unknown[0]]);
  }
  if (!problem && words.length > PHRASE_WORDS) {
    problem = t("pairing.errWordCount").replace("{count}", String(words.length));
  }

  const join = useCallback(
    (entered: string) => {
      onJoin(checkPhrase(entered).words.join(" "));
      setSearching(true);
      window.setTimeout(() => setSearching(false), SETTLE_MS);
    },
    [onJoin]
  );

  // A scanned code joins at once: it carries the words Pair would be pressed with.
  const scanned = useCallback(
    (code: string) => {
      setScanning(false);
      const [phrase] = code.trim().split("\n");
      setText(phrase);
      onJoin(code.trim());
      setSearching(true);
      window.setTimeout(() => setSearching(false), SETTLE_MS);
    },
    [onJoin]
  );
  const closeScanner = useCallback(() => setScanning(false), []);

  const joining = group.joining;

  return (
    <main className="mx-auto flex min-h-dvh w-full max-w-xl flex-col gap-4 px-4 py-5">
      <header className="flex items-center gap-3">
        <Badge as="button" shape="square" size="icon" tone="neutral" tip={t("common.back")} onClick={onBack}>
          <IconBack />
        </Badge>
        <h1 className="min-w-0 truncate text-xl font-semibold text-carbon-text">{t("launcher.pairTitle")}</h1>
        <InfoBubble tip={t("launcher.pairHint")} />
      </header>

      {group.paired && !joining ? (
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-2 rounded-card bg-carbon-surface p-4">
            <span className="min-w-0 flex-1 truncate text-sm font-semibold text-carbon-text">{t("launcher.paired")}</span>
            <Badge tone={group.connected ? "ok" : "warn"}>{group.connected ? t("launcher.relayConnected") : t("launcher.relayOffline")}</Badge>
          </div>
          <Button
            label={t("pairing.leave")}
            labelKey="pairing.leave"
            tone="neutral"
            onClick={() =>
              void confirm(t("launcher.leaveConfirm"), { confirmKey: "pairing.leave" }).then((ok) => {
                if (ok) onLeave();
              })
            }
            className="glim-btn-key w-full"
          />
        </div>
      ) : (
        <>
          <div className="mt-2 flex items-baseline justify-between gap-3">
            <label htmlFor={fieldId} className="text-xs text-carbon-textMuted">
              {t("pairing.wordsLabel")}
            </label>
            <span className={`text-xs tabular-nums ${complete ? "text-statusOk" : "text-carbon-textMuted"}`} aria-live="polite">
              {t("pairing.wordCount").replace("{n}", String(words.length))}
            </span>
          </div>
          {/* Twelve words do not fit on one phone line, and a field that
              scrolls sideways hides the typo somebody is looking for. */}
          <textarea
            id={fieldId}
            value={text}
            onChange={(e) => setText(e.target.value)}
            spellCheck={false}
            autoComplete="off"
            autoCapitalize="none"
            autoCorrect="off"
            dir="ltr"
            placeholder={t("pairing.enterPlaceholder")}
            className="min-h-36 resize-none rounded-control bg-carbon-surface px-3.5 py-3 text-xl leading-7 text-carbon-text placeholder:text-base placeholder:text-carbon-textMuted glim-field-focus"
          />
          <WordSlots words={words} unknown={unknown} filled />
          {problem && (
            <p role="alert" className="text-sm text-statusFail">
              {problem}
            </p>
          )}

          {/* One button for each way forward, in the order the hands move:
              paste the words, pair with them, or scan them. */}
          <div className="mt-2 flex flex-col gap-3">
            <Button
              label={t("pairing.paste")}
              labelKey="pairing.paste"
              glyph={<IconCopy />}
              tone="accent"
              hueIndex={0}
              disabled={searching}
              onClick={() => void onPaste().then((clip) => clip.trim() && setText(clip.trim()))}
              className="glim-btn-key w-full"
            />
            <Button
              key={`pair-${shake}`}
              label={t("pairing.join")}
              labelKey="pairing.join"
              glyph={<IconLink />}
              tone="accent"
              hueIndex={1}
              busy={searching}
              disabled={!complete}
              onClick={() => join(text)}
              className={`glim-btn-key w-full ${shake ? "glim-shake" : ""}`}
            />
            <Button
              label={t("launcher.scan")}
              labelKey="launcher.scan"
              glyph={<IconSearch />}
              tone="accent"
              hueIndex={2}
              onClick={() => setScanning(true)}
              className="glim-btn-key w-full"
            />
          </div>

          {joining && (
            <section className="mt-3 flex flex-col gap-1.5">
              <h2 className="text-xs font-semibold uppercase tracking-wide text-carbon-textMuted">{t("launcher.groupFound")}</h2>
              {joining.members.map((m) => (
                <div key={m.id} className="rounded-card bg-carbon-surface p-3.5">
                  <p className="truncate text-sm font-semibold text-carbon-text">{m.name || m.id}</p>
                  <p className="truncate text-xs text-carbon-textMuted">{m.version}</p>
                </div>
              ))}
              {joining.members.length === 0 && !searching && (
                <div className="mt-2 flex flex-col items-center gap-2.5 rounded-card bg-carbon-surface p-6 text-center">
                  <span className="text-carbon-textMuted opacity-50 [&_svg]:h-7 [&_svg]:w-7">
                    <IconLink />
                  </span>
                  <p className="text-sm text-carbon-textMuted">{t("launcher.noInstances")}</p>
                </div>
              )}
              {joining.members.length > 0 && (
                <Button
                  label={
                    joining.members.length === 1
                      ? t("launcher.adoptOne")
                      : t("launcher.adoptAll").replace("{count}", String(joining.members.length))
                  }
                  labelKey="launcher.adoptAll"
                  glyph={<IconAdd />}
                  tone="accent"
                  hueIndex={3}
                  onClick={onAdopt}
                  className="glim-btn-key mt-1 w-full"
                />
              )}
            </section>
          )}
        </>
      )}

      <section className="mt-4 flex flex-col gap-1.5">
        <h2 className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-carbon-textMuted">
          {t("launcher.found")}
          <InfoBubble tip={t("launcher.foundHint")} />
        </h2>
        {found.map((f, i) => (
          <button
            key={f.url}
            type="button"
            onClick={() => onPickFound(f)}
            style={hueVars(i) as CSSProperties}
            className="flex min-w-0 items-center gap-3 rounded-card bg-carbon-surface p-3.5 text-start glim-hue glim-field-focus"
          >
            <span className="flex min-w-0 flex-1 flex-col gap-0.5">
              <span className="truncate text-sm font-semibold text-carbon-text">{f.name}</span>
              <span dir="ltr" className="truncate text-start font-mono text-xs text-carbon-textMuted">
                {`${displayAddress(f.url)} · ${f.version}`}
              </span>
            </span>
            <svg aria-hidden width="10" height="10" viewBox="0 0 12 12" fill="none" className="shrink-0 text-accentText">
              <path fill="currentColor" d="M4 1.3 8.5 6 4 10.7Z" />
            </svg>
          </button>
        ))}
        <p className="text-xs text-carbon-textMuted">{found.length === 0 ? t("launcher.searching") : t("launcher.foundLead")}</p>
        <Button label={t("launcher.add")} labelKey="launcher.add" tone="neutral" onClick={onByAddress} className="glim-btn-key w-full" />
      </section>

      {scanning && (
        <QRScanner t={t} hint={t("launcher.scanHint")} camera={camera} onAskCamera={onAskCamera} onScanned={scanned} onClose={closeScanner} />
      )}
      {confirmDialog}
    </main>
  );
}
