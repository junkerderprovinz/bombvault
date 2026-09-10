// The donation addresses, grouped BY CHAIN rather than by coin (#3524).
//
// That grouping is the whole safety property of this file, and it came out of
// the list as it was first written down: it named "Tether" with the networks
// "BNB, Tron, Solana, Ethereum" above a single 0x… address. That address exists
// on EVM chains only. It is not a Tron address (those start with T) and not a
// Solana one, so a donor picking Tron would have sent USDT into nothing and the
// money would be gone. Nobody would ever have reported it, because the person
// it happens to is a stranger who never writes.
//
// One line per ADDRESS, with the coins it can receive as a subtitle: then the
// donor picks a chain that the address actually lives on, and the wrong choice
// is not offered. A coin that needs a chain we have no address for is simply
// absent (Tron, deliberately).
//
// Verified before shipping, as far as each format allows: the Bitcoin address
// passes its bech32 checksum, the XRP address its base58check, the Solana one
// decodes to 32 bytes, and the two EVM/Sui ones are well-formed hex of the
// right length. The XRP account was also checked on the ledger — an XRP account
// must hold a base reserve before it exists at all, and a donation to a
// non-existent account is REJECTED rather than lost.

export interface CryptoChain {
  /** Stable id, for the copy toast and for tests. */
  id: string;
  /** What the donor picks: the chain, never the coin. */
  name: string;
  /** What can be sent to this address, as a subtitle. */
  coins: string;
  /** The networks the address is valid on, where more than one applies. */
  networks?: string;
  address: string;
  /** A short line shown with the address, for what a donor has to know. */
  noteKey?: "about.cryptoNoTag";
}

export const CRYPTO_CHAINS: CryptoChain[] = [
  {
    id: "btc",
    name: "Bitcoin",
    coins: "BTC",
    address: "bc1q078lt57t4n5zq5md3knz3ythum0w78zmjw5eda",
  },
  {
    id: "evm",
    name: "Ethereum",
    coins: "ETH · USDT · USDC · BNB",
    networks: "Ethereum · BNB Smart Chain · Base · Optimism",
    address: "0xFF6726C5bd76C8FD6b6bE7Ea5CEd4621fde5e841",
  },
  {
    id: "sol",
    name: "Solana",
    coins: "SOL · USDT · USDC",
    address: "GrTyhSbZVArdaZAr3TqWDrkEGahomLtNZJ41qPLm3dHd",
  },
  {
    id: "sui",
    name: "Sui",
    coins: "SUI",
    address: "0xa76677f71d107c9a957c2d1814b68027cbd4ad9dbf28d911a97fad5d2fc3a414",
  },
  {
    id: "xrp",
    name: "XRP",
    coins: "XRP",
    address: "rwMK2nXqChT4JYWVypMypnpctDcM9jgWmG",
    // Worth saying out loud: plenty of exchanges demand a destination tag, and
    // somebody who has been trained by one will go looking for a field that is
    // not there. This is a self-custody account and its RequireDest flag is
    // off, checked on the ledger.
    noteKey: "about.cryptoNoTag",
  },
];
