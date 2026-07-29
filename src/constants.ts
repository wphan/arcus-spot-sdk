import type { Address } from "viem";
import {
  ARBITRUM_CHAIN_ID,
  ARBITRUM_DEPLOYMENTS,
  CHAIN_DEPLOYMENTS_BY_ID,
  PERMIT2_ADDRESS,
  ROBINHOOD_MAINNET_DEPLOYMENTS,
  ROBINHOOD_TESTNET_DEPLOYMENTS,
} from "./chains.js";

export {
  ARBITRUM_CHAIN_ID,
  CHAIN_DEPLOYMENTS_BY_ID,
  getChainDeployments,
  getSettlementSourceAddresses,
  PERMIT2_ADDRESS,
  ROBINHOOD_MAINNET_CHAIN_ID,
  ROBINHOOD_MAINNET_DEPLOYMENTS,
  ROBINHOOD_TESTNET_CHAIN_ID,
  ROBINHOOD_TESTNET_DEPLOYMENTS,
  type ChainDeployments,
} from "./chains.js";

export const ARBITRUM_SWAP_SHELL = ARBITRUM_DEPLOYMENTS.swapShell;

export const ROBINHOOD_TESTNET_SWAP_SHELL = ROBINHOOD_TESTNET_DEPLOYMENTS.swapShell;

export const ROBINHOOD_MAINNET_SWAP_SHELL = ROBINHOOD_MAINNET_DEPLOYMENTS.swapShell;

export const SWAP_SHELL_BY_CHAIN_ID: Readonly<Partial<Record<number, Address>>> =
  Object.fromEntries(
    Object.values(CHAIN_DEPLOYMENTS_BY_ID)
      .filter((deployments): deployments is NonNullable<typeof deployments> => Boolean(deployments))
      .map((deployments) => [deployments.chainId, deployments.swapShell]),
  );

export function getSwapShellAddress(chainId: number): Address | undefined {
  return SWAP_SHELL_BY_CHAIN_ID[chainId];
}
