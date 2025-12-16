# Enygma Payment Demo - Quick Start Guide

This guide will walk you through running a complete Enygma payment demo, including smart contract deployment, ZK proof server setup, and executing a private transaction.

### Overview

The Enygma payment demo demonstrates a complete private transaction workflow:

1. Smart Contract Deployment: Deploy the Enygma payment contract and ZK verifier to an EVM-compatible blockchain

2. ZK Proof Server: Start the gnark-based server that generates and verifies zero-knowledge proofs

3. Private Transaction: Execute a confidential payment using the Go client

What happens during a demo transaction:

- The client generates a zero-knowledge proof for a payment of 100 tokens
- The proof ensures transaction privacy while maintaining verifiability
- The smart contract verifies the proof on-chain and processes the payment

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
go run ./transaction/main.go <qtyBank> <value> <senderId> <sk> <previousV> <previousR>
```

### Step-by-Step Instructions

Step 1: Deploy Smart Contracts

The first step is to deploy the Enygma payment contract and the ZK proof verifier to your chosen blockchain.

```bash
cd run_scripts
python deploy_enygma.py

```

What this script does:

- Compiles the Enygma.sol and Verifier.sol contracts
- Deploys them to the configured network
- Saves deployment addresses to a configuration file
- Verifies contracts on block explorers (if configured)

Expected output:

```
2025-11-24 14:15:11,481 : Printing out Enygma data
2025-11-24 14:15:11,481 : token_address = 0xF879a1569B591AA8CcC7e0317aab0d2672eE63D6
2025-11-24 14:15:11,482 : [Token information]
name =  Enygma
symbol =  EN
verifier =  0x3E8Ff3685CCe5E44e79204FC497aDdB9fb7916A7
BankCount =  6
2025-11-24 14:15:11,512 : Demo.transactions
2025-11-24 14:15:11,512 : enygma.check
2025-11-24 14:15:12,547 : Done enygma.check
2025-11-24 14:15:12,547 : Minting 1000 tokens for account 0
2025-11-24 14:15:13,581 : Done minting.
2025-11-24 14:15:13,581 :
2025-11-24 14:15:13,581 : Enygma.check
2025-11-24 14:15:14,614 : Done Enygma.check
2025-11-24 14:15:14,615 :
```

Step 2: Start the Gnark Server

The gnark server generates zero-knowledge proofs for private transactions.
Open a new terminal window and run:

```bash
cd gnark_server
go run cmd/server/main.go
```

What this server does:

- Loads the ZK circuit definition
- Exposes REST/gRPC endpoints for proof generation
- Validates transaction inputs
- Generates proofs using the gnark library

Expected output:

```
[GIN-debug] [WARNING] Creating an Engine instance with the Logger and Recovery middleware already attached.

[GIN-debug] [WARNING] Running in "debug" mode. Switch to "release" mode in production.
 - using env:   export GIN_MODE=release
 - using code:  gin.SetMode(gin.ReleaseMode)

[GIN-debug] POST   /proof/enygma             --> enygma-server/pkg/circuits/enygma.NewHandler.func1 (3 handlers)
[GIN-debug] POST   /proof/withdraw/1         --> enygma-server/pkg/circuits/withdraw.NewHandler.func1 (3 handlers)
[GIN-debug] POST   /proof/withdraw/2         --> enygma-server/pkg/circuits/withdraw.NewHandler.func1 (3 handlers)
[GIN-debug] POST   /proof/withdraw/3         --> enygma-server/pkg/circuits/withdraw.NewHandler.func1 (3 handlers)
[GIN-debug] POST   /proof/withdraw/4         --> enygma-server/pkg/circuits/withdraw.NewHandler.func1 (3 handlers)
[GIN-debug] POST   /proof/withdraw/5         --> enygma-server/pkg/circuits/withdraw.NewHandler.func1 (3 handlers)
[GIN-debug] POST   /proof/withdraw/6         --> enygma-server/pkg/circuits/withdraw.NewHandler.func1 (3 handlers)
[GIN-debug] POST   /proof/deposit            --> enygma-server/pkg/circuits/deposit.NewHandler.func1 (3 handlers)
[GIN-debug] [WARNING] You trusted all proxies, this is NOT safe. We recommend you to set a value.
Please check https://pkg.go.dev/github.com/gin-gonic/gin#readme-don-t-trust-all-proxies for details.
[GIN-debug] Listening and serving HTTP on :8080
```

Step 3: Execute a Private Transaction

Now execute a confidential payment using the Go client.

Open another new terminal window and run:

```bash
cd go_client
go run ./transaction/main.go <qtyBank> <value> <senderId> <sk> <previousV> <previousR> <blockHash>

```

What happens:

1. Client validates input parameters
2. Connects to gnark server to generate ZK proof
3. Constructs transaction with proof
4. Submits transaction to Enygma smart contract
5. Contract verifies proof and processes payment

Expected output

```bash
Transfer was successful
```
