package engine

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"enygma_retail_payments/layout"
	dvpcore "github.com/raylsnetwork/enygma_dvp/src/core"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"
)

// grantUsdr has the issuer mint public USDr and shield a UsdrGrant note to
// the sender and each participant, encrypted to their view key; each then
// finds theirs by scanning the USDr vault.
func (e *Engine) grantUsdr(ctx context.Context, s *step) error {
	c, issuer := e.chain, e.issuer()
	recipients := e.registrable()
	grant := big.NewInt(UsdrGrant)
	total := new(big.Int).Mul(grant, big.NewInt(int64(len(recipients))))

	if _, err := c.transact(ctx, issuer.keyHex, c.usdrErc20, "mint", issuer.Address, total); err != nil {
		return err
	}
	if _, err := c.transact(ctx, issuer.keyHex, c.usdrErc20, "approve", c.usdrVaultAddr, total); err != nil {
		return err
	}
	s.log("chain", "UsdrERC20.mint(%s) → Issuer (public), approved for the USDr vault", total)
	for _, a := range recipients {
		n, receipt, err := c.depositNote(ctx, issuer.keyHex, c.usdrVault, c.usdrVaultAddr, a, grant)
		if err != nil {
			s.hopFailed(ActorNode(issuer.ID), NodeUsdrVault, "grant USDr", err)
			return fmt.Errorf("USDr grant to %s: %w", a.Name, err)
		}
		s.hop(ActorNode(issuer.ID), NodeUsdrVault, "grant USDr", fmt.Sprintf("a %s USDr note for %s, encrypted to their view key, block %d", grant, a.Name, receipt.BlockNumber))
		s.log("chain", "UsdrCoinVault.depositV2(%s) → %s's note 0x%s…  block %d", grant, a.Name, n.commitment.Text(16)[:12], receipt.BlockNumber)
	}
	s.actor(issuer, "public", "0 USDr ("+total.String()+" minted and shielded)")

	events, err := c.noteEvents(ctx, c.usdrVaultAddr, usdrVaultId)
	if err != nil {
		return err
	}
	for _, a := range recipients {
		if found := a.scan(events, &a.usdr); found != 1 {
			return fmt.Errorf("%s found %d USDr notes by scanning, want 1", a.Name, found)
		}
		s.hop(NodeUsdrVault, ActorNode(a.ID), "scan", fmt.Sprintf("%s tried %d notes with their view key and opened their grant", a.Name, len(events)))
		s.actor(a, "usdr", a.usdr.balance().String()+" USDr")
	}
	s.log("key", "sender and %d participants each decrypted their USDr note with their ML-KEM view key", K)
	s.done("Issuer gave the sender and %d participants %s USDr each of shielded fee credit", K, grant)
	return nil
}

// usdrFeeLeg is a proved UsdrFee spend: payer's note in, the relayer's fee
// note (output 0) and payer's change (output 1) out.
type usdrFeeLeg struct {
	proof   [8]string
	signals [layout.UsdrFeeLen]string
	feeSalt *big.Int
	feeNote *big.Int
	in      *note
	change  *note
	// proveTime is how long the gnark server took to prove it.
	proveTime time.Duration
}

// proveUsdrFee proves payer spending their smallest USDr note that covers
// the relayer's fixed USDr fee to pay it, the rest back to payer as change.
func (e *Engine) proveUsdrFee(ctx context.Context, payer *Actor) (*usdrFeeLeg, error) {
	c := e.chain
	prev, err := payer.usdr.spendable(payer.Name, "USDr", e.usdrFixedFee)
	if err != nil {
		return nil, err
	}
	changeAmt := new(big.Int).Sub(prev.amount, e.usdrFixedFee)

	tree, err := c.vaultTree(ctx, c.usdrVaultAddr)
	if err != nil {
		return nil, err
	}
	mp, err := tree.GenerateProof(prev.commitment)
	if err != nil {
		return nil, fmt.Errorf("merkle proof for %s's USDr note: %w", payer.Name, err)
	}
	treeNum := big.NewInt(int64(mp.TreeNumber))
	nullifier, err := dvpcore.GetNullifierWithTree(payer.Spend.PrivateKey, treeNum, mp.Indices, merkleDepth)
	if err != nil {
		return nil, err
	}
	feeSalt, err := dvpcore.RandomInField()
	if err != nil {
		return nil, err
	}
	changeSalt, err := dvpcore.RandomInField()
	if err != nil {
		return nil, err
	}
	feeNote, err := rpcore.Erc20CommitmentV2(e.relayerFeePk, feeSalt, e.usdrFixedFee, tokenId)
	if err != nil {
		return nil, err
	}
	change := &note{salt: changeSalt, amount: changeAmt}
	if change.commitment, err = rpcore.Erc20CommitmentV2(payer.Spend.PublicKey, changeSalt, changeAmt, tokenId); err != nil {
		return nil, err
	}

	var path [merkleDepth]string
	for i := range path {
		path[i] = mp.Elements[i].String()
	}
	req := map[string]any{
		"stMessage":            "0",
		"stTreeNumbers":        [1]string{treeNum.String()},
		"stMerkleRoots":        [1]string{mp.Root.String()},
		"stNullifiers":         [1]string{nullifier.String()},
		"stCommitmentsOut":     [2]string{feeNote.String(), change.commitment.String()},
		"stContractAddress":    new(big.Int).SetBytes(c.usdrVaultAddr.Bytes()).String(),
		"stFee":                e.usdrFixedFee.String(),
		"stTokenId":            tokenId.String(),
		"wtPrivateKeysIn":      [1]string{payer.Spend.PrivateKey.String()},
		"wtValuesIn":           [1]string{prev.amount.String()},
		"wtSaltsIn":            [1]string{prev.salt.String()},
		"wtPathElements":       [1][merkleDepth]string{path},
		"wtPathIndices":        [1]string{mp.Indices.String()},
		"wtSpendPublicKeysOut": [2]string{e.relayerFeePk.String(), payer.Spend.PublicKey.String()},
		"wtValuesOut":          [2]string{e.usdrFixedFee.String(), changeAmt.String()},
		"wtSaltsOut":           [2]string{feeSalt.String(), changeSalt.String()},
	}
	var out struct {
		Proof        []*big.Int `json:"proof"`
		PublicSignal []*big.Int `json:"publicSignal"`
	}
	proveStart := time.Now()
	if err := postJSON(ctx, e.cfg.GnarkURL+"/proof/usdrFee", nil, req, &out); err != nil {
		return nil, err
	}
	proveTime := time.Since(proveStart)
	if len(out.Proof) != 8 || len(out.PublicSignal) != layout.UsdrFeeLen {
		return nil, fmt.Errorf("usdrFee proof: got %d proof elements and %d signals, want 8 and %d",
			len(out.Proof), len(out.PublicSignal), layout.UsdrFeeLen)
	}
	leg := &usdrFeeLeg{feeSalt: feeSalt, feeNote: feeNote, in: prev, change: change, proveTime: proveTime}
	for i, v := range out.Proof {
		leg.proof[i] = v.String()
	}
	for i, v := range out.PublicSignal {
		leg.signals[i] = v.String()
	}
	return leg, nil
}
