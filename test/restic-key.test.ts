import { test } from "node:test";
import assert from "node:assert/strict";
import { deriveResticPassword } from "../lib/restic-key";

const KEY_A = "a".repeat(64); // valid 32-byte APP_KEY in hex
const KEY_B = "b".repeat(64);

test("deriveResticPassword is deterministic for the same APP_KEY", () => {
  assert.equal(deriveResticPassword(KEY_A), deriveResticPassword(KEY_A));
});

test("deriveResticPassword differs for different APP_KEYs", () => {
  assert.notEqual(deriveResticPassword(KEY_A), deriveResticPassword(KEY_B));
});

test("deriveResticPassword returns 64 lowercase hex chars (sha256)", () => {
  assert.match(deriveResticPassword(KEY_A), /^[0-9a-f]{64}$/);
});

test("deriveResticPassword never equals the APP_KEY itself", () => {
  assert.notEqual(deriveResticPassword(KEY_A), KEY_A);
});
