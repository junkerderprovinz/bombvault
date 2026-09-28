// PhraseInput takes the twelve words of another instance. It reads a paste
// with line breaks, list numbers and commas, fills twelve numbered slots as
// the words arrive and names an unknown word with its place before anything
// is sent. Pair stays off until twelve known words are there; whether they
// belong together only the server can say, from the checksum.
import { useState } from "react";
import { Button } from "../../components/Button";
import { IconLink } from "../../components/glyphs";
import { InfoBubble } from "../../components/InfoBubble";
import type { useT } from "../../lib/i18n";
import { PHRASE_WORDS, checkPhrase } from "../../lib/phraseWords";

type T = ReturnType<typeof useT>["t"];

export function PhraseInput({
  id,
  label,
  tip,
  bare = false,
  disabled = false,
  busy,
  onPair,
  onCancel,
  t,
}: {
  id: string;
  label: string;
  tip: string;
  /** Inside a numbered step that already says what goes here, the label is
   *  only read out, not shown. */
  bare?: boolean;
  disabled?: boolean;
  busy: boolean;
  /** Sends the words, one space apart, and resolves to the refusal to show,
   *  or null once paired. */
  onPair: (phrase: string) => Promise<string | null>;
  onCancel?: () => void;
  t: T;
}) {
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

  async function pair() {
    const answer = await onPair(words.join(" "));
    if (answer === null) {
      setText("");
      return;
    }
    setRefusal(answer);
    setShake((n) => n + 1);
  }

  const bad = new Set(unknown);
  return (
    <div className={`flex flex-col gap-2 ${disabled ? "opacity-50" : ""}`} data-testid={id}>
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
        {!bare && (
          <label htmlFor={id} className="inline-flex items-center gap-1.5 text-[13px] font-medium text-carbon-text">
            {label}
            <InfoBubble tip={tip} />
          </label>
        )}
        <span
          className={`ms-auto text-xs tabular-nums ${complete ? "text-statusOk" : "text-carbon-textMuted"}`}
          aria-live="polite"
        >
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
        rows={3}
        spellCheck={false}
        autoComplete="off"
        autoCapitalize="none"
        dir="ltr"
        disabled={disabled}
        aria-label={bare ? label : undefined}
        aria-invalid={problem ? true : undefined}
        placeholder={t("pairing.enterPlaceholder")}
        className="min-h-19 resize-y rounded-control bg-carbon-surface2 px-3 py-2.5 font-mono text-[13px] leading-normal text-carbon-text placeholder:font-sans placeholder:text-carbon-textMuted glim-field-focus"
      />
      <ol className="grid grid-cols-3 gap-1 sm:grid-cols-6 sm:gap-1.5" aria-hidden="true">
        {Array.from({ length: PHRASE_WORDS }, (_, i) => {
          const w = words[i];
          const tone =
            w === undefined
              ? "ring-1 ring-inset ring-carbon-border text-carbon-textMuted"
              : bad.has(i)
                ? "bg-statusFailBg text-statusFail"
                : "bg-carbon-surface2 text-carbon-text";
          return (
            <li key={i} className={`flex min-w-0 items-baseline gap-1.5 rounded-lg px-2 py-0.5 text-xs ${tone}`}>
              <span className={`shrink-0 text-end tabular-nums ${bad.has(i) ? "" : "text-carbon-textMuted"}`}>{i + 1}</span>
              <span dir="ltr" className="truncate font-mono">
                {w ?? "·"}
              </span>
            </li>
          );
        })}
      </ol>
      <p role="alert" className="min-h-0 text-[13px] text-statusFail empty:hidden">
        {problem ?? ""}
      </p>
      <div className="flex flex-wrap items-center justify-end gap-2">
        {onCancel && (
          <Button label={t("common.cancel")} labelKey="common.cancel" tone="neutral" onClick={onCancel} disabled={busy} />
        )}
        <Button
          key={`pair-${shake}`}
          label={t("pairing.join")}
          labelKey="pairing.join"
          glyph={<IconLink />}
          tone="accent"
          onClick={() => void pair()}
          disabled={disabled || busy || !complete}
          busy={busy}
          className={shake ? "glim-shake" : ""}
        />
      </div>
    </div>
  );
}
