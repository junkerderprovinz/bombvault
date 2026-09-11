# Donate with crypto

BombVault is free and stays free. This page exists because some people would rather send
crypto than use a card, and the app's own donation window cannot be opened from a README.

Everything here is also in the app, under **Settings → About BombVault → Crypto**, where you
pick a coin and a chain and get a QR code. **If you are reading this on a phone, use the app
instead**: a QR code cannot be mistyped, and an address can.

## Pick the CHAIN first, not the coin

The list below is grouped by chain, and that is not a formatting choice. What decides whether
money arrives is the network you send on, not the coin you hold, and the two are easy to
confuse: the same USDT exists on three chains here, each with a different address. Find the
row for the network your wallet or exchange is sending from, then check that the coin you are
sending is listed on it.

| Chain | Address | Accepts |
| --- | --- | --- |
| Bitcoin | `bc1q078lt57t4n5zq5md3knz3ythum0w78zmjw5eda` | BTC |
| Ethereum | `0xFF6726C5bd76C8FD6b6bE7Ea5CEd4621fde5e841` | ETH, USDT, USDC |
| Base | `0xFF6726C5bd76C8FD6b6bE7Ea5CEd4621fde5e841` | ETH, USDC |
| Optimism | `0xFF6726C5bd76C8FD6b6bE7Ea5CEd4621fde5e841` | ETH |
| BNB Smart Chain | `0xFF6726C5bd76C8FD6b6bE7Ea5CEd4621fde5e841` | USDT, BNB |
| Solana | `GrTyhSbZVArdaZAr3TqWDrkEGahomLtNZJ41qPLm3dHd` | USDT, USDC, SOL |
| Sui | `0xa76677f71d107c9a957c2d1814b68027cbd4ad9dbf28d911a97fad5d2fc3a414` | SUI |
| XRP Ledger | `rwMK2nXqChT4JYWVypMypnpctDcM9jgWmG` | XRP |

**The four EVM rows really are the same address.** Ethereum, Base, Optimism and BNB Smart
Chain share one account, because one key controls it on all of them. It looks like a mistake
and it is not.

**XRP needs no destination tag.** Plenty of exchanges insist on one and will not let you
continue without it; this is a self-custody account with the RequireDest flag off, checked on
the ledger. If a form demands a tag, any value will do, including none.

**Only what is listed.** A coin that is not named on a row is not supported on that chain, and
sending it anyway is how coins are lost for good. ETH on BNB Smart Chain is the trap worth
naming: what trades there as ETH is a bridged token, not the real thing, and it is
deliberately not offered above.

## Other ways

If crypto is not your thing, the [README](https://github.com/junkerderprovinz/bombvault#readme)
has a card route and a PayPal one. All three go to the same person, and none of them changes
anything about the software: BombVault has no paid tier, no licence key and nothing held back.
