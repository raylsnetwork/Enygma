package endpoints

/*
Port of src/core/endpoints/user.js

User-level operations: depositing tokens into vault contracts, generating ZK
proofs for transfer/ownership, and withdrawing tokens using those proofs.

JS mapping:
  - JS `contract.connect(account).method(...)` → Go: bind.NewBoundContract + auth
  - JS `prover.Erc20Proof(...)`                → Go: gnarkClient.Erc20JoinSplitProof(...)
  - JS `prover.Erc721Proof(...)`               → Go: gnarkClient.Erc721OwnershipProof(...)
  - JS `prover.Erc1155FungibleProof(...)`      → Go: gnarkClient.Erc1155FungibleJoinSplitProof(...)
  - JS `prover.Erc1155NonFungibleProof(...)`   → Go: gnarkClient.Erc1155NonFungibleOwnershipProof(...)

Notes:
  - ProofResult.Proof is not populated by the current gnark response parser; callers
    submitting proofs on-chain receive a ProofReceipt with a zero-valued SnarkProof.
    Populate Receipt.Proof from the server's raw response before submitting.
  - getCommitmentsFromTx scans for a "Commitment" event absent from the current ABI
    (see relayer.go). It will always return an error on current contracts.
  - MixErc20's JS source passes erc20Address as the treeDepth argument (a bug).
    The Go signature requires correct argument types; see the function comment.
  - GenerateOwnershipProof takes a raw tokenId, not a pre-hashed uniqueId.
    Erc721OwnershipProof computes the uniqueId internally (JS did it externally).
  - GenerateErc1155BatchProof is not yet implemented: Erc1155BatchProof is missing
    from the Go gnark client.
  - withdrawERC1155Batch is unexported and non-functional: the JS source references
    undefined variables (amount, withdrawKey, merkleProof, etc.).
*/

import (
	"context"
	"crypto/mlkem"
	"fmt"
	"math/big"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/raylsnetwork/enygma_dvp/src/core"
	"github.com/raylsnetwork/enygma_dvp/src/web3"
)

// --- Internal helpers ---

// buildProofReceipt converts a ProofResult to a ProofReceipt for ABI encoding.
// NOTE: The SnarkProof (a, b, c) is left zero-valued because ProofResult.Proof is
// not currently populated by the gnark server response parser.
func buildProofReceipt(r *core.ProofResult) ProofReceipt {
	return ProofReceipt{
		Statement:       r.Statement,
		NumberOfInputs:  big.NewInt(int64(r.NumberOfInputs)),
		NumberOfOutputs: big.NewInt(int64(r.NumberOfOutputs)),
	}
}

// addressToBigInt converts a common.Address to *big.Int for use as a circuit input.
func addressToBigInt(addr common.Address) *big.Int {
	return new(big.Int).SetBytes(addr.Bytes())
}

// getCommitmentsFromTx scans a transaction receipt for all "Commitment" events and
// returns the commitment values as a slice.
// NOTE: the "Commitment" event is absent from the current EnygmaDvp ABI; this
// function is a port of the JS helper and will error on current contracts.
func getCommitmentsFromTx(contractABI abi.ABI, receipt *types.Receipt) ([]*big.Int, error) {
	fmt.Printf("Gas used: %d\n", receipt.GasUsed)

	event, ok := contractABI.Events["Commitment"]
	if !ok {
		return nil, fmt.Errorf("Commitment event not found in ABI (may have been renamed in current contract version)")
	}

	var commitments []*big.Int
	for _, log := range receipt.Logs {
		if len(log.Topics) == 0 {
			continue
		}
		if log.Topics[0] != event.ID {
			continue
		}

		values := make(map[string]interface{})
		if err := contractABI.UnpackIntoMap(values, event.Name, log.Data); err != nil {
			continue
		}

		if cmt, ok := values["commitment"].(*big.Int); ok {
			commitments = append(commitments, cmt)
		}
	}

	if len(commitments) == 0 {
		return nil, fmt.Errorf("no Commitment events found in transaction logs")
	}
	fmt.Printf("new commitments: %v\n", commitments)
	return commitments, nil
}

// --- Deposit functions ---

// DepositErc20 approves the vault to spend ERC-20 tokens, then deposits them.
// Returns the commitment values emitted by the "Commitment" event.
//
// C-1 fix: the contract now computes commitment = Poseidon4(pkSpend, salt, amount, tokenId)
// on-chain, so callers must supply pkSpend, salt and tokenId instead of a pre-computed
// commitment.  The contract emits the resulting commitment in the Commitment event.
func DepositErc20(
	client *ethclient.Client,
	auth *bind.TransactOpts,
	vaultABI abi.ABI,
	vaultAddr common.Address,
	erc20ABI abi.ABI,
	erc20Addr common.Address,
	depositAmount *big.Int,
	pkSpend *big.Int,
	salt *big.Int,
	tokenId *big.Int,
) ([]*big.Int, error) {
	// Step 1: erc20.approve(vaultAddr, depositAmount)
	erc20Contract := bind.NewBoundContract(erc20Addr, erc20ABI, client, client, client)
	approveTx, err := erc20Contract.Transact(auth, "approve", vaultAddr, depositAmount)
	if err != nil {
		return nil, fmt.Errorf("approve failed: %w", err)
	}
	approveReceipt, err := bind.WaitMined(context.Background(), client, approveTx)
	if err != nil {
		return nil, fmt.Errorf("waiting for approve receipt failed: %w", err)
	}
	fmt.Printf("...approves the transfer of Erc20 to ZkDvp\n")
	fmt.Printf("gasUsed: %d\n", approveReceipt.GasUsed)

	// Step 2: vault.deposit([amount, pkSpend, salt, tokenId]) — commitment computed on-chain.
	params := []*big.Int{depositAmount, pkSpend, salt, tokenId}
	vaultContract := bind.NewBoundContract(vaultAddr, vaultABI, client, client, client)
	depositTx, err := vaultContract.Transact(auth, "deposit", params)
	if err != nil {
		return nil, fmt.Errorf("deposit failed: %w", err)
	}
	depositReceipt, err := bind.WaitMined(context.Background(), client, depositTx)
	if err != nil {
		return nil, fmt.Errorf("waiting for deposit receipt failed: %w", err)
	}

	return getCommitmentsFromTx(vaultABI, depositReceipt)
}

// DepositErc20V2 deposits ERC-20 tokens using the non-interactive V2 flow.
// It runs ML-KEM encapsulation to derive saltB, computes a V2 commitment
// Poseidon(pk_spend, saltB_field, amount, tokenId), encrypts (tokenId||amount)
// with ChaCha20-Poly1305, then calls depositV2 on-chain so the recipient can
// scan for the EncryptedNote event and discover the note without prior interaction.
func DepositErc20V2(
	client *ethclient.Client,
	auth *bind.TransactOpts,
	vaultABI abi.ABI,
	vaultAddr common.Address,
	erc20ABI abi.ABI,
	erc20Addr common.Address,
	depositAmount *big.Int,
	tokenId *big.Int,
	recipientSpendPk *big.Int,
	recipientViewEncapKey []byte,
) error {
	// Encapsulate to derive raw shared secret ss and ML-KEM capsule for the recipient
	ss, cipherText, err := core.Encapsulate(recipientViewEncapKey)
	if err != nil {
		return fmt.Errorf("encapsulate failed: %w", err)
	}
	saltB, err := core.DerivePaymentSalt(ss)
	if err != nil {
		return fmt.Errorf("derive payment salt failed: %w", err)
	}
	encKey, err := core.DerivePaymentKey(ss)
	if err != nil {
		return fmt.Errorf("derive payment key failed: %w", err)
	}
	saltBField := core.SaltBToField(saltB)

	// Encrypt (tokenId||amount) with AES-GCM so the recipient can learn what was sent
	encTxData, err := core.EncryptPayload(encKey, tokenId, depositAmount)
	if err != nil {
		return fmt.Errorf("encrypt payload failed: %w", err)
	}

	// Compute V2 commitment: Poseidon(pk_spend, saltB_field, amount, tokenId)
	commitment, err := core.Erc20CommitmentV2(recipientSpendPk, saltBField, depositAmount, tokenId)
	if err != nil {
		return fmt.Errorf("compute V2 commitment failed: %w", err)
	}

	// Step 1: erc20.approve(vaultAddr, depositAmount)
	erc20Contract := bind.NewBoundContract(erc20Addr, erc20ABI, client, client, client)
	approveTx, err := erc20Contract.Transact(auth, "approve", vaultAddr, depositAmount)
	if err != nil {
		return fmt.Errorf("approve failed: %w", err)
	}
	approveReceipt, err := bind.WaitMined(context.Background(), client, approveTx)
	if err != nil {
		return fmt.Errorf("waiting for approve receipt failed: %w", err)
	}
	fmt.Printf("...approves the transfer of Erc20 to ZkDvp\n")
	fmt.Printf("gasUsed: %d\n", approveReceipt.GasUsed)

	// Step 2: vault.depositV2([amount, pkSpend, saltBField, tokenId], cipherText, encTxData)
	// C-1 fix: contract computes commitment on-chain from the key material.
	_ = commitment // retained for local logging; not sent to contract
	params := []*big.Int{depositAmount, recipientSpendPk, saltBField, tokenId}
	vaultContract := bind.NewBoundContract(vaultAddr, vaultABI, client, client, client)
	depositTx, err := vaultContract.Transact(auth, "depositV2", params, cipherText, encTxData)
	if err != nil {
		return fmt.Errorf("depositV2 failed: %w", err)
	}
	depositReceipt, err := bind.WaitMined(context.Background(), client, depositTx)
	if err != nil {
		return fmt.Errorf("waiting for depositV2 receipt failed: %w", err)
	}
	fmt.Printf("depositV2 gasUsed: %d\n", depositReceipt.GasUsed)

	return nil
}

// WithdrawErc20 generates a JoinSplit proof and withdraws tokens from the ERC-20 vault.
// Returns statement[1 + numberOfInputs*3], the first output commitment index.
//
// NOTE: the on-chain withdraw function checks the output commitment against
// Poseidon(uid, recipient_address). Full V2 compatibility requires a withdrawV2
// contract function; this function updates the proof API but the on-chain check
// may not match V2 output commitments.
//
// Corresponds to: withdrawErc20(account, amount, withdrawKey, erc20VaultContract, erc20Contract,
//
//	merkleDepth, merkleProof, merkleRoot, treeNumber)
func WithdrawErc20(
	client *ethclient.Client,
	auth *bind.TransactOpts,
	vaultABI abi.ABI,
	vaultAddr common.Address,
	gnarkClient *core.GnarkClient,
	amount *big.Int,
	withdrawKey core.KeyPair,
	wtSaltIn *big.Int,
	accountAddr common.Address,
	accountSpendPk *big.Int,    // pk_spend of the withdrawer (for output commitment)
	accountViewEncapKey []byte, // view encapsulation key of the withdrawer (for note encryption)
	tokenId *big.Int,
	merkleDepth int,
	merkleProof *core.MerkleProof,
	stTreeNumber *big.Int,
	use10_2 bool,
) (*big.Int, error) {
	dummySpendKp, err := core.NewSpendKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate dummy spend key pair: %w", err)
	}
	dummyViewKp, err := core.NewViewKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate dummy view key pair: %w", err)
	}
	dummyKey := core.KeyPair{PrivateKey: dummySpendKp.PrivateKey, PublicKey: dummySpendKp.PublicKey}

	result, err := gnarkClient.Erc20JoinSplitProof(
		big.NewInt(0),
		[]*big.Int{amount, big.NewInt(0)},
		[]core.KeyPair{withdrawKey, dummyKey},
		[]*big.Int{wtSaltIn, big.NewInt(0)},
		[]*big.Int{amount, big.NewInt(0)},
		[]*big.Int{accountSpendPk, dummySpendKp.PublicKey},
		[][]byte{accountViewEncapKey, dummyViewKp.EncapsKey},
		merkleDepth,
		[]*core.MerkleProof{merkleProof, nil}, // nil safe: dummy input has value=0
		[]*big.Int{stTreeNumber, big.NewInt(0)},
		tokenId,
		use10_2,
	)
	if err != nil {
		return nil, fmt.Errorf("generateErc20JoinSplitProof failed: %w", err)
	}

	proofReceipt := buildProofReceipt(result)

	// vault.withdraw([amount], accountAddr, proof)
	vaultContract := bind.NewBoundContract(vaultAddr, vaultABI, client, client, client)
	withdrawTx, err := vaultContract.Transact(auth, "withdraw",
		[]*big.Int{amount}, accountAddr, proofReceipt)
	if err != nil {
		return nil, fmt.Errorf("withdraw failed: %w", err)
	}
	if _, err := bind.WaitMined(context.Background(), client, withdrawTx); err != nil {
		return nil, fmt.Errorf("waiting for withdraw receipt failed: %w", err)
	}

	// JS: return proof.statement[1 + proof.numberOfInputs * 3]
	idx := 1 + result.NumberOfInputs*3
	if idx >= len(result.Statement) {
		return nil, fmt.Errorf("statement too short: need index %d, got length %d",
			idx, len(result.Statement))
	}
	return result.Statement[idx], nil
}

// WithdrawErc20V2 generates a V2 withdrawal proof and calls withdrawV2 on-chain.
//
// The withdrawal output commitment is computed as:
//
//	Poseidon4(uint160(recipientAddr), 0, amount, tokenId)
//
// No KEM encapsulation is performed — the salt is fixed to 0.
func WithdrawErc20V2(
	client *ethclient.Client,
	auth *bind.TransactOpts,
	vaultABI abi.ABI,
	vaultAddr common.Address,
	gnarkClient *core.GnarkClient,
	amount *big.Int,
	withdrawKey core.KeyPair,
	wtSaltIn *big.Int,
	recipientAddr common.Address,
	tokenId *big.Int,
	merkleDepth int,
	merkleProof *core.MerkleProof,
	stTreeNumber *big.Int,
	use10_2 bool,
) error {
	recipient := addressToBigInt(recipientAddr)

	dummySpendKp, err := core.NewSpendKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate dummy spend key pair: %w", err)
	}
	dummyKey := core.KeyPair{PrivateKey: dummySpendKp.PrivateKey, PublicKey: dummySpendKp.PublicKey}

	result, err := gnarkClient.Erc20WithdrawProof(
		big.NewInt(0),
		[]*big.Int{amount, big.NewInt(0)},
		[]core.KeyPair{withdrawKey, dummyKey},
		[]*big.Int{wtSaltIn, big.NewInt(0)},
		amount,
		recipient,
		dummySpendKp.PublicKey,
		merkleDepth,
		[]*core.MerkleProof{merkleProof, nil}, // nil safe: dummy input has value=0
		[]*big.Int{stTreeNumber, big.NewInt(0)},
		tokenId,
		use10_2,
	)
	if err != nil {
		return fmt.Errorf("erc20WithdrawProof failed: %w", err)
	}

	proofReceipt := buildProofReceipt(result)

	// vault.withdrawV2([amount, tokenId], recipientAddr, receipt)
	vaultContract := bind.NewBoundContract(vaultAddr, vaultABI, client, client, client)
	withdrawTx, err := vaultContract.Transact(auth, "withdrawV2",
		[]*big.Int{amount, tokenId}, recipientAddr, proofReceipt)
	if err != nil {
		return fmt.Errorf("withdrawV2 failed: %w", err)
	}
	if _, err := bind.WaitMined(context.Background(), client, withdrawTx); err != nil {
		return fmt.Errorf("waiting for withdrawV2 receipt failed: %w", err)
	}

	return nil
}

// DepositErc721 approves the vault to transfer an ERC-721 NFT, then deposits it.
// Returns the commitment values emitted by the "Commitment" event.
//
// C-1 fix: the contract now computes commitment = Poseidon4(pkSpend, salt, 1, tokenId)
// on-chain (matching Erc721Commitment circuit formula), binding the commitment to the
// actual deposited NFT.  Callers supply pkSpend and salt instead of a pre-computed commitment.
func DepositErc721(
	client *ethclient.Client,
	auth *bind.TransactOpts,
	vaultABI abi.ABI,
	vaultAddr common.Address,
	erc721ABI abi.ABI,
	erc721Addr common.Address,
	nftID *big.Int,
	pkSpend *big.Int,
	salt *big.Int,
) ([]*big.Int, error) {
	// Step 1: erc721.approve(vaultAddr, nftID)
	erc721Contract := bind.NewBoundContract(erc721Addr, erc721ABI, client, client, client)
	approveTx, err := erc721Contract.Transact(auth, "approve", vaultAddr, nftID)
	if err != nil {
		return nil, fmt.Errorf("approve failed: %w", err)
	}
	approveReceipt, err := bind.WaitMined(context.Background(), client, approveTx)
	if err != nil {
		return nil, fmt.Errorf("waiting for approve receipt failed: %w", err)
	}
	fmt.Printf("...approves the transfer of NFT to ZkDvp\n")
	fmt.Printf("gasUsed: %d\n", approveReceipt.GasUsed)

	// Step 2: vault.deposit([nftID, pkSpend, salt]) — commitment computed on-chain.
	params := []*big.Int{nftID, pkSpend, salt}
	vaultContract := bind.NewBoundContract(vaultAddr, vaultABI, client, client, client)
	depositTx, err := vaultContract.Transact(auth, "deposit", params)
	if err != nil {
		return nil, fmt.Errorf("deposit failed: %w", err)
	}
	depositReceipt, err := bind.WaitMined(context.Background(), client, depositTx)
	if err != nil {
		return nil, fmt.Errorf("waiting for deposit receipt failed: %w", err)
	}

	return getCommitmentsFromTx(vaultABI, depositReceipt)
}

// DepositErc1155 approves the vault as operator for an ERC-1155 token, then deposits it.
// If the contract reverts with a "FungibilityMerkle" custom error, the error is logged
// and nil is returned (matching the JS try/catch behaviour).
//
// C-1 fix: the contract now computes commitment = Poseidon4(pkSpend, salt, amount, tokenId)
// on-chain; callers supply pkSpend and salt instead of a pre-computed commitment.
func DepositErc1155(
	client *ethclient.Client,
	auth *bind.TransactOpts,
	vaultABI abi.ABI,
	vaultAddr common.Address,
	erc1155ABI abi.ABI,
	erc1155Addr common.Address,
	tokenID *big.Int,
	amount *big.Int,
	pkSpend *big.Int,
	salt *big.Int,
) ([]*big.Int, error) {
	// Step 1: erc1155.setApprovalForAll(vaultAddr, true)
	erc1155Contract := bind.NewBoundContract(erc1155Addr, erc1155ABI, client, client, client)
	approveTx, err := erc1155Contract.Transact(auth, "setApprovalForAll", vaultAddr, true)
	if err != nil {
		return nil, fmt.Errorf("setApprovalForAll failed: %w", err)
	}
	if _, err := bind.WaitMined(context.Background(), client, approveTx); err != nil {
		return nil, fmt.Errorf("waiting for setApprovalForAll receipt failed: %w", err)
	}
	fmt.Printf("...approved the transfer of Erc1155 to ZkDvp\n")

	// Step 2: vault.deposit([amount, tokenID, pkSpend, salt]) — commitment computed on-chain.
	params := []*big.Int{amount, tokenID, pkSpend, salt}
	vaultContract := bind.NewBoundContract(vaultAddr, vaultABI, client, client, client)
	depositTx, err := vaultContract.Transact(auth, "deposit", params)
	if err != nil {
		// Mirror JS try/catch: parse and log the FungibilityMerkle custom error,
		// then return nil (no error propagated) matching the JS behaviour.
		// Note: go-ethereum encodes revert data differently from JS err.error;
		// ParseCustomError may not match on all clients.
		errorText := web3.ParseCustomError(err.Error(), "FungibilityMerkle")
		fmt.Printf("Error Text: %s\n", errorText)
		return nil, nil //nolint:nilerr
	}
	depositReceipt, err := bind.WaitMined(context.Background(), client, depositTx)
	if err != nil {
		return nil, fmt.Errorf("waiting for deposit receipt failed: %w", err)
	}

	return getCommitmentsFromTx(vaultABI, depositReceipt)
}

// DepositErc1155Batch approves the vault as operator and batch-deposits ERC-1155 tokens.
//
// C-1 fix: params are now packed as: [...tokenIds, ...amounts, ...pkSpeends, ...salts].
// The contract computes each commitment = Poseidon4(pkSpend, salt, amount, tokenId) on-chain.
func DepositErc1155Batch(
	client *ethclient.Client,
	auth *bind.TransactOpts,
	vaultABI abi.ABI,
	vaultAddr common.Address,
	erc1155ABI abi.ABI,
	erc1155Addr common.Address,
	tokenIDs []*big.Int,
	amounts []*big.Int,
	pkSpeends []*big.Int,
	salts []*big.Int,
) ([]*big.Int, error) {
	// Step 1: erc1155.setApprovalForAll(vaultAddr, true)
	erc1155Contract := bind.NewBoundContract(erc1155Addr, erc1155ABI, client, client, client)
	approveTx, err := erc1155Contract.Transact(auth, "setApprovalForAll", vaultAddr, true)
	if err != nil {
		return nil, fmt.Errorf("setApprovalForAll failed: %w", err)
	}
	if _, err := bind.WaitMined(context.Background(), client, approveTx); err != nil {
		return nil, fmt.Errorf("waiting for setApprovalForAll receipt failed: %w", err)
	}
	fmt.Printf("...approves the BatchTransfer of Erc1155 to ZkDvp\n")

	// Pack params as: [...tokenIds, ...amounts, ...pkSpeends, ...salts]
	params := make([]*big.Int, 0, len(tokenIDs)*4)
	params = append(params, tokenIDs...)
	params = append(params, amounts...)
	params = append(params, pkSpeends...)
	params = append(params, salts...)

	fmt.Printf("tokenIds: %v amounts: %v\n", tokenIDs, amounts)

	// Step 2: vault.deposit(params) — commitments computed on-chain.
	vaultContract := bind.NewBoundContract(vaultAddr, vaultABI, client, client, client)
	depositTx, err := vaultContract.Transact(auth, "deposit", params)
	if err != nil {
		return nil, fmt.Errorf("deposit failed: %w", err)
	}
	depositReceipt, err := bind.WaitMined(context.Background(), client, depositTx)
	if err != nil {
		return nil, fmt.Errorf("waiting for deposit receipt failed: %w", err)
	}

	return getCommitmentsFromTx(vaultABI, depositReceipt)
}

// --- Withdrawal functions ---

// WithdrawERC1155 generates an ERC-1155 proof (fungible or non-fungible) and withdraws.
// Returns statement[4], the output commitment (matches JS proof.commitment).
// Corresponds to: withdrawERC1155(account, amount, tokenId, withdrawKey, vaultContract,
//
//	erc1155Contract, merkleDepth, merkleProof, merkleRoot, treeNumber,
//	assetGroup_merkleRoot, assetGroup_merkleProof, isFungible)
func WithdrawERC1155(
	client *ethclient.Client,
	auth *bind.TransactOpts,
	vaultABI abi.ABI,
	vaultAddr common.Address,
	gnarkClient *core.GnarkClient,
	amount *big.Int,
	tokenID *big.Int,
	withdrawKey core.KeyPair,
	wtSaltIn *big.Int,
	accountAddr common.Address,
	erc1155Addr common.Address,
	merkleDepth int,
	merkleProof *core.MerkleProof,
	stTreeNumber *big.Int,
	assetGroupTreeNumber *big.Int,
	assetGroupMerkleProof *core.MerkleProof,
	isFungible bool,
) (*big.Int, error) {
	addrBig := addressToBigInt(erc1155Addr)
	outKey := core.KeyPair{PublicKey: addressToBigInt(accountAddr)}

	var result *core.ProofResult
	var err error

	if isFungible {
		result, err = gnarkClient.Erc1155FungibleJoinSplitProof(
			big.NewInt(0),
			[]*big.Int{amount},
			[]core.KeyPair{withdrawKey},
			[]*big.Int{wtSaltIn},
			[]*big.Int{amount},
			[]core.KeyPair{outKey},
			[][]byte{nil}, // withdrawal: no recipient view key needed (output is public)
			merkleDepth,
			[]*core.MerkleProof{merkleProof},
			[]*big.Int{stTreeNumber},
			addrBig,
			tokenID,
			assetGroupTreeNumber,
			assetGroupMerkleProof,
		)
	} else {
		result, err = gnarkClient.Erc1155NonFungibleOwnershipProof(
			big.NewInt(0),
			amount,
			withdrawKey,
			wtSaltIn,
			outKey,
			nil, // withdrawal: no recipient view key needed (output is public)
			merkleDepth,
			merkleProof,
			stTreeNumber,
			addrBig,
			tokenID,
			assetGroupTreeNumber,
			assetGroupMerkleProof,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("generateSingleErc1155Proof failed: %w", err)
	}

	proofReceipt := buildProofReceipt(result)

	// vault.withdraw([amount, tokenId], accountAddr, proof)
	vaultContract := bind.NewBoundContract(vaultAddr, vaultABI, client, client, client)
	withdrawTx, err := vaultContract.Transact(auth, "withdraw",
		[]*big.Int{amount, tokenID}, accountAddr, proofReceipt)
	if err != nil {
		return nil, fmt.Errorf("withdraw failed: %w", err)
	}
	withdrawReceipt, err := bind.WaitMined(context.Background(), client, withdrawTx)
	if err != nil {
		return nil, fmt.Errorf("waiting for withdraw receipt failed: %w", err)
	}
	fmt.Printf("gasUsed: %d\n", withdrawReceipt.GasUsed)

	// JS: return proof.commitment = statement[4] (commitmentOut)
	if len(result.Statement) <= 4 {
		return nil, fmt.Errorf("statement too short: need index 4, got length %d",
			len(result.Statement))
	}
	return result.Statement[4], nil
}

// WithdrawErc721 generates an ERC-721 ownership proof and withdraws the NFT from the vault.
// Returns statement[4], the output commitment (matches JS proof.commitment).
// Corresponds to: withdrawErc721(account, nft_id, withdrawKey, erc721VaultContract,
//
//	erc721Contract, merkleDepth, merkleProof, merkleRoot, treeNumber)
func WithdrawErc721(
	client *ethclient.Client,
	auth *bind.TransactOpts,
	vaultABI abi.ABI,
	vaultAddr common.Address,
	gnarkClient *core.GnarkClient,
	nftID *big.Int,
	withdrawKey core.KeyPair,
	wtSaltIn *big.Int,
	accountAddr common.Address,
	erc721Addr common.Address,
	merkleDepth int,
	merkleProof *core.MerkleProof,
	stTreeNumber *big.Int,
) (*big.Int, error) {
	outKey := core.KeyPair{PublicKey: addressToBigInt(accountAddr)}

	// JS computes uid = erc721UniqueId(erc721Address, nft_id) externally, then passes uid.
	// Go's Erc721OwnershipProof takes the raw nftID and computes uniqueId internally.
	result, err := gnarkClient.Erc721OwnershipProof(
		big.NewInt(0),
		nftID,
		withdrawKey,
		wtSaltIn,
		outKey,
		nil, // withdrawal: no recipient view key needed (output is public)
		merkleDepth,
		merkleProof,
		stTreeNumber,
		addressToBigInt(erc721Addr),
	)
	if err != nil {
		return nil, fmt.Errorf("generateOwnershipProof failed: %w", err)
	}

	proofReceipt := buildProofReceipt(result)

	// vault.withdraw([nftID], accountAddr, proof)
	vaultContract := bind.NewBoundContract(vaultAddr, vaultABI, client, client, client)
	withdrawTx, err := vaultContract.Transact(auth, "withdraw",
		[]*big.Int{nftID}, accountAddr, proofReceipt)
	if err != nil {
		return nil, fmt.Errorf("withdraw failed: %w", err)
	}
	withdrawReceipt, err := bind.WaitMined(context.Background(), client, withdrawTx)
	if err != nil {
		return nil, fmt.Errorf("waiting for withdraw receipt failed: %w", err)
	}
	fmt.Printf("gasUsed: %d\n", withdrawReceipt.GasUsed)

	// JS: return proof.commitment = statement[4] (commitmentOut)
	if len(result.Statement) <= 4 {
		return nil, fmt.Errorf("statement too short: need index 4, got length %d",
			len(result.Statement))
	}
	return result.Statement[4], nil
}

// withdrawERC1155Batch is unexported because the JS source did not export it.
// NOTE: the JS source has undefined-variable bugs (references amount, withdrawKey,
// merkleProof, merkleRoot, treeNumber, tokenId which are not in scope). This Go
// function documents the intended signature and returns an error until the bugs
// are resolved upstream.
func withdrawERC1155Batch(
	_ *ethclient.Client,
	_ *bind.TransactOpts,
	_ abi.ABI,
	_ common.Address,
	_ *core.GnarkClient,
	_ []*big.Int, // amounts
	_ []*big.Int, // tokenIds
	_ []core.KeyPair, // withdrawKeys
	_ common.Address, // accountAddr
	_ common.Address, // erc1155Addr
	_ int, // merkleDepth
	_ []*core.MerkleProof,
	_ []*big.Int, // stTreeNumbers
	_ []*big.Int, // assetGroupTreeNumbers
	_ []*core.MerkleProof,
) (*big.Int, error) {
	return nil, fmt.Errorf("withdrawERC1155Batch: not implemented (JS source has undefined-variable bugs)")
}

// --- Ownership check ---

// CheckOwnership returns true if accountAddr owns the given ERC-721 token.
// Corresponds to: checkOwnership(account, nft_id, erc721Contract)
func CheckOwnership(
	client *ethclient.Client,
	erc721ABI abi.ABI,
	erc721Addr common.Address,
	accountAddr common.Address,
	nftID *big.Int,
) (bool, error) {
	erc721Contract := bind.NewBoundContract(erc721Addr, erc721ABI, client, client, client)

	var out []interface{}
	if err := erc721Contract.Call(nil, &out, "ownerOf", nftID); err != nil {
		fmt.Printf("error in CheckOwnership: %v\n", err)
		return false, nil
	}

	tokenOwner, ok := out[0].(common.Address)
	if !ok {
		return false, fmt.Errorf("unexpected ownerOf return type")
	}

	fmt.Printf("nft %s belongs to %s %s\n", nftID.String(), tokenOwner.Hex(), accountAddr.Hex())
	return tokenOwner == accountAddr, nil
}

// --- Proof generation wrappers ---

// MixErc20 generates a JoinSplit proof to re-randomize keys without an on-chain call.
// If fewer than 2 inputs are provided, a dummy key pair is appended to reach 2 inputs.
// NOTE: the JS source has a parameter-ordering bug (passes erc20Address as treeDepth
// and merkleTree as proofs). The Go signature requires correct argument types.
// Corresponds to: mixErc20(account, inAmounts, inKeys, outAmounts, outKeys,
//
//	zkDvpContract, erc20Contract, merkleTree)
func MixErc20(
	gnarkClient *core.GnarkClient,
	inAmounts []*big.Int,
	inKeys []core.KeyPair,
	wtSaltsIn []*big.Int,
	outAmounts []*big.Int,
	recipientSpendPks []*big.Int,
	recipientViewEncapKeys [][]byte,
	tokenId *big.Int,
	merkleDepth int,
	merkleProofs []*core.MerkleProof,
	stTreeNumbers []*big.Int,
	use10_2 bool,
) (*core.ProofResult, error) {
	if len(inAmounts) < 2 || len(inKeys) < 2 {
		dummySpendKp, err := core.NewSpendKeyPair()
		if err != nil {
			return nil, fmt.Errorf("failed to generate dummy spend key pair: %w", err)
		}
		dummyViewKp, err := core.NewViewKeyPair()
		if err != nil {
			return nil, fmt.Errorf("failed to generate dummy view key pair: %w", err)
		}
		dummyKey := core.KeyPair{PrivateKey: dummySpendKp.PrivateKey, PublicKey: dummySpendKp.PublicKey}

		return gnarkClient.Erc20JoinSplitProof(
			big.NewInt(0),
			[]*big.Int{inAmounts[0], big.NewInt(0)},
			[]core.KeyPair{inKeys[0], dummyKey},
			[]*big.Int{wtSaltsIn[0], big.NewInt(0)},
			[]*big.Int{inAmounts[0], big.NewInt(0)},
			[]*big.Int{recipientSpendPks[0], dummySpendKp.PublicKey},
			[][]byte{recipientViewEncapKeys[0], dummyViewKp.EncapsKey},
			merkleDepth,
			merkleProofs,
			stTreeNumbers,
			tokenId,
			use10_2,
		)
	}

	return gnarkClient.Erc20JoinSplitProof(
		big.NewInt(0),
		inAmounts,
		inKeys,
		wtSaltsIn,
		outAmounts,
		recipientSpendPks,
		recipientViewEncapKeys,
		merkleDepth,
		merkleProofs,
		stTreeNumbers,
		tokenId,
		use10_2,
	)
}

// GenerateErc20JoinSplitProof generates an ERC-20 JoinSplit proof.
// Corresponds to: generateErc20JoinSplitProof(nftCommitment, inAmounts, inKeys,
//
//	outAmounts, outKeys, treeDepth, proofs, roots, treeNumbers, erc20ContractAddress)
func GenerateErc20JoinSplitProof(
	gnarkClient *core.GnarkClient,
	stMessage *big.Int,
	wtValuesIn []*big.Int,
	keysIn []core.KeyPair,
	wtSaltsIn []*big.Int,
	wtValuesOut []*big.Int,
	recipientSpendPks []*big.Int,
	recipientViewEncapKeys [][]byte,
	merkleDepth int,
	merkleProofs []*core.MerkleProof,
	stTreeNumbers []*big.Int,
	tokenId *big.Int,
	use10_2 bool,
) (*core.ProofResult, error) {
	return gnarkClient.Erc20JoinSplitProof(
		stMessage,
		wtValuesIn,
		keysIn,
		wtSaltsIn,
		wtValuesOut,
		recipientSpendPks,
		recipientViewEncapKeys,
		merkleDepth,
		merkleProofs,
		stTreeNumbers,
		tokenId,
		use10_2,
	)
}

// TransferErc20V2 generates a JoinSplit proof and calls transferV2 on-chain,
// publishing an EncryptedNote event for each output so recipients can scan for them.
func TransferErc20V2(
	client *ethclient.Client,
	auth *bind.TransactOpts,
	vaultABI abi.ABI,
	vaultAddr common.Address,
	gnarkClient *core.GnarkClient,
	stMessage *big.Int,
	wtValuesIn []*big.Int,
	keysIn []core.KeyPair,
	wtSaltsIn []*big.Int,
	wtValuesOut []*big.Int,
	recipientSpendPks []*big.Int,
	recipientViewEncapKeys [][]byte,
	tokenId *big.Int,
	merkleDepth int,
	merkleProofs []*core.MerkleProof,
	stTreeNumbers []*big.Int,
	use10_2 bool,
) error {
	result, err := gnarkClient.Erc20JoinSplitProof(
		stMessage,
		wtValuesIn,
		keysIn,
		wtSaltsIn,
		wtValuesOut,
		recipientSpendPks,
		recipientViewEncapKeys,
		merkleDepth,
		merkleProofs,
		stTreeNumbers,
		tokenId,
		use10_2,
	)
	if err != nil {
		return fmt.Errorf("erc20JoinSplitProof failed: %w", err)
	}

	proofReceipt := buildProofReceipt(result)

	// vault.transferV2(receipt, cipherText[], encTxData[])
	vaultContract := bind.NewBoundContract(vaultAddr, vaultABI, client, client, client)
	transferTx, err := vaultContract.Transact(auth, "transferV2",
		proofReceipt, result.CipherText, result.EncTxData)
	if err != nil {
		return fmt.Errorf("transferV2 failed: %w", err)
	}
	if _, err := bind.WaitMined(context.Background(), client, transferTx); err != nil {
		return fmt.Errorf("waiting for transferV2 receipt failed: %w", err)
	}

	return nil
}

// GenerateOwnershipProof generates an ERC-721 ownership proof.
// Takes the raw tokenId (nftID); Erc721OwnershipProof computes the uniqueId internally.
// Note: the JS computed uniqueId = erc721UniqueId(address, tokenId) externally and
// passed it to the prover directly; Go internalises that step.
// Corresponds to: generateOwnershipProof(paymentCommitment, uid, inKey, outKey,
//
//	merkleDepth, merkleProof, merkleRoot, treeNumber)
func GenerateOwnershipProof(
	gnarkClient *core.GnarkClient,
	stMessage *big.Int,
	nftID *big.Int,
	keyIn core.KeyPair,
	wtSaltIn *big.Int,
	keyOut core.KeyPair,
	merkleDepth int,
	merkleProof *core.MerkleProof,
	stTreeNumber *big.Int,
	erc721ContractAddr common.Address,
) (*core.ProofResult, error) {
	return gnarkClient.Erc721OwnershipProof(
		stMessage,
		nftID,
		keyIn,
		wtSaltIn,
		keyOut,
		nil, // legacy wrapper: caller must use direct API for non-interactive delivery
		merkleDepth,
		merkleProof,
		stTreeNumber,
		addressToBigInt(erc721ContractAddr),
	)
}

// GenerateSingleErc1155Proof generates a single ERC-1155 proof (fungible or non-fungible).
// Corresponds to: generateSingleErc1155Proof(message, amountOrOne, inKey, outKey,
//
//	merkleDepth, merkleProof, merkleRoot, treeNumber, erc1155ContractAddress, erc1155TokenId,
//	assetGroup_treeNumber, assetGroup_merkleProof, isFungible)
func GenerateSingleErc1155Proof(
	gnarkClient *core.GnarkClient,
	stMessage *big.Int,
	amountOrOne *big.Int,
	keyIn core.KeyPair,
	wtSaltIn *big.Int,
	keyOut core.KeyPair,
	merkleDepth int,
	merkleProof *core.MerkleProof,
	stTreeNumber *big.Int,
	erc1155ContractAddr common.Address,
	erc1155TokenID *big.Int,
	assetGroupTreeNumber *big.Int,
	assetGroupMerkleProof *core.MerkleProof,
	isFungible bool,
) (*core.ProofResult, error) {
	addrBig := addressToBigInt(erc1155ContractAddr)

	if isFungible {
		return gnarkClient.Erc1155FungibleJoinSplitProof(
			stMessage,
			[]*big.Int{amountOrOne},
			[]core.KeyPair{keyIn},
			[]*big.Int{wtSaltIn},
			[]*big.Int{amountOrOne},
			[]core.KeyPair{keyOut},
			[][]byte{nil}, // legacy wrapper: caller must use direct API for non-interactive delivery
			merkleDepth,
			[]*core.MerkleProof{merkleProof},
			[]*big.Int{stTreeNumber},
			addrBig,
			erc1155TokenID,
			assetGroupTreeNumber,
			assetGroupMerkleProof,
		)
	}

	return gnarkClient.Erc1155NonFungibleOwnershipProof(
		stMessage,
		amountOrOne,
		keyIn,
		wtSaltIn,
		keyOut,
		nil, // legacy wrapper: caller must use direct API for non-interactive delivery
		merkleDepth,
		merkleProof,
		stTreeNumber,
		addrBig,
		erc1155TokenID,
		assetGroupTreeNumber,
		assetGroupMerkleProof,
	)
}

// GenerateErc1155JoinSplitProof generates a fungible ERC-1155 JoinSplit proof.
// Corresponds to: generateErc1155JoinSplitProof(message, inAmounts, inKeys,
//
//	outAmounts, outKeys, merkleDepth, merkleProofs, merkleRoots, treeNumbers,
//	erc1155ContractAddress, erc1155TokenId, fungibilityMerkleTreeNumber, fungibilityMerkleProof)
func GenerateErc1155JoinSplitProof(
	gnarkClient *core.GnarkClient,
	stMessage *big.Int,
	wtValuesIn []*big.Int,
	keysIn []core.KeyPair,
	wtSaltsIn []*big.Int,
	wtValuesOut []*big.Int,
	keysOut []core.KeyPair,
	merkleDepth int,
	merkleProofs []*core.MerkleProof,
	stTreeNumbers []*big.Int,
	erc1155ContractAddr common.Address,
	erc1155TokenID *big.Int,
	assetGroupTreeNumber *big.Int,
	assetGroupMerkleProof *core.MerkleProof,
) (*core.ProofResult, error) {
	// Build nil encap key slice — legacy wrapper, caller must use direct API for non-interactive delivery
	nilEncapKeys := make([][]byte, len(keysOut))
	return gnarkClient.Erc1155FungibleJoinSplitProof(
		stMessage,
		wtValuesIn,
		keysIn,
		wtSaltsIn,
		wtValuesOut,
		keysOut,
		nilEncapKeys,
		merkleDepth,
		merkleProofs,
		stTreeNumbers,
		addressToBigInt(erc1155ContractAddr),
		erc1155TokenID,
		assetGroupTreeNumber,
		assetGroupMerkleProof,
	)
}

// GenerateErc1155BatchProof generates an ERC-1155 batch proof.
// NOTE: the underlying gnark client method (Erc1155BatchProof) is not yet ported to Go.
// Corresponds to: generateErc1155BatchProof(message, amounts, inKeys, outKeys,
//
//	merkleDepth, merkleProofs, merkleRoots, treeNumbers, erc1155ContractAddress,
//	erc1155TokenIds, assetGroup_treeNumbers, assetGroup_merkleProofs)
func GenerateErc1155BatchProof(
	_ *core.GnarkClient,
	_ *big.Int, // stMessage
	_ []*big.Int, // amounts
	_ []core.KeyPair, // keysIn
	_ []core.KeyPair, // keysOut
	_ int, // merkleDepth
	_ []*core.MerkleProof,
	_ []*big.Int, // stTreeNumbers
	_ common.Address, // erc1155ContractAddr
	_ []*big.Int, // erc1155TokenIDs
	_ []*big.Int, // assetGroupTreeNumbers
	_ []*core.MerkleProof,
) (*core.ProofResult, error) {
	// TODO: implement when Erc1155BatchProof is ported to the gnark client.
	return nil, fmt.Errorf("GenerateErc1155BatchProof: Erc1155BatchProof not yet implemented in Go")
}

// WithdrawEnygma generates an ERC-20 JoinSplit proof for an Enygma withdrawal.
// Corresponds to: withdrawEnygma(publicKey, amount, withdrawKey, enygmaAddress,
//
//	commitmentProof, merkleTree)
func WithdrawEnygma(
	gnarkClient *core.GnarkClient,
	outSpendPk *big.Int,    // pk_spend of the output recipient
	outViewEncapKey []byte, // view encapsulation key of the output recipient
	amount *big.Int,
	withdrawKey core.KeyPair,
	wtSaltIn *big.Int,
	tokenId *big.Int,
	commitmentProof *core.MerkleProof,
	merkleDepth int,
	stTreeNumber *big.Int,
	use10_2 bool,
) (*core.ProofResult, error) {
	dummySpendKp, err := core.NewSpendKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate dummy spend key pair: %w", err)
	}
	dummyViewKp, err := core.NewViewKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate dummy view key pair: %w", err)
	}
	dummyKey := core.KeyPair{PrivateKey: dummySpendKp.PrivateKey, PublicKey: dummySpendKp.PublicKey}

	return gnarkClient.Erc20JoinSplitProof(
		big.NewInt(0),
		[]*big.Int{amount, big.NewInt(0)},
		[]core.KeyPair{withdrawKey, dummyKey},
		[]*big.Int{wtSaltIn, big.NewInt(0)},
		[]*big.Int{amount, big.NewInt(0)},
		[]*big.Int{outSpendPk, dummySpendKp.PublicKey},
		[][]byte{outViewEncapKey, dummyViewKp.EncapsKey},
		merkleDepth,
		[]*core.MerkleProof{commitmentProof, nil}, // nil safe: dummy input has value=0
		[]*big.Int{stTreeNumber, big.NewInt(0)},
		tokenId,
		use10_2,
	)
}

// --- Non-interactive scanning ---

// Erc20Note holds the decrypted contents of an EncryptedNote event emitted on-chain.
// SaltBField is the saltB reduced to the SNARK scalar field; use it as wt_saltIn
// when constructing a future JoinSplit proof that spends this note.
type Erc20Note struct {
	VaultId    *big.Int
	Commitment *big.Int
	TokenId    *big.Int
	Amount     *big.Int
	SaltBField *big.Int // saltB mod SNARK_SCALAR_FIELD; witness input for future proofs
}

// ScanErc20Notes scans EncryptedNote events emitted by a vault contract between
// fromBlock and toBlock. For each event it attempts to decapsulate cipherText with
// dk (Bob's ML-KEM view decapsulation key) and then decrypt encTxData. Events
// that pass AEAD authentication belong to Bob and are returned as Erc20Note values.
func ScanErc20Notes(
	client *ethclient.Client,
	vaultABI abi.ABI,
	vaultAddr common.Address,
	fromBlock, toBlock uint64,
	dk *mlkem.DecapsulationKey768,
) ([]*Erc20Note, error) {
	event, ok := vaultABI.Events["EncryptedNote"]
	if !ok {
		return nil, fmt.Errorf("EncryptedNote event not found in ABI")
	}

	query := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(fromBlock),
		ToBlock:   new(big.Int).SetUint64(toBlock),
		Addresses: []common.Address{vaultAddr},
		Topics:    [][]common.Hash{{event.ID}},
	}

	logs, err := client.FilterLogs(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("FilterLogs failed: %w", err)
	}

	var notes []*Erc20Note
	for _, log := range logs {
		// Topics: [0] = event sig, [1] = vaultId (indexed), [2] = commitment (indexed)
		if len(log.Topics) < 3 {
			continue
		}
		vaultId := new(big.Int).SetBytes(log.Topics[1].Bytes())
		commitment := new(big.Int).SetBytes(log.Topics[2].Bytes())

		// Decode non-indexed fields: cipherText, encTxData
		values := make(map[string]interface{})
		if err := vaultABI.UnpackIntoMap(values, "EncryptedNote", log.Data); err != nil {
			continue
		}
		cipherText, ok1 := values["cipherText"].([]byte)
		encTxData, ok2 := values["encTxData"].([]byte)
		if !ok1 || !ok2 {
			continue
		}

		// Decapsulate the ML-KEM capsule → raw shared secret ss
		ss, err := core.Decapsulate(dk, cipherText)
		if err != nil {
			continue // ciphertext malformed or not from a valid encapsulation
		}

		// HKDF-derive commitment salt and AES-GCM encryption key
		saltB, err := core.DerivePaymentSalt(ss)
		if err != nil {
			continue
		}
		encKey, err := core.DerivePaymentKey(ss)
		if err != nil {
			continue
		}

		// Try to decrypt (tokenId||amount) — AES-GCM auth failure means note is not ours
		tokenId, amount, err := core.DecryptPayload(encKey, encTxData)
		if err != nil {
			continue
		}

		saltBField := core.SaltBToField(saltB)
		notes = append(notes, &Erc20Note{
			VaultId:    vaultId,
			Commitment: commitment,
			TokenId:    tokenId,
			Amount:     amount,
			SaltBField: saltBField,
		})
	}

	return notes, nil
}
