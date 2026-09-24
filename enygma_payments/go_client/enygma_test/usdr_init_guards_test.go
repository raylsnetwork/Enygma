package enygma_test

// initializeUsdrBalance guards. The function seeds an account's USDr balance
// with a caller-precomputed commitment and adds it to usdrTotalSupply, so a
// misordered call corrupts the supply invariant permanently (there is no USDr
// burn). It must therefore refuse to run:
//
//   - before initialize()      (initialize() resets usdrTotalSupply to the
//                               neutral element, discarding the addition)
//   - for an unregistered id   (the commitment would sit outside the set
//                               checkUsdr() sums over)
//   - over a non-empty balance (e.g. after mintUsdrSupply, which would
//                               otherwise be overwritten and lost)
//
// Run:
//   ENYGMA_CHAIN_URL=http://127.0.0.1:8545 ENYGMA_CHAIN_ID=1337 \
//   MY_KEY=<hardhat key> go test -run TestInitializeUsdrBalanceGuards -v .

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"strings"
	"testing"

	enygma "enygma_payments/go_client/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

func TestInitializeUsdrBalanceGuards(t *testing.T) {
	if !chainAvailable() {
		t.Skipf("chain not reachable at %s — set ENYGMA_CHAIN_URL / ENYGMA_CHAIN_ID for local Hardhat", chainURL)
	}
	ctx := context.Background()
	client, err := ethclient.Dial(chainURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	ownerKey := mustPrivKey(t)
	ownerAddr := crypto.PubkeyToAddress(*ownerKey.Public().(*ecdsa.PublicKey))
	auth := func() *bind.TransactOpts {
		nonce, _ := client.PendingNonceAt(ctx, ownerAddr)
		gasPrice, _ := client.SuggestGasPrice(ctx)
		a, _ := bind.NewKeyedTransactorWithChainID(ownerKey, big.NewInt(chainID))
		a.Nonce = big.NewInt(int64(nonce))
		a.Value = big.NewInt(0)
		a.GasLimit = 16_000_000
		a.GasPrice = gasPrice
		return a
	}
	const artifactBase = "../../contracts/enygma/artifacts/contracts"
	enygmaAddr := deployFromArtifact(t, client, auth(), artifactBase+"/Enygma.sol/Enygma.json", big.NewInt(30))
	instance, err := enygma.NewEnygma(enygmaAddr, client)
	if err != nil {
		t.Fatalf("bind: %v", err)
	}

	wantRevert := func(what, errName string, sendErr error) {
		t.Helper()
		if sendErr == nil {
			t.Errorf("%s: expected revert %s, but the call succeeded", what, errName)
			return
		}
		if !strings.Contains(sendErr.Error(), errName) {
			t.Errorf("%s: expected %s, got: %v", what, errName, sendErr)
			return
		}
		t.Logf("%s: correctly reverted with %s", what, errName)
	}

	cx, cy := regCommit(big.NewInt(usdrPrevR))

	// 1. Before initialize().
	_, sendErr := instance.InitializeUsdrBalance(auth(), big.NewInt(1), cx, cy)
	wantRevert("before initialize()", "NotInitialized", sendErr)

	tx, err := instance.Initialize(auth())
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if _, err := bind.WaitMined(ctx, client, tx); err != nil {
		t.Fatalf("wait initialize: %v", err)
	}

	// 2. Unregistered account.
	_, sendErr = instance.InitializeUsdrBalance(auth(), big.NewInt(1), cx, cy)
	wantRevert("unregistered account", "UnregisteredParticipant", sendErr)

	// Register accounts 1 and 2.
	for id := int64(1); id <= 2; id++ {
		pk, _ := poseidon.Hash([]*big.Int{big.NewInt(2000 + id), big.NewInt(2000 + id)})
		pk.Mod(pk, curveP)
		rx, ry := regCommit(big.NewInt(senderPrevR))
		var addr common.Address
		addr[19] = byte(id)
		tx, err := instance.RegisterAccount(auth(), addr, big.NewInt(id), pk, rx, ry, []byte{})
		if err != nil {
			t.Fatalf("register %d: %v", id, err)
		}
		if _, err := bind.WaitMined(ctx, client, tx); err != nil {
			t.Fatalf("wait register %d: %v", id, err)
		}
	}

	// 3. Account 1 already holds minted USDr: initializing would overwrite it.
	tx, err = instance.MintUsdrSupply(auth(), big.NewInt(usdrMintAmt), big.NewInt(1))
	if err != nil {
		t.Fatalf("mintUsdrSupply: %v", err)
	}
	if _, err := bind.WaitMined(ctx, client, tx); err != nil {
		t.Fatalf("wait mintUsdrSupply: %v", err)
	}
	_, sendErr = instance.InitializeUsdrBalance(auth(), big.NewInt(1), cx, cy)
	wantRevert("over a minted balance", "AlreadyRegistered", sendErr)

	// 4. Account 2 is registered and empty: must succeed, then not repeat.
	tx, err = instance.InitializeUsdrBalance(auth(), big.NewInt(2), cx, cy)
	if err != nil {
		t.Fatalf("initializeUsdrBalance on a registered, empty account failed: %v", err)
	}
	if r, err := bind.WaitMined(ctx, client, tx); err != nil || r.Status != 1 {
		t.Fatalf("initializeUsdrBalance reverted (status=%v err=%v)", r, err)
	}
	t.Log("registered, empty account initialized")
	_, sendErr = instance.InitializeUsdrBalance(auth(), big.NewInt(2), cx, cy)
	wantRevert("second initialization", "AlreadyRegistered", sendErr)

	if ok, err := instance.CheckUsdr(&bind.CallOpts{}); err != nil || !ok {
		t.Fatalf("checkUsdr() failed after the guarded sequence: ok=%v err=%v", ok, err)
	}
	t.Log("checkUsdr() holds after the guarded sequence")
}
