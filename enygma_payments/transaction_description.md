# How to run the Enygma payment demo

#### Setup Stage

1. Deploy Enygma.sol and Verifieries Smart Contract on chain
2. Deploy Gnark Server
3. Run Go Client Transaction command.

#### Example

1. Deployment of Enygma.sol and Verifier.sol command line

```javascript
cd run_scripts

python deploy_enygma.py
```

2. Deploy Gnark Server

```javascript
cd gnark_server

go run cmd/server/main.go
```

3. Transaction

```javascript
cd go_client

go run . <qtyBank> <value> <senderId> <sk> <previousV> <previousR> <blockHash>

```
