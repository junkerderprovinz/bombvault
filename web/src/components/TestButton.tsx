import type { ReactNode } from "react";
import { groupStage } from "../lib/controls";
import { useT, type TranslationKey } from "../lib/i18n";
import type { TestVerdictState, Verdict } from "../lib/useTestVerdict";
import { Button, type ButtonTone } from "./Button";
import { CheckDraw } from "./CheckDraw";
import { IconClose } from "./navGlyphs";

// The words a verdict puts on the button, by what the button tests.
const WORDS = {
  connection: ["verdict.connected", "verdict.notConnected"],
  delivery: ["verdict.sent", "verdict.notSent"],
  check: ["verdict.passed", "verdict.failed"],
} as const satisfies Record<string, readonly [TranslationKey, TranslationKey]>;

export type VerdictWords = keyof typeof WORDS;

/** The verdict's word for a button of the given kind. */
export function verdictKey(verdict: Verdict, words: VerdictWords): TranslationKey {
  return WORDS[words][verdict.ok ? 0 : 1];
}

/** The check a passed test shows, and the cross a failed one shows. */
export function verdictGlyph(verdict: Verdict): ReactNode {
  return verdict.ok ? <CheckDraw /> : <IconClose />;
}

/**
 * TestButton is a button that runs a test and then shows the verdict on
 * itself: the success fill with a word like Connected and a check, or the
 * danger fill with Not connected and a cross, so colour is never the only
 * signal. The resting label stays the hover text, since clicking again runs
 * the test again. All three words share one width stage, so the verdict does
 * not resize the button.
 */
export function TestButton({
  label,
  labelKey,
  words = "connection",
  test,
  onClick,
  tone = "neutral",
  glyph,
  hueIndex,
  disabled = false,
  title,
  className = "",
}: {
  label: string;
  labelKey: string;
  words?: VerdictWords;
  /** From useTestVerdict, or the same fields from a page that keeps its own
   *  result. */
  test: Pick<TestVerdictState, "verdict" | "running" | "shake" | "shaking">;
  onClick: () => void;
  /** The tone at rest. */
  tone?: ButtonTone;
  glyph?: ReactNode;
  hueIndex?: number;
  disabled?: boolean;
  /** Extra explanation at rest. */
  title?: string;
  className?: string;
}) {
  const { t } = useT();
  const { verdict, running, shake, shaking } = test;
  const [passKey, failKey] = WORDS[words];
  const key = verdict ? verdictKey(verdict, words) : null;
  return (
    <Button
      key={shake}
      label={key ? t(key) : label}
      labelKey={key ?? labelKey}
      glyph={verdict ? verdictGlyph(verdict) : glyph}
      tone={verdict ? (verdict.ok ? "ok" : "danger") : tone}
      stage={groupStage([label, t(passKey), t(failKey)])}
      hueIndex={hueIndex}
      onClick={onClick}
      disabled={disabled || running}
      busy={running}
      title={verdict ? label : title}
      className={`${shaking ? "glim-shake" : ""} ${className}`.trim()}
    />
  );
}

/**
 * VerdictLine is the one line above a test's buttons: why the test failed, or
 * what its pass leaves out. It renders nothing for a plain pass.
 */
export function VerdictLine({ verdict, className = "" }: { verdict: Verdict | null; className?: string }) {
  if (!verdict) return null;
  const text = verdict.ok ? verdict.note : verdict.reason;
  if (!text) return null;
  const tone = !verdict.ok ? "text-statusFail" : verdict.caveat ? "text-statusWarn" : "text-carbon-textSub";
  return <p className={`text-xs wrap-break-word ${tone} ${className}`.trim()}>{text}</p>;
}
