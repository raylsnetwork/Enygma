async function getCommitmentFromTx(tx) {
  const rc = await tx.wait();
  console.log("Gas used: " + rc["gasUsed"].toString());
  const event = rc.events.find((ev) => ev.event === "Commitment");
  const commitment = event.args.commitment;
  // console.log("new commitment:", commitment)
  return commitment.toBigInt();
}

async function sendMixFundsToChain(relayer, proof, zkDvpContract) {
  const dvpRelayer = zkDvpContract.connect(relayer);

  const tx = await dvpRelayer.mixFunds(proof);

  const commitments = [...proof.commitments];

  return commitments;
}

async function sendUnspentErc20(relayer, proof, zkDvpContract) {
  const dvpRelayer = zkDvpContract.connect(relayer);
  await dvpRelayer.submitUnspentErc20(proof);
}

async function submitPartialSettlement(relayer, proof, vaultId, groupId, zkDvpContract) {
  const dvpRelayer = zkDvpContract.connect(relayer);
  const tx = await dvpRelayer.submitPartialSettlement(proof, vaultId, groupId);
  const rc = await tx.wait();
  return rc;
}

async function swap(
    relayer, 
    paymentReceipt, 
    deliveryReceipt, 
    paymentVaultId, 
    deliveryVaultId,
    zkDvpContract
) {
  const dvpRelayer = zkDvpContract.connect(relayer);

  tx = await dvpRelayer.swap(
      paymentReceipt, 
      deliveryReceipt, 
      paymentVaultId, 
      deliveryVaultId
    );
  const rc = await tx.wait();
  // console.log("Gas used: " + rc["gasUsed"].toString());
  const commitments = [
    paymentReceipt.statement[7], 
    paymentReceipt.statement[8], 
    deliveryReceipt.statement[4]
  ];

  return commitments;

}


async function exchange(relayer, paymentReceipt1, paymentReceipt2, paymentVaultId1, paymentVaultId2, zkDvpContract) {
  const dvpRelayer = zkDvpContract.connect(relayer);

  tx = await dvpRelayer.exchange(paymentReceipt1, paymentReceipt2, paymentVaultId1, paymentVaultId2);
  const rc = await tx.wait();
  // console.log("Gas used: " + rc["gasUsed"].toString());
  const commitments = [
      paymentReceipt1.statement[7], 
      paymentReceipt1.statement[8], 
      paymentReceipt2.statement[4]
    ];

  return commitments;

}

module.exports = {
  swap,
  exchange,
  sendMixFundsToChain,
  submitPartialSettlement
};
