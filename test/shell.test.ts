import { describe, expect, test } from "bun:test";
import {
  encodeAbiParameters,
  encodeEventTopics,
  type Address,
  type Hex,
  type Log,
} from "viem";
import { decodeSwapExecutedLogs, swapShellAbi } from "../src";

describe("decodeSwapExecutedLogs", () => {
  test("decodes quoted amount fields from SwapExecuted logs", () => {
    const shell = "0x0000000000000000000000000000000000000001";
    const taker = "0x0000000000000000000000000000000000000002";
    const tokenIn = "0x0000000000000000000000000000000000000003";
    const tokenOut = "0x0000000000000000000000000000000000000004";
    const router = "0x0000000000000000000000000000000000000005";
    const routeTag = `0x${"00".repeat(32)}` as Hex;

    const topics = encodeEventTopics({
      abi: swapShellAbi,
      eventName: "SwapExecuted",
      args: { taker, tokenIn, tokenOut },
    });
    const data = encodeAbiParameters(
      [
        { type: "uint256" },
        { type: "uint256" },
        { type: "uint256" },
        { type: "uint256" },
        { type: "uint256" },
        { type: "uint256" },
        { type: "uint256" },
        { type: "address" },
        { type: "bytes32" },
        { type: "bool" },
        { type: "string" },
      ],
      [98n, 100n, 100n, 123n, 120n, 1n, 2n, router, routeTag, true, ""],
    );

    const decoded = decodeSwapExecutedLogs([
      {
        address: shell as Address,
        data,
        topics,
        blockHash: null,
        blockNumber: null,
        logIndex: 0,
        transactionHash: null,
        transactionIndex: null,
      } as Log,
    ]);

    expect(decoded).toHaveLength(1);
    expect(decoded[0]?.args.taker.toLowerCase()).toBe(taker);
    expect(decoded[0]?.args.quotedAmountIn).toBe(100n);
    expect(decoded[0]?.args.quotedAmountOut).toBe(123n);
    expect(decoded[0]?.args.amountOut).toBe(120n);
  });
});
