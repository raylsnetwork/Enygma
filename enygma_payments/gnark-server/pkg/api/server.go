package api

import (
    
    "github.com/gin-gonic/gin"
    "enygma-server/config"
    "enygma-server/pkg/circuits/enygma" 
    "enygma-server/pkg/circuits/withdraw"
    "enygma-server/pkg/circuits/deposit"
    
)


func NewServer(cfg *config.Config) *gin.Engine {
    r := gin.Default()
 
    r.POST("/proof/enygma", enygma.NewHandler(cfg.EnygmaPk, cfg.EnygmaVk))
    r.POST("/proof/withdraw/1",  withdraw.NewHandler(cfg.WithdrawPk1,  cfg.WithdrawVk1,1))
    r.POST("/proof/withdraw/2",  withdraw.NewHandler(cfg.WithdrawPk2,  cfg.WithdrawVk2,2))
    r.POST("/proof/withdraw/3",  withdraw.NewHandler(cfg.WithdrawPk3,  cfg.WithdrawVk3,3))
    r.POST("/proof/withdraw/4",  withdraw.NewHandler(cfg.WithdrawPk4,  cfg.WithdrawVk4,4))
    r.POST("/proof/withdraw/5",  withdraw.NewHandler(cfg.WithdrawPk5,  cfg.WithdrawVk5,5))
    r.POST("/proof/withdraw/6",  withdraw.NewHandler(cfg.WithdrawPk6,  cfg.WithdrawVk6,6))
    r.POST("/proof/deposit", deposit.NewHandler(cfg.DepositPk, cfg.DepositVk))
    return r
}

