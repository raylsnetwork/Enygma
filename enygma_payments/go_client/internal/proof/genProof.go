package proof

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"

	"enygma/config"
	enygma "enygma/contracts"
	"enygma/internal/types"
)

// GenerateProof POSTs a transfer request to the gnark proving server and
// returns the decoded proof.
//
// Fix L-09: this used to have no error return at all — transport and
// JSON-syntax errors panicked, but a well-formed HTTP 400
// {"error": "..."} response (the gnark server's own failure format)
// unmarshalled cleanly into types.Response with Proof/PublicSignal left
// nil, so a total prover failure was structurally indistinguishable from
// success and the caller forwarded eight empty strings to the relayer
// with a Bearer token. Now the status code is checked before decoding,
// and the decoded proof's shape (exactly 8 elements) is validated before
// returning — both failure modes now surface as an error naming this
// function, not a plausible-looking downstream error three steps later.
func GenerateProof(args *types.TransactionArgs, nullifier *big.Int,
	blockHash *big.Int, publicKey []*big.Int,
	previousCommit []enygma.IEnygmaPoint, txCommit []enygma.IEnygmaPoint,
	txValue []*big.Int, txRandom []*big.Int, secrets []*big.Int,
	k_index []*big.Int, fingerPrint [][]*big.Int, tagMessage []*big.Int,
	domainId *big.Int, cfg *config.Config) (*types.Response, error) {

	var pkFinal []string
	var prevCommitFinal [][]string
	var txCommitFinal [][]string

	for _, pkVal := range publicKey {
		pkFinal = append(pkFinal, pkVal.String())
	}

	for _, value := range previousCommit {
		prevCommitFinal = append(prevCommitFinal, []string{value.C1.String(), value.C2.String()})
	}

	for _, commVal := range txCommit {
		txCommitFinal = append(txCommitFinal, []string{commVal.C1.String(), commVal.C2.String()})
	}

	fingerPrintFinal := make([][]string, len(fingerPrint))
	for i, row := range fingerPrint {
		fingerPrintFinal[i] = convertBigIntsToStrings(row)
	}
	sharedSecrets := convertBigIntsToStrings(secrets)

	txValuesString := convertBigIntsToStrings(txValue)
	txRandomString := convertBigIntsToStrings(txRandom)
	kIndexString := convertBigIntsToStrings(k_index)
	tagMessageString := convertBigIntsToStrings(tagMessage)

	jsonInfo := types.Proof{
		FingerPrintofSharedSecrets: fingerPrintFinal,
		PublicKey:                  pkFinal,
		PreviousCommit:             prevCommitFinal,
		TxCommit:                   txCommitFinal,
		BlockNumber:                blockHash.String(),
		AnonymitySet:               kIndexString,
		MessageTags:                tagMessageString,
		Nullifier:                  nullifier.String(),
		SenderID:                   strconv.FormatInt(int64(args.SenderId), 10),
		SharedSecrets:              sharedSecrets,
		SecretKey:                  args.Sk.String(),
		PreviousSenderBalance:      args.PreviousV.String(),
		PreviousSenderRandomValue:  args.PreviousR.String(),
		TxValues:                   txValuesString,
		TxRandomValues:             txRandomString,
		SenderTxValue:              args.Value.String(),
		DomainId:                   domainId.String(),
	}

	jsonData, err := json.Marshal(jsonInfo)
	if err != nil {
		return nil, fmt.Errorf("marshal proof request: %w", err)
	}

	request, err := http.NewRequest(http.MethodPost, cfg.ProofServerURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("build proof request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("contact proving server: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read proving server response: %w", err)
	}

	// Fix L-09: the gnark server reports every failure (bad request,
	// witness build failure, proof generation failure, self-verify
	// failure) as a non-200 status with a JSON {"error": "..."} body —
	// never a 200 with an empty proof. Checking StatusCode here is what
	// actually distinguishes "no proof" from "a proof no one asked for".
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("proving server returned %d: %s", response.StatusCode, body)
	}

	var result types.Response
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode proving server response: %w", err)
	}

	// Defense in depth even on a 200: a proof isn't usable unless it's
	// actually shaped like one.
	if len(result.Proof) != 8 {
		return nil, fmt.Errorf("proving server returned a malformed proof: got %d elements, want 8", len(result.Proof))
	}
	if len(result.PublicSignal) == 0 {
		return nil, fmt.Errorf("proving server returned an empty public signal")
	}

	return &result, nil
}

func convertBigIntsToStrings(bigInts []*big.Int) []string {
	strings := make([]string, len(bigInts))
	for i, bigInt := range bigInts {
		strings[i] = bigInt.String()
	}
	return strings
}
