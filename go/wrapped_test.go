package arcusspot

import (
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

// Vectors mirror test/wrapped.test.ts: computed independently with
// `cast create2` against the deployed WrappedTokenFactory/beacon addresses.
func TestPredictWrappedToken(t *testing.T) {
	t.Run("matches deployed wWEEK for WEEK on Robinhood Mainnet (4663)", func(t *testing.T) {
		got := PredictWrappedToken(PredictWrappedTokenParams{
			WrappedTokenFactory: common.HexToAddress("0x8bc71aE8EaC8B25F30c2990930Cc3A80E72e169e"),
			WrappedTokenBeacon:  common.HexToAddress("0x27fEB332759F8d2f351D7fC72D29af37664ffd77"),
			Underlying:          common.HexToAddress("0xc93a8c440CEa26D7445dF01729f193b27965099f"),
		})
		want := common.HexToAddress("0x4B17e556568bB02709a50cA67db7F4DBD46E3d17")
		if got != want {
			t.Fatalf("got %s, want %s", got, want)
		}
	})

	t.Run("matches cast create2 on Robinhood Testnet (46630)", func(t *testing.T) {
		got := PredictWrappedToken(PredictWrappedTokenParams{
			WrappedTokenFactory: common.HexToAddress("0xF361dD4cd631175648f54B657d44A3128903D92c"),
			WrappedTokenBeacon:  common.HexToAddress("0xDd70cD7BbE53ac3d67c1959C9De84423f5Bf2bca"),
			Underlying:          common.HexToAddress("0x1111111111111111111111111111111111111111"),
		})
		want := common.HexToAddress("0xEFEbf34C80F78d85C085908EB30eb0C6b873d47A")
		if got != want {
			t.Fatalf("got %s, want %s", got, want)
		}
	})
}

func TestPredictWrappedTokenForChain(t *testing.T) {
	t.Run("matches deployed wWEEK for WEEK on Robinhood Mainnet (4663)", func(t *testing.T) {
		got, err := PredictWrappedTokenForChain(
			RobinhoodMainnetChainID,
			common.HexToAddress("0xc93a8c440CEa26D7445dF01729f193b27965099f"),
		)
		if err != nil {
			t.Fatal(err)
		}
		want := common.HexToAddress("0x4B17e556568bB02709a50cA67db7F4DBD46E3d17")
		if got != want {
			t.Fatalf("got %s, want %s", got, want)
		}
	})

	t.Run("matches cast create2 on Robinhood Testnet (46630)", func(t *testing.T) {
		got, err := PredictWrappedTokenForChain(
			RobinhoodTestnetChainID,
			common.HexToAddress("0x1111111111111111111111111111111111111111"),
		)
		if err != nil {
			t.Fatal(err)
		}
		want := common.HexToAddress("0xEFEbf34C80F78d85C085908EB30eb0C6b873d47A")
		if got != want {
			t.Fatalf("got %s, want %s", got, want)
		}
	})

	t.Run("errors when the chain has no wrapped-token deployment", func(t *testing.T) {
		_, err := PredictWrappedTokenForChain(
			ArbitrumChainID,
			common.HexToAddress("0x82af49447d8a07e3bd95bd0d56f35241523fbab1"),
		)
		if err == nil || !strings.Contains(err.Error(), "no wrapped-token factory/beacon") {
			t.Fatalf("expected wrapped-token deployment error, got %v", err)
		}
	})
}
