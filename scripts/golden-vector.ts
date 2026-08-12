// Prints golden vectors for the Go SDK parity tests. Run: bun scripts/golden-vector.ts
import { privateKeyToAccount } from "viem/accounts";

const account = privateKeyToAccount(
  "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
);

const lifiTypedData = {
  domain: {
    name: "Permit2",
    chainId: 4663,
    verifyingContract: "0x000000000022d473030f116ddee9f6b43ac78ba3",
  },
  types: {
    PermitWitnessTransferFrom: [
      { name: "permitted", type: "TokenPermissions" },
      { name: "spender", type: "address" },
      { name: "nonce", type: "uint256" },
      { name: "deadline", type: "uint256" },
      { name: "witness", type: "LiFiCall" },
    ],
    TokenPermissions: [
      { name: "token", type: "address" },
      { name: "amount", type: "uint256" },
    ],
    LiFiCall: [
      { name: "diamondAddress", type: "address" },
      { name: "diamondCalldataHash", type: "bytes32" },
    ],
  },
  primaryType: "PermitWitnessTransferFrom",
  message: {
    permitted: {
      token: "0xaf88d065e77c8cC2239327C5EDb3A432268e5831",
      amount: "1000000",
    },
    spender: "0x8eABB4E117fB70b346592e013855f6d825F50af1",
    nonce: "0",
    deadline: "9999999999",
    witness: {
      diamondAddress: "0xB477751B76CF82d00a686A1232f5fCD772414Af3",
      diamondCalldataHash: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    },
  },
} as const;

console.log("signer:", account.address);
console.log("lifi signature:", await account.signTypedData(lifiTypedData));

const permitTypedData = {
  domain: {
    name: "Mock USD",
    version: "1",
    chainId: 46630,
    verifyingContract: "0xf64780eAE9CFe162EF38f5224459a014a1007cd5",
  },
  types: {
    Permit: [
      { name: "owner", type: "address" },
      { name: "spender", type: "address" },
      { name: "value", type: "uint256" },
      { name: "nonce", type: "uint256" },
      { name: "deadline", type: "uint256" },
    ],
  },
  primaryType: "Permit",
  message: {
    owner: account.address,
    spender: "0x000000000022D473030F116dDEE9F6B43aC78BA3",
    value: (1n << 256n) - 1n,
    nonce: 5n,
    deadline: 9999999999n,
  },
} as const;

console.log("permit signature:", await account.signTypedData(permitTypedData));
