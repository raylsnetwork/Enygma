package enygma_test

// HTTP-level tests for the relayer's fee-slot check (Config.VerifyFeeSlot).
//
// The USDr circuit leaves the fee's recipient and split to the prover, so the
// relayer must verify that its own slot's commitment opens to the fixed fee
// before it pays gas. These drive POST /relay/transfer end to end against the
// mock contract and assert the HTTP outcome; the check's own edge cases live in
// relayer/server/fee_slot_test.go.
//
// Run:
//
//	cd enygma_payments/test && go test -run TestRelayHandler_FeeSlot -v

import (
	"math/big"
	"net/http"
	"testing"

	"enygma_payments/relayer/config"
	"enygma_payments/relayer/server"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/iden3/go-iden3-crypto/babyjub"
)

// Same generators as Enygma.sol / gnark-server/utils.
var (
	testG = &babyjub.Point{
		X: bigFromDecimal("12337812418750581066638756637363471856433191340622504180842886595232027947307"),
		Y: bigFromDecimal("15225366398330386329633463986700597127113326976080712967801565482915963669722"),
	}
	testH = &babyjub.Point{
		X: bigFromDecimal("10100005861917718053548237064487763771145251762383025193119768015180892676690"),
		Y: bigFromDecimal("7512830269827713629724023825249861327768672768516116945507944076335453576011"),
	}
)

func bigFromDecimal(s string) *big.Int {
	n, ok := new(big.Int).SetString(s, 10)
	if !ok {
		panic(s)
	}
	return n
}

func testPedersen(v, r *big.Int) *babyjub.Point {
	vG := babyjub.NewPoint().Mul(v, testG)
	rH := babyjub.NewPoint().Mul(r, testH)
	return babyjub.NewPoint().Projective().Add(vG.Projective(), rH.Projective()).Affine()
}

func newFeeCheckingHandler(c *mockContract, m *mockMiner) *server.Handler {
	privKey, _ := crypto.HexToECDSA(hardhatKey0)
	auth, _ := bind.NewKeyedTransactorWithChainID(privKey, big.NewInt(1337))
	cfg := &config.Config{
		APIKeys:       testAPIKeys,
		ChainID:       big.NewInt(1337),
		GasLimit:      300_000_000,
		VerifyFeeSlot: true,
	}
	return server.NewHandlerWithDeps(cfg, "0x1234567890123456789012345678901234567890", auth, m, c)
}

// paidTransferBody returns a request whose USDr leg pays the relayer (account
// 6, slot 5) `fee` with blinding r, and nobody else.
func paidTransferBody(fee, r *big.Int) server.RelayTransferRequest {
	body := validTransferBody()
	const slot = 5
	body.UsdrPublicSignal[80] = fee.String()
	for i := 0; i < 6; i++ {
		pt := testPedersen(big.NewInt(0), big.NewInt(int64(2000+i)))
		if i == slot {
			pt = testPedersen(fee, r)
		}
		body.UsdrPublicSignal[54+2*i] = pt.X.String()
		body.UsdrPublicSignal[54+2*i+1] = pt.Y.String()
		body.UsdrCommitments[i] = []string{pt.X.String(), pt.Y.String()}
	}
	body.UsdrFeeRandomness = r.String()
	return body
}

func TestRelayHandler_FeeSlot_PaidRelayerIsRelayed(t *testing.T) {
	tx := dummyTx()
	mc := &mockContract{tx: tx, usdrFee: big.NewInt(10), accountID: big.NewInt(6)}
	r := server.NewWithHandler(testAPIKeys, newFeeCheckingHandler(mc, &mockMiner{receipt: successReceipt(tx)}))
	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, paidTransferBody(big.NewInt(10), big.NewInt(424242)))
	if w.Code != http.StatusOK {
		t.Fatalf("a transfer that pays the relayer was rejected: %d %s", w.Code, w.Body.String())
	}
}

func TestRelayHandler_FeeSlot_UnpaidRelayerGets402AndNothingIsSubmitted(t *testing.T) {
	tx := dummyTx()
	mc := &mockContract{tx: tx, usdrFee: big.NewInt(10), accountID: big.NewInt(6)}
	r := server.NewWithHandler(testAPIKeys, newFeeCheckingHandler(mc, &mockMiner{receipt: successReceipt(tx)}))

	body := paidTransferBody(big.NewInt(10), big.NewInt(424242))
	body.UsdrFeeRandomness = "777" // does not open the relayer's commitment
	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, body)
	if w.Code != http.StatusPaymentRequired {
		t.Fatalf("an unpaid relayer got %d, want 402: %s", w.Code, w.Body.String())
	}
	if mc.gotBankTag != "" {
		t.Fatal("the relayer submitted a transaction for a transfer that did not pay it")
	}
}

func TestRelayHandler_FeeSlot_MissingRandomnessIsRejected(t *testing.T) {
	tx := dummyTx()
	mc := &mockContract{tx: tx, usdrFee: big.NewInt(10), accountID: big.NewInt(6)}
	r := server.NewWithHandler(testAPIKeys, newFeeCheckingHandler(mc, &mockMiner{receipt: successReceipt(tx)}))

	body := paidTransferBody(big.NewInt(10), big.NewInt(424242))
	body.UsdrFeeRandomness = ""
	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, body)
	if w.Code != http.StatusPaymentRequired {
		t.Fatalf("a request without the fee opening got %d, want 402: %s", w.Code, w.Body.String())
	}
}

func TestRelayHandler_FeeSlot_VerificationCanBeDisabled(t *testing.T) {
	tx := dummyTx()
	mc := &mockContract{tx: tx}
	// newTestHandler leaves VerifyFeeSlot at its zero value (off), which is
	// what every pre-existing handler test relies on.
	r := server.NewWithHandler(testAPIKeys, newTestHandler(mc, &mockMiner{receipt: successReceipt(tx)}))
	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, validTransferBody())
	if w.Code != http.StatusOK {
		t.Fatalf("with verification off the request was rejected: %d %s", w.Code, w.Body.String())
	}
}
