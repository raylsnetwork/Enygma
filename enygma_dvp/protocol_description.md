User Alice has:
* a view key pair:
   * $$sk_{A}^{view}$$, $$pk_{A}^{view}$$
* a spend key pair:
   * $$sk_{A}^{spend}$$, $$pk_{A}^{spend}$$


In Enygma DvP, the commitments have the following form:

$$C = Hash(pk^{spend}, salt, id_{token}, amount)$$


## Private Issuance




It is trivial to have a non-interactive protocol that allows the recipient to open this corresp

Issuer:

* generates a random salt:
   * $$salt \longleftarrow \lbrace0, 1\rbrace^{\lambda}$$
