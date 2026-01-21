#!/bin/zsh

# All circuits that are being compiled
CIRCUITS=(
    "OwnershipErc721"
    "JoinSplitErc20"
    "OwnershipErc1155NonFungible"
    "OwnershipErc1155Fungible"
    "JoinSplitErc1155"
    "JoinSplitErc20_10_2"
    "AuctionInit"
    "AuctionBid"
    "AuctionNotWinningBid"
    "AuctionPrivateOpening"
    "BrokerRegistration"
    "LegitBroker"
    "JoinSplitErc20WithBrokerV1"
    "JoinSplitErc1155WithBrokerV1"
    "BatchErc1155"
    "JoinSplitErc1155WithAuditor"
    "OwnershipErc1155NonFungibleWithAuditor"
    "BatchErc1155NonFungibleWithAuditor"
    "OwnershipErc721WithAuditor"
    "JoinSplitErc20WithAuditor"
    "JoinSplitErc20_10_2_WithAuditor"
    "AuctionInit_Auditor"
    "AuctionBid_Auditor"
)

# Circuits that you need to inject the treeDepth from config to circom files
TREE_CIRCUITS=(
    "OwnershipErc721"
    "JoinSplitErc20"
    "OwnershipErc1155NonFungible"
    "OwnershipErc1155Fungible"
    "JoinSplitErc1155"
    "JoinSplitErc20_10_2"
    "AuctionInit"
    "AuctionBid"
    "BrokerRegistration"
    "JoinSplitErc20WithBrokerV1"
    "JoinSplitErc1155WithBrokerV1"
    "BatchErc1155"
    "JoinSplitErc1155WithAuditor"
    "OwnershipErc1155NonFungibleWithAuditor"
    "BatchErc1155NonFungibleWithAuditor"
    "OwnershipErc721WithAuditor"
    "JoinSplitErc20WithAuditor"
    "JoinSplitErc20_10_2_WithAuditor"
    "AuctionInit_Auditor"
    "AuctionBid_Auditor"
)

# Circuits that you need to inject the fungibility range from config to circom files.
FUNGIBLE_CIRCUITS=(
    "JoinSplitErc20"
    "OwnershipErc1155Fungible"
    "JoinSplitErc1155"
    "JoinSplitErc20_10_2"
    "AuctionBid"
    "AuctionNotWinningBid"
    "AuctionPrivateOpening"
    "BrokerRegistration"
    "LegitBroker"
    "JoinSplitErc20WithBrokerV1"
    "JoinSplitErc1155WithBrokerV1"
    "JoinSplitErc1155WithAuditor"
    "JoinSplitErc20WithAuditor"
    "JoinSplitErc20_10_2_WithAuditor"
    "AuctionBid_Auditor"
)
##########################################################
copy_raw_circuit () {
    var1="$1"
    cp "raw/${var1}.circom" ./
}
##########################################################
inject_tree_depth () {
    local tree_depth="$1"
    local circuit_name="$2"
    if sed -i "s/TREE_DEPTH/${tree_depth}/g" "${circuit_name}.circom"; then
        echo "Injected tree_depth value into ${circuit_name}.circom."
    else
        echo 'Failed to inject, changing sed syntax'
        sed -i '' -e "s/TREE_DEPTH/${tree_depth}/g" "${circuit_name}.circom"
    fi
}
##########################################################
inject_fungibility_range () {
    local fungible_range="$1"
    local circuit_name="$2"
    if sed -i "s/FUNGIBLE_RANGE/${fungible_range}/g" "${circuit_name}.circom"; then
        echo "Injected fungible_range value into ${circuit_name}.circom."
    else
        echo 'Failed to inject, changing sed syntax'
        sed -i '' -e "s/FUNGIBLE_RANGE/${fungible_range}/g" "${circuit_name}.circom"
    fi
}
##########################################################
compile_circuit(){
    local circuit_name="$1"

    if ! ${CIRCOM:-circom} "../circuits/${circuit_name}.circom" --r1cs --wasm ||
            ! [[ -s ./${circuit_name}_js/${circuit_name}.wasm ]]

    then
        echo >&2 "${circuit_name} compilation failed."
        exit 1
    fi
}
##########################################################
remove_temp_file(){
    local circuit_name="$1"
    mv "./${circuit_name}_js/${circuit_name}.wasm" ./
    rm -r "./${circuit_name}_js"
}
##########################################################
resolve_ptua(){

    ptau=powersOfTau28_hez_final_20.ptau

    if [ -f $ptau ]; then
        echo "$ptau already exists. Skipping."
    else
        echo 'Downloading $ptau'
        curl https://hermez.s3-eu-west-1.amazonaws.com/$ptau -o $ptau
    fi
}
##########################################################
resolve_circomlib(){
    circomlib=circomlib

    if [ -d $circomlib ]; then
        echo "${circomlib} repo already cloned. Skipping."
    else
        echo 'cloning git repository $circomlib'
        git clone git@github.com:iden3/circomlib.git
    fi
}

##########################################################
#                    MAIN SCRIPT
##########################################################

set -e

cd "$(dirname "$0")"

mkdir -p ../build

cd ../build

# resolving powers of tua file
resolve_ptua

cd ../circuits/

# resolving circomlib repo files
resolve_circomlib

echo "------------------\nCopying raw circuits to set parameters universally."
for element in "${CIRCUITS[@]}"; do
 copy_raw_circuit "$element"
done

echo "------------------\nInjecting Template parameters from config to circuits."
tree_depth=`grep -o '"tree-depth":.*' ../zkdvp.config.json | sed -e 's/"tree-depth":/''/g'| sed -e 's/','/''/g'`
echo treeDepth= $tree_depth
for element in "${TREE_CIRCUITS[@]}"; do
 inject_tree_depth "$tree_depth" "$element"
done

fungible_range=`grep -o '"fungibleRange":.*' ../zkdvp.config.json | sed -e 's/"fungibleRange":/''/g'| sed -e 's/','/''/g'`
echo fungibleRange= $fungible_range
for element in "${FUNGIBLE_CIRCUITS[@]}"; do
 inject_fungibility_range "$fungible_range" "$element"
done


echo "------------------\nCompiling circom circuits."

cd ../build
 
for element in "${CIRCUITS[@]}"; do
 compile_circuit "$element"
done

echo "Circuit compilation succeeded"

echo "------------------\nRemoving temp files"

for element in "${CIRCUITS[@]}"; do
 remove_temp_file "$element"
done

