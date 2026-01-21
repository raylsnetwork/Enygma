pragma circom 2.0.0;
include "./templates/Erc1155Template.circom";

component main {public [st_message, st_treeNumbers, st_merkleRoots, st_nullifiers, st_commitmentsOut]} =  Erc1155Template(1,1,8);