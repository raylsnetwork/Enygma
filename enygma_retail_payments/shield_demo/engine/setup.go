package engine

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	tags "github.com/raylsnetwork/enygma_retail_payments/private_tags/src"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// gasFunding is the ETH each new account gets for its public steps.
var gasFunding = new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

func (e *Engine) checkPrereqs(ctx context.Context, s *step) error {
	if missing := Unreachable(e.cfg); len(missing) > 0 {
		return fmt.Errorf("services not reachable: %s", strings.Join(missing, ", "))
	}
	s.log("", "✓ chain · gnark server · relayer are reachable")

	c, err := dialChain(e.cfg)
	if err != nil {
		return err
	}
	e.chain = c

	info, err := e.relayer.info(ctx)
	if err != nil {
		return err
	}
	if info.RelayerAddr == "" {
		return fmt.Errorf("relayer /relay/info returned no relayer address")
	}
	c.relayerAddr = common.HexToAddress(info.RelayerAddr)
	c.tagRegistryAddr = common.HexToAddress(info.TagRegistryAddr)
	c.channelRegistryAddr = common.HexToAddress(info.TagChannelRegistryAddr)
	if e.opts.Tags && (c.tagRegistryAddr == (common.Address{}) || c.channelRegistryAddr == (common.Address{})) {
		return fmt.Errorf("the relayer has no tag registries (set RELAYER_TAG_REGISTRY_ADDR and RELAYER_TAG_CHANNEL_REGISTRY_ADDR), needed with tags on")
	}
	// The relayer takes USDr fees only if it publishes a fee spend key; the
	// fan-out's USDr-fee route checks for it.
	e.relayerFeePk = nil
	if pk, ok := new(big.Int).SetString(info.FeeSpendPubKey, 10); ok {
		e.relayerFeePk = pk
	}
	if e.usdrFixedFee, err = c.usdrFixedFee(); err != nil {
		return err
	}
	if e.startBlock, err = c.client.BlockNumber(ctx); err != nil {
		return err
	}
	e.startBlock++
	e.startChannel = 0
	if c.channelRegistryAddr != (common.Address{}) {
		if e.startChannel, err = tags.GetChannelCount(c.client, c.channelRegistryAddr); err != nil {
			return err
		}
	}

	s.proto("relayerAddr", c.relayerAddr.Hex())
	s.proto("dvpAddr", c.dvpAddr.Hex())
	s.proto("vaultAddr", c.vaultAddr.Hex())
	s.proto("erc20Addr", c.erc20Addr.Hex())
	s.proto("registryAddr", c.registryAddr.Hex())
	s.proto("usdrVaultAddr", c.usdrVaultAddr.Hex())
	s.proto("usdrFixedFee", e.usdrFixedFee.String()+" USDr")
	s.log("chain", "EnygmaDvp:      %s", c.dvpAddr.Hex())
	s.log("chain", "Erc20CoinVault: %s", c.vaultAddr.Hex())
	s.log("chain", "UserRegistry:   %s", c.registryAddr.Hex())
	s.log("chain", "UsdrCoinVault:  %s  (fixed fee %s USDr)", c.usdrVaultAddr.Hex(), e.usdrFixedFee)
	s.log("chain", "relayer:        %s", c.relayerAddr.Hex())
	if e.opts.Tags {
		s.proto("tagRegistryAddr", c.tagRegistryAddr.Hex())
		s.proto("tagChannelAddr", c.channelRegistryAddr.Hex())
		s.log("chain", "TagRegistry:    %s", c.tagRegistryAddr.Hex())
		s.log("chain", "TagChannelReg.: %s", c.channelRegistryAddr.Hex())
	}
	s.done("Chain, gnark server and relayer are running")
	return nil
}

func (e *Engine) generateKeys(ctx context.Context, s *step) error {
	auditor, err := rpcore.NewAuditorKeyPair()
	if err != nil {
		return err
	}
	e.auditor = auditor
	s.proto("auditorPk", "0x"+hex.EncodeToString(auditor.EncapsKey))
	s.log("key", "auditor ML-KEM-768 ek: 0x%s…", hex.EncodeToString(auditor.EncapsKey)[:16])

	funder, err := crypto.HexToECDSA(e.cfg.FunderKeyHex)
	if err != nil {
		return fmt.Errorf("parse funder key: %w", err)
	}
	e.actors = nil
	newActor := func(name string, role Role, keyHex string, addr common.Address) error {
		spend, err := rpcore.NewSpendKeyPair()
		if err != nil {
			return err
		}
		view, err := rpcore.NewViewKeyPair()
		if err != nil {
			return err
		}
		a := &Actor{ID: len(e.actors), Name: name, Role: role, Address: addr, Spend: spend, View: view, keyHex: keyHex}
		e.actors = append(e.actors, a)
		s.actor(a, "name", name)
		s.actor(a, "role", string(role))
		s.actor(a, "ethAddr", addr.Hex())
		s.actor(a, "pkSpend", "0x"+spend.PublicKey.Text(16))
		s.actor(a, "pkView", "0x"+hex.EncodeToString(view.EncapsKey))
		s.log("key", "%-7s pk_spend 0x%s…  ML-KEM ek 0x%s…", name,
			spend.PublicKey.Text(16)[:12], hex.EncodeToString(view.EncapsKey)[:12])
		return nil
	}
	if err := newActor("Issuer", Issuer, e.cfg.FunderKeyHex, crypto.PubkeyToAddress(funder.PublicKey)); err != nil {
		return err
	}
	names := append([]string{"Sender"}, participantNames[:]...)
	for i, name := range names {
		role := Participant
		if i == 0 {
			role = Sender
		}
		keyHex, addr, err := newAccount()
		if err != nil {
			return err
		}
		if err := newActor(name, role, keyHex, addr); err != nil {
			return err
		}
	}

	var toFund []common.Address
	for _, a := range e.registrable() {
		toFund = append(toFund, a.Address)
	}
	if err := e.chain.fund(ctx, e.cfg.FunderKeyHex, gasFunding, toFund); err != nil {
		return err
	}
	s.log("chain", "funded %d new accounts with 1 ETH of gas each", len(toFund))
	s.done("BabyJubJub spend keys and ML-KEM-768 view keys for the issuer, the sender and %d participants", K)
	return nil
}

// register publishes the sender's and each participant's keys in the
// UserRegistry, with their view key encrypted to the auditor.
func (e *Engine) register(ctx context.Context, s *step) error {
	for _, a := range e.registrable() {
		mlKemCt, aesCt, err := rpcore.EncryptViewKeyForAuditor(e.auditor.EncapsKey, a.View.DecapsKey)
		if err != nil {
			return fmt.Errorf("%s: encrypt view key for auditor: %w", a.Name, err)
		}
		auth, err := e.chain.auth(a.keyHex)
		if err != nil {
			return err
		}
		if err := rpcore.Register(e.chain.client, auth, e.chain.registryAddr,
			a.Spend.PublicKey, a.View.EncapsKey, mlKemCt, aesCt); err != nil {
			return fmt.Errorf("%s: %w", a.Name, err)
		}
		s.actor(a, "status", "Registered")
		s.log("chain", "UserRegistry.register(%s) — view key encrypted to the auditor (%d + %d bytes)", a.Name, len(mlKemCt), len(aesCt))
	}
	s.done("Sender and %d participants registered, view keys encrypted to the auditor", K)
	return nil
}
