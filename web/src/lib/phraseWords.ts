// phraseWords reads a pasted or typed pairing phrase the way people hand one
// over: one word per line, numbered, separated by commas. The list is the
// server's own (internal/seedphrase/english.txt, kept identical by a Go test),
// so a word flagged here is one the server would refuse too.
import list from "./bip39-english.txt?raw";

export const PHRASE_WORDS = 12;

const KNOWN = new Set(list.split("\n").filter(Boolean));

/** splitPhrase returns the words in text, lower case, without the list
 *  numbers and punctuation around them, so "1. orbit," reads as "orbit". */
export function splitPhrase(text: string): string[] {
  return text
    .toLowerCase()
    .split(/[\s,;]+/)
    .map((w) => w.replace(/^[^\p{L}]+|[^\p{L}]+$/gu, ""))
    .filter(Boolean);
}

export interface PhraseCheck {
  words: string[];
  /** Positions, from 0, of words not on the list. */
  unknown: number[];
  /** Twelve words, all on the list. The checksum is left to the server. */
  complete: boolean;
}

/** checkPhrase looks up every word of text. The word still being typed, the
 *  last one with nothing after it, is not flagged yet, since a prefix of a
 *  good word is rarely a word itself. */
export function checkPhrase(text: string): PhraseCheck {
  const words = splitPhrase(text);
  const typing = text !== "" && !/[\s,;]$/.test(text);
  const unknown: number[] = [];
  words.forEach((w, i) => {
    if (!KNOWN.has(w) && !(typing && i === words.length - 1)) unknown.push(i);
  });
  const complete = words.length === PHRASE_WORDS && words.every((w) => KNOWN.has(w));
  return { words, unknown, complete };
}
