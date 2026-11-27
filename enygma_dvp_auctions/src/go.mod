module github.com/raylsnetwork/enygma_dvp_auctions/src

go 1.26.11

require github.com/raylsnetwork/enygma_dvp/src v0.0.0

require (
	enygma_dvp/layout v0.0.0 // indirect
	github.com/ProjectZKM/Ziren/crates/go-runtime/zkvm_runtime v0.0.0-20251001021608-1fe7b43fc4d6 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.0.1 // indirect
	github.com/ethereum/go-ethereum v1.17.10 // indirect
	github.com/holiman/uint256 v1.3.2 // indirect
)

require (
	github.com/dchest/blake512 v1.0.0 // indirect
	github.com/iden3/go-iden3-crypto v0.0.17 // indirect
	golang.org/x/crypto v0.59.0
	golang.org/x/sys v0.50.0 // indirect
)

// Local replace — remove once enygma_dvp is published as a standalone Go module.
// Both repos must be cloned side-by-side: enygma_dvp/ and enygma_dvp_auctions/
// in the same parent directory.
replace github.com/raylsnetwork/enygma_dvp/src => ../../enygma_dvp/src

replace enygma_dvp/layout => ../../enygma_dvp/layout
