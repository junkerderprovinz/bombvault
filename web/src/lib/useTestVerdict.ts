import { useState } from "react";

/** What a test found. */
export interface Verdict {
  ok: boolean;
  /** Why the test failed, for the line above the buttons. */
  reason?: string;
  /** What a pass alone does not say, such as a repository that is not there
   *  yet. */
  note?: string;
  /** Shows the note as a caveat rather than as plain information. */
  caveat?: boolean;
}

/**
 * useTestVerdict runs a test and holds its verdict until the inputs it tested
 * change, which drops it for good. `inputs` is compared by value, so a config
 * object rebuilt on every render keeps the verdict until one of its fields
 * changes.
 * `fallback` is the reason given when the test throws without a message.
 */
export function useTestVerdict(inputs: unknown, fallback: string) {
  const tested = JSON.stringify(inputs ?? null);
  const [result, setResult] = useState<{ tested: string; verdict: Verdict; shook: boolean } | null>(null);
  const [running, setRunning] = useState(false);
  // Bumped on each failure a click produced; TestButton keys itself on it so
  // the shake replays.
  const [shake, setShake] = useState(0);
  const [seen, setSeen] = useState(tested);
  if (seen !== tested) {
    setSeen(tested);
    setResult(null);
  }

  /** Runs the test. A run nobody clicked, such as one on mount, passes
   *  `clicked: false` and does not shake on a failure. */
  async function run(test: () => Promise<Verdict>, clicked = true): Promise<Verdict> {
    setRunning(true);
    setResult(null);
    let verdict: Verdict;
    try {
      verdict = await test();
    } catch (e) {
      verdict = { ok: false, reason: e instanceof Error && e.message ? e.message : fallback };
    }
    const shook = clicked && !verdict.ok;
    setResult({ tested, verdict, shook });
    if (shook) setShake((n) => n + 1);
    setRunning(false);
    return verdict;
  }

  const current = result?.tested === tested ? result : null;
  return { verdict: current?.verdict ?? null, shaking: !!current?.shook, shake, running, run };
}

export type TestVerdictState = ReturnType<typeof useTestVerdict>;
