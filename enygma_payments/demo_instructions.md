# Enygma Payment Demo - Quick Start Guide

This guide will walk you through running a complete Enygma payment demo, including smart contract deployment, ZK proof server setup, and executing a private transaction.

### Overview

The Enygma payment demo demonstrates a complete private transaction workflow:

1. Smart Contract Deployment: Deploy the Enygma payment contract and ZK verifier to an EVM-compatible blockchain
2. ZK Proof Server: Start the gnark-based server that generates and verifies zero-knowledge proofs

3. Private Transaction: Execute a confidential payment using the Go client

### Environment Setup

Before starting, ensure you have:

1. Cloned the repository:

```bash
    git clone https://github.com/raylsnetwork/Enygma.git

    cd Enygma/enygma_payments
```

2. Installed dependencies:

```bash
   # Python dependencies
   pip install -r requirements.txt

   # Go dependencies
   cd gnark_server && go mod download && cd ..
   cd go_client && go mod download && cd ..
```

### Architecture

    ┌─────────────┐
    │  Go Client  │ (Initiates transaction)
    └──────┬──────┘
           │
           │ 1. Request proof generation
           ▼
    ┌─────────────────┐
    │  Gnark Server   │ (Generates ZK proofs)
    └──────┬──────────┘
           │
           │ 2. Returns proof
           ▼
    ┌─────────────┐
    │  Go Client  │
    └──────┬──────┘
           │
           │ 3. Submit proof + transaction
           ▼
    ┌─────────────────────┐
    │   Enygma.sol        │ (On-chain verification)
    │ + Verifier.sol      │
    └──────┬──────────────┘
           │
           │ 4. Verifies proof
           │ 5. Processes payment
           ▼
      [Blockchain]

### Quick Start

```bash
# 1. Deploy contracts

cd run_scripts
python deploy_enygma.py

# 2. Start gnark server (in a new terminal)

cd ../gnark_server
go run cmd/server/main.go

# 3. Execute transaction (in another terminal)

cd ../go_client
go run . <qtyBank> <value> <senderId> <sk> <previousV> <previousR> <blockHash>
```
