import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import {
  resolveTheme,
  DEFAULT_THEME,
  THEME_COOKIE,
  THEMES,
} from "../lib/theme";

// ── resolveTheme ─────────────────────────────────────────────────────────────

test("resolveTheme: dark cookie → dark", () => {
  assert.equal(resolveTheme("dark"), "dark");
});

test("resolveTheme: light cookie → light", () => {
  assert.equal(resolveTheme("light"), "light");
});

test("resolveTheme: undefined → DEFAULT_THEME", () => {
  assert.equal(resolveTheme(undefined), DEFAULT_THEME);
});

test("resolveTheme: null → DEFAULT_THEME", () => {
  assert.equal(resolveTheme(null), DEFAULT_THEME);
});

test("resolveTheme: empty string → DEFAULT_THEME", () => {
  assert.equal(resolveTheme(""), DEFAULT_THEME);
});

test("resolveTheme: unknown value → DEFAULT_THEME", () => {
  assert.equal(resolveTheme("solarized"), DEFAULT_THEME);
});

// ── THEMES constant ──────────────────────────────────────────────────────────

test("THEMES contains exactly dark and light", () => {
  assert.deepEqual([...THEMES].sort(), ["dark", "light"]);
});

test("DEFAULT_THEME is in THEMES", () => {
  assert.ok((THEMES as readonly string[]).includes(DEFAULT_THEME));
});

// ── THEME_COOKIE name ────────────────────────────────────────────────────────

test("THEME_COOKIE is the app-namespaced name", () => {
  assert.equal(THEME_COOKIE, "bv_theme");
});

// ── lib/theme.ts must NOT be a client module ─────────────────────────────────

test("lib/theme.ts must NOT be a client module", () => {
  const src = readFileSync(
    fileURLToPath(new URL("../lib/theme.ts", import.meta.url)),
    "utf8",
  );
  const firstCode = src
    .split("\n")
    .map((l) => l.trim())
    .find((l) => l.length > 0 && !l.startsWith("//"));
  assert.ok(
    firstCode !== undefined && !/^["']use client["']/.test(firstCode),
    `lib/theme.ts first statement is ${JSON.stringify(firstCode)} — it must not be a "use client" directive`,
  );
});
