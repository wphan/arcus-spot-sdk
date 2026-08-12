//go:build live

// Package e2e holds live end-to-end tests against the hosted Robinhood mainnet
// router. They spend real funds and are therefore excluded from normal builds
// behind the "live" tag. Run them explicitly:
//
//	PRIVATE_KEY=... go test -tags live -v -run TestLiveReadOnly ./e2e
//	PRIVATE_KEY=... go test -tags live -v -run TestLiveTrade ./e2e
//
// PRIVATE_KEY is the taker key (hex, no 0x needed). Optional overrides:
//
//	LIVE_ROUTER_URL  (default https://router.spot.arcus.xyz/v1)
//	LIVE_RPC_URL     (default https://rpc.chain.robinhood.com)
//	LIVE_SELL_SYMBOL (default USDG)
//	LIVE_BUY_SYMBOL  (default AAPL)
//	LIVE_SELL_USD    (default 1, whole tokens of the sell asset per venue)
package e2e

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	arcusspot "github.com/arcus-xyz/arcus-spot-sdk/go"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

type liveEnv struct {
	client *arcusspot.SpotRouterClient
	rpc    *ethclient.Client
	signer *arcusspot.PrivateKeySigner
	key    *ecdsa.PrivateKey

	sellToken arcusspot.TokenInfo
	buyToken  arcusspot.TokenInfo
	// sellAmount is atoms of sellToken traded per venue.
	sellAmount *big.Int
}

func setupLive(t *testing.T) *liveEnv {
	t.Helper()
	ctx := context.Background()

	keyHex := strings.TrimPrefix(os.Getenv("PRIVATE_KEY"), "0x")
	if keyHex == "" {
		t.Fatal("PRIVATE_KEY is required (source arcus-spot-sdk/.env)")
	}
	key, err := crypto.HexToECDSA(keyHex)
	if err != nil {
		t.Fatalf("parse PRIVATE_KEY: %v", err)
	}
	signer := arcusspot.NewPrivateKeySigner(key)
	t.Logf("taker: %s", signer.Address())

	client, err := arcusspot.NewSpotRouterClient(arcusspot.ClientOptions{
		BaseURL: envOr("LIVE_ROUTER_URL", "https://router.spot.arcus.xyz/v1"),
		Timeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	rpc, err := ethclient.Dial(envOr("LIVE_RPC_URL", "https://rpc.chain.robinhood.com"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(rpc.Close)

	health, err := client.Health(ctx)
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	t.Logf("health: ok=%v chainId=%d", health.OK, health.ChainID)
	if !health.OK || health.ChainID != arcusspot.RobinhoodMainnetChainID {
		t.Fatalf("router is not healthy mainnet: %+v", health)
	}

	tokens, err := client.GetTokenList(ctx)
	if err != nil {
		t.Fatalf("token list: %v", err)
	}
	t.Logf("token list: %d tokens", len(tokens))

	sellSymbol := envOr("LIVE_SELL_SYMBOL", "USDG")
	buySymbol := envOr("LIVE_BUY_SYMBOL", "AAPL")
	env := &liveEnv{client: client, rpc: rpc, signer: signer, key: key}
	for _, token := range tokens {
		if strings.EqualFold(token.Symbol, sellSymbol) {
			env.sellToken = token
		}
		if strings.EqualFold(token.Symbol, buySymbol) {
			env.buyToken = token
		}
	}
	if env.sellToken.Symbol == "" || env.buyToken.Symbol == "" {
		symbols := make([]string, 0, len(tokens))
		for _, token := range tokens {
			symbols = append(symbols, token.Symbol)
		}
		t.Fatalf("pair %s→%s not in token list; available: %s", sellSymbol, buySymbol, strings.Join(symbols, ", "))
	}
	t.Logf("pair: %s (%s, %d decimals) → %s (%s, %d decimals)",
		env.sellToken.Symbol, env.sellToken.Address, env.sellToken.Decimals,
		env.buyToken.Symbol, env.buyToken.Address, env.buyToken.Decimals)

	whole, ok := new(big.Int).SetString(envOr("LIVE_SELL_USD", "1"), 10)
	if !ok {
		t.Fatal("invalid LIVE_SELL_USD")
	}
	env.sellAmount = new(big.Int).Mul(whole, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(env.sellToken.Decimals)), nil))
	t.Logf("sell amount per venue: %s atoms", env.sellAmount)

	sellBalance := env.erc20Balance(t, env.sellToken.Address)
	nativeBalance, err := rpc.BalanceAt(ctx, signer.Address(), nil)
	if err != nil {
		t.Fatalf("native balance: %v", err)
	}
	t.Logf("balances: %s %s atoms, native %s wei", env.sellToken.Symbol, sellBalance, nativeBalance)
	if sellBalance.Cmp(env.sellAmount) < 0 {
		t.Fatalf("wallet holds %s %s atoms, need at least %s per venue", sellBalance, env.sellToken.Symbol, env.sellAmount)
	}
	return env
}

func (e *liveEnv) erc20Balance(t *testing.T, token common.Address) *big.Int {
	t.Helper()
	selector := crypto.Keccak256([]byte("balanceOf(address)"))[:4]
	data := append(selector, common.LeftPadBytes(e.signer.Address().Bytes(), 32)...)
	out, err := e.rpc.CallContract(context.Background(), ethereum.CallMsg{To: &token, Data: data}, nil)
	if err != nil {
		t.Fatalf("balanceOf(%s): %v", token, err)
	}
	return new(big.Int).SetBytes(out)
}

func (e *liveEnv) permit2Allowance(t *testing.T, token common.Address) *big.Int {
	t.Helper()
	input, err := arcusspot.ERC20PermitABI.Pack("allowance", e.signer.Address(), arcusspot.Permit2Address)
	if err != nil {
		t.Fatal(err)
	}
	out, err := e.rpc.CallContract(context.Background(), ethereum.CallMsg{To: &token, Data: input}, nil)
	if err != nil {
		t.Fatalf("allowance(%s): %v", token, err)
	}
	return new(big.Int).SetBytes(out)
}

// sendApprove submits an on-chain approve(spender, amount) from the taker and
// waits for the receipt.
func (e *liveEnv) sendApprove(t *testing.T, token, spender common.Address, amount *big.Int) {
	t.Helper()
	ctx := context.Background()

	input, err := arcusspot.ERC20PermitABI.Pack("approve", spender, amount)
	if err != nil {
		t.Fatal(err)
	}
	chainID, err := e.rpc.ChainID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	nonce, err := e.rpc.PendingNonceAt(ctx, e.signer.Address())
	if err != nil {
		t.Fatal(err)
	}
	gasPrice, err := e.rpc.SuggestGasPrice(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Headroom for base-fee drift between suggestion and inclusion.
	gasPrice = new(big.Int).Mul(gasPrice, big.NewInt(2))
	gasLimit, err := e.rpc.EstimateGas(ctx, ethereum.CallMsg{
		From: e.signer.Address(),
		To:   &token,
		Data: input,
	})
	if err != nil {
		t.Fatalf("estimate approve gas: %v", err)
	}
	tx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		To:       &token,
		Gas:      gasLimit + gasLimit/4,
		GasPrice: gasPrice,
		Data:     input,
	})
	signedTx, err := types.SignTx(tx, types.LatestSignerForChainID(chainID), e.key)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.rpc.SendTransaction(ctx, signedTx); err != nil {
		t.Fatalf("send approve: %v", err)
	}
	t.Logf("approve(%s, %s) sent: %s", spender, amount, signedTx.Hash())

	deadline := time.Now().Add(2 * time.Minute)
	for {
		receipt, err := e.rpc.TransactionReceipt(ctx, signedTx.Hash())
		if err == nil {
			if receipt.Status != types.ReceiptStatusSuccessful {
				t.Fatalf("approve tx %s reverted", signedTx.Hash())
			}
			t.Logf("approve mined in block %d", receipt.BlockNumber)
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("approve tx %s not mined within 2m", signedTx.Hash())
		}
		time.Sleep(2 * time.Second)
	}
}

func (e *liveEnv) quoteRequest() arcusspot.QuoteRequest {
	slippage := 100
	return arcusspot.QuoteRequest{
		PriceRequest: arcusspot.PriceRequest{
			ChainID:    arcusspot.RobinhoodMainnetChainID,
			SellToken:  e.sellToken.Address.Hex(),
			BuyToken:   e.buyToken.Address.Hex(),
			SellAmount: e.sellAmount.String(),
		},
		Taker:        e.signer.Address(),
		SlippageBps:  &slippage,
		AllowWrapped: true,
	}
}

// TestLiveReadOnly exercises every read path (health, tokens, price, quote,
// permit probing, wrapped prediction, trade history) without signing or
// submitting anything.
func TestLiveReadOnly(t *testing.T) {
	env := setupLive(t)
	ctx := context.Background()

	price, err := env.client.GetPrice(ctx, env.quoteRequest().PriceRequest)
	if err != nil {
		t.Fatalf("price: %v", err)
	}
	t.Logf("price: recommended=%s venues=%d errors=%d", price.Recommended, len(price.All), len(price.Errors))
	for _, entry := range price.All {
		t.Logf("  price %s: sell %s → buy %s", entry.Venue, entry.SellAmount, entry.BuyAmount)
	}
	for _, entry := range price.Errors {
		t.Logf("  price error %s: %s (%s %d)", entry.Venue, entry.Error.Message, entry.Error.Kind, entry.Error.Status)
	}

	quotes, err := env.client.GetQuote(ctx, env.quoteRequest())
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	t.Logf("quote: recommended=%s venues=%d errors=%d", quotes.Recommended, len(quotes.All), len(quotes.Errors))
	for _, quote := range quotes.All {
		typed := quote.TradeTypedData()
		t.Logf("  quote %s: primaryType=%s domain=%s chainId=%s",
			quote.FirmQuoteVenue(), typed.PrimaryType, typed.Domain.Name, typed.Domain.ChainID.Int())
		digest, err := arcusspot.HashTypedData(*typed)
		if err != nil {
			t.Errorf("  hash typed data (%s): %v", quote.FirmQuoteVenue(), err)
		} else {
			t.Logf("  eip712 digest: %s", digest)
		}
		tasks := arcusspot.GetQuoteSigningTasks(quote)
		if len(tasks) != 1 || tasks[0].Kind != "trade" {
			t.Errorf("  unexpected signing tasks: %+v", tasks)
		}
	}
	for _, entry := range quotes.Errors {
		t.Logf("  quote error %s: %s (%s %d)", entry.Venue, entry.Error.Message, entry.Error.Kind, entry.Error.Status)
	}

	// Wrapped-token prediction against the router's own token list.
	predicted, err := arcusspot.PredictWrappedTokenForChain(arcusspot.RobinhoodMainnetChainID, env.buyToken.Address)
	if err != nil {
		t.Fatalf("predict wrapped: %v", err)
	}
	t.Logf("predicted wrapped %s: %s (token list says %v)", env.buyToken.Symbol, predicted, env.buyToken.WrappedTokenAddress)
	if env.buyToken.WrappedTokenAddress != nil && *env.buyToken.WrappedTokenAddress != predicted {
		t.Errorf("wrapped prediction mismatch: predicted %s, token list %s", predicted, env.buyToken.WrappedTokenAddress)
	}

	env.logTradeHistory(t, 5000)
}

// TestLiveTrade signs and submits one real swap per venue the router returns,
// then polls status to confirmation and verifies the SwapExecuted log appears
// in trade history. Spends LIVE_SELL_USD of the sell token per venue.
func TestLiveTrade(t *testing.T) {
	env := setupLive(t)
	ctx := context.Background()

	quotes, err := env.client.GetQuote(ctx, env.quoteRequest())
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	if len(quotes.All) == 0 {
		t.Fatalf("router returned no firm quotes; errors: %+v", quotes.Errors)
	}
	for _, entry := range quotes.Errors {
		t.Logf("quote error %s: %s", entry.Venue, entry.Error.Message)
	}

	startBlock, err := env.rpc.BlockNumber(ctx)
	if err != nil {
		t.Fatalf("block number: %v", err)
	}

	venuesTraded := make([]string, 0, len(quotes.All))
	for _, quote := range quotes.All {
		venue := quote.FirmQuoteVenue()
		t.Run(string(venue), func(t *testing.T) {
			// Re-fetch a fresh quote per venue: firm quotes expire quickly and
			// earlier submits in this loop consume time and allowance.
			fresh := env.freshQuoteForVenue(t, venue)
			txHash := env.tradeQuote(t, fresh)
			env.awaitConfirmed(t, venue, txHash)
			venuesTraded = append(venuesTraded, string(venue))
		})
	}
	t.Logf("venues traded: %s", strings.Join(venuesTraded, ", "))

	// Trade history should now contain the swaps we just executed.
	endBlock, err := env.rpc.BlockNumber(ctx)
	if err != nil {
		t.Fatalf("block number: %v", err)
	}
	history, err := arcusspot.GetSwapShellTradeHistory(ctx, arcusspot.SwapShellTradeHistoryRequest{
		Client:    env.rpc,
		ChainID:   arcusspot.RobinhoodMainnetChainID,
		Taker:     env.signer.Address(),
		FromBlock: new(big.Int).SetUint64(startBlock),
		ToBlock:   new(big.Int).SetUint64(endBlock),
	})
	if err != nil {
		t.Fatalf("trade history: %v", err)
	}
	t.Logf("trade history since block %d: %d SwapExecuted logs", startBlock, len(history))
	for _, entry := range history {
		t.Logf("  block %d tx %s: %s in → %s out (router %s, tag %s, success=%v)",
			entry.Log.BlockNumber, entry.Log.TxHash, entry.Args.AmountIn, entry.Args.AmountOut,
			entry.Args.Router, strings.TrimRight(string(entry.Args.RouteTag[:]), "\x00"), entry.Args.Success)
	}
	if len(history) < len(venuesTraded) {
		t.Errorf("expected at least %d SwapExecuted logs, found %d", len(venuesTraded), len(history))
	}
}

func (e *liveEnv) freshQuoteForVenue(t *testing.T, venue arcusspot.Venue) arcusspot.FirmQuote {
	t.Helper()
	quotes, err := e.client.GetQuote(context.Background(), e.quoteRequest())
	if err != nil {
		t.Fatalf("re-quote for %s: %v", venue, err)
	}
	for _, quote := range quotes.All {
		if quote.FirmQuoteVenue() == venue {
			return quote
		}
	}
	t.Skipf("venue %s no longer quoting on re-fetch", venue)
	return nil
}

// tradeQuote builds the venue-appropriate permit if needed, signs, and submits.
func (e *liveEnv) tradeQuote(t *testing.T, quote arcusspot.FirmQuote) common.Hash {
	t.Helper()
	ctx := context.Background()

	permitOptions := arcusspot.BuildPermitOptions{
		Caller: e.rpc,
		Signer: e.signer,
		Owner:  e.signer.Address(),
	}
	var permit *arcusspot.Permit
	var err error
	switch q := quote.(type) {
	case *arcusspot.ArcusFirmQuote:
		permit, err = arcusspot.BuildArcusSellTokenPermitIfNeeded(ctx, q, permitOptions)
	case *arcusspot.RialtoFirmQuote:
		permit, err = arcusspot.BuildRialtoSellTokenPermitIfNeeded(ctx, q, permitOptions)
	case *arcusspot.LifiFirmQuote:
		permit, err = arcusspot.BuildLifiSellTokenPermitIfNeeded(ctx, q, permitOptions)
	default:
		t.Fatalf("unexpected quote type %T", quote)
	}
	var unsupported *arcusspot.PermitUnsupportedError
	if errors.As(err, &unsupported) {
		t.Fatalf("sell token is not EIP-2612 and Permit2 allowance is missing (%v); send the approve tx manually and re-run", unsupported)
	}
	if err != nil {
		t.Fatalf("build permit: %v", err)
	}
	options := &arcusspot.SignQuoteOptions{Taker: e.signer.Address()}
	if permit != nil {
		t.Logf("attaching EIP-2612 permit for %s (deadline %s)", permit.Token, permit.Deadline)
		options.Permits = []arcusspot.Permit{*permit}
	} else {
		t.Log("permit not needed (allowance already covers sellAmount)")
	}

	signed, err := arcusspot.SignQuote(quote, e.signer, options)
	if err != nil {
		t.Fatalf("sign quote: %v", err)
	}

	submitted, err := e.client.SubmitSignedQuote(ctx, signed)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	wrapped := "<nil>"
	if submitted.Wrapped != nil {
		wrapped = fmt.Sprintf("%v", *submitted.Wrapped)
	}
	t.Logf("submitted %s: tx=%s status=%s maker=%v wrapped=%s settledToken=%v orderId=%s",
		submitted.Venue, submitted.TxHash, submitted.Status, submitted.Maker, wrapped,
		submitted.SettledToken, submitted.OrderID)
	if submitted.Venue != quote.FirmQuoteVenue() || submitted.Status != "submitted" {
		t.Fatalf("unexpected submit response: %+v", submitted)
	}
	return submitted.TxHash
}

func (e *liveEnv) awaitConfirmed(t *testing.T, venue arcusspot.Venue, txHash common.Hash) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Minute)
	for {
		status, err := e.client.GetStatus(context.Background(), arcusspot.StatusRequest{
			Venue:   venue,
			ID:      txHash.Hex(),
			ChainID: arcusspot.RobinhoodMainnetChainID,
		})
		if err != nil {
			t.Logf("status poll error (retrying): %v", err)
		} else {
			t.Logf("status %s %s: %s (raw %s, tx %s)", venue, txHash, status.Status, status.RawStatus, status.TxHash)
			switch status.Status {
			case arcusspot.StatusConfirmed:
				return
			case arcusspot.StatusFailed:
				t.Fatalf("%s trade %s failed: raw=%s", venue, txHash, status.RawStatus)
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s trade %s not confirmed within 2m", venue, txHash)
		}
		time.Sleep(3 * time.Second)
	}
}

// TestLivePermitTrade exercises the live EIP-2612 permit path end to end:
// revoke the sellToken→Permit2 allowance with an on-chain approve(0), verify
// the builder now produces a permit (live allowance/nonces/name/version reads +
// EIP-712 signing), trade with permits[] attached, and confirm the router
// applied the permit by checking the allowance is unlimited afterwards.
func TestLivePermitTrade(t *testing.T) {
	env := setupLive(t)

	before := env.permit2Allowance(t, env.sellToken.Address)
	t.Logf("current %s→Permit2 allowance: %s", env.sellToken.Symbol, before)

	if before.Sign() != 0 {
		env.sendApprove(t, env.sellToken.Address, arcusspot.Permit2Address, big.NewInt(0))
	}

	quote := env.freshQuoteForVenue(t, arcusspot.VenueArcus)
	arcusQuote, ok := quote.(*arcusspot.ArcusFirmQuote)
	if !ok {
		t.Fatalf("expected arcus quote, got %T", quote)
	}
	permit, err := arcusspot.BuildArcusSellTokenPermitIfNeeded(context.Background(), arcusQuote, arcusspot.BuildPermitOptions{
		Caller: env.rpc,
		Signer: env.signer,
	})
	if err != nil {
		t.Fatalf("build permit after revoke: %v", err)
	}
	if permit == nil {
		t.Fatal("expected a permit after revoking allowance, got nil")
	}
	t.Logf("built permit: token=%s value=%s deadline=%s v=%d", permit.Token, permit.Value, permit.Deadline, permit.V)

	signed, err := arcusspot.SignQuote(quote, env.signer, &arcusspot.SignQuoteOptions{
		Permits: []arcusspot.Permit{*permit},
	})
	if err != nil {
		t.Fatalf("sign quote: %v", err)
	}
	submitted, err := env.client.SubmitSignedQuote(context.Background(), signed)
	if err != nil {
		t.Fatalf("submit with permit: %v", err)
	}
	t.Logf("submitted arcus with permit: tx=%s", submitted.TxHash)
	env.awaitConfirmed(t, arcusspot.VenueArcus, submitted.TxHash)

	after := env.permit2Allowance(t, env.sellToken.Address)
	t.Logf("post-trade %s→Permit2 allowance: %s", env.sellToken.Symbol, after)
	// The permit granted MaxUint256; Permit2 spent sellAmount from it.
	expected := new(big.Int).Sub(arcusspot.MaxUint256, env.sellAmount)
	if after.Cmp(expected) != 0 && after.Cmp(arcusspot.MaxUint256) != 0 {
		t.Errorf("allowance not restored by permit: got %s", after)
	}
}

// TestLiveLifi waits for the lifi venue to quote (its upstream rate-limits and
// drops out intermittently) and trades it once it appears.
func TestLiveLifi(t *testing.T) {
	env := setupLive(t)

	var quote arcusspot.FirmQuote
	for attempt := 1; attempt <= 10; attempt++ {
		quotes, err := env.client.GetQuote(context.Background(), env.quoteRequest())
		if err != nil {
			t.Fatalf("quote: %v", err)
		}
		for _, candidate := range quotes.All {
			if candidate.FirmQuoteVenue() == arcusspot.VenueLifi {
				quote = candidate
				break
			}
		}
		if quote != nil {
			break
		}
		for _, entry := range quotes.Errors {
			if entry.Venue == arcusspot.VenueLifi {
				t.Logf("attempt %d: lifi error: %s", attempt, entry.Error.Message)
			}
		}
		time.Sleep(15 * time.Second)
	}
	if quote == nil {
		t.Skip("lifi venue did not quote within ~2.5m (upstream unavailable); rerun later")
	}

	txHash := env.tradeQuote(t, quote)
	env.awaitConfirmed(t, arcusspot.VenueLifi, txHash)
}

func (e *liveEnv) logTradeHistory(t *testing.T, lookback uint64) {
	t.Helper()
	ctx := context.Background()
	head, err := e.rpc.BlockNumber(ctx)
	if err != nil {
		t.Fatalf("block number: %v", err)
	}
	from := uint64(0)
	if head > lookback {
		from = head - lookback
	}
	history, err := arcusspot.GetSwapShellTradeHistory(ctx, arcusspot.SwapShellTradeHistoryRequest{
		Client:    e.rpc,
		ChainID:   arcusspot.RobinhoodMainnetChainID,
		Taker:     e.signer.Address(),
		FromBlock: new(big.Int).SetUint64(from),
		ToBlock:   new(big.Int).SetUint64(head),
	})
	if err != nil {
		t.Fatalf("trade history: %v", err)
	}
	t.Logf("trade history blocks %d–%d: %d SwapExecuted logs for taker", from, head, len(history))
	for _, entry := range history {
		fmt.Printf("  block %d tx %s: in %s out %s tag %s success=%v\n",
			entry.Log.BlockNumber, entry.Log.TxHash, entry.Args.AmountIn, entry.Args.AmountOut,
			strings.TrimRight(string(entry.Args.RouteTag[:]), "\x00"), entry.Args.Success)
	}
}
