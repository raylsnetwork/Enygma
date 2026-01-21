pragma circom 2.0.0;
include "../circomlib/circuits/poseidon.circom";
include "../circomlib/circuits/bitify.circom";
include "../circomlib/circuits/mux4.circom";
include "./UniqueId.circom";
include "./Erc1155UniqueId.circom";


// Gets in vaultId and generates uniqueId based on it
// It is to simplify the circom circuits
// and have a more flexible design and more modularity

template UniqueIdMux(tm_numOfIdParams){
    signal input vaultId;
    signal input contractAddress;

    // 5 parameters to generate the unique Id from,
    // the order matches both on-chain uniqueId generation
    // and in utils.js
    signal input idParams[tm_numOfIdParams];

    signal output out;
    
    component cp_uniqueIdErc20;
    component cp_uniqueIdErc721;
    component cp_uniqueIdErc1155;
    component cp_bitifyVaultId;
    component cp_mux;

    cp_bitifyVaultId = Num2Bits(4); // reserving for 16 vaults/standards
    cp_bitifyVaultId.in <== vaultId;

    cp_mux = Mux4();
    cp_mux.s[0] <== cp_bitifyVaultId.out[0];
    cp_mux.s[1] <== cp_bitifyVaultId.out[1];
    cp_mux.s[2] <== cp_bitifyVaultId.out[2];
    cp_mux.s[3] <== cp_bitifyVaultId.out[3];

    // 0: ERC20
        cp_uniqueIdErc20 = UniqueId();
        cp_uniqueIdErc20.contractAddress <== contractAddress;
        cp_uniqueIdErc20.amount <== idParams[0];

        cp_mux.c[0] <== cp_uniqueIdErc20.out;

    // 1: ERC721
        cp_uniqueIdErc721 = UniqueId();
        cp_uniqueIdErc721.contractAddress <== contractAddress;
        cp_uniqueIdErc721.amount <== idParams[0];

        cp_mux.c[1] <== cp_uniqueIdErc721.out;

    // 2: ERC1155
        cp_uniqueIdErc1155 = Erc1155UniqueId();
        cp_uniqueIdErc1155.erc1155ContractAddress <== contractAddress;
        cp_uniqueIdErc1155.amount <== idParams[0];
        cp_uniqueIdErc1155.erc1155TokenId <== idParams[1];

        cp_mux.c[2] <== cp_uniqueIdErc1155.out;

    // 3: Enygma // not implemented
        cp_mux.c[3] <== 0;

    // setting reserved vault's uniqueId output to zero
        for(var i =4; i<16; i++){
            cp_mux.c[i] <== 0;
        }

    out <== cp_mux.out;
}
