import json
from pprint import pprint
from web3 import Web3
import os
import pathlib
import shutil
from src.py.logger import info, debug, section
from src.py.helpers.path_helpers import TokenJsonPath, VerifierJsonPath, Web3BuildPath,WithdrawVerifierJsonPath,DepositVerifierJsonPath
from src.py.web3 import W3b3
from src.py.helpers.string_helpers import replace_in_file
import subprocess
#*******************************************************************************
def s2_deploy_enygma(w3, project_name):
    debug(f"Deploying Enygma ...")
    args = {}
    token_receipt = w3.deploy_enygma(TokenJsonPath(w3.root_path, project_name), **args)
    print(token_receipt)
    debug(f"enygma has been deployed to {token_receipt.contractAddress}")
    w3.set_token_address(token_receipt.contractAddress)
    return token_receipt
#*******************************************************************************
def s2_deploy_enygmaverifier(w3, project_name):
    debug(f"Deploying enygmaverifier ...")
    args = {}
    verifier_receipt = w3.deploy_enygma(VerifierJsonPath(w3.root_path, project_name), **args)
    print(verifier_receipt)
    debug(f"enygmaverifier has been deployed to {verifier_receipt.contractAddress}")
    w3.set_verifier_address(verifier_receipt.contractAddress)
    return verifier_receipt

# def s2_deploy_withdrawverifier(w3, project_name):
#     debug(f"Deploying withdrawverifier ...")
#     args = {}
#     verifier_receipt = w3.deploy_enygma(WithdrawVerifierJsonPath(w3.root_path, project_name), **args)
#     print(verifier_receipt)
#     debug(f"enygmaverifier has been deployed to {verifier_receipt.contractAddress}")
#     w3.set_withdraw_verifier_address(verifier_receipt.contractAddress)
#     return verifier_receipt

def s2_deploy_withdrawverifier(w3, project_name,k):
    debug(f"Deploying withdrawverifier {k} ...")
    args = {}
    verifier_receipt = w3.deploy_enygma(WithdrawVerifierJsonPath(w3.root_path, project_name,k), **args)
    print(verifier_receipt)
    debug(f"enygmaverifier has been deployed to {verifier_receipt.contractAddress}")
    w3.set_withdraw_verifier_address(verifier_receipt.contractAddress,k)
    return verifier_receipt

def s2_deploy_depositverifier(w3, project_name):
    debug(f"Deploying depositverifier ...")
    args = {}
    verifier_receipt = w3.deploy_enygma(DepositVerifierJsonPath(w3.root_path, project_name), **args)
    print(verifier_receipt)
    debug(f"enygmaverifier has been deployed to {verifier_receipt.contractAddress}")
    w3.set_deposit_verifier_address(verifier_receipt.contractAddress)
    return verifier_receipt
#*******************************************************************************
def s2_deploy(w3, root_path, conf, scenario, receipts):
    section("[[DEPLOY]]")

    project_name = conf["id"]

    receipts["TOKEN"] = s2_deploy_enygma(w3, project_name)
    receipts["VERIFIER"] = s2_deploy_enygmaverifier(w3, project_name)
    for i in range(7):
        if i ==0:
            continue
        else:
            receipts["WITHDRAWVERIFIER"] = s2_deploy_withdrawverifier(w3, project_name,i)
 
    receipts["DEPOSITVERIFIER"] = s2_deploy_depositverifier(w3, project_name)

    return receipts