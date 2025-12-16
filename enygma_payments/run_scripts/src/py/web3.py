import os
import json
from pprint import pprint
from web3 import Web3
from src.py.logger import info, error, debug
from src.py.helpers.path_helpers import TokenJsonPath
from IPython.utils.capture import capture_output
from web3.middleware import geth_poa_middleware
import time
from typing import List

# import brownie
#*******************************************************************************
class W3b3:
    W3 = None
    Providers = {}
    Token = {}
    Verifier = {}
    reader = ""
    Contracts = {}
    WithdrawVerifier={}
    DepositVerifier={}
   

    costs = []

    def __init__(self, root_path, config, scenario, receipts=[]):

        # brownie.network.connect(config["network"]["brownie-network"])
        # gas_strategy = LinearScalingStrategy("100 wei", "150 wei", 1.5)
        # brownie.network.gas_limit(90000000000)
        self.config = config
        self.scenario = scenario
        self.root_path = root_path
        try:
            # connection_string = "http://"+ network["host"]
            # connection_string = "https://"+ network["host"]            
            connection_string = "http://"+ self.config["network"]["host"] + ":" + self.config["network"]["port"]
            self.W3 = Web3(Web3.HTTPProvider(connection_string, request_kwargs={"timeout": 300}))
            # self.W3.middleware_onion.inject(geth_poa_middleware, layer=0)
            # brownie.network.gas_price(self.W3.eth.gas_price)
            # print(self.W3.eth.get_accounts()[0])
            self.chain_id = int(self.config["network"]["chain-id"])
            self.accounts = self.config["network"]["accounts"]
            self.Token = {}
            if "TOKEN" in receipts.keys():
                self.Token["address"] = receipts["TOKEN"]["contractAddress"]
                self.Providers[self.Token["address"]] = {}

            self.project_name = self.config["id"]

            self.reader = self.accounts[0]['address']
        except Exception as ex:
            error(ex)

    #*******************************************************************************
    def set_token_address(self, token_address):
        self.Token["address"] = token_address
        self.Providers[token_address] = {}
    #*******************************************************************************
    def set_verifier_address(self, verifier_address):
        self.Verifier["address"] = verifier_address
        self.Providers[verifier_address] = {}
     
    #*******************************************************************************
    def set_withdraw_verifier_address(self, verifier_address,k):
        self.WithdrawVerifier[f"address_{k}"] = verifier_address
        self.Providers[verifier_address] = {}
     #*******************************************************************************
    def set_deposit_verifier_address(self, verifier_address):
        self.DepositVerifier["address"] = verifier_address
        self.Providers[verifier_address] = {}
    #*******************************************************************************
    def toWei(self, value, unit):
        return self.W3.toWei(value, unit)
    #*******************************************************************************
    def get_contract(self, deploy_address, compiled_path):
        with open(compiled_path, "r") as file:
            compiled_sol = json.load(file)
        compiled_abi = compiled_sol["abi"]
        return self.W3.eth.contract(address=deploy_address, abi=compiled_abi)
    #*******************************************************************************
    def token_contract(self):
        token_address = self.Token["address"]
        if token_address not in self.Contracts.keys():
            self.Contracts[token_address] = self.get_contract(token_address, TokenJsonPath(self.root_path, self.project_name))

        return self.Contracts[token_address]
    #*******************************************************************************
    def sign_and_send(self, txn, send_key):
        # contract_signed_txn = self.W3.eth.account.sign_transaction(txn, private_key=send_key)
        # contract_tx_hash = self.W3.eth.send_raw_transaction(contract_signed_txn.rawTransaction)
        # return self.W3.eth.wait_for_transaction_receipt(contract_tx_hash)
        contract_signed_txn = self.W3.eth.account.sign_transaction(txn, private_key=send_key)
        raw = getattr(contract_signed_txn, "rawTransaction", None) or getattr(contract_signed_txn, "raw_transaction")
        contract_tx_hash = self.W3.eth.send_raw_transaction(raw)
        return self.W3.eth.wait_for_transaction_receipt(contract_tx_hash)
    #******************************************************************************* 
    def print_token_data(self):
        
        token_address = self.Token["address"]
        debug(f"token_address = {token_address}")

        
        data ={ "address" : f"{token_address}"}
        path = self.root_path.replace("run_scripts", "go_client/config")
       
        with open(f"{path}/address.json", 'w') as file:
            json.dump(data, file, indent=4)
            
        
        debug("[Token information]")
        token_name = self.token_contract().functions.Name().call()
        print(f"name = ",token_name)

        token_symbol = self.token_contract().functions.Symbol().call()
        print(f"symbol = ",token_symbol)

        verifier_address = self.token_contract().functions.VerifierAddress().call()
        print(f"verifier = ",verifier_address)

        bank_count = self.token_contract().functions.TotalRegisteredBanks().call()
        print(f"BankCount = ", bank_count)

    #*******************************************************************************
    def deploy_enygma(self, contract_path, **constructor_args):
        return self.deploy_contract(0, contract_path, **constructor_args)   
    #*******************************************************************************
    def deploy_contract(self, deployer_id, contract_path, **constructor_args):
        try:
            deployer_key = self.accounts[0]["private"]
            deployer_address = self.accounts[0]["address"]
            nonce = self.W3.eth.get_transaction_count(deployer_address)

            # deploying chess lobby
            debug(f"Deploying {contract_path} ...")

            with open(contract_path, "r") as file:
                compiled_contract = json.load(file)

            contract_bytecode = compiled_contract["bytecode"]
            contract_abi = compiled_contract["abi"]
            # print("max fee ", self.W3.eth.max_priority_fee)
            contract = self.W3.eth.contract(abi=contract_abi, bytecode=contract_bytecode)
            contract_transaction = contract.constructor(**constructor_args).build_transaction(  
                {"chainId": self.chain_id, "from": deployer_address, "nonce": nonce, 'gasPrice': 875000000}
            )
            # contract_signed_txn = self.W3.eth.account.sign_transaction(contract_transaction, private_key=deployer_key)
            # contract_tx_hash = self.W3.eth.send_raw_transaction(contract_signed_txn.rawTransaction)
            contract_signed_txn = self.W3.eth.account.sign_transaction(contract_transaction, private_key=deployer_key)
            raw = getattr(contract_signed_txn, "rawTransaction", None) or getattr(contract_signed_txn, "raw_transaction")
            contract_tx_hash = self.W3.eth.send_raw_transaction(raw)
            time.sleep(1)
            contract_receipt = self.W3.eth.wait_for_transaction_receipt(contract_tx_hash)

            debug(f"contract has been deployed to {contract_receipt.contractAddress}")
            return contract_receipt
        except Exception as ex:
            error(ex)
            return None
    #*******************************************************************************

