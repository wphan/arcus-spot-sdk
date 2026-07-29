# Demo UI

Minimal local browser console for exercising `@arcus-xyz/arcus-spot-sdk` against a v5 spot router server.

## Run

1. Start `v5-spot-router` with `ZEROX_API_KEY`, `RELAYER_PRIVATE_KEY`, and `RPC_URL`.
2. From the repo root, run `bun install`.
3. Run `bun run demo`.
4. Open the Vite URL, connect a wallet, fetch quotes, and select the quote to sign.

The router base-URL preset drives the selected chain: the hosted preset selects Arbitrum `42161` (0x) and the local preset selects Robinhood testnet `46630` (Arcus). Changing the chain ID re-syncs the SwapShell address and the token presets, defaulting the pair to **USDC → WETH** (Arbitrum), **USDG → WEEK** (RH mainnet `4663`), or **mUSDG → mTSLA** (RH testnet `46630`). The token dropdowns show only tokens for the selected chain and merge `/tokens` from the versioned base URL when available.
