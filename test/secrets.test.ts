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

// --- Negative tests: segment-length and format guards ---

test("rejects a token with an 8-byte (16-hex-char) tag — truncated tag must not authenticate", () => {
  const token = encryptSecret("secret", KEY);
  const parts = token.split(":");
  // Replace the 32-hex-char (16-byte) tag with a 16-hex-char (8-byte) truncation.
  const truncatedTag = parts[3].slice(0, 16);
  const tampered = [parts[0], parts[1], parts[2], truncatedTag].join(":");
  assert.throws(() => decryptSecret(tampered, KEY), /malformed secret token/i);
});

test("rejects a token with a wrong-length IV (8 bytes)", () => {
  const token = encryptSecret("secret", KEY);
  const parts = token.split(":");
  // Replace the 24-hex-char (12-byte) IV with a 16-hex-char (8-byte) one.
  const shortIv = parts[1].slice(0, 16);
  const tampered = [parts[0], shortIv, parts[2], parts[3]].join(":");
  assert.throws(() => decryptSecret(tampered, KEY), /malformed secret token/i);
});

test("rejects a token with a non-hex segment", () => {
  const token = encryptSecret("secret", KEY);
  const parts = token.split(":");
  // Replace the ciphertext segment with clearly non-hex characters.
  const tampered = [parts[0], parts[1], "not-hex!!", parts[3]].join(":");
  assert.throws(() => decryptSecret(tampered, KEY), /malformed secret token/i);
});
