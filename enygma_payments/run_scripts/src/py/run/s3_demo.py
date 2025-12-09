import json
import os
from pprint import pprint

from src.py.web3 import W3b3
from src.py.logger import info, error, debug, plot_gas_costs, section
from src.py.helpers.json_helpers import read_json_file, save_demo_data_to_json
from src.py.helpers.path_helpers import Web3BuildPath, BuildPath
import web3
import time
import subprocess

#*******************************************************************************
def s3_demo_initialization(w3, root_path, project_name, network, banks, receipts):
    info("Demo.Initialization")
    # ################################################################################

    token_address = w3.Token["address"]
    owner_key = w3.accounts[0]["private"]
    owner_address = w3.accounts[0]["address"]


    info("Registering banks to Enygma")

    for i in range(len(banks)):
        rValue = banks[i]["r"]
        pub = banks[i]["pub"]
        

        nonce = w3.W3.eth.get_transaction_count(owner_address)
        contract_transaction = w3.token_contract().functions.registerAccount(owner_address, i, pub, rValue).build_transaction(
                {"chainId": w3.chain_id, "from": owner_address, "nonce": nonce, 'gasPrice': 875000000})
        contract_signed_txn = w3.W3.eth.account.sign_transaction(contract_transaction, private_key=owner_key)
        contract_tx_hash = w3.W3.eth.send_raw_transaction(contract_signed_txn.rawTransaction)
        time.sleep(1)
        contract_receipt = w3.W3.eth.wait_for_transaction_receipt(contract_tx_hash)
        
        # pprint(contract_receipt)
        info(f"Private Bank {i} registered to Enygma")


    info("calling enygma.addVerifier")
    nonce = w3.W3.eth.get_transaction_count(owner_address)
    contract_transaction = w3.token_contract().functions.addVerifier(w3.Verifier["address"]).build_transaction(
                {"chainId": w3.chain_id, "from": owner_address, "nonce": nonce, 'gasPrice': 875000000})
    contract_signed_txn = w3.W3.eth.account.sign_transaction(contract_transaction, private_key=owner_key)
    contract_tx_hash = w3.W3.eth.send_raw_transaction(contract_signed_txn.rawTransaction)
    time.sleep(1)
    contract_receipt = w3.W3.eth.wait_for_transaction_receipt(contract_tx_hash)

    for i in range(7):
        if i ==0:
            continue
        else:
            info(f"calling enygma.addWithdrawVerifier {i}")
            nonce = w3.W3.eth.get_transaction_count(owner_address)
            contract_transaction = w3.token_contract().functions.addWithdrawVerifier(w3.WithdrawVerifier[f"address_{i}"],i).build_transaction(
                        {"chainId": w3.chain_id, "from": owner_address, "nonce": nonce, 'gasPrice': 875000000})
            contract_signed_txn = w3.W3.eth.account.sign_transaction(contract_transaction, private_key=owner_key)
            contract_tx_hash = w3.W3.eth.send_raw_transaction(contract_signed_txn.rawTransaction)
            time.sleep(1)
            contract_receipt = w3.W3.eth.wait_for_transaction_receipt(contract_tx_hash)

    info("calling enygma.addDepositVerifier")
    nonce = w3.W3.eth.get_transaction_count(owner_address)
    contract_transaction = w3.token_contract().functions.addDepositVerifier(w3.DepositVerifier["address"]).build_transaction(
                {"chainId": w3.chain_id, "from": owner_address, "nonce": nonce, 'gasPrice': 875000000})
    contract_signed_txn = w3.W3.eth.account.sign_transaction(contract_transaction, private_key=owner_key)
    contract_tx_hash = w3.W3.eth.send_raw_transaction(contract_signed_txn.rawTransaction)
    time.sleep(1)
    contract_receipt = w3.W3.eth.wait_for_transaction_receipt(contract_tx_hash)

    info("calling enygma.initialize")
    nonce = w3.W3.eth.get_transaction_count(owner_address)
    contract_transaction = w3.token_contract().functions.initialize().build_transaction(
                {"chainId": w3.chain_id, "from": owner_address, "nonce": nonce, 'gasPrice': 875000000 })
    print(f"owner {owner_key}")
    contract_signed_txn = w3.W3.eth.account.sign_transaction(contract_transaction, private_key=owner_key)
    contract_tx_hash = w3.W3.eth.send_raw_transaction(contract_signed_txn.rawTransaction)
    time.sleep(1)
    contract_receipt = w3.W3.eth.wait_for_transaction_receipt(contract_tx_hash)


    info("Printing out Enygma data")
    w3.print_token_data()
#*******************************************************************************
def s3_demo_transactions(w3, root_path, project_name, network, banks, receipts):
    info("Demo.transactions")
    # ################################################################################

    token_address = w3.Token["address"]
    owner_key = w3.accounts[0]["private"]
    owner_address = w3.accounts[0]["address"]


    info("enygma.check")
    nonce = w3.W3.eth.get_transaction_count(owner_address)
    contract_transaction = w3.token_contract().functions.check().build_transaction(
                {"chainId": w3.chain_id, "from": owner_address, "nonce": nonce, 'gasPrice': 875000000})
    contract_signed_txn = w3.W3.eth.account.sign_transaction(contract_transaction, private_key=owner_key)
    contract_tx_hash = w3.W3.eth.send_raw_transaction(contract_signed_txn.rawTransaction)
    time.sleep(1)
    contract_receipt = w3.W3.eth.wait_for_transaction_receipt(contract_tx_hash)
    info("Done enygma.check")
    

    mint_amount = 1000;
    receiver_id = 0;


    info(f"Minting {mint_amount} tokens for account {receiver_id}")
    nonce = w3.W3.eth.get_transaction_count(owner_address)
    contract_transaction = w3.token_contract().functions.mintSupply(mint_amount, receiver_id).build_transaction(
                {"chainId": w3.chain_id, "from": owner_address, "nonce": nonce, 'gasPrice': 875000000})
    contract_signed_txn = w3.W3.eth.account.sign_transaction(contract_transaction, private_key=owner_key)
    contract_tx_hash = w3.W3.eth.send_raw_transaction(contract_signed_txn.rawTransaction)
    time.sleep(1)
    contract_receipt = w3.W3.eth.wait_for_transaction_receipt(contract_tx_hash)
    info("Done minting.")
    info("")

    info("Enygma.check")
    nonce = w3.W3.eth.get_transaction_count(owner_address)
    contract_transaction = w3.token_contract().functions.check().build_transaction(
                {"chainId": w3.chain_id, "from": owner_address, "nonce": nonce, 'gasPrice': 875000000})
    contract_signed_txn = w3.W3.eth.account.sign_transaction(contract_transaction, private_key=owner_key)
    contract_tx_hash = w3.W3.eth.send_raw_transaction(contract_signed_txn.rawTransaction)
    time.sleep(1)
    contract_receipt = w3.W3.eth.wait_for_transaction_receipt(contract_tx_hash)
    info("Done Enygma.check")
    info("")

#*******************************************************************************
def s3_demo(w3, root_path, conf, banks_conf, receipts):

    section("[[DEMO]]")
    # Reading the last saved receipt
    project_name = conf["id"]

    s3_demo_initialization(w3, root_path, project_name, conf["network"], banks_conf, receipts)
    s3_demo_transactions(w3, root_path, project_name, conf["network"], banks_conf, receipts)

    return receipts
