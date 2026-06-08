import { test } from "node:test";
import assert from "node:assert/strict";
import {
  resolveLanguage,
  parseAcceptLanguage,
  pickLanguage,
  COOKIE,
} from "../lib/i18n/detect";
import { SUPPORTED, DEFAULT_LANGUAGE } from "../lib/i18n/locales";

// ── COOKIE name ───────────────────────────────────────────────────────────────

test("COOKIE is the app-namespaced name", () => {
  assert.equal(COOKIE, "bv_lang");
});

// ── resolveLanguage ───────────────────────────────────────────────────────────

test("resolveLanguage: exact match returned", () => {
  assert.equal(resolveLanguage(["de"], SUPPORTED, DEFAULT_LANGUAGE), "de");
});

test("resolveLanguage: region variant falls back to base language (de-AT → de)", () => {
  assert.equal(resolveLanguage(["de-AT"], SUPPORTED, DEFAULT_LANGUAGE), "de");
});

test("resolveLanguage: unsupported language falls through to fallback", () => {
  assert.equal(resolveLanguage(["xx"], SUPPORTED, DEFAULT_LANGUAGE), DEFAULT_LANGUAGE);
});

test("resolveLanguage: empty candidates → fallback", () => {
  assert.equal(resolveLanguage([], SUPPORTED, DEFAULT_LANGUAGE), DEFAULT_LANGUAGE);
});

test("resolveLanguage: first supported candidate wins (ja before de)", () => {
  assert.equal(resolveLanguage(["xx", "ja", "de"], SUPPORTED, DEFAULT_LANGUAGE), "ja");
});

test("resolveLanguage: matching is case-insensitive (DE-AT → de)", () => {
  assert.equal(resolveLanguage(["DE-AT"], SUPPORTED, DEFAULT_LANGUAGE), "de");
});

// ── parseAcceptLanguage ───────────────────────────────────────────────────────

test("parseAcceptLanguage: null → empty array", () => {
  assert.deepEqual(parseAcceptLanguage(null), []);
});

test("parseAcceptLanguage: undefined → empty array", () => {
  assert.deepEqual(parseAcceptLanguage(undefined), []);
});

test("parseAcceptLanguage: empty string → empty array", () => {
  assert.deepEqual(parseAcceptLanguage(""), []);
});

test("parseAcceptLanguage: single tag", () => {
  assert.deepEqual(parseAcceptLanguage("de"), ["de"]);
});

test("parseAcceptLanguage: q-values determine order (highest q first)", () => {
  const result = parseAcceptLanguage("en;q=0.5, de;q=0.9, fr;q=0.7");
  assert.deepEqual(result, ["de", "fr", "en"]);
});

test("parseAcceptLanguage: wildcard * is dropped", () => {
  const result = parseAcceptLanguage("de, *;q=0.1");
  assert.ok(!result.includes("*"));
});

test("parseAcceptLanguage: tags without q-value default to q=1", () => {
  const result = parseAcceptLanguage("de, en;q=0.8");
  assert.equal(result[0], "de");
  assert.equal(result[1], "en");
});

test("parseAcceptLanguage: stable sort keeps original order for equal q", () => {
  const result = parseAcceptLanguage("de, fr, en");
  assert.deepEqual(result, ["de", "fr", "en"]);
});

// ── pickLanguage ──────────────────────────────────────────────────────────────

test("pickLanguage: cookie wins over Accept-Language header", () => {
  // cookie = "fr", header prefers "de" → should return "fr"
  assert.equal(
    pickLanguage("fr", "de, en;q=0.9", SUPPORTED, DEFAULT_LANGUAGE),
    "fr",
  );
});

test("pickLanguage: browser language picked when no cookie (exact match)", () => {
  assert.equal(
    pickLanguage(null, "ja, en;q=0.9", SUPPORTED, DEFAULT_LANGUAGE),
    "ja",
  );
});

test("pickLanguage: browser language picked when no cookie (region variant de-AT → de)", () => {
  assert.equal(
    pickLanguage(null, "de-AT, en;q=0.5", SUPPORTED, DEFAULT_LANGUAGE),
    "de",
  );
});

test("pickLanguage: unsupported browser language falls back to next candidate", () => {
  // xx is unsupported, fr is; should pick fr
  assert.equal(
    pickLanguage(null, "xx, fr;q=0.8", SUPPORTED, DEFAULT_LANGUAGE),
    "fr",
  );
});

test("pickLanguage: unsupported browser language with no further candidates → DEFAULT_LANGUAGE (en)", () => {
  assert.equal(
    pickLanguage(null, "xx-YY", SUPPORTED, DEFAULT_LANGUAGE),
    DEFAULT_LANGUAGE,
  );
});

test("pickLanguage: no cookie, no Accept-Language header → DEFAULT_LANGUAGE (en)", () => {
  assert.equal(
    pickLanguage(null, null, SUPPORTED, DEFAULT_LANGUAGE),
    DEFAULT_LANGUAGE,
  );
});

test("pickLanguage: no cookie, empty Accept-Language header → DEFAULT_LANGUAGE (en)", () => {
  assert.equal(
    pickLanguage(undefined, "", SUPPORTED, DEFAULT_LANGUAGE),
    DEFAULT_LANGUAGE,
  );
});

test("pickLanguage: cookie with region variant is resolved (de-AT → de)", () => {
  assert.equal(
    pickLanguage("de-AT", null, SUPPORTED, DEFAULT_LANGUAGE),
    "de",
  );
});
