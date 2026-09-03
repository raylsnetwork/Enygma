module github.com/raylsnetwork/enygma_dvp_auctions/src

go 1.26.0
toolchain go1.26.6

require github.com/raylsnetwork/enygma_dvp/src v0.0.0

require (
	github.com/dchest/blake512 v1.0.0 // indirect
	github.com/iden3/go-iden3-crypto v0.0.16 // indirect
	golang.org/x/crypto v0.56.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
)

// Local replace — remove once enygma_dvp is published as a standalone Go module.
// Both repos must be cloned side-by-side: enygma_dvp/ and enygma_dvp_auctions/
// in the same parent directory.
replace github.com/raylsnetwork/enygma_dvp/src => ../../enygma_dvp/src
