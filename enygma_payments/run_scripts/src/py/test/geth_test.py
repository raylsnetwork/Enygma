import web3
w3 = web3.Web3(web3.HTTPProvider("http://127.0.0.1:8545"))
from web3.middleware import geth_poa_middleware
w3.middleware_onion.inject(geth_poa_middleware, layer=0)
from pprint import pprint

pprint(dir(w3))
print(w3.eth.accounts[0])
# me = w3.eth.get_accounts()[0];
# temp = w3.eth.contract(bytecode=bytecode, abi=abi)
# txn = temp.constructor().buildTransaction({"from": me}); 
# txn_hash = w3.eth.send_transaction(txn)
# txn_receipt = w3.eth.wait_for_transaction_receipt(txn_hash)
# address = txn_receipt['contractAddress']

with open('/home/witch/.ethereum/keystore/UTC--2024-02-24T15-42-39.918780300Z--7e9cfc68e90a6c363d145a30cb059b15ce744fc4') as keyfile:
    encrypted_key = keyfile.read()
    private_key = w3.eth.account.decrypt(encrypted_key, 'test123test')
    print(w3.to_hex(private_key))
    # 0xc2d987e485fc413135813fe6fc94aa2378dee2e37cbdf5591abd19c7116c02e7


print("balance: ", w3.eth.get_balance('0x7E9cFC68E90a6c363D145a30cb059b15cE744fc4'))
