// The donation addresses, listed coin first the way a donor thinks ("I have
// USDT"), with the chain as a second choice underneath. Every network carries
// its own address, so no chain can be picked without a wallet that lives on it.
// Tron is absent because there is no Tron address, and the EVM one would be
// lost there.
//
// donate.test.ts checks each address as far as its format allows. The XRP
// account was also checked on the ledger; a payment to an XRP account that
// does not exist is rejected rather than lost.

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

// Each wallet is defined once, so the networks that share it cannot drift apart.
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
    // Native ETH only. ETH on BNB Smart Chain is a bridged token, and offering
    // it beside the real thing invites sending the wrong one.
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
        // Exchanges often demand a destination tag, so say that none is needed.
        // This is a self-custody account with RequireDest off, checked on the
        // ledger.
        noteKey: "about.cryptoNoTag",
      },
    ],
  },
];

/**
 * ADDRESS_BY_CHAIN is the wallet each chain must resolve to. It is written out
 * by hand because a table derived from CRYPTO_COINS would agree with any
 * mistake in it.
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
