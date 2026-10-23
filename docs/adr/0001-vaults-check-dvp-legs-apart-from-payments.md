# Coin vaults check DvP legs apart from payment receipts

A coin vault used to verify every receipt through one `checkReceiptConditions`, which chose the circuit from the caller-supplied `numberOfInputs`/`numberOfOutputs` and statement length. Statement length cannot pick the family on its own (`DvpInitiator` and `PaymentFee` both have 8 public signals; `PaymentRelayerFee` and `UsdrFee` both have 9), and every caller already knows which family it holds. So the vault interface is split: `checkDvpReceiptConditions` takes only DvP legs, read through `DvpReceipt`, and each vault accepts only its own leg kind (Erc20: initiator; Erc721: destination; any other vault reverts by default). `checkReceiptConditions` takes only that vault's non-DvP receipts. Within the Payment family, the receipt's (inputs, outputs) shape picks a row of a fixed table, and the statement length must equal that row's circuit's length before anything is read.

## Considered Options

- **Tell the families apart by `StMessage`** (0 or `WITHDRAW_MESSAGE` for a payment, otherwise DvP), as Erc721's HIGH-3 fix does. Rejected: the family would then hinge on a value the prover chooses, not on what the caller means, and a DvP leg sent to a payment path would still reach the payment verifier unless a guard caught it.
- **Keep `numberOfOutputs` as the family selector.** Rejected: GLOSSARY's *DvP leg* rule says a leg is never read by caller-supplied counts.

## Consequences

The vault ABI changes, so retail's copies of the `EnygmaDvp` and `Erc20CoinVault` artifacts must be refreshed with it. `Erc20CoinVault._requireStandaloneTransfer` stays, but only as extra protection: a DvP leg can no longer pass `checkReceiptConditions` at all.
