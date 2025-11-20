## Protocol Flows

### System Setup


#### Transparent Setup for Generator H

We use Pedersen commitments to mask the balances of the parties in the system and balances of the transactions. Such commitment relies on two generators: $$G$$ and $$H$$. We highlight, however, that knowing the relationship between these two generators is insecure as it breaks the binding property of the scheme. Concretely, if an entity knows the relationship between the generators $$G$$ and $$H = dG$$ (i.e., knows the value $$d$$), then such entity can open their commitments in any way they want. To avoid this, Enygma uses a [nothing-up-my-sleeve number](https://en.wikipedia.org/wiki/Nothing-up-my-sleeve_number) obtained by hashing a constant to the curve that is used in the system. This adds an additional layer of transparency. 

```mermaid
---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
flowchart LR

    zero["00...00"]
    hash(["Hash-To-Curve(x)"])
    generator(["H"])

    %% Flow Connections
    zero -.-> hash -.-> generator

```

#### ZK Trusted Setup (Groth16)
Enygma relies on the Groth16 ZK scheme, which requires an initial trusted setup. Ideally, such a trusted setup is in the form of an MPC protocol where different participants contribute with random secrets, which must be destroyed after the ceremony to ensure that a single party does not have the ability to subvert the system (i.e., forge proofs). The output of this trusted setup is the Common Reference String (CRS) for the circuit. 

We envision this step to take place involving different Privacy Nodes in the system. We note that each Privacy Node represents a regulated financial institution. Therefore, it is reasonable to assume that at least one of the institutions will abide by the protocol and preserve the security of the trusted setup stage. 

### Issuer

#### Issuer - Setup
The setup for the issuer is straightforward as it consists simply of deploying the corresponding Enygma smart contract. Optionally, to support certain private functionalities, the issuer may need to register some key material. 

```mermaid
---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
flowchart LR

    %% I (Setup)
    i_setup["Issuer<br>(Setup)"]
    register(["(optional)<br>Register Key Material"])
    deploy(["Deploy Enygma<br>Contract"])

    %% Flow Connections
    i_setup -.-> register -.-> deploy
```

#### Issuer - Mint

There are two minting flows for the issuer is able to mint funds on the underlying smart contract. The issuer can either mint a commitment with the random factor set to zero which publicly discloses the minted amount or, alternatively, act as a participant in the network and mint a shielded balance in the form of Pedersen commitment where the random factor is derived from the shared secret between the issuer and the receiver of funds. 

```mermaid
---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
flowchart LR

    %% I (Mint)
    i_mint(["Issuer<br>(Mint)"])
    mint_transparent(["Mint<br>(Transparent) Funds"])
    mint_ttx(["Mint Comm = vG"])
    mint_shield(["Mint<br>(Shielded) Funds"])
    calculate_r(["Calculate<br>Random Factor"])
    mint_stx(["Mint Comm = vG+rH"])


    %% Flow Connections
    i_mint -.-> mint_transparent -.-> mint_ttx
    i_mint -.-> mint_shield -.-> calculate_r -.-> mint_stx

```

### Privacy Node

#### Privacy Node - Setup
First, each Privacy Node generates and registers two keypairs (view and spend) on the underlying blockchain. This blockchain effectively acts as a Public-Key Infrastructure (PKI) containing a registry of all public-keys of the registered Privacy Nodes. Subsequently, each Privacy Node performs a post-quantum key agreement (i.e., ML-KEM) and establishes individual shared secrets with all the other Privacy Nodes. At this point, Privacy Nodes can start transacting privately with each other. The transaction protocol includes a hash-based private messaging tag component that allow recipients to detect privately whether or not a transaction is for them. Therefore, we also introduce a protocol to fetch (and decrypt) transactions.

```mermaid
---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
flowchart LR

    %% PL (Setup)
    pl_setup["Privacy Node<br>(Setup)"]
    keygen(["Key<br>Generation"])
    register(["Key<br>Registration"])
    kem(["Key<br>Agreement"])
    publish(["Publish<br>Key Fingerprints"])

    pl_setup -.-> keygen -.-> register -.-> kem -.-> publish
```

#### Privacy Node - Sending a TX
To send a transaction, the privacy node needs to be in sync with the latest block on the blockchain. The purpose for this is twofold: first, the privacy node needs to create a nullifier and random factors that depend on that specific last block; and second, the privacy node needs to know what is the latest shielded balance it has in order to be able to spend funds. Therefore, the first step to send a transaction is to obtain the latest block. From the latest block number, the privacy node derives the ephemeral symmetric key used to encrypt additional/associated data of the transaction. The privacy node then calculates the corresponding random factors to be used in the transaction, and the nullifier for this block. The privacy node also calculates a set of $$k$$ (i.e., anonymity set) Pedersen commitments using the previously obtained random factors and the amount to be sent to each party along with a nullifier that proves that the sender is submitting its only allowed transaction in this block, without revealing details about who they are. Once these values are calculated, the privacy node creates a ZK proof that proves the following: 

* I know the secret key of one of the items in this anonymity set of $$k$$ public keys;
* I know the amount and random factor of the commitment that contains my balance in this set of $$k$$ commitments;
* The nullifier is well-formed and uses my secret key and the latest block number;
* The private messaging tags are well-formed using the shared secret I have obtained previously with each of the $$k-1$$ participants and the latest block number.

```mermaid
---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
flowchart LR

    %% PL (Send)
    pl_send["Privacy Node<br>(Send Tx)"]
    getblock_send(["Get Latest<br>Block"])
    derivesendkey(["Derive Ephemeral<br>(Symmetric) Key"])
    calc_r(["Calculate<br>Random Factor"])
    calc_tags(["Calculate Private<br>Messaging Tags"])
    tx_commits(["Generate 'k'<br>Pedersen Commitments"])
    nullifier(["Calculate<br>Nullifier"])
    zk_proof(["Create<br>ZK Proof"])
    encrypt_ad(["Encrypt Additional Data<br>(w/ ephemeral key)"])
    send_tx(["Send commits, nullifier, zk proof, and ciphertext"])

    pl_send -.-> getblock_send -.-> derivesendkey -.-> calc_r -.-> calc_tags -.-> tx_commits -.-> nullifier -.-> zk_proof -.-> encrypt_ad -.-> send_tx

```

#### Privacy Node - Receiving a TX
```mermaid
---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
flowchart LR

    %% PL (Receive)
    pl_receive["Privacy Node<br>(Receive Tx)"]
    getblock_receive(["Download Latest<br>Block"])
    get_anon_set(["Get Subset of<br>Enygma Transactions"])
    calc_tags(["Calculate Private<br>Messaging Tags"])
    detect_sender(["Detect<br>Sender"])
    derivereceivekey(["Derive Ephemeral<br>(Symmetric) Key"])
    calc_r(["Calculate<br>Random Factor(s)"])
    obtain_vG(["Obtain vG"])
    obtain_v(["Obtain received amount"])
    calc_c(["Calculate Latest<br>(Shielded) Balance"])


    pl_receive -.-> getblock_receive -.-> get_anon_set -.->  calc_tags -.-> detect_sender -.-> derivereceivekey -.-> calc_r -.-> obtain_vG -.-> obtain_v -.-> calc_c

```

### Blockchain
We now describe the flows associated with the underlying blockchain. In this case, we have just one: the verification of enygma transactions. 

#### Blockchain - Verify TX
The smart contract receives a set of commitments, a nullifier, a ZK proof, and an encrypted payload. 

We note that the encrypted payload is not checked by the smart contract and can be maliciously formed as its correctness is not included in the ZK proof. We note that this attack forces the sender to send funds, which the recipient is able to open since the Pedersen commitment is well-formed. The same recipient is then able to prove that the sender maliciously formed the ciphertext and eventually  have the sender face repercussions for a purposeful malicious action. The ciphertext is not included in the ZK proof because proving the correctness of an AES-GCM encryption is too expensive to perform for the execution of real-time transactions. 

```mermaid
---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
flowchart LR

    %% Blockchain (Verifier)
    verify_tx["Blockchain<br>(Verify Tx)"]
    check_nullifier(["Check if nullifier<br>exists"])
    check_commit(["Check if commitments add up to 0"])
    check_zk(["Check ZK Proof"])
    tally(["Approve Tx"])
    store(["Store encrypted payload"])

    verify_tx -.-> check_nullifier -.-> check_commit -.-> check_zk -.-> tally -.-> store
```

### Auditing

```mermaid
---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
flowchart LR

    auditor1["Auditor<br>(Registration)"]
    register_key(["Register Auditor pk"])
    
    auditor1 -.-> register_key
```

#### Auditing - Long-term View Key Sharing
The privacy nodes register their view (public) keys on the underlying blockchain, which acts as a public-key infrastructure (PKI). Upon successful registration of the key, if a view key is to be shared with the auditor, then the privacy node encapsulates its secret key using the public key of the auditor and publishes this ciphertext on-chain. The auditor is then able to download this ciphertext and obtain the corresponding secret (view) key of the privacy node. This gives the auditor the ability to see all the transactions that the privacy node performs in the network. 


```mermaid
---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
flowchart LR

    pn["Privacy Node<br>(View Key Sharing)"]
    keygen(["(View) Key<br>Generation"])
    key_registration(["(View) Key<br>Registration"])
    get_auditor_key(["Get pk of<br>Auditor"])
    encrypt(["Encapsulate sk<br>(using pk of Auditor)"])
    publish_ciphertext(["Publish<br>Ciphertext"])

    pn -.-> keygen -.-> key_registration -.-> get_auditor_key -.-> encrypt -.-> publish_ciphertext


    auditor2["Auditor<br>(View Key Sharing)"]
    get_ciphertext(["Get Ciphertext"])
    decrypt(["Decapsulate and Obtain<br>sk of Privacy Node"])
    check(["Check Key Correctness"])

    auditor2 -.-> get_ciphertext -.-> decrypt -.-> check
```

#### Auditing - Ephemeral View Key Sharing
The privacy nodes have previously register their view (public) keys on the underlying blockchain, which acts as a public-key infrastructure (PKI). Upon successful registration of the key, the privacy nodes register a fingerprint (i.e., a hash) of the shared secret with all the other participants in the network. This protocol allows a privacy node to open individual transactions for specific blocks without compromising the confidentiality of past nor future transactions. 

```mermaid
---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
flowchart LR

    pn["Privacy Node<br>(Ephemeral View Key Sharing)"]
    keygen(["(View) Key<br>Generation"])
    key_registration(["(View) Key<br>Registration"])
    get_auditor_key(["Get pk of<br>Auditor"])
    receive_request(["Receive Auditor Request"])
    encrypt(["Encapsulate symmetric encryption key <br>(using pk of Auditor)"])
    zk_prove(["Create ZK proof that k<br> is derived from shared secret"])
    publish(["Publish<br>Ciphertext and ZK Proof"])

    pn -.-> keygen -.-> key_registration -.-> get_auditor_key -.-> receive_request -.-> encrypt -.-> publish


    auditor2["Auditor<br>(View Key Sharing)"]
    get_ciphertext(["Get Ciphertext"])
    decrypt(["Decapsulate and Obtain<br>k of Privacy Node"])
    check_zk(["Check ZK Proof Correctness"])
    check_enc(["Check k correctly decrypts payload"])
    audit_tx(["Audit Transaction"])

    auditor2 -.-> get_ciphertext -.-> decrypt -.-> check_zk -.-> check_enc -.-> audit_tx

```
