package engine

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
)

// SetUsdrFixedFee sets EnygmaDvp's governance USDr fixed fee from the
// deployer's account and returns the fee the contract then reports.
func SetUsdrFixedFee(cfg Config, fee *big.Int) (*big.Int, error) {
	c, err := dialChain(cfg)
	if err != nil {
		return nil, err
	}
	defer c.close()
	dvpABI, err := loadABI(cfg.RetailRoot, "EnygmaDvp")
	if err != nil {
		return nil, err
	}
	dvp := bind.NewBoundContract(c.dvpAddr, dvpABI, c.client, c.client, c.client)
	if _, err := c.transact(context.Background(), cfg.FunderKeyHex, dvp, "setUsdrFixedFee", fee); err != nil {
		return nil, err
	}
	var out []any
	if err := dvp.Call(&bind.CallOpts{}, &out, "usdrFixedFeeAmount"); err != nil {
		return nil, fmt.Errorf("usdrFixedFeeAmount: %w", err)
	}
	return out[0].(*big.Int), nil
}
