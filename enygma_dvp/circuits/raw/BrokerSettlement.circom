pragma circom 2.0.0;
include "./templates/BrokerSettlementTemplate.circom";

component main {public [message, treeNumber, merkleRoot, nullifiers, commitmentsOut]} =  BrokerSettlementTemplate(1,1,TREE_DEPTH);