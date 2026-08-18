package arcusspot

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// Mirrors test/shell.test.ts: decode quoted amount fields from SwapExecuted logs.
func TestDecodeSwapExecutedLogs(t *testing.T) {
	shell := common.HexToAddress("0x0000000000000000000000000000000000000001")
	taker := common.HexToAddress("0x0000000000000000000000000000000000000002")
	tokenIn := common.HexToAddress("0x0000000000000000000000000000000000000003")
	tokenOut := common.HexToAddress("0x0000000000000000000000000000000000000004")
	router := common.HexToAddress("0x0000000000000000000000000000000000000005")

	data, err := SwapShellABI.Events["SwapExecuted"].Inputs.NonIndexed().Pack(
		big.NewInt(98),  // minAmountOut
		big.NewInt(100), // amountIn
		big.NewInt(100), // quotedAmountIn
		big.NewInt(123), // quotedAmountOut
		big.NewInt(120), // amountOut
		big.NewInt(1),   // tokenInBenchmarkPrice
		big.NewInt(2),   // tokenOutBenchmarkPrice
		router,
		[32]byte{},
		true,
		"",
	)
	if err != nil {
		t.Fatal(err)
	}

	decoded := DecodeSwapExecutedLogs([]types.Log{
		{
			Address: shell,
			Data:    data,
			Topics: []common.Hash{
				SwapExecutedEventID,
				addressTopic(taker),
				addressTopic(tokenIn),
				addressTopic(tokenOut),
			},
		},
		// Unrelated log shapes are ignored, mirroring the TS decoder.
		{Address: shell, Topics: []common.Hash{{}}},
	})

	if len(decoded) != 1 {
		t.Fatalf("expected 1 decoded log, got %d", len(decoded))
	}
	args := decoded[0].Args
	if args.Taker != taker {
		t.Errorf("taker: got %s, want %s", args.Taker, taker)
	}
	if args.QuotedAmountIn.Cmp(big.NewInt(100)) != 0 {
		t.Errorf("quotedAmountIn: got %s, want 100", args.QuotedAmountIn)
	}
	if args.QuotedAmountOut.Cmp(big.NewInt(123)) != 0 {
		t.Errorf("quotedAmountOut: got %s, want 123", args.QuotedAmountOut)
	}
	if args.AmountOut.Cmp(big.NewInt(120)) != 0 {
		t.Errorf("amountOut: got %s, want 120", args.AmountOut)
	}
	if args.Router != router {
		t.Errorf("router: got %s, want %s", args.Router, router)
	}
	if !args.Success {
		t.Error("success: got false, want true")
	}
}
