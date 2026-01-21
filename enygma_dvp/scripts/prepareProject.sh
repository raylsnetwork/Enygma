#!/bin/sh

echo "Preparing project"

echo "Removing local Merkle tree"
mkdir -p ./src/appdata/
rm ./src/appdata/erc20MerkleTreeState.json
rm ./src/appdata/erc721MerkleTreeState.json
rm ./src/appdata/erc1155MerkleTreeState.json

echo "Done making. Next step: initialize ZkDvp by calling:"
echo "npm run init"