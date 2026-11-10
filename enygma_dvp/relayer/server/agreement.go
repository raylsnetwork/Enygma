package server

import (
	"fmt"
	"math/big"
	"net/http"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
)

// On-chain trade agreement (DvpAgreementRegistry). The relayer submits offers
// and replies for traders so that msg.sender does not reveal who is trading.
// Both payloads are opaque ciphertexts; the relayer only checks the limits the
// contract enforces, so it does not pay for a submission that would revert.

// Limits mirrored from DvpAgreementRegistry.
const (
	agreementC1Size       = 1088
	agreementCtxtMax      = 4096
	agreementBitmapMax    = 16384
	agreementTagWindowMax = 16
)

// RelayOfferRequest is the body for POST /relay/offer.
// Calls DvpAgreementRegistry.openOffer(c1, c2, bitmap); all fields are hex.
type RelayOfferRequest struct {
	C1     string `json:"c1"     binding:"required"`
	C2     string `json:"c2"     binding:"required"`
	Bitmap string `json:"bitmap" binding:"required"`
}

// RelayReplyRequest is the body for POST /relay/reply.
// Calls DvpAgreementRegistry.publishReply(startBlock, tags, ctxt): tags[i] is
// the tag for block startBlock+i, and the contract publishes the one for the
// block the transaction lands in.
type RelayReplyRequest struct {
	StartBlock uint64   `json:"startBlock"`
	Tags       []string `json:"tags" binding:"required"`
	Ctxt       string   `json:"ctxt" binding:"required"`
}

func validateOffer(c1, c2, bitmap []byte) error {
	if len(c1) != agreementC1Size {
		return fmt.Errorf("c1 must be %d bytes (ML-KEM-768 ciphertext), got %d", agreementC1Size, len(c1))
	}
	if len(c2) > agreementCtxtMax {
		return fmt.Errorf("c2 exceeds %d bytes", agreementCtxtMax)
	}
	if len(bitmap) > agreementBitmapMax {
		return fmt.Errorf("bitmap exceeds %d bytes", agreementBitmapMax)
	}
	for _, b := range bitmap {
		if b != 0 {
			return nil
		}
	}
	return fmt.Errorf("bitmap is empty: set at least the recipient's bit")
}

func parseReply(req *RelayReplyRequest) ([][32]byte, []byte, error) {
	if len(req.Tags) == 0 || len(req.Tags) > agreementTagWindowMax {
		return nil, nil, fmt.Errorf("tags must hold 1 to %d entries, got %d", agreementTagWindowMax, len(req.Tags))
	}
	tags := make([][32]byte, len(req.Tags))
	for i, s := range req.Tags {
		b, err := decodeHex(s, fmt.Sprintf("tags[%d]", i))
		if err != nil {
			return nil, nil, err
		}
		if len(b) != 32 {
			return nil, nil, fmt.Errorf("tags[%d] must be 32 bytes, got %d", i, len(b))
		}
		copy(tags[i][:], b)
	}
	ctxt, err := decodeHex(req.Ctxt, "ctxt")
	if err != nil {
		return nil, nil, err
	}
	if len(ctxt) > agreementCtxtMax {
		return nil, nil, fmt.Errorf("ctxt exceeds %d bytes", agreementCtxtMax)
	}
	return tags, ctxt, nil
}

func (h *Handler) agreementAvailable(c *gin.Context) bool {
	if h.agreementAddr == (common.Address{}) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "DvpAgreementRegistry is not configured (set RELAYER_AGREEMENT_ADDR or deploy it)",
		})
		return false
	}
	return true
}

// RelayOffer handles POST /relay/offer.
func (h *Handler) RelayOffer(c *gin.Context) {
	if !h.agreementAvailable(c) {
		return
	}
	var req RelayOfferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c1, err := decodeHex(req.C1, "c1")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c2, err := decodeHex(req.C2, "c2")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	bitmap, err := decodeHex(req.Bitmap, "bitmap")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateOffer(c1, c2, bitmap); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	receipt, err := h.transactTo(h.agreementAddr, h.agreementABI, "openOffer", c1, c2, bitmap)
	if err != nil {
		relayError(c, "openOffer", err)
		return
	}
	c.JSON(http.StatusOK, RelayResponse{
		TxHash:      receipt.TxHash.Hex(),
		BlockNumber: receipt.BlockNumber.Uint64(),
		GasUsed:     receipt.GasUsed,
	})
}

// RelayReply handles POST /relay/reply.
func (h *Handler) RelayReply(c *gin.Context) {
	if !h.agreementAvailable(c) {
		return
	}
	var req RelayReplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tags, ctxt, err := parseReply(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	receipt, err := h.transactTo(h.agreementAddr, h.agreementABI, "publishReply",
		new(big.Int).SetUint64(req.StartBlock), tags, ctxt)
	if err != nil {
		relayError(c, "publishReply", err)
		return
	}
	c.JSON(http.StatusOK, RelayResponse{
		TxHash:      receipt.TxHash.Hex(),
		BlockNumber: receipt.BlockNumber.Uint64(),
		GasUsed:     receipt.GasUsed,
	})
}
