/**
 * Example usage of the logs-lookup API.
 * 
 * 
ARBITRUM_RPC_ENDPOINT="..." \
SWAP_SHELL=0xF8698CA109583Ea2CBc57EF560d4Be2E0DF349A2 \
TAKER=0x501BCab05Aec979E78EDD54cCf714D9d4c38Dc7F \
FROM_BLOCK=0x1BBB345D \
TO_BLOCK=0x1BBB345F \
bun run logs:lookup
$ bun examples/logs-lookup.ts
[
  {
    "address": "0xf8698ca109583ea2cbc57ef560d4be2e0df349a2",
    "blockHash": "0x35fe583a8483ce2eb82c4b74fc127287542f13b357e869281f5b5367221a5dc7",
    "blockNumber": "465253469",
    "logIndex": 17,
    "transactionHash": "0x2b4551d22a6ac1279d540707e52e42e308f42f97ada394ef85b223f1fceeb914",
    "transactionIndex": 3,
    "args": {
      "taker": "0x501BCab05Aec979E78EDD54cCf714D9d4c38Dc7F",
      "tokenIn": "0xaf88d065e77c8cC2239327C5EDb3A432268e5831",
      "tokenOut": "0x82aF49447D8a07e3bd95BD0d56f35241523fBab1",
      "minAmountOut": "248205331866756",
      "amountIn": "550000",
      "amountOut": "249462247570965",
      "tokenInBenchmarkPrice": "0",
      "tokenOutBenchmarkPrice": "0",
      "router": "0xfbeCF057d93430a15A936Dd57A7424D4F0A8772b",
      "routeTag": "0x5a45524f45580000000000000000000000000000000000000000000000000000",
      "success": true,
      "reason": ""
    },
    "routeTagText": "ZEROEX"
  }
]
 */
import { createPublicClient, http, type Address, type Hex } from "viem";
import { arbitrum } from "viem/chains";
import { ARBITRUM_SWAP_SHELL, getSwapShellTradeHistory } from "../src/index";

declare const process: { env: Record<string, string | undefined> };

const rpcUrl = requiredEnv("ARBITRUM_RPC_ENDPOINT");
const swapShell = optionalAddressEnv("SWAP_SHELL") ?? ARBITRUM_SWAP_SHELL;
const taker = addressEnv("TAKER");
const fromBlock = blockEnv("FROM_BLOCK");
const toBlock = blockEnv("TO_BLOCK");

const publicClient = createPublicClient({
  chain: arbitrum,
  transport: http(rpcUrl),
});

const logs = await getSwapShellTradeHistory({
  publicClient,
  chainId: arbitrum.id,
  swapShell,
  taker,
  fromBlock,
  toBlock,
  tokenIn: optionalAddressEnv("TOKEN_IN"),
  tokenOut: optionalAddressEnv("TOKEN_OUT"),
});

console.log(
  JSON.stringify(
    logs.map((log) => ({
      ...log,
      routeTagText: bytes32ToText(log.args.routeTag),
    })),
    (_key, value) => (typeof value === "bigint" ? value.toString() : value),
    2,
  ),
);

function requiredEnv(name: string): string {
  const value = process.env[name];
  if (!value) throw new Error(`Missing ${name}`);
  return value;
}

function addressEnv(name: string): Address {
  return parseAddress(requiredEnv(name), name);
}

function optionalAddressEnv(name: string): Address | undefined {
  const value = process.env[name];
  return value ? parseAddress(value, name) : undefined;
}

function parseAddress(value: string, name: string): Address {
  if (!/^0x[0-9a-fA-F]{40}$/.test(value)) {
    throw new Error(`${name} must be a 0x address`);
  }
  return value as Address;
}

function blockEnv(name: string): bigint {
  const value = requiredEnv(name);
  try {
    return BigInt(value);
  } catch {
    throw new Error(`${name} must be a decimal or 0x block number`);
  }
}

function bytes32ToText(value: Hex): string {
  const bytes = value.slice(2).match(/.{1,2}/g) ?? [];
  const chars = bytes.map((byte) => Number.parseInt(byte, 16)).filter((code) => code !== 0);
  return String.fromCharCode(...chars);
}
