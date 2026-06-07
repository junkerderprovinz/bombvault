import { test } from "node:test";
import assert from "node:assert/strict";
import { encryptSecret, decryptSecret } from "../lib/secrets";

const KEY = "b".repeat(64); // 32 bytes hex

test("roundtrip recovers the exact plaintext", () => {
  const token = encryptSecret("hunter2-restic-repo-password", KEY);
  assert.equal(decryptSecret(token, KEY), "hunter2-restic-repo-password");
});

test("two encryptions of the same plaintext differ (random IV)", () => {
  const a = encryptSecret("same", KEY);
  const b = encryptSecret("same", KEY);
  assert.notEqual(a, b);
  assert.equal(decryptSecret(a, KEY), "same");
  assert.equal(decryptSecret(b, KEY), "same");
});

test("tampering with the ciphertext is detected (auth tag fails)", () => {
  const token = encryptSecret("secret", KEY);
  const parts = token.split(":");
  // Flip the last hex char of the ciphertext segment.
  const ct = parts[2];
  const flipped = ct.slice(0, -1) + (ct.slice(-1) === "0" ? "1" : "0");
  const tampered = [parts[0], parts[1], flipped, parts[3]].join(":");
  assert.throws(() => decryptSecret(tampered, KEY));
});

test("decrypting with the wrong key throws", () => {
  const token = encryptSecret("secret", KEY);
  assert.throws(() => decryptSecret(token, "c".repeat(64)));
});

test("rejects a malformed token", () => {
  assert.throws(() => decryptSecret("not-a-valid-token", KEY), /token/i);
});
