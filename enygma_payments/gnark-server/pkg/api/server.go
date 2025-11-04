package api

import (
    
    "github.com/gin-gonic/gin"
    "enygma-server/config"
    "enygma-server/pkg/circuits/enygma" 
    "enygma-server/pkg/circuits/withdraw"
    "enygma-server/pkg/circuits/deposit"
    // "github.com/consensys/gnark/frontend"
	// "github.com/consensys/gnark-crypto/ecc"
    // "github.com/consensys/gnark/frontend/cs/r1cs"
	// "github.com/consensys/gnark/backend/groth16"
    // "os"
    // "log"
   

)


func NewServer(cfg *config.Config) *gin.Engine {
    r := gin.Default()

    // var circuit enygma.EnygmaCircuit
    // ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
    // pk, vk, err := groth16.Setup(ccs)
    // fpk, err := os.Create("EnygmaPk.key")
    // if err != nil {
    //     log.Fatalf("could not create proving.key: %v", err)
    // }
    // defer fpk.Close()
    // if _, err := pk.WriteTo(fpk); err != nil {
    //     log.Fatalf("failed to write proving key: %v", err)
    // }

    // // 4) save the verifying key
    // fvk, err := os.Create("EnygmaVk.key")
    // if err != nil {
    //     log.Fatalf("could not create verifying.key: %v", err)
    // }
    // defer fvk.Close()
    // if _, err := vk.WriteTo(fvk); err != nil {
    //     log.Fatalf("failed to write verifying key: %v", err)
    // }

    // log.Println("✅  Keys written to proving.key and verifying.key")
    // f, err := os.Create("verifierEnygma.sol")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer f.Close()
    // err = vk.ExportSolidity(f)
    // if err != nil {
    //     log.Fatal(err)
    // }

    
 
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

