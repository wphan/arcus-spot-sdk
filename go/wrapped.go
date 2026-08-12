package arcusspot

// Pure, offline CREATE2 derivation of the canonical wrapped representation of
// an underlying token, mirroring WrappedTokenFactory.predictWrappedToken
// on-chain. No RPC calls: the address is well-defined whether or not the
// wrapped token has been deployed yet — the escrow/factory creates it on the
// first fill — so the result must be treated as the canonical address, not
// proof of deployment.

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

// BeaconProxyCreationCodeHex is the OpenZeppelin BeaconProxy creation code
// embedded in the deployed WrappedTokenFactory implementation (solc 0.8.27).
// CREATE2 hashes creation code byte-for-byte, so this must match the factory's
// compile exactly; the contracts repo pins solc and the factory upgrade script
// asserts prediction stability to keep this constant valid across upgrades.
const BeaconProxyCreationCodeHex = "0x60a08060405261045880380380916100178285610271565b833981016040828203126101b45761002e82610294565b602083015190926001600160401b0382116101b4570181601f820112156101b4578051906001600160401b03821161025d5760405192610078601f8401601f191660200185610271565b828452602083830101116101b457815f9260208093018386015e83010152813b1561023c577fa3f0ad74e5423aebfd80d3ef4346578335a9a72aeaee59ff6cb3582b35133d5080546001600160a01b0319166001600160a01b038416908117909155604051635c60da1b60e01b8152909190602081600481865afa9081156101c0575f91610202575b50803b156101e25750817f1cf3b03a6cf19fa2baba4df148e9dcabedea7f8a5c07840e207e5c089be95d3e5f80a28051156101cb57602060049260405193848092635c60da1b60e01b82525afa80156101c0575f90610181575b61016592506102a8565b505b608052604051610123908161033582396080518160180152f35b506020823d6020116101b8575b8161019b60209383610271565b810103126101b4576101af61016592610294565b61015b565b5f80fd5b3d915061018e565b6040513d5f823e3d90fd5b505034156101675763b398979f60e01b5f5260045ffd5b634c9c8ce360e01b5f9081526001600160a01b0391909116600452602490fd5b90506020813d602011610234575b8161021d60209383610271565b810103126101b45761022e90610294565b5f610101565b3d9150610210565b50631933b43b60e21b5f9081526001600160a01b0391909116600452602490fd5b634e487b7160e01b5f52604160045260245ffd5b601f909101601f19168101906001600160401b0382119082101761025d57604052565b51906001600160a01b03821682036101b457565b905f8091602081519101845af48080610321575b156102dc5750506040513d81523d5f602083013e60203d82010160405290565b1561030157639996b31560e01b5f9081526001600160a01b0391909116600452602490fd5b3d15610312576040513d5f823e3d90fd5b63d6bda27560e01b5f5260045ffd5b503d1515806102bc5750813b15156102bc56fe60806040819052635c60da1b60e01b81526020906004817f00000000000000000000000000000000000000000000000000000000000000006001600160a01b03165afa801560a2575f901560d1575060203d602011609c575b601f19601f820116608001906080821067ffffffffffffffff83111760885760849160405260800160ad565b60d1565b634e487b7160e01b5f52604160045260245ffd5b503d6058565b6040513d5f823e3d90fd5b602090607f19011260cd576080516001600160a01b038116810360cd5790565b5f80fd5b5f8091368280378136915af43d5f803e1560e9573d5ff35b3d5ffdfea26469706673582212201f937648d1c071d933d4dca5d97675e09dd5d41ec87bd91d9944da9e287695b064736f6c634300081b0033"

// BeaconProxyCreationCode is BeaconProxyCreationCodeHex decoded to bytes. Do
// not mutate.
var BeaconProxyCreationCode = hexutil.MustDecode(BeaconProxyCreationCodeHex)

// PredictWrappedTokenParams identifies the factory deployment to derive from.
type PredictWrappedTokenParams struct {
	// WrappedTokenFactory is the factory proxy address (the CREATE2 deployer).
	WrappedTokenFactory common.Address
	// WrappedTokenBeacon is the UpgradeableBeacon address baked into the proxy
	// constructor args.
	WrappedTokenBeacon common.Address
	// Underlying is the token whose canonical wrapped representation to derive.
	Underlying common.Address
}

var beaconProxyConstructorArgs = func() abi.Arguments {
	addressType, err := abi.NewType("address", "", nil)
	if err != nil {
		panic(err)
	}
	bytesType, err := abi.NewType("bytes", "", nil)
	if err != nil {
		panic(err)
	}
	return abi.Arguments{{Type: addressType}, {Type: bytesType}}
}()

// PredictWrappedToken derives the canonical wrapped-token address for
// Underlying:
//
//	create2(factory, keccak(abi.encode(underlying)),
//	        BeaconProxy.creationCode ++ abi.encode(beacon, ""))
//
// Deterministic and offline; wrong factory/beacon inputs yield a deterministic
// but incorrect address, so callers must pass the deployment's configured
// addresses (see contracts/README.md in the router repo).
func PredictWrappedToken(params PredictWrappedTokenParams) common.Address {
	salt := crypto.Keccak256Hash(common.LeftPadBytes(params.Underlying.Bytes(), 32))
	constructorArgs, err := beaconProxyConstructorArgs.Pack(params.WrappedTokenBeacon, []byte{})
	if err != nil {
		// Static argument shapes make packing infallible.
		panic(err)
	}
	initCode := make([]byte, 0, len(BeaconProxyCreationCode)+len(constructorArgs))
	initCode = append(initCode, BeaconProxyCreationCode...)
	initCode = append(initCode, constructorArgs...)
	return crypto.CreateAddress2(params.WrappedTokenFactory, salt, crypto.Keccak256(initCode))
}

// PredictWrappedTokenForChain derives the canonical wrapped-token address using
// the configured factory and beacon for chainID. It returns an error when the
// chain has no wrapped-token deployment.
func PredictWrappedTokenForChain(chainID uint64, underlying common.Address) (common.Address, error) {
	deployments, ok := GetChainDeployments(chainID)
	if !ok ||
		deployments.ArcusWrappedTokenFactory == (common.Address{}) ||
		deployments.ArcusWrappedTokenBeacon == (common.Address{}) {
		return common.Address{}, fmt.Errorf("arcusspot: chain %d has no wrapped-token factory/beacon configured", chainID)
	}
	return PredictWrappedToken(PredictWrappedTokenParams{
		WrappedTokenFactory: deployments.ArcusWrappedTokenFactory,
		WrappedTokenBeacon:  deployments.ArcusWrappedTokenBeacon,
		Underlying:          underlying,
	}), nil
}
