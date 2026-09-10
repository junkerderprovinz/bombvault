// The donation addresses ([3524], regrouped as coin-then-chain in [3554]).
//
// WHAT MAKES THIS LIST SAFE: every network a donor can pick carries its OWN
// address. There is no line anywhere that names a chain without an address to
// go with it, so the wrong choice is not merely discouraged, it cannot be
// made.
//
// That property is the whole reason this file is shaped the way it is, and it
// came out of the list as it was FIRST written: it named "Tether" with the
// networks "BNB, Tron, Solana, Ethereum" above a single 0x… address. That
// address exists on EVM chains only. It is not a Tron address (those start
// with T) and not a Solana one, so a donor picking Tron would have sent USDT
// into nothing and the money would be gone. Nobody would ever have reported
// it, because the person it happens to is a stranger who never writes.
//
// The second version fixed that by offering CHAINS only, with the coins as a
// subtitle. This one offers coins again, the way a donor actually thinks ("I
// have USDT"), and keeps the property by making the chain a second, real
// choice underneath: pick USDT and you pick between Ethereum, BNB Smart Chain
// and Solana, each of which resolves to an address that lives there. Tron is
// absent, as it was before, because there is no Tron address.
//
// Verified before shipping, as far as each format allows, and in a test rather
// than by eye (donate.test.ts): the Bitcoin address passes its bech32
// checksum, the XRP address its base58check, the Solana one decodes to 32
// bytes, and the two EVM/Sui ones are well-formed hex of the right length. The
// XRP account was also checked on the ledger — an XRP account must hold a base
// reserve before it exists at all, and a donation to a non-existent account is
// REJECTED rather than lost.

/** One address, and the chain it lives on. */
export interface CryptoNetwork {
  /** Stable id, for the copy toast and for tests. */
  id: string;
  /** The chain as a donor's wallet names it. */
  name: string;
  address: string;
  /** A short line shown with the address, for what a donor has to know. */
  noteKey?: "about.cryptoNoTag";
}

/** One coin, and every chain it can be sent on here. */
export interface CryptoCoin {
  /** Stable id, and the key components/donateMarks.tsx draws the mark by. */
  id: string;
  /** The ticker, on the tile under the mark. */
  symbol: string;
  /** The full name, for the accessible label. */
  name: string;
  /** Never empty, and every entry carries an address. */
  networks: CryptoNetwork[];
}

// The five wallets, named once. They are written into the networks below, so a
// chain can never be listed without one — but defined here, so one wallet is
// one string rather than four copies that can drift apart.
const BTC = "bc1q078lt57t4n5zq5md3knz3ythum0w78zmjw5eda";
/** One address for every EVM chain: the same key controls it on all of them. */
const EVM = "0xFF6726C5bd76C8FD6b6bE7Ea5CEd4621fde5e841";
const SOL = "GrTyhSbZVArdaZAr3TqWDrkEGahomLtNZJ41qPLm3dHd";
const SUI = "0xa76677f71d107c9a957c2d1814b68027cbd4ad9dbf28d911a97fad5d2fc3a414";
const XRP = "rwMK2nXqChT4JYWVypMypnpctDcM9jgWmG";

const ETHEREUM = { id: "ethereum", name: "Ethereum", address: EVM };
const BASE = { id: "base", name: "Base", address: EVM };
const OPTIMISM = { id: "optimism", name: "Optimism", address: EVM };
const BSC = { id: "bsc", name: "BNB Smart Chain", address: EVM };
const SOLANA = { id: "solana", name: "Solana", address: SOL };

export const CRYPTO_COINS: CryptoCoin[] = [
  {
    id: "btc",
    symbol: "BTC",
    name: "Bitcoin",
    networks: [{ id: "bitcoin", name: "Bitcoin", address: BTC }],
  },
  {
    id: "eth",
    symbol: "ETH",
    name: "Ethereum",
    // Native ETH on all three. NOT BNB Smart Chain: what trades as ETH there
    // is a bridged token, and offering it beside the real thing invites
    // somebody to send the wrong one.
    networks: [ETHEREUM, BASE, OPTIMISM],
  },
  {
    id: "usdt",
    symbol: "USDT",
    name: "Tether",
    networks: [ETHEREUM, BSC, SOLANA],
  },
  {
    id: "usdc",
    symbol: "USDC",
    name: "USD Coin",
    networks: [ETHEREUM, BASE, SOLANA],
  },
  {
    id: "bnb",
    symbol: "BNB",
    name: "BNB",
    networks: [BSC],
  },
  {
    id: "sol",
    symbol: "SOL",
    name: "Solana",
    networks: [SOLANA],
  },
  {
    id: "sui",
    symbol: "SUI",
    name: "Sui",
    networks: [{ id: "sui", name: "Sui", address: SUI }],
  },
  {
    id: "xrp",
    symbol: "XRP",
    name: "XRP",
    networks: [
      {
        id: "xrpl",
        name: "XRP Ledger",
        address: XRP,
        // Worth saying out loud: plenty of exchanges demand a destination tag,
        // and somebody trained by one will go looking for a field that is not
        // there. This is a self-custody account and its RequireDest flag is
        // off, checked on the ledger.
        noteKey: "about.cryptoNoTag",
      },
    ],
  },
];

/**
 * Which wallet each chain must resolve to, so the test can check the list
 * against something other than itself.
 *
 * Written out by hand on purpose. Derived from the list it is meant to guard,
 * it would agree with any mistake in it; written here, a chain that ever gets
 * pointed at the wrong wallet fails immediately.
 */
export const ADDRESS_BY_CHAIN: Record<string, string> = {
  bitcoin: BTC,
  ethereum: EVM,
  base: EVM,
  optimism: EVM,
  bsc: EVM,
  solana: SOL,
  sui: SUI,
  xrpl: XRP,
};
