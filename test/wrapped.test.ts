import { describe, expect, test } from "bun:test";
import {
  ARBITRUM_CHAIN_ID,
  predictWrappedToken,
  predictWrappedTokenForChain,
  ROBINHOOD_MAINNET_CHAIN_ID,
  ROBINHOOD_TESTNET_CHAIN_ID,
} from "../src";

// Vectors computed independently with `cast create2` against the deployed
// WrappedTokenFactory/beacon addresses from the router contracts README:
//   salt      = cast keccak $(cast abi-encode "f(address)" $UNDERLYING)
//   initHash  = cast keccak "${BEACON_PROXY_CREATION_CODE}${abi.encode(beacon, "")}"
//   predicted = cast create2 --deployer $FACTORY --salt $salt --init-code-hash $initHash
describe("predictWrappedToken", () => {
  // Deployed on-chain pair: WEEK -> wWEEK on Robinhood Mainnet (4663). The
  // wrapped address is live contract state, not a recomputation of this code.
  test("matches deployed wWEEK for WEEK on Robinhood Mainnet (4663)", () => {
    expect(
      predictWrappedToken({
        wrappedTokenFactory: "0x8bc71aE8EaC8B25F30c2990930Cc3A80E72e169e",
        wrappedTokenBeacon: "0x27fEB332759F8d2f351D7fC72D29af37664ffd77",
        underlying: "0xc93a8c440CEa26D7445dF01729f193b27965099f",
      }),
    ).toBe("0x4B17e556568bB02709a50cA67db7F4DBD46E3d17");
  });

  test("matches cast create2 on Robinhood Testnet (46630)", () => {
    expect(
      predictWrappedToken({
        wrappedTokenFactory: "0xF361dD4cd631175648f54B657d44A3128903D92c",
        wrappedTokenBeacon: "0xDd70cD7BbE53ac3d67c1959C9De84423f5Bf2bca",
        underlying: "0x1111111111111111111111111111111111111111",
      }),
    ).toBe("0xEFEbf34C80F78d85C085908EB30eb0C6b873d47A");
  });
});

describe("predictWrappedTokenForChain", () => {
  test("matches deployed wWEEK for WEEK on Robinhood Mainnet (4663)", () => {
    expect(
      predictWrappedTokenForChain({
        chainId: ROBINHOOD_MAINNET_CHAIN_ID,
        underlying: "0xc93a8c440CEa26D7445dF01729f193b27965099f",
      }),
    ).toBe("0x4B17e556568bB02709a50cA67db7F4DBD46E3d17");
  });

  test("matches cast create2 on Robinhood Testnet (46630)", () => {
    expect(
      predictWrappedTokenForChain({
        chainId: ROBINHOOD_TESTNET_CHAIN_ID,
        underlying: "0x1111111111111111111111111111111111111111",
      }),
    ).toBe("0xEFEbf34C80F78d85C085908EB30eb0C6b873d47A");
  });

  test("throws when the chain has no wrapped-token deployment", () => {
    expect(() =>
      predictWrappedTokenForChain({
        chainId: ARBITRUM_CHAIN_ID,
        underlying: "0x82af49447d8a07e3bd95bd0d56f35241523fbab1",
      }),
    ).toThrow(
      `Chain ${ARBITRUM_CHAIN_ID} has no wrapped-token factory/beacon configured`,
    );
  });
});
