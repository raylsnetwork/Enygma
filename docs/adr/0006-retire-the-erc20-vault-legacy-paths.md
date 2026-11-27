# Retire Erc20CoinVault's legacy withdraw and verifyOwnership

`Erc20CoinVault.withdraw` and `verifyOwnership` were the vault's legacy uid-commitment paths. `withdraw` paid out a receipt whose output 0 was `Poseidon(Poseidon(asset, amount), recipient)`, and `verifyOwnership` one whose output 0 was `Poseidon(Poseidon(asset, amount), 0)`, taking the receipt's `StMessage` as its challenge. Both read the receipt through the Payment-family table, so it had to be a Payment, Payment2in, PaymentFee, PaymentRelayerFeePublic or UsdrFee proof. Each of those circuits constrains every output to a four-input note commitment, `Poseidon(pkSpend, salt, value, tokenId)`, which uses a different Poseidon width from the two-input hash these paths open; matching one would need a collision between the two. In the main deployment the other circuits in the slots the table reads cannot be proved either: slot 1 holds `OwnershipErc721`, whose template is gone and whose keygen is commented out, and slots 2 to 4 hold unprovable placeholders (ADR 0003). So no receipt reached either function, and no client calls them.

Their checks were also weaker than `withdrawV2`'s. `withdraw` did not check `StMessage`, so it would take a payment receipt (`StMessage = 0`), and it spent the inputs without inserting output 1, the change note. `verifyOwnership`'s challenge was the receipt's `StMessage`, which the Payment-family circuits restrict to 0 and 1, and `EnygmaDvp.checkAndRegisterChallenge` lets any account register a challenge, so both values could be used up by anyone.

So the two functions are retired, not hardened: each reverts `Erc20LegacyPathRetired`. They stay in the ABI, as `IAbstractCoinVault` declares them. `withdrawV2`, `partialWithdraw` and `partialWithdrawWithRelayerFee` are the vault's exits.

## Considered Options

- **Harden the legacy paths**: require `StMessage == WITHDRAW_MESSAGE` and a non-zero amount in `withdraw`, insert its output 1, and give `verifyOwnership` challenges no one else can use up. Rejected: no circuit produces either opening, so the hardened code could never run, and no test with a real proof could cover it. The exits it would duplicate already have those checks.

## Consequences

An ERC-20 note leaves the vault only through `withdrawV2`, `partialWithdraw` or `partialWithdrawWithRelayerFee`, and there is no ownership proof for ERC-20 notes. `generateUniqueId` stays: `EnygmaDvp` and the legacy `EnygmaAuction` call it. `Erc721CoinVault`'s legacy `transfer`, `withdraw` and `verifyOwnership` are outside this decision: they verify against VK slot 1, for which no proof can be made, and `Erc721CoinVault.verifyOwnership` is now the only caller of `EnygmaDvp.checkAndRegisterChallenge`. Already-deployed vaults are not upgraded: an existing chain keeps the old code until it is redeployed.
