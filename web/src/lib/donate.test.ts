// ---------------------------------------------------------------------------
// The donation addresses, checked as far as each format allows (#3524).
//
// This is the one list in the app where a typo costs a stranger real money and
// nobody ever finds out: the person it happens to is not a user, they are
// someone who tried to give something away and got nothing back. Two of the
// five formats carry a real checksum, so those are verified rather than
// eyeballed; the rest are pinned by length and alphabet.
//
// The FIRST version of this list was grouped by coin and named "Tether" with
// the networks "BNB, Tron, Solana, Ethereum" above a single 0x… address. That
// address exists on EVM chains only. A donor picking Tron would have sent USDT
// into nothing. The grouping by chain is what makes the wrong choice
// unofferable, and that is what the last test here holds down.
// ---------------------------------------------------------------------------
import { createHash } from "node:crypto";
import { expect, it } from "vitest";
import { CRYPTO_CHAINS } from "./donate";

const BECH32 = "qpzry9x8gf2tvdw0s3jn54khce6mua7l";
const B58 = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz";
const XRP58 = "rpshnaf39wBUDNEGHJKLM4PQRST7VWXYZ2bcdeCg65jkm8oFqi1tuvAxyz";

function bech32Polymod(values: number[]): number {
  const gen = [0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3];
  let chk = 1;
  for (const v of values) {
    const b = chk >> 25;
    chk = ((chk & 0x1ffffff) << 5) ^ v;
    for (let i = 0; i < 5; i++) if ((b >> i) & 1) chk ^= gen[i]!;
  }
  return chk;
}

/** BIP-173/350: the checksum an address carries about itself. A single wrong
 *  character fails this, which is the whole point of the format. */
function bech32Ok(addr: string): boolean {
  const lower = addr.toLowerCase();
  if (addr !== lower && addr !== addr.toUpperCase()) return false;
  const pos = lower.lastIndexOf("1");
  if (pos < 1 || pos + 7 > lower.length || lower.length > 90) return false;
  const hrp = lower.slice(0, pos);
  const data = [...lower.slice(pos + 1)].map((c) => BECH32.indexOf(c));
  if (data.some((v) => v < 0)) return false;
  const expand = [...hrp].map((c) => c.charCodeAt(0) >> 5).concat([0], [...hrp].map((c) => c.charCodeAt(0) & 31));
  const chk = bech32Polymod(expand.concat(data));
  return chk === 1 || chk === 0x2bc830a3;
}

function b58Decode(s: string, alphabet: string): Buffer | null {
  let num = 0n;
  for (const c of s) {
    const i = alphabet.indexOf(c);
    if (i < 0) return null;
    num = num * 58n + BigInt(i);
  }
  let hex = num.toString(16);
  if (hex.length % 2) hex = "0" + hex;
  const body = Buffer.from(hex, "hex");
  let pad = 0;
  while (pad < s.length && s[pad] === alphabet[0]) pad++;
  return Buffer.concat([Buffer.alloc(pad), body]);
}

function chain(id: string) {
  const found = CRYPTO_CHAINS.find((c) => c.id === id);
  expect(found, `no chain with id ${id}`).toBeTruthy();
  return found!;
}

it("the Bitcoin address passes its own bech32 checksum", () => {
  expect(bech32Ok(chain("btc").address)).toBe(true);
  // And the check is not a rubber stamp: one flipped character must fail it.
  const broken = chain("btc").address.replace(/.$/, (c) => (c === "a" ? "q" : "a"));
  expect(bech32Ok(broken)).toBe(false);
});

it("the XRP address passes its base58check checksum", () => {
  const raw = b58Decode(chain("xrp").address, XRP58);
  expect(raw).toBeTruthy();
  expect(raw!.length).toBe(25);
  const body = raw!.subarray(0, 21);
  const chk = raw!.subarray(21);
  const want = createHash("sha256").update(createHash("sha256").update(body).digest()).digest().subarray(0, 4);
  expect(chk.equals(want)).toBe(true);
  // Prefix 0 is an account address rather than some other XRPL object.
  expect(body[0]).toBe(0);
});

it("the Solana address is a 32-byte key in the base58 alphabet", () => {
  const raw = b58Decode(chain("sol").address, B58);
  expect(raw).toBeTruthy();
  expect(raw!.length).toBe(32);
});

it("the EVM and Sui addresses are hex of the right length", () => {
  expect(chain("evm").address).toMatch(/^0x[0-9a-fA-F]{40}$/);
  expect(chain("sui").address).toMatch(/^0x[0-9a-fA-F]{64}$/);
});

it("offers no chain an address cannot live on", () => {
  // The rule this list is built on, stated as a check: an EVM address is
  // offered for EVM networks only. Tron and any other non-EVM chain must not
  // appear beside a 0x… address, because a donation sent there is destroyed.
  const evm = chain("evm");
  expect(evm.networks).toBeTruthy();
  expect(evm.networks!.toLowerCase()).not.toContain("tron");
  expect(evm.networks!.toLowerCase()).not.toContain("solana");
  // Nothing in the whole list mentions Tron, since there is no Tron address.
  const all = JSON.stringify(CRYPTO_CHAINS).toLowerCase();
  expect(all).not.toContain("tron");
});

it("every entry says what can be sent and where it goes", () => {
  for (const c of CRYPTO_CHAINS) {
    expect(c.name.length, c.id).toBeGreaterThan(1);
    expect(c.coins.length, c.id).toBeGreaterThan(1);
    expect(c.address.length, c.id).toBeGreaterThan(20);
  }
  // Ids are unique, since the copy toast and these tests address them by id.
  expect(new Set(CRYPTO_CHAINS.map((c) => c.id)).size).toBe(CRYPTO_CHAINS.length);
});
