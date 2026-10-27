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
	if _, err := c.transact(context.Background(), cfg.FunderKeyHex, c.dvp, "setUsdrFixedFee", fee); err != nil {
		return nil, err
	}
	return c.usdrFixedFee()
}

// usdrFixedFee is the USDr fee EnygmaDvp requires on every USDr-fee payment.
func (c *chain) usdrFixedFee() (*big.Int, error) {
	var out []any
	if err := c.dvp.Call(&bind.CallOpts{}, &out, "usdrFixedFeeAmount"); err != nil {
		return nil, fmt.Errorf("usdrFixedFeeAmount: %w", err)
	}
	return out[0].(*big.Int), nil
}
