// usePhraseEntry takes the twelve words of another instance. It reads a paste
// with line breaks, list numbers and commas, fills twelve numbered slots as
// the words arrive and names an unknown word with its place before anything
// is sent. Pair stays off until twelve known words are there; whether they
// belong together only the server can say, from the checksum.
import { useState, type ReactNode } from "react";
import { Button } from "../../../components/Button";
import { IconLink } from "../../../components/glyphs";
import { IconCopy } from "../../../components/navGlyphs";
import { InfoBubble } from "../../../components/InfoBubble";
import type { useT } from "../../../lib/i18n";
import { PHRASE_WORDS, checkPhrase } from "../../../lib/phraseWords";
import { WordSlots } from "./WordSlots";

type T = ReturnType<typeof useT>["t"];

interface PhraseEntryOptions {
  id: string;
  label: string;
  tip: string;
  /** Where a title already says what goes here, the label is only read out,
   *  not shown. */
  bare?: boolean;
  busy: boolean;
  /** Sends the words, one space apart, and resolves to the refusal to show,
   *  or null once paired. */
  onPair: (phrase: string) => Promise<string | null>;
  t: T;
}

/**
 * usePhraseEntry is the word field and its Paste and Pair buttons apart, for
 * a window that puts the buttons in its footer.
 */
export function usePhraseEntry({ id, label, tip, bare = false, busy, onPair, t }: PhraseEntryOptions): {
  field: ReactNode;
  paste: ReactNode;
  pair: ReactNode;
} {
  const [text, setText] = useState("");
  const [refusal, setRefusal] = useState<string | null>(null);
  const [shake, setShake] = useState(0);
  const { words, unknown, complete } = checkPhrase(text);

  let problem = refusal;
  if (!problem && unknown.length > 0) {
    const i = unknown[0];
    problem = t("pairing.errUnknownWord")
      .replace("{position}", String(i + 1))
      .replace("{word}", words[i]);
  }
  if (!problem && words.length > PHRASE_WORDS) {
    problem = t("pairing.errWordCount").replace("{count}", String(words.length));
  }

  // The clipboard text goes through the same parser as a paste into the
  // field. A browser that will not hand it over (plain HTTP, or the
  // permission refused) leaves the field to paste into by hand.
  async function pasteClipboard() {
    try {
      const clip = await navigator.clipboard.readText();
      setText(clip);
      setRefusal(null);
    } catch {
      setRefusal(t("pairing.pasteRefused"));
    }
    document.getElementById(id)?.focus();
  }

  async function pair() {
    const answer = await onPair(words.join(" "));
    if (answer === null) {
      setText("");
      return;
    }
    setRefusal(answer);
    setShake((n) => n + 1);
  }

  const field = (
    <div className="flex flex-col gap-2" data-testid={id}>
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
        {!bare && (
          <label htmlFor={id} className="inline-flex items-center gap-1.5 text-[13px] font-medium text-carbon-text">
            {label}
            <InfoBubble tip={tip} />
          </label>
        )}
        <span className={`ms-auto text-xs tabular-nums ${complete ? "text-statusOk" : "text-carbon-textMuted"}`} aria-live="polite">
          {t("pairing.wordCount").replace("{n}", String(words.length))}
        </span>
      </div>
      <textarea
        id={id}
        value={text}
        onChange={(e) => {
          setText(e.target.value);
          setRefusal(null);
        }}
        rows={2}
        spellCheck={false}
        autoComplete="off"
        autoCapitalize="none"
        dir="ltr"
        aria-label={bare ? label : undefined}
        aria-invalid={problem ? true : undefined}
        placeholder={t("pairing.enterPlaceholder")}
        className="min-h-16 resize-y rounded-control bg-carbon-surface2 px-3 py-2.5 font-mono text-[13px] leading-normal text-carbon-text placeholder:font-sans placeholder:text-carbon-textMuted glim-field-focus"
      />
      <WordSlots words={words} unknown={unknown} />
      <p role="alert" className="text-[13px] text-statusFail empty:hidden">
        {problem ?? ""}
      </p>
    </div>
  );
  const pairButton = (
    <Button
      key={`pair-${shake}`}
      label={t("pairing.join")}
      labelKey="pairing.join"
      glyph={<IconLink />}
      tone="accent"
      onClick={() => void pair()}
      disabled={busy || !complete}
      busy={busy}
      className={shake ? "glim-shake" : ""}
    />
  );
  // Its key would take the link glyph by pattern, which belongs to Pair.
  const pasteButton = (
    <Button
      label={t("pairing.paste")}
      labelKey="pairing.paste"
      glyph={<IconCopy />}
      tone="neutral"
      onClick={() => void pasteClipboard()}
      disabled={busy}
    />
  );
  return { field, paste: pasteButton, pair: pairButton };
}
