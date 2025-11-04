# ZkToken run script

### How to run

1. Create a `Python 3.10.12` virtual environment and activate it.
2. install the following packages using :
  +   `pip install web3`
  +   `pip install eth-brownie`
  +   `pip install matplotlib`
  +   `pip install IPython`
  +   `pip install objsize`
3. Run your testnode

+ For Rayls Testnet, no setup is needed.
+ For Geth, first change the settings in the default config file to match ``genesis.json```. Use the following commands to run a local geth node:

```
geth --miner.gaslimit 9000000000 --rpc.gascap 900000000 --datadir .chain/ --dev --http --http.api web3,eth,net init zkledger2/conf/genesis.json

geth --miner.gaslimit 9000000000 --rpc.gascap 900000000 --datadir .chain/ --dev --http --http.api web3,eth,net
```

+ For Ganache, you can find the seedPhrase in the config file. Set the seed phrase of Ganache. Additionally, change the settings in the default config file.

```
chainId = 1337
port = 8545
hardfork = Merge
```
4. Run the script.
```
python run_zkToken.py
```
You can find the compiled contracts as well as the deploy receipts in ```build/``` directory.
