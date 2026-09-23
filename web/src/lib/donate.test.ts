// A typo in a donation address loses a stranger's money without anyone noticing,
// so each address is checked as far as its format allows: Bitcoin and XRP carry
// a real checksum, the rest are held to length and alphabet. Each chain is also
// held to the wallet it is meant to reach, from a table written out by hand.
import { createHash } from "node:crypto";
import { expect, it } from "vitest";
import { ADDRESS_BY_CHAIN, CRYPTO_COINS } from "./donate";
import { hasCoinMark } from "../components/donateMarks";

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

/** bech32Ok verifies the BIP-173/350 checksum, which any single wrong character
 *  fails. */
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

const networks = CRYPTO_COINS.flatMap((c) => c.networks);
const chain = (id: string) => networks.find((n) => n.id === id);

it("points every chain at the wallet it is supposed to reach", () => {
  // An address that belongs to another chain looks fine in review, so compare
  // against the hand-written table in donate.ts rather than the list itself.
  for (const coin of CRYPTO_COINS) {
    expect(coin.networks.length, `${coin.id} offers no network`).toBeGreaterThan(0);
    for (const n of coin.networks) {
      expect(ADDRESS_BY_CHAIN[n.id], `${coin.id}/${n.id} is not a chain this app knows`).toBeTruthy();
      expect(n.address, `${coin.id}/${n.id} points at the wrong wallet`).toBe(ADDRESS_BY_CHAIN[n.id]);
    }
  }
  // Exactly five wallets, each covered by a format test below.
  expect(new Set(Object.values(ADDRESS_BY_CHAIN)).size).toBe(5);
});

it("the Bitcoin address passes its own bech32 checksum", () => {
  const btc = chain("bitcoin")!.address;
  expect(bech32Ok(btc)).toBe(true);
  // One flipped character must fail it.
  expect(bech32Ok(btc.replace(/.$/, (c) => (c === "a" ? "q" : "a")))).toBe(false);
});

it("the XRP address passes its base58check checksum", () => {
  const raw = b58Decode(chain("xrpl")!.address, XRP58);
  expect(raw).toBeTruthy();
  expect(raw!.length).toBe(25);
  const body = raw!.subarray(0, 21);
  const want = createHash("sha256").update(createHash("sha256").update(body).digest()).digest().subarray(0, 4);
  expect(raw!.subarray(21).equals(want)).toBe(true);
  // Prefix 0 is an account address rather than some other XRPL object.
  expect(body[0]).toBe(0);
});

it("the Solana address is a 32-byte key in the base58 alphabet", () => {
  const raw = b58Decode(chain("solana")!.address, B58);
  expect(raw).toBeTruthy();
  expect(raw!.length).toBe(32);
});

it("the EVM and Sui addresses are hex of the right length", () => {
  expect(chain("ethereum")!.address).toMatch(/^0x[0-9a-fA-F]{40}$/);
  expect(chain("sui")!.address).toMatch(/^0x[0-9a-fA-F]{64}$/);
});

it("offers no chain an address cannot live on", () => {
  // There is no Tron address, so Tron must not be offered at all.
  expect(JSON.stringify(CRYPTO_COINS).toLowerCase()).not.toContain("tron");
  // The EVM wallet is offered for EVM chains only. The others have their own,
  // and a 0x… address on any of them is unreachable.
  const evm = ADDRESS_BY_CHAIN.ethereum;
  for (const id of ["solana", "bitcoin", "xrpl", "sui"]) {
    expect(ADDRESS_BY_CHAIN[id], `${id} must not share the EVM wallet`).not.toBe(evm);
  }
});

it("gives every coin a mark, a ticker and a unique id", () => {
  for (const c of CRYPTO_COINS) {
    expect(c.symbol.length, c.id).toBeGreaterThan(1);
    expect(c.name.length, c.id).toBeGreaterThan(1);
    // A tile without a mark reads as a missing image.
    expect(hasCoinMark(c.id), `${c.id} has no mark in donateMarks.tsx`).toBe(true);
  }
  expect(new Set(CRYPTO_COINS.map((c) => c.id)).size).toBe(CRYPTO_COINS.length);
  // Network ids are unique within a coin too, since the chain row keys on them.
  for (const c of CRYPTO_COINS) {
    expect(new Set(c.networks.map((n) => n.id)).size, c.id).toBe(c.networks.length);
  }
});
