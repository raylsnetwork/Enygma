pragma circom 2.0.0;
include "./templates/BrokerRegistrationTemplate.circom";

component main {
	public [
			st_beacon, 
			st_vaultId, 
			st_groupId,
			st_delegator_treeNumbers, 
			st_delegator_merkleRoots, 
			st_delegator_nullifiers,
			st_broker_blindedPublicKey,
			st_broker_minCommissionRate,
			st_broker_maxCommissionRate,
			st_assetGroup_treeNumber,
			st_assetGroup_merkleRoot
		]
	} =  BrokerRegistrationTemplate(2, 8, 8, 1000000000000000000000000000000000000, 10, 2);

