package api

import (
	"github.com/gin-gonic/gin"

	"gnark_server/server/circuits/payment"
	"gnark_server/server/circuits/payment2in"
	"gnark_server/server/circuits/paymentFee"
	"gnark_server/server/circuits/paymentRelayer"
	"gnark_server/server/circuits/privateMint"
	"gnark_server/server/config"
)

func NewServer(cfg *config.Config) *gin.Engine {
	r := gin.Default()

	r.POST("/proof/payment", payment.NewHandler(cfg.PaymentPk, cfg.PaymentVk))
	r.POST("/proof/payment2in", payment2in.NewHandler(cfg.Payment2inPk, cfg.Payment2inVk))
	r.POST("/proof/paymentFee", paymentFee.NewHandler(cfg.PaymentFeePk, cfg.PaymentFeeVk))
	r.POST("/proof/paymentRelayer", paymentRelayer.NewHandler(cfg.PaymentRelayerPk, cfg.PaymentRelayerVk))
	r.POST("/proof/privateMint", privateMint.NewHandler(cfg.PrivateMintPk, cfg.PrivateMintVk))

	return r
}
