package api

import (
	"github.com/gin-gonic/gin"

	"enygma_retail_payments/gnark_circuits/server/circuits/payment"
	"enygma_retail_payments/gnark_circuits/server/circuits/payment2in"
	"enygma_retail_payments/gnark_circuits/server/circuits/paymentFee"
	"enygma_retail_payments/gnark_circuits/server/circuits/paymentRelayerFeePublic"
	"enygma_retail_payments/gnark_circuits/server/circuits/privateMint"
	"enygma_retail_payments/gnark_circuits/server/circuits/usdrFee"
	"enygma_retail_payments/gnark_circuits/server/config"
)

func NewServer(cfg *config.Config) *gin.Engine {
	r := gin.Default()

	r.POST("/proof/payment", payment.NewHandler(cfg.PaymentPk, cfg.PaymentVk))
	r.POST("/proof/payment2in", payment2in.NewHandler(cfg.Payment2inPk, cfg.Payment2inVk))
	r.POST("/proof/paymentFee", paymentFee.NewHandler(cfg.PaymentFeePk, cfg.PaymentFeeVk))
	r.POST("/proof/paymentRelayerFeePublic", paymentRelayerFeePublic.NewHandler(cfg.PaymentRelayerFeePublicPk, cfg.PaymentRelayerFeePublicVk))
	r.POST("/proof/usdrFee", usdrFee.NewHandler(cfg.UsdrFeePk, cfg.UsdrFeeVk))
	r.POST("/proof/privateMint", privateMint.NewHandler(cfg.PrivateMintPk, cfg.PrivateMintVk))

	return r
}
