import os

import sys
import argparse
import pathlib
import json
import time
# from test.suites import run_test_suite
from pprint import pprint
from src.py.logger import setup_logger, info, section, debug, error
from src.py.helpers.yaml_helpers import load_configuration
from src.py.run.s0_prepare import s0_prepare
from src.py.run.s1_compile import s1_compile
from src.py.run.s2_deploy import s2_deploy
from src.py.run.s3_demo import s3_demo
from src.py.web3 import W3b3
from src.py.helpers.path_helpers import findFilesByExtention, RootPath, RecreatePath, Web3BuildPath
from src.py.helpers.json_helpers import read_json_file, save_receipts_to_json
from objsize import get_deep_size
############################################################################
def main(**args):

    root_path = RootPath()
    print(f"{root_path=}")

    configuration_path = os.path.join(root_path, args["conf_path"][1:-1])
    print(f"{configuration_path=}")
    conf = load_configuration(configuration_path)

    scenario_path = os.path.join(root_path, "conf","scenarios", conf["scenario"] + ".yaml")
    print(f"{scenario_path=}")

    scenario = load_configuration(scenario_path)
    # Get the total number of banks from qtyBanks argument
    qtyBanks = int(args['qtyBanks'])

    # Modify the banks_conf to limit it to qtyBanks
    banks_conf = scenario["banks"][:qtyBanks]  # Slice the list to get only qtyBanks entries
    print(f"Using {qtyBanks} banks from scenario: {banks_conf}")

    project_name = conf["id"]
    receipts_path = os.path.join(Web3BuildPath(root_path, project_name), "deploy_receipts.json")

    log_path = os.path.join(root_path, "log")
    RecreatePath(log_path)
    setup_logger(log_path, conf["id"])

    receipts = {}
    should_reset = False
    try:
        receipts = read_json_file(receipts_path)
        info(f"found the Receipt file at {receipts_path}.")
    except:
        info(f"Can not find the Receipt file at {receipts_path}. Redeploying")
        should_reset = True

    if "'reset'" in args['commands']:
        var = input("Are you sure you want to clean up and reset everything? (Y/n)")
        print("You entered: " + var)
        if var == "" or str(var) == "Y" or str(var) == "y":
            print("Cleaning up everything")
            should_reset = True
            RecreatePath(log_path)

    w3 = W3b3(root_path, conf, scenario, receipts)

    if "'pack'" in args['commands'] or should_reset:

        build_path = os.path.join(root_path, "build")
        # RecreatePath(log_path)

        s0_prepare(root_path, project_name, banks_conf)

        s1_compile(root_path, project_name, banks_conf)

    if "'deploy'" in args['commands'] or should_reset:
        receipts = s2_deploy(w3, root_path, conf, scenario, receipts)
        save_receipts_to_json(receipts_path, receipts)

    if "'demo'" in args['commands'] or should_reset:
        receipts = s3_demo(w3, root_path, conf, banks_conf, receipts)
        save_receipts_to_json(receipts_path, receipts)


############################################################################33
if __name__ == '__main__':
    defined_commands = ["clean", "pack", "deploy", "demo", "reset"]
    
    parser = argparse.ArgumentParser(description='Enygma System demo script')
    parser.add_argument('-f', dest='conf_path', type=ascii, default=os.path.join("conf", "00_default_configuration_ganache.yaml"),
                        help='configuration YAML file path')
    parser.add_argument('-s', dest='scenario_path', type=ascii, default=os.path.join("conf", "scenarios", "00_default_scenario.yaml"),
                        help='Scenario YAML file path')
    # parser.add_argument('-p', dest='proof_path', type=ascii, default="",
    #                     help='Proof YAML file path')
    parser.add_argument('-c', dest='commands', type=ascii, nargs='+',
                         default='reset',help='list of commands to execute.')
    parser.add_argument('--qtyBanks', dest='qtyBanks', type=int, required=True, help='Number of banks to include from the scenario file.')
    # parser.add_argument('-b', dest='bank_id', type=ascii, default='',
    #                     help='BankId')
    # parser.add_argument('-l', dest='ledger_address', type=ascii, default='',
    #                     help='LedgerAddress')
    # parser.add_argument('-t', dest='txn_id', type=ascii, default='',
    #                     help='txn address or path')
    # parser.add_argument('-y', dest='txn_type', type=ascii, default="",
    #                     help='Transaction Type: 0:Undefined 1:Issuance 2:Withdrawal 3:Transfer')

    args = parser.parse_args()
    main(**vars(args))
############################################################################