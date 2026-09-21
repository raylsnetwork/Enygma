package script

import (
	"github.com/consensys/gnark/backend/groth16"
	"log"
	"os"
)

func SavingFiles(pkFile string, vkFile string, pk groth16.ProvingKey, vk groth16.VerifyingKey) error {

	fpk, err := os.Create(pkFile)
	if err != nil {
		log.Fatalf("could not create proving.key: %v", err)
	}
	defer fpk.Close()
	if _, err := pk.WriteTo(fpk); err != nil {
		log.Fatalf("failed to write proving key: %v", err)
	}

	// 4) save the verifying key
	fvk, err := os.Create(vkFile)
	if err != nil {
		log.Fatalf("could not create verifying.key: %v", err)
	}
	defer fvk.Close()
	if _, err := vk.WriteTo(fvk); err != nil {
		log.Fatalf("failed to write verifying key: %v", err)
	}

	return nil
}
