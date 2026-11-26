package userregistry

import (
	"crypto/mlkem"

	"github.com/raylsnetwork/enygma_retail_payments/src/core/auditor"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// AuditedRegistration is one registration in the UserRegistry and what the
// auditor finds in its audit ciphertexts.
type AuditedRegistration struct {
	User common.Address
	// Keys are the spend and view keys the user registered.
	Keys           *UserKeys
	MlKemCt, AesCt []byte
	auditor.AuditCheck
}

// AuditRegistrations has the auditor check every registration in the
// UserRegistry, in registration order: whether its audit ciphertexts open
// with auditorDK, and whether they open to the view key the user registered.
// The contract checks only their lengths, so every registration is checked.
func AuditRegistrations(
	client *ethclient.Client,
	registryAddr common.Address,
	auditorDK *mlkem.DecapsulationKey768,
) ([]AuditedRegistration, error) {
	count, err := GetUserCount(client, registryAddr)
	if err != nil {
		return nil, err
	}
	regs := make([]AuditedRegistration, 0, count)
	for i := range count {
		user, err := GetUserAt(client, registryAddr, i)
		if err != nil {
			return nil, err
		}
		keys, err := LookupKeys(client, registryAddr, user)
		if err != nil {
			return nil, err
		}
		mlKemCt, aesCt, err := LookupAuditCiphertexts(client, registryAddr, user)
		if err != nil {
			return nil, err
		}
		regs = append(regs, AuditedRegistration{User: user, Keys: keys, MlKemCt: mlKemCt, AesCt: aesCt,
			AuditCheck: auditor.CheckAuditCiphertexts(auditorDK, mlKemCt, aesCt, keys.ViewKey)})
	}
	return regs, nil
}
